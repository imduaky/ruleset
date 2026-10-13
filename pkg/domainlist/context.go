package domainlist

import (
	"slices"
	"sync"
)

const (
	// maxPooledFrames, maxPooledEntries and maxPooledMaps limit the buffers kept by a pooled mergeContext,
	// see mergeContext.reset.
	maxPooledFrames  = 1024
	maxPooledEntries = 4096
	maxPooledMaps    = 16
)

var contextPool = sync.Pool{New: func() any { return new(mergeContext) }}

// mergeContext merges the lists looked up by a Lookuper in one call of a Merger, which is pooled to reuse
// its buffers. The looked up and merged lists are cached in the context only, so that each list is looked
// up and merged once in a call, e.g. c is merged once for both `a: include:c` and `b: include:c`, while the
// changes of the Lookuper are seen by the next calls.
//
// Its stack is the path of inclusions from the merged list. A list is pushed when its inclusion is entered,
// and popped after all its included lists are merged into it, each with the attribute filter of its inclusion.
// The filters of the inclusions do not accumulate along the path, since an included list is merged before its
// parent, with the filters of its own inclusions applied already, so that it is reused by all its parents.
// The filter of the context applies to all lists instead, since it applies to all paths of the call.
// Merging a list after all its included lists also knows their sizes, so that its domains and seen map are
// allocated once.
type mergeContext struct {
	lookuper Lookuper
	filter   *attrFilter           // applied to all lists, e.g. the attributes of Merger.MergeAttrs
	lists    map[string]DomainList // looked up lists
	merged   map[string]DomainList // lists with the inclusions merged

	stack   []frame
	onStack map[string]int // index in the stack by name
	seens   []seenMap      // released maps, which are reused by the next lists
}

// frame is a list being merged, whose inclusions before next are merged.
type frame struct {
	name string
	list DomainList
	next int
}

type seenMap struct {
	m map[domainKey]struct{}
	n int // the number of the entries before being cleared
}

func getContext(lookuper Lookuper, filter *attrFilter) *mergeContext {
	c := contextPool.Get().(*mergeContext)
	c.lookuper, c.filter = lookuper, filter
	if c.lists == nil {
		c.lists, c.merged = make(map[string]DomainList), make(map[string]DomainList)
	}
	return c
}

func putContext(c *mergeContext) {
	c.reset()
	contextPool.Put(c)
}

// lookup looks up the list of the name once in the context.
func (c *mergeContext) lookup(name string) (DomainList, error) {
	if list, ok := c.lists[name]; ok {
		return list, nil
	}
	list, err := c.lookuper.Lookup(name)
	if err != nil {
		return DomainList{}, err
	}
	c.lists[name] = list
	return list, nil
}

// merge merges the list of the name depth first by the stack instead of recursion.
// A list included again by the lists on the stack is circular.
func (c *mergeContext) merge(name string) (DomainList, error) {
	if list, ok := c.merged[name]; ok {
		return list, nil
	}
	list, err := c.lookup(name)
	if err != nil {
		return DomainList{}, &MergeError{Err: err, Path: []string{name}, Cycle: -1}
	}

	c.pushStack(name, list)
	for {
		top := &c.stack[len(c.stack)-1]
		if top.next == len(top.list.Inclusions) {
			// All included lists are merged.
			merged := c.mergeIncluded(top.list)
			c.merged[top.name] = merged
			c.popStack()
			if len(c.stack) == 0 {
				return merged, nil
			}
			continue
		}

		included := top.list.Inclusions[top.next].Name
		top.next++
		if _, ok := c.merged[included]; ok {
			continue
		}
		if i, ok := c.onStack[included]; ok {
			return DomainList{}, c.fail(ErrCircularInclusion, included, i)
		}
		list, err := c.lookup(included)
		if err != nil {
			return DomainList{}, c.fail(err, included, -1)
		}
		c.pushStack(included, list)
	}
}

func (c *mergeContext) mergeOnce(name string) (DomainList, error) {
	list, err := c.lookup(name)
	if err != nil {
		return DomainList{}, &MergeError{Err: err, Path: []string{name}, Cycle: -1}
	}
	if len(list.Inclusions) == 0 {
		return list, nil
	}
	n := len(list.Domains)
	for _, inc := range list.Inclusions {
		included, err := c.lookup(inc.Name)
		if err != nil {
			return DomainList{}, &MergeError{Err: err, Path: []string{name, inc.Name}, Cycle: -1}
		}
		n += len(included.Domains)
	}

	seen := c.newSeen(n)
	defer c.releaseSeen(seen)
	for _, d := range list.Domains {
		seen[d.key()] = struct{}{}
	}
	domains := append(make([]Domain, 0, n), list.Domains...)
	var incs []Inclusion
	seenIncs := make(map[inclusionKey]bool)
	for _, inc := range list.Inclusions {
		included := c.lists[inc.Name]
		filter := newAttrFilter(inc.MustAttrs, inc.BanAttrs)
		domains = appendMerged(domains, seen, included.Domains, &filter)
		for _, ii := range included.Inclusions {
			ii = Inclusion{
				Name:      ii.Name,
				MustAttrs: mergeAttrs(inc.MustAttrs, ii.MustAttrs),
				BanAttrs:  mergeAttrs(inc.BanAttrs, ii.BanAttrs),
			}

			key := ii.key()
			if !seenIncs[key] {
				seenIncs[key] = true
				incs = append(incs, ii)
			}
		}
	}
	if len(domains) == 0 {
		domains = nil
	}
	return DomainList{Inclusions: incs, Domains: domains}, nil
}

// mergeAll looks up all lists before merging them, so that each of them is looked up once,
// then merges them in the order of names, see Merger.MergeAll.
func (c *mergeContext) mergeAll() (*DomainListSet, error) {
	nameList := make([]string, 0)
	for list, lookupErr := range c.lookuper.All() {
		if lookupErr != nil {
			return nil, lookupErr
		}

		// Kept for merging, otherwise each list is looked up again, e.g. a file is read twice.
		if _, ok := c.lists[list.Name]; !ok {
			c.lists[list.Name] = list.DomainList
		}
		nameList = append(nameList, list.Name)
	}

	slices.Sort(nameList)
	nameList = slices.Compact(nameList)

	set := &DomainListSet{}
	set.growInit(len(nameList))
	for _, name := range nameList {
		mergedList, err := c.merge(name)
		if err != nil {
			return nil, err
		}

		// The list is copied already, which Add would copy again.
		set.put(NamedDomainList{Name: name, DomainList: mergedList.Copy()})
	}

	return set, nil
}

func (c *mergeContext) pushStack(name string, list DomainList) {
	if c.onStack == nil {
		c.onStack = make(map[string]int)
	}
	c.onStack[name] = len(c.stack)
	c.stack = append(c.stack, frame{name: name, list: list})
}

func (c *mergeContext) popStack() {
	delete(c.onStack, c.stack[len(c.stack)-1].name)
	c.stack[len(c.stack)-1] = frame{} // the list is not referenced by the buffer
	c.stack = c.stack[:len(c.stack)-1]
}

// mergeIncluded merges the merged lists into the list, each with the attribute filter of its inclusion.
// The domains of the list are kept as is, even the duplicates, unless they are not matched by the filter
// of the context. The merged lists are matched by the filter of the context already.
func (c *mergeContext) mergeIncluded(list DomainList) DomainList {
	if len(list.Inclusions) == 0 && c.filter.none() {
		return list
	}
	n := len(list.Domains)
	for _, inc := range list.Inclusions {
		n += len(c.merged[inc.Name].Domains)
	}
	seen := c.newSeen(n)
	defer c.releaseSeen(seen)

	domains := make([]Domain, 0, n)
	for _, d := range list.Domains {
		if c.filter.match(d.Attrs) {
			seen[d.key()] = struct{}{}
			domains = append(domains, d)
		}
	}

	for _, inc := range list.Inclusions {
		filter := newAttrFilter(inc.MustAttrs, inc.BanAttrs)
		domains = appendMerged(domains, seen, c.merged[inc.Name].Domains, &filter)
	}

	if len(domains) == 0 {
		// set to nil so that the GC can work.
		domains = nil
	}

	return DomainList{Domains: domains}
}

// fail returns the error of including the list of the name by the stack,
// cycle is the index in the stack where the circle starts, or -1.
func (c *mergeContext) fail(err error, name string, cycle int) *MergeError {
	path := make([]string, len(c.stack)+1)
	for i, f := range c.stack {
		path[i] = f.name
	}
	path[len(c.stack)] = name
	return &MergeError{Err: err, Path: path, Cycle: cycle}
}

// newSeen returns a released map, unless the hint is so large that growing the map costs more than a new one.
func (c *mergeContext) newSeen(hint int) map[domainKey]struct{} {
	if n := len(c.seens); n > 0 && (hint <= maxPooledEntries || hint <= c.seens[n-1].n) {
		m := c.seens[n-1].m
		c.seens[n-1] = seenMap{}
		c.seens = c.seens[:n-1]
		return m
	}
	return make(map[domainKey]struct{}, hint)
}

func (c *mergeContext) releaseSeen(m map[domainKey]struct{}) {
	n := len(m)
	clear(m)
	c.seens = append(c.seens, seenMap{m: m, n: n})
}

// reset prepares the context to be put into the pool.
//
// A pooled context lives until the pool is cleaned by GC, so it must not reference the Lookuper and the lists,
// otherwise they could not be collected after the call, so the frames and the maps of the lists are cleared.
// It must not keep large buffers either: clearing a map removes its entries but keeps its buckets, so a map
// grown by a large list, e.g. the 30k domains of geolocation-!cn, or a stack grown by deep inclusions, would be
// held for all the following small calls. So the large buffers are set to nil to be collected by GC, while the
// smaller ones, which are the most, are kept to save the allocations of the next calls.
func (c *mergeContext) reset() {
	c.lookuper, c.filter = nil, nil
	if len(c.lists) > maxPooledEntries {
		c.lists, c.merged = nil, nil
	} else {
		clear(c.lists)
		clear(c.merged)
	}

	clear(c.stack[:cap(c.stack)])
	c.stack = c.stack[:0]
	if cap(c.stack) > maxPooledFrames {
		// onStack has been as large as the stack, though it is empty now.
		c.stack, c.onStack = nil, nil
	} else {
		// Not empty if the merge failed.
		clear(c.onStack)
	}

	kept := c.seens[:0]
	for _, s := range c.seens {
		if s.n <= maxPooledEntries && len(kept) < maxPooledMaps {
			kept = append(kept, s)
		}
	}
	clear(c.seens[len(kept):])
	c.seens = kept
}

// appendMerged appends the domains matched by the filter to dst unless they are in seen, which are added
// to seen. The affiliations of them are dropped, they belong to the included list.
func appendMerged(dst []Domain, seen map[domainKey]struct{}, domains []Domain, filter *attrFilter) []Domain {
	for _, d := range domains {
		if !filter.match(d.Attrs) {
			continue
		}
		key := d.key()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		d.Affiliations = nil
		dst = append(dst, d)
	}
	return dst
}

func mergeAttrs(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	if len(a) == 0 {
		return b
	}
	s := slices.Concat(a, b)
	slices.Sort(s)
	return slices.Compact(s)
}

package domainlist

import "sync"

const (
	// maxPooledFrames, maxPooledEntries and maxPooledMaps limit the buffers kept by a pooled mergeContext,
	// see mergeContext.reset.
	maxPooledFrames  = 1024
	maxPooledEntries = 4096
	maxPooledMaps    = 16
)

var contextPool = sync.Pool{New: func() any { return new(mergeContext) }}

// mergeContext is the state of merging a list depth first, which is pooled to reuse its buffers.
//
// Its stack is the path of inclusions from the merged list. A list is pushed when its inclusion is entered,
// and popped after all its included lists are merged into it, each with the attribute filter of its inclusion.
// The filters do not accumulate along the path, since an included list is merged before its parent, with the
// filters of its own inclusions applied already, so that the merged list is cached and reused, e.g. c is
// merged once for both `a: include:c` and `b: include:c @cn`. Merging a list after all its included lists also
// knows their sizes, so that its domains and seen map are allocated once.
type mergeContext struct {
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

func getContext() *mergeContext {
	return contextPool.Get().(*mergeContext)
}

func putContext(c *mergeContext) {
	c.reset()
	contextPool.Put(c)
}

func (c *mergeContext) push(name string, list DomainList) {
	if c.onStack == nil {
		c.onStack = make(map[string]int)
	}
	c.onStack[name] = len(c.stack)
	c.stack = append(c.stack, frame{name: name, list: list})
}

func (c *mergeContext) pop() {
	delete(c.onStack, c.stack[len(c.stack)-1].name)
	c.stack[len(c.stack)-1] = frame{} // the list is not referenced by the buffer
	c.stack = c.stack[:len(c.stack)-1]
}

// mergeIncluded merges the merged lists into the list, each with the attribute filter of its inclusion.
// The domains of the list are kept as is, even the duplicates.
func (c *mergeContext) mergeIncluded(list DomainList, merged map[string]DomainList) DomainList {
	if len(list.Inclusions) == 0 {
		return list
	}
	n := len(list.Domains)
	for _, inc := range list.Inclusions {
		n += len(merged[inc.Name].Domains)
	}
	seen := c.newSeen(n)
	defer c.release(seen)
	for _, d := range list.Domains {
		seen[keyOf(d)] = struct{}{}
	}
	domains := append(make([]Domain, 0, n), list.Domains...)
	for _, inc := range list.Inclusions {
		filter := newAttrFilter(inc.MustAttrs, inc.BanAttrs)
		domains = appendMerged(domains, seen, merged[inc.Name].Domains, &filter)
	}
	if len(domains) == 0 {
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

func (c *mergeContext) release(m map[domainKey]struct{}) {
	n := len(m)
	clear(m)
	c.seens = append(c.seens, seenMap{m: m, n: n})
}

// reset prepares the context to be put into the pool.
//
// A pooled context lives until the pool is cleaned by GC, so it must not reference the lists, otherwise
// they could not be collected after being merged, and the frames are zeroed. It must not keep large buffers
// either: clearing a map removes its entries but keeps its buckets, so a map grown by a large list, e.g. the
// 30k domains of geolocation-!cn, or a stack grown by deep inclusions, would be held for all the following
// small merges. So the large buffers are set to nil to be collected by GC, while the smaller ones, which are
// the most, are kept to save the allocations of the next merges.
func (c *mergeContext) reset() {
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
		key := keyOf(d)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		d.Affiliations = nil
		dst = append(dst, d)
	}
	return dst
}

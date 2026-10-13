package domainlist

import (
	"slices"
	"strings"
)

// Merger merges the inclusions of the lists looked up by a Lookuper, the same as the official resolves them.
// The lists are looked up lazily by a CachedLookuper, and the merged lists are cached and reused, e.g. c is
// merged once for both `a: include:c` and `b: include:c`. The looked up lists are not modified.
//
// The domains of a list are kept as is, the domains of its included lists are appended unless
// the same type, value and attributes exist already. Affiliations of the included domains are dropped,
// they belong to the included lists. The returned lists share Attrs with the looked up lists.
//
// A Merger is not safe for concurrent use, see LockedLookuper to share a Lookuper.
type Merger struct {
	lookuper *CachedLookuper
	merged   map[string]DomainList // lists with the inclusions merged
}

func NewMerger(lookuper Lookuper) *Merger {
	cached, ok := lookuper.(*CachedLookuper)
	if !ok {
		cached = NewCachedLookuper(lookuper)
	}
	return &Merger{lookuper: cached, merged: make(map[string]DomainList)}
}

// Merge returns the list of the name with its inclusions merged recursively.
// It fails on a missing or circular inclusion, see MergeError.
func (m *Merger) Merge(name string) (DomainList, error) {
	list, err := m.merge(name)
	if err != nil {
		return DomainList{}, err
	}
	return list.Copy(), nil
}

// MergeAttrs returns the domains of the list of the name with its inclusions merged recursively,
// which have the attributes, e.g. MergeAttrs("list", "ads", "-cn") is the same as `include:list @ads @-cn`.
// The attributes prefixed with SymbolBan are ban attributes, which the domains do not have.
func (m *Merger) MergeAttrs(name string, attrs ...string) (DomainList, error) {
	list, err := m.merge(name)
	if err != nil {
		return DomainList{}, err
	}
	var must, ban []string
	for _, attr := range attrs {
		if b, ok := strings.CutPrefix(attr, SymbolBan); ok {
			ban = append(ban, b)
		} else {
			must = append(must, attr)
		}
	}
	return DomainList{Domains: list.Filter(must, ban)}, nil
}

// MergeOnce returns the list of the name with its inclusions merged once, which might have inclusions still.
// The inclusions are replaced by the domains of the included lists, whose inclusions become the inclusions
// of the list with the attributes of both, e.g. `include:b @ads` and `b: include:c @-cn` become
// `include:c @ads @-cn`. It fails on a missing inclusion.
func (m *Merger) MergeOnce(name string) (DomainList, error) {
	list, err := m.lookuper.Lookup(name)
	if err != nil {
		return DomainList{}, &MergeError{Err: err, Path: []string{name}, Cycle: -1}
	}
	included := make(map[string]DomainList, len(list.Inclusions))
	for _, inc := range list.Inclusions {
		included[inc.Name], err = m.lookuper.Lookup(inc.Name)
		if err != nil {
			return DomainList{}, &MergeError{Err: err, Path: []string{name, inc.Name}, Cycle: -1}
		}
	}
	return mergeOnce(list, included).Copy(), nil
}

// MergeAll returns all lists with their inclusions merged recursively, see Merge. All lists are looked up
// before being merged, so that each of them is looked up once, then they are merged in the order of names.
func (m *Merger) MergeAll() (*DomainListSet, error) {
	var names []string
	for list, err := range m.lookuper.All() {
		if err != nil {
			return nil, err
		}
		names = append(names, list.Name)
	}
	slices.Sort(names)
	names = slices.Compact(names)

	set := &DomainListSet{index: make(map[string]int, len(names)), lists: make([]NamedDomainList, 0, len(names))}
	for _, name := range names {
		list, err := m.merge(name)
		if err != nil {
			return nil, err
		}
		// The list is copied already, which Add would copy again.
		set.put(NamedDomainList{Name: name, DomainList: list.Copy()})
	}
	return set, nil
}

// merge merges the list of the name depth first by the stack of a mergeContext instead of recursion,
// see mergeContext. A list included again by the lists on the stack is circular.
func (m *Merger) merge(name string) (DomainList, error) {
	if list, ok := m.merged[name]; ok {
		return list, nil
	}
	list, err := m.lookuper.Lookup(name)
	if err != nil {
		return DomainList{}, &MergeError{Err: err, Path: []string{name}, Cycle: -1}
	}

	c := getContext()
	defer putContext(c)
	c.push(name, list)
	for {
		top := &c.stack[len(c.stack)-1]
		if top.next == len(top.list.Inclusions) {
			// All included lists are merged.
			merged := c.mergeIncluded(top.list, m.merged)
			m.merged[top.name] = merged
			c.pop()
			if len(c.stack) == 0 {
				return merged, nil
			}
			continue
		}

		included := top.list.Inclusions[top.next].Name
		top.next++
		if _, ok := m.merged[included]; ok {
			continue
		}
		if i, ok := c.onStack[included]; ok {
			return DomainList{}, c.fail(ErrCircularInclusion, included, i)
		}
		list, err := m.lookuper.Lookup(included)
		if err != nil {
			return DomainList{}, c.fail(err, included, -1)
		}
		c.push(included, list)
	}
}

// mergeOnce merges the included lists into the list. The inclusions of the included lists are kept,
// with the attributes of both inclusions.
func mergeOnce(list DomainList, included map[string]DomainList) DomainList {
	if len(list.Inclusions) == 0 {
		return list
	}
	n := len(list.Domains)
	for _, inc := range list.Inclusions {
		n += len(included[inc.Name].Domains)
	}

	seen := make(map[domainKey]struct{}, n)
	for _, d := range list.Domains {
		seen[keyOf(d)] = struct{}{}
	}
	domains := append(make([]Domain, 0, n), list.Domains...)
	var incs []Inclusion
	seenIncs := make(map[inclusionKey]struct{})
	for _, inc := range list.Inclusions {
		filter := newAttrFilter(inc.MustAttrs, inc.BanAttrs)
		domains = appendMerged(domains, seen, included[inc.Name].Domains, &filter)
		for _, ii := range included[inc.Name].Inclusions {
			ii = Inclusion{Name: ii.Name, MustAttrs: union(inc.MustAttrs, ii.MustAttrs), BanAttrs: union(inc.BanAttrs, ii.BanAttrs)}
			key := inclusionKey{name: ii.Name, must: strings.Join(ii.MustAttrs, " "), ban: strings.Join(ii.BanAttrs, " ")}
			if _, ok := seenIncs[key]; !ok {
				seenIncs[key] = struct{}{}
				incs = append(incs, ii)
			}
		}
	}
	if len(domains) == 0 {
		domains = nil
	}
	return DomainList{Inclusions: incs, Domains: domains}
}

func union(a, b []string) []string {
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

// domainKey identifies a domain without affiliations, the same as the official.
type domainKey struct {
	typ   RuleType
	value string
	attrs string
}

func keyOf(d Domain) domainKey {
	// Attrs never contain spaces, and Join does not allocate for less than 2 attrs, which are the most.
	return domainKey{typ: d.Type, value: d.Value, attrs: strings.Join(d.Attrs, " ")}
}

type inclusionKey struct {
	name, must, ban string
}

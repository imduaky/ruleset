package domainlist

import "strings"

// Merger merges the inclusions of the lists looked up by a Lookuper, the same as the official resolves them.
//
// A Merger keeps no state, each call looks up and merges the lists again, so that the changes of the Lookuper
// are seen. In a call, each list is looked up and merged once, e.g. c is merged once for both `a: include:c`
// and `b: include:c`. Wrap the Lookuper by CachedLookuper to cache the looked up lists, or add the merged lists
// to a DomainListSet to keep them. A Merger is safe for concurrent use if its Lookuper is, see LockedLookuper.
//
// The domains of a list are kept as is, the domains of its included lists are appended unless
// the same type, value and attributes exist already. Affiliations of the included domains are dropped,
// they belong to the included lists. The looked up lists are not modified.
type Merger struct {
	lookuper Lookuper
}

func NewMerger(lookuper Lookuper) *Merger {
	return &Merger{lookuper: lookuper}
}

// Merge returns the list of the name with its inclusions merged recursively.
// It fails on a missing or circular inclusion, see MergeError.
func (m *Merger) Merge(name string) (DomainList, error) {
	c := getContext(m.lookuper, nil)
	defer putContext(c)
	list, err := c.merge(name)
	if err != nil {
		return DomainList{}, err
	}
	return list.Copy(), nil
}

// MergeAttrs returns the domains of the list of the name with its inclusions merged recursively,
// which have the attributes, e.g. MergeAttrs("list", "ads", "-cn") is the same as `include:list @ads @-cn`.
// The attributes prefixed with SymbolBan are ban attributes, which the domains do not have.
// The domains are filtered while being merged, instead of after all lists are merged.
func (m *Merger) MergeAttrs(name string, attrs ...string) (DomainList, error) {
	var must, ban []string
	for _, attr := range attrs {
		if b, ok := strings.CutPrefix(attr, SymbolBan); ok {
			ban = append(ban, b)
		} else {
			must = append(must, attr)
		}
	}
	filter := newAttrFilter(must, ban)
	c := getContext(m.lookuper, &filter)
	defer putContext(c)
	list, err := c.merge(name)
	if err != nil {
		return DomainList{}, err
	}
	return list.Copy(), nil
}

// MergeOnce returns the list of the name with its inclusions merged once, which might have inclusions still.
// The inclusions are replaced by the domains of the included lists, whose inclusions become the inclusions
// of the list with the attributes of both, e.g. `include:b @ads` and `b: include:c @-cn` become
// `include:c @ads @-cn`. It fails on a missing inclusion.
func (m *Merger) MergeOnce(name string) (DomainList, error) {
	c := getContext(m.lookuper, nil)
	defer putContext(c)
	list, err := c.mergeOnce(name)
	if err != nil {
		return DomainList{}, err
	}
	return list.Copy(), nil
}

// MergeAll returns all lists with their inclusions merged recursively in the order of names, see Merge.
// All lists are looked up before being merged, so that each of them is looked up once.
func (m *Merger) MergeAll() (*DomainListSet, error) {
	c := getContext(m.lookuper, nil)
	defer putContext(c)
	return c.mergeAll()
}

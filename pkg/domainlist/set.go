package domainlist

import (
	"fmt"
	"iter"
	"maps"
	"slices"
)

// NamedDomainList is a list with its name, e.g. a list file under the data directory of
// v2fly/domain-list-community, whose name is the file name.
type NamedDomainList struct {
	DomainList

	Name string
}

// DomainListSet is a set of lists by name, e.g. the data directory of v2fly/domain-list-community.
// The lists are kept in the order of being added.
type DomainListSet struct {
	index map[string]int // index of the list in lists by name
	lists []NamedDomainList
}

func NewDomainListSet(lists ...NamedDomainList) *DomainListSet {
	set := &DomainListSet{index: make(map[string]int, len(lists)), lists: make([]NamedDomainList, 0, len(lists))}
	set.Add(lists...)
	return set
}

// Add adds the lists, whose rules are appended to the list of the same name if any.
// The rules are copied, so that modifying the added lists does not affect the set.
func (s *DomainListSet) Add(lists ...NamedDomainList) {
	if s.index == nil {
		s.index = make(map[string]int, len(lists))
	}
	s.lists = slices.Grow(s.lists, len(lists))
	for _, list := range lists {
		i, ok := s.index[list.Name]
		if !ok {
			i = len(s.lists)
			s.index[list.Name] = i
			s.lists = append(s.lists, NamedDomainList{Name: list.Name})
		}
		old := &s.lists[i]
		old.Inclusions = slices.Concat(old.Inclusions, list.Inclusions)
		old.Domains = slices.Concat(old.Domains, list.Domains)
	}
}

// put adds the list without copying it, whose name must not be added.
func (s *DomainListSet) put(list NamedDomainList) {
	if s.index == nil {
		s.index = make(map[string]int)
	}
	s.index[list.Name] = len(s.lists)
	s.lists = append(s.lists, list)
}

func (s *DomainListSet) Len() int {
	return len(s.lists)
}

func (s *DomainListSet) Lookup(name string) (DomainList, error) {
	i, ok := s.index[name]
	if !ok {
		return DomainList{}, fmt.Errorf("%w: %q", ErrListNotFound, name)
	}
	return s.lists[i].DomainList, nil
}

// All yields the lists in the order of being added, the error is always nil.
func (s *DomainListSet) All() iter.Seq2[NamedDomainList, error] {
	return func(yield func(NamedDomainList, error) bool) {
		for _, list := range s.lists {
			if !yield(list, nil) {
				return
			}
		}
	}
}

// Copy returns a deep copy of the set, so that modifying either does not affect the other.
func (s *DomainListSet) Copy() *DomainListSet {
	if s == nil {
		return nil
	}
	set := &DomainListSet{index: maps.Clone(s.index), lists: make([]NamedDomainList, len(s.lists))}
	for i, list := range s.lists {
		set.lists[i] = NamedDomainList{Name: list.Name, DomainList: list.Copy()}
	}
	return set
}

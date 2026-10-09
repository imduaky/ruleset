// Copied from sing, Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>, GPL-3.0-or-later.
// https://github.com/SagerNet/sing/blob/6f21f2425a959912c37d2ef43d61e2a663315dea/common/x/linkedhashmap/map.go

package singjson

// Merged from collections.Map, which is renamed to mapCollection to not conflict with Map.
// https://github.com/SagerNet/sing/blob/6f21f2425a959912c37d2ef43d61e2a663315dea/common/x/collections/map.go
type mapCollection[K comparable, V any] interface {
	Size() int
	IsEmpty() bool
	ContainsKey(key K) bool
	Get(key K) (V, bool)
	Put(key K, value V) V
	Remove(key K) bool
	PutAll(other mapCollection[K, V])
	Clear()
	Keys() []K
	Values() []V
	Entries() []MapEntry[K, V]
}

// MapEntry is the entry returned by TypedMap.Entries.
type MapEntry[K comparable, V any] struct {
	Key   K
	Value V
}

var _ mapCollection[string, any] = (*linkedHashMap[string, any])(nil)

type linkedHashMap[K comparable, V any] struct {
	raw    linkedList[MapEntry[K, V]]
	rawMap map[K]*linkedListElement[MapEntry[K, V]]
}

func (m *linkedHashMap[K, V]) init() {
	if m.rawMap == nil {
		m.rawMap = make(map[K]*linkedListElement[MapEntry[K, V]])
	}
}

func (m *linkedHashMap[K, V]) Size() int {
	return m.raw.Size()
}

func (m *linkedHashMap[K, V]) IsEmpty() bool {
	return m.raw.IsEmpty()
}

func (m *linkedHashMap[K, V]) ContainsKey(key K) bool {
	m.init()
	_, loaded := m.rawMap[key]
	return loaded
}

func (m *linkedHashMap[K, V]) Get(key K) (V, bool) {
	m.init()
	value, loaded := m.rawMap[key]
	if loaded {
		return value.Value.Value, true
	} else {
		var zero V
		return zero, false
	}
}

func (m *linkedHashMap[K, V]) Put(key K, value V) V {
	m.init()
	entry, loaded := m.rawMap[key]
	if loaded {
		oldValue := entry.Value.Value
		entry.Value.Value = value
		return oldValue
	}
	entry = m.raw.PushBack(MapEntry[K, V]{Key: key, Value: value})
	m.rawMap[key] = entry
	var zero V
	return zero
}

func (m *linkedHashMap[K, V]) Remove(key K) bool {
	m.init()
	entry, loaded := m.rawMap[key]
	if !loaded {
		return false
	}
	m.raw.Remove(entry)
	delete(m.rawMap, key)
	return true
}

func (m *linkedHashMap[K, V]) PutAll(other mapCollection[K, V]) {
	m.init()
	for _, item := range other.Entries() {
		entry, loaded := m.rawMap[item.Key]
		if loaded {
			entry.Value.Value = item.Value
			continue
		}
		entry = m.raw.PushBack(item)
		m.rawMap[item.Key] = entry
	}
}

func (m *linkedHashMap[K, V]) Clear() {
	*m = linkedHashMap[K, V]{}
}

func (m *linkedHashMap[K, V]) Keys() []K {
	result := make([]K, 0, m.raw.Len())
	for item := m.raw.Front(); item != nil; item = item.Next() {
		result = append(result, item.Value.Key)
	}
	return result
}

func (m *linkedHashMap[K, V]) Values() []V {
	result := make([]V, 0, m.raw.Len())
	for item := m.raw.Front(); item != nil; item = item.Next() {
		result = append(result, item.Value.Value)
	}
	return result
}

func (m *linkedHashMap[K, V]) Entries() []MapEntry[K, V] {
	result := make([]MapEntry[K, V], 0, m.raw.Len())
	for item := m.raw.Front(); item != nil; item = item.Next() {
		result = append(result, item.Value)
	}
	return result
}

package common

func ReverseMap[T comparable, V comparable](in map[T]V) map[V]T {
	var draft map[V]T
	for k, v := range in {
		draft[v] = k
	}
	return draft
}

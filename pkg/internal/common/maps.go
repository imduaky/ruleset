package common

func ReverseMap[T comparable, V comparable](in map[T]V) map[V]T {
	draft := make(map[V]T, len(in))
	for k, v := range in {
		draft[v] = k
	}
	return draft
}

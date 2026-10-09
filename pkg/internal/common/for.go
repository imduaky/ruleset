package common

import "slices"

func Or[E any, S ~[]E](s S, f func(E) bool) bool {
	return slices.ContainsFunc(s, f)
}

func All[E any, S ~[]E](s S, f func(E) bool) bool {
	return !slices.ContainsFunc(s, func(e E) bool { return !f(e) })
}

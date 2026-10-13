package common

import "slices"

func Or[E any, S ~[]E](s S, f func(E) bool) bool {
	return slices.ContainsFunc(s, f)
}

func All[E any, S ~[]E](s S, f func(E) bool) bool {
	return !slices.ContainsFunc(s, func(e E) bool { return !f(e) })
}

func Reduce[E any, S ~[]E](fn func(E, E) E, s S) E {
	var prev E
	for i, e := range s {
		if i == 0 {
			prev = e
			continue
		}
		prev = fn(prev, e)
	}
	return prev
}

func Reduce0[A, E any, S ~[]E](fn func(A, E) A, init A, s S) A {
	acc := init
	for _, e := range s {
		acc = fn(acc, e)
	}
	return acc
}

func Unique[T comparable, S []T](s S) S {
	if len(s) == 0 {
		return slices.Clone(s)
	}

	seen := make(map[T]bool)
	ans := make(S, 0, len(s))
	for _, a := range s {
		if seen[a] {
			continue
		}
		seen[a] = true
		ans = append(ans, a)
	}
	return ans
}

func UniqueCount[T comparable, S []T](s S) int {
	if len(s) == 0 {
		return 0
	}
	seen := make(map[T]bool)
	for _, a := range s {
		if seen[a] {
			continue
		}
		seen[a] = true
	}
	return len(seen)
}

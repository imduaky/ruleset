// Copied from sing, Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>, GPL-3.0-or-later.
// https://github.com/SagerNet/sing/blob/6f21f2425a959912c37d2ef43d61e2a663315dea/common/x/list/cond.go

package singjson

func (l linkedList[T]) Size() int {
	return l.len
}

func (l linkedList[T]) IsEmpty() bool {
	return l.len == 0
}

func (l *linkedList[T]) PopBack() T {
	if l.len == 0 {
		var zero T
		return zero
	}
	entry := l.root.prev
	l.remove(entry)
	return entry.Value
}

func (l *linkedList[T]) PopFront() T {
	if l.len == 0 {
		var zero T
		return zero
	}
	entry := l.root.next
	l.remove(entry)
	return entry.Value
}

func (l *linkedList[T]) Array() []T {
	if l.len == 0 {
		return nil
	}
	array := make([]T, 0, l.len)
	for element := l.Front(); element != nil; element = element.Next() {
		array = append(array, element.Value)
	}
	return array
}

func (e *linkedListElement[T]) List() *linkedList[T] {
	return e.list
}

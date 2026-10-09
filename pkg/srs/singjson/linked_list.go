// Copied from sing, which is copied from Go container/list with generics.
// https://github.com/SagerNet/sing/blob/6f21f2425a959912c37d2ef43d61e2a663315dea/common/x/list/list.go

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package singjson

// linkedListElement is an element of a linked list.
type linkedListElement[T any] struct {
	// Next and previous pointers in the doubly-linked list of elements.
	// To simplify the implementation, internally a list l is implemented
	// as a ring, such that &l.root is both the next element of the last
	// list element (l.Back()) and the previous element of the first list
	// element (l.Front()).
	next, prev *linkedListElement[T]

	// The list to which this element belongs.
	list *linkedList[T]

	// The value stored with this element.
	Value T
}

// Next returns the next list element or nil.
func (e *linkedListElement[T]) Next() *linkedListElement[T] {
	if p := e.next; e.list != nil && p != &e.list.root {
		return p
	}
	return nil
}

// Prev returns the previous list element or nil.
func (e *linkedListElement[T]) Prev() *linkedListElement[T] {
	if p := e.prev; e.list != nil && p != &e.list.root {
		return p
	}
	return nil
}

// linkedList represents a doubly linked list.
// The zero value for List is an empty list ready to use.
type linkedList[T any] struct {
	root linkedListElement[T] // sentinel list element, only &root, root.prev, and root.next are used
	len  int                  // current list length excluding (this) sentinel element
}

// Init initializes or clears list l.
func (l *linkedList[T]) Init() *linkedList[T] {
	l.root.next = &l.root
	l.root.prev = &l.root
	l.len = 0
	return l
}

// Len returns the number of elements of list l.
// The complexity is O(1).
func (l *linkedList[T]) Len() int { return l.len }

// Front returns the first element of list l or nil if the list is empty.
func (l *linkedList[T]) Front() *linkedListElement[T] {
	if l.len == 0 {
		return nil
	}
	return l.root.next
}

// Back returns the last element of list l or nil if the list is empty.
func (l *linkedList[T]) Back() *linkedListElement[T] {
	if l.len == 0 {
		return nil
	}
	return l.root.prev
}

// lazyInit lazily initializes a zero List value.
func (l *linkedList[T]) lazyInit() {
	if l.root.next == nil {
		l.Init()
	}
}

// insert inserts e after at, increments l.len, and returns e.
func (l *linkedList[T]) insert(e, at *linkedListElement[T]) *linkedListElement[T] {
	e.prev = at
	e.next = at.next
	e.prev.next = e
	e.next.prev = e
	e.list = l
	l.len++
	return e
}

// insertValue is a convenience wrapper for insert(&Element{Value: v}, at).
func (l *linkedList[T]) insertValue(v any, at *linkedListElement[T]) *linkedListElement[T] {
	e := new(linkedListElement[T])
	e.Value = v.(T)
	return l.insert(e, at)
}

// remove removes e from its list, decrements l.len
func (l *linkedList[T]) remove(e *linkedListElement[T]) {
	e.prev.next = e.next
	e.next.prev = e.prev
	e.next = nil // avoid memory leaks
	e.prev = nil // avoid memory leaks
	e.list = nil
	l.len--
}

// move moves e to next to at.
func (l *linkedList[T]) move(e, at *linkedListElement[T]) {
	if e == at {
		return
	}
	e.prev.next = e.next
	e.next.prev = e.prev

	e.prev = at
	e.next = at.next
	e.prev.next = e
	e.next.prev = e
}

// Remove removes e from l if e is an element of list l.
// It returns the element value e.Value.
// The element must not be nil.
func (l *linkedList[T]) Remove(e *linkedListElement[T]) T {
	if e.list == l {
		// if e.list == l, l must have been initialized when e was inserted
		// in l or l == nil (e is a zero Element) and l.remove will crash
		l.remove(e)
	}
	return e.Value
}

// PushFront inserts a new element e with value v at the front of list l and returns e.
func (l *linkedList[T]) PushFront(v T) *linkedListElement[T] {
	l.lazyInit()
	return l.insertValue(v, &l.root)
}

// PushBack inserts a new element e with value v at the back of list l and returns e.
func (l *linkedList[T]) PushBack(v T) *linkedListElement[T] {
	l.lazyInit()
	return l.insertValue(v, l.root.prev)
}

// InsertBefore inserts a new element e with value v immediately before mark and returns e.
// If mark is not an element of l, the list is not modified.
// The mark must not be nil.
func (l *linkedList[T]) InsertBefore(v T, mark *linkedListElement[T]) *linkedListElement[T] {
	if mark.list != l {
		return nil
	}
	// see comment in List.Remove about initialization of l
	return l.insertValue(v, mark.prev)
}

// InsertAfter inserts a new element e with value v immediately after mark and returns e.
// If mark is not an element of l, the list is not modified.
// The mark must not be nil.
func (l *linkedList[T]) InsertAfter(v T, mark *linkedListElement[T]) *linkedListElement[T] {
	if mark.list != l {
		return nil
	}
	// see comment in List.Remove about initialization of l
	return l.insertValue(v, mark)
}

// MoveToFront moves element e to the front of list l.
// If e is not an element of l, the list is not modified.
// The element must not be nil.
func (l *linkedList[T]) MoveToFront(e *linkedListElement[T]) {
	if e.list != l || l.root.next == e {
		return
	}
	// see comment in List.Remove about initialization of l
	l.move(e, &l.root)
}

// MoveToBack moves element e to the back of list l.
// If e is not an element of l, the list is not modified.
// The element must not be nil.
func (l *linkedList[T]) MoveToBack(e *linkedListElement[T]) {
	if e.list != l || l.root.prev == e {
		return
	}
	// see comment in List.Remove about initialization of l
	l.move(e, l.root.prev)
}

// MoveBefore moves element e to its new position before mark.
// If e or mark is not an element of l, or e == mark, the list is not modified.
// The element and mark must not be nil.
func (l *linkedList[T]) MoveBefore(e, mark *linkedListElement[T]) {
	if e.list != l || e == mark || mark.list != l {
		return
	}
	l.move(e, mark.prev)
}

// MoveAfter moves element e to its new position after mark.
// If e or mark is not an element of l, or e == mark, the list is not modified.
// The element and mark must not be nil.
func (l *linkedList[T]) MoveAfter(e, mark *linkedListElement[T]) {
	if e.list != l || e == mark || mark.list != l {
		return
	}
	l.move(e, mark)
}

// PushBackList inserts a copy of another list at the back of list l.
// The lists l and other may be the same. They must not be nil.
func (l *linkedList[T]) PushBackList(other *linkedList[T]) {
	l.lazyInit()
	for i, e := other.Len(), other.Front(); i > 0; i, e = i-1, e.Next() {
		l.insertValue(e.Value, l.root.prev)
	}
}

// PushFrontList inserts a copy of another list at the front of list l.
// The lists l and other may be the same. They must not be nil.
func (l *linkedList[T]) PushFrontList(other *linkedList[T]) {
	l.lazyInit()
	for i, e := other.Len(), other.Back(); i > 0; i, e = i-1, e.Prev() {
		l.insertValue(e.Value, &l.root)
	}
}

// Copyright 2022, Pulumi Corporation.  All rights reserved.

package util

type Set[T comparable] map[T]struct{}

func NewSet[T comparable](elements ...T) Set[T] {
	s := make(map[T]struct{}, len(elements))
	for _, e := range elements {
		s[e] = struct{}{}
	}
	return s
}

func (s Set[T]) Has(element T) bool {
	_, ok := s[element]
	return ok
}

// Map f over arr.
func MapOver[T any, U any, F func(T) U](arr []T, f F) []U {
	l := make([]U, len(arr))
	for i, t := range arr {
		l[i] = f(t)
	}
	return l
}

type Tuple[A any, B any] struct {
	A A
	B B
}

// Check if slice contains the element.
func SliceContains[T comparable](slice []T, element T) bool {
	for _, t := range slice {
		if t == element {
			return true
		}
	}
	return false
}

// Retrieve the keys of a map in any order.
func MapKeys[K comparable, V any](m map[K]V) []K {
	arr := make([]K, len(m))
	i := 0
	for k := range m {
		arr[i] = k
		i++
	}
	return arr
}

// Retrieve the values of a map in any order.
func MapValues[K comparable, V any](m map[K]V) []V {
	arr := make([]V, len(m))
	i := 0
	for _, v := range m {
		arr[i] = v
		i++
	}
	return arr
}

// Dereference each element in a list.
func DerefList[T any](l []*T) []T {
	return MapOver(l, func(t *T) T { return *t })
}

// ReverseList reverses a list. It runs in O(n) time and O(n) space.
func ReverseList[T any](l []T) []T {
	rev := make([]T, 0, len(l))
	for i := len(l) - 1; i >= 0; i-- {
		rev = append(rev, l[i])
	}
	return rev
}

func StripNils[T any](l []*T) []*T {
	out := make([]*T, 0, len(l))
	for _, v := range l {
		if v == nil {
			continue
		}
		out = append(out, v)
	}
	return out
}

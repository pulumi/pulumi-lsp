// Copyright 2022, Pulumi Corporation.  All rights reserved.

package util

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

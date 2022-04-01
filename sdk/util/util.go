// Copyright 2022, Pulumi Corporation.  All rights reserved.

package util

func MapOver[T any, U any](in []T, f func(T) U) []U {
	l := make([]U, len(in))
	for i, t := range in {
		l[i] = f(t)
	}
	return l
}

type Tuple[A any, B any] struct {
	A A
	B B
}

func SliceContains[T comparable](slice []T, el T) bool {
	for _, t := range slice {
		if t == el {
			return true
		}
	}
	return false
}

func MapKeys[K comparable, V any](m map[K]V) []K {
	arr := make([]K, len(m))
	i := 0
	for k := range m {
		arr[i] = k
		i++
	}
	return arr
}

func MapValues[K comparable, V any](m map[K]V) []V {
	arr := make([]V, len(m))
	i := 0
	for _, v := range m {
		arr[i] = v
		i++
	}
	return arr
}

func DerefList[T any](l []*T) []T {
	ls := make([]T, len(l))
	for i := range l {
		ls[i] = *l[i]
	}
	return ls
}

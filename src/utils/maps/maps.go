package maps

import (
	"golang.org/x/exp/constraints"
	"sort"
)

func Keys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func Copy[K comparable, V any](m map[K]V) map[K]V {
	res := make(map[K]V)
	for k, v := range m {
		res[k] = v
	}
	return res
}

func SortedKeys[K constraints.Ordered, V any](m map[K]V) []K {
	keys := Keys(m)
	sort.Slice(keys, func(i int, j int) bool { return keys[i] < keys[j] })
	return keys
}

func Chain[K comparable, V any](ms ...map[K]V) map[K]V {
	res := make(map[K]V)
	for _, m := range ms {
		for k, v := range m {
			if _, ok := res[k]; !ok {
				res[k] = v
			}
		}
	}
	return res
}

func Update[K comparable, V any](m map[K]V, update map[K]V) map[K]V {
	for k, v := range update {
		m[k] = v
	}
	return m
}

func Values[K comparable, V any](m map[K]V) []V {
	values := make([]V, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

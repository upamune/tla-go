package main

import (
	"slices"
	"unicode/utf16"
)

// TLC's DOT writer uses Java HashSet and HashMap iteration order for ranks.
type javaHashSet[T comparable] struct {
	buckets [][]T
	size    int
	hash    func(T) uint32
}

type (
	javaLongSet struct{ javaHashSet[uint64] }
	javaIntSet  struct{ javaHashSet[int] }
)

func (s *javaLongSet) add(value uint64) {
	if s.hash == nil {
		s.hash = func(v uint64) uint32 { return javaSpread(uint32(v) ^ uint32(v>>32)) }
	}
	s.javaHashSet.add(value)
}

func (s *javaIntSet) add(value int) {
	if s.hash == nil {
		s.hash = func(v int) uint32 { return javaSpread(uint32(v)) }
	}
	s.javaHashSet.add(value)
}

func javaSpread(hash uint32) uint32 { return hash ^ hash>>16 }

func (s *javaHashSet[T]) add(value T) {
	if len(s.buckets) == 0 {
		s.buckets = make([][]T, 16)
	}
	index := int(s.hash(value) & uint32(len(s.buckets)-1))
	if slices.Contains(s.buckets[index], value) {
		return
	}
	s.buckets[index] = append(s.buckets[index], value)
	s.size++
	if s.size > 3*len(s.buckets)/4 {
		s.grow()
	}
}

func (s *javaHashSet[T]) grow() {
	old := s.buckets
	s.buckets = make([][]T, len(old)*2)
	for _, bucket := range old {
		for _, value := range bucket {
			index := int(s.hash(value) & uint32(len(s.buckets)-1))
			s.buckets[index] = append(s.buckets[index], value)
		}
	}
}

func (s *javaHashSet[T]) values() []T {
	result := make([]T, 0, s.size)
	for _, bucket := range s.buckets {
		result = append(result, bucket...)
	}
	return result
}

// javaStringSet follows java.util.HashSet<String> iteration order.
type javaStringSet struct{ javaHashSet[string] }

// newJavaStringSet creates a set with Java's table size for a requested
// initial capacity, as HashSet(int) and HashSet(Collection) do.
func newJavaStringSet(capacity int) *javaStringSet {
	size := 1
	for size < capacity {
		size *= 2
	}
	s := &javaStringSet{}
	s.hash = func(v string) uint32 { return javaSpread(javaStringHash(v)) }
	s.buckets = make([][]string, size)
	return s
}

// javaStringSetOf copies values as new HashSet<>(collection) does.
func javaStringSetOf(values []string) *javaStringSet {
	s := newJavaStringSet(max(int(float32(len(values))/0.75)+1, 16))
	for _, v := range values {
		s.add(v)
	}
	return s
}

func javaStringHash(s string) uint32 {
	var h uint32
	for _, unit := range utf16.Encode([]rune(s)) {
		h = 31*h + uint32(unit)
	}
	return h
}

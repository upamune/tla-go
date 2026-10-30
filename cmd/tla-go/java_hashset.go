package main

import "slices"

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

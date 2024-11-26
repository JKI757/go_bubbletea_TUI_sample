// Set is a custom type using a map to represent unique elements
package main

type Set[T comparable] struct {
	data map[T]struct{}
}

// NewSet creates and returns a new Set
func NewSet[T comparable]() *Set[T] {
	return &Set[T]{data: make(map[T]struct{})}
}

// Add inserts an element into the Set
func (s *Set[T]) Add(value T) {
	s.data[value] = struct{}{}
}

// Contains checks if the element is in the Set
func (s *Set[T]) Contains(value T) bool {
	_, exists := s.data[value]
	return exists
}

// Remove deletes an element from the Set
func (s *Set[T]) Remove(value T) {
	delete(s.data, value)
}

// MyStruct with a Set as a member

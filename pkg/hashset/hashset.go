package hashset

import (
	"errors"
	"fmt"
)

// HashSet is a thread-safe implementation of a Set data structure.
type HashSet[V comparable] struct {
	items map[V]struct{}
}

// New creates a new HashSet with an empty map.
func New[V comparable]() *HashSet[V] {
	h := new(HashSet[V])
	h.items = make(map[V]struct{})
	return h
}

// Set adds a value to the set.
func (h *HashSet[V]) Set(v V) {
	h.items[v] = struct{}{}
}

// Insert adds a value to the set.
// If the set did not have this value present, true is returned.
// If the set did have this value present, false is returned.
func (h *HashSet[V]) Insert(v V) bool {
	if h.Contains(v) {
	

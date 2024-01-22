package hashset

import "errors"

type HashSet[V comparable] struct {
	items map[V]struct{}
}

func New[V comparable]() *HashSet[V] {
	h := new(HashSet[V])
	h.items = make(map[V]struct{})
	return h
}

func (h *HashSet[V]) Set(v V) {
	h.items[v] = struct{}{}
}

// Insert adds a value to the set.
// If the set did not have this value present, true is returned.
// If the set did have this value present, false is returned.
func (h *HashSet[V]) Insert(v V) bool {
	if h.Contains(v) {
		return false
	}
	h.items[v] = struct{}{}
	return true
}

// Contains returns true if the set contains a value.
func (h *HashSet[V]) Contains(v V) bool {
	_, found := h.items[v]
	return found
}

// Len returns the number of elements in the set.
func (h *HashSet[V]) Len() int {
	return len(h.items)
}

func (h *HashSet[V]) Empty() bool {
	return len(h.items) == 0
}

// Clear clears the set, removing all values.
func (h *HashSet[V]) Clear() {
	h.items = make(map[V]struct{})
}

// Difference visits the values representing the difference, i.e., the values that are in self but not in other.
func (h *HashSet[V]) Difference(other *HashSet[V]) *HashSet[V] {
	n := New[V]()
	for k, v := range h.items {
		if !other.Contains(k) {
			n.items[k] = v
		}
	}
	return n
}

// SymmetricDifference visits the values representing the symmetric difference, i.e., the values that are in self or in other but not in both.
func (h *HashSet[V]) SymmetricDifference(other *HashSet[V]) *HashSet[V] {
	n := New[V]()
	for k, v := range h.items {
		if !other.Contains(k) {
			n.items[k] = v
		}
	}
	for k, v := range other.items {
		if !h.Contains(k) {
			n.items[k] = v
		}
	}
	return n
}

// Intersection visits the values representing the intersection, i.e., the values that are both in self and other.
func (h *HashSet[V]) Intersection(other *HashSet[V]) *HashSet[V] {
	n := New[V]()
	for k, v := range h.items {
		if other.Contains(k) {
			n.items[k] = v
		}
	}
	return n
}

// Union visits the values representing the union, i.e., all the values in self or other, without duplicates.
func (h *HashSet[V]) Union(other *HashSet[V]) *HashSet[V] {
	n := New[V]()
	for k, v := range h.items {
		n.items[k] = v
	}
	for k, v := range other.items {
		n.items[k] = v
	}
	return n
}

// Get returns a reference to the value in the set, if any, that is equal to the given value.
func (h *HashSet[V]) Get(v V) (out V, err error) {
	if !h.Contains(v) {
		return out, errors.New("item not found")
	}
	return v, nil
}

// IsSubset returns true if the set is a subset of another, i.e., other contains at least all the values in self.
func (h *HashSet[V]) IsSubset(other *HashSet[V]) bool {
	for k := range h.items {
		if !other.Contains(k) {
			return false
		}
	}
	return true
}

// IsSuperset returns true if the set is a superset of another, i.e., self contains at least all the values in other.
func (h *HashSet[V]) IsSuperset(other *HashSet[V]) bool {
	for k := range other.items {
		if !h.Contains(k) {
			return false
		}
	}
	return true
}

// Replace adds a value to the set, replacing the existing value, if any, that is equal to the given one. Returns the replaced value.
func (h *HashSet[V]) Replace(v V) (V, bool) {
	var zero V
	if h.Contains(v) {
		return v, true
	}
	h.items[v] = struct{}{}
	return zero, false
}

// Remove a value from the set. Returns whether the value was present in the set.
func (h *HashSet[V]) Remove(v V) bool {
	if !h.Contains(v) {
		return false
	}
	delete(h.items, v)
	return true
}

func (h *HashSet[V]) Delete(v V) {
	delete(h.items, v)
}

// Take removes and returns the value in the set, if any, that is equal to the given one.
func (h *HashSet[V]) Take(v V) (out V, err error) {
	if !h.Contains(v) {
		return out, errors.New("item not found")
	}
	delete(h.items, v)
	return v, nil
}

func (h *HashSet[V]) Each(clb func(V)) {
	for k := range h.items {
		clb(k)
	}
}

func (h *HashSet[V]) ToArray() []V {
	out := make([]V, len(h.items))
	i := 0
	for k := range h.items {
		out[i] = k
		i++
	}
	return out
}

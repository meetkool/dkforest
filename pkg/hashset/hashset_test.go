package hashset

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	h := New[string]()
	assert.NotNil(t, h)
}

func TestInsert(t *testing.T) {
	h := New[string]()
	r1 := h.Insert("key1")
	assert.True(t, r1)
	r2 := h.Insert("key1")
	assert.False(t, r2)
	assert.True(t, h.Contains("key1"))
}

func TestUnion(t *testing.T) {
	h1 := New[string]()
	h1.Insert("key1")
	h2 := New[string]()
	h2.Insert("key2")
	h3 := h1.Union(h2)
	assert.True(t, h3.Contains("key1"))
	assert.True(t, h3.Contains("key2"))
}

func TestDifference(t *testing.T) {
	h1 := New[string]()
	h1.Insert("key1")
	h1.Insert("key2")
	h2 := New[string]()
	h2.Insert("key2")
	h3 := h1.Difference(h2)
	assert.True(t, h3.Contains("key1"))
	assert.False(t, h3.Contains("key2"))

	h1 = New[string]()
	h1.Insert("key1")
	h1.Insert("key2")
	h2 = New[string]()
	h2.Insert("key1")
	h3 = h1.Difference(h2)
	assert.False(t, h3.Contains("key1"))
	assert.True(t, h3.Contains("key2"))

	h1 = New[string]()
	h1.Insert("key1")
	h1.Insert("key2")
	h2 = New[string]()
	h2.Insert("key1")
	h2.Insert("key3")
	h3 = h1.Difference(h2)
	assert.False(t, h3.Contains("key1"))
	assert.True(t, h3.Contains("key2"))
	assert.False(t, h3.Contains("key3"))
}

func TestSymmetricDifference(t *testing.T) {
	h1 := New[string]()
	h1.Insert("key1")
	h1.Insert("key2")
	h2 := New[string]()
	h2.Insert("key1")
	h2.Insert("key3")
	h3 := h1.SymmetricDifference(h2)
	assert.False(t, h3.Contains("key1"))
	assert.True(t, h3.Contains("key2"))
	assert.True(t, h3.Contains("key3"))
}

func TestIntersection(t *testing.T) {
	h1 := New[string]()
	h1.Insert("key1")
	h1.Insert("key2")
	h2 := New[string]()
	h2.Insert("key1")
	h2.Insert("key3")
	h3 := h1.Intersection(h2)
	assert.True(t, h3.Contains("key1"))
	assert.False(t, h3.Contains("key2"))
	assert.False(t, h3.Contains("key3"))
}

func TestIsSubset(t *testing.T) {
	h1 := New[string]()
	h1.Insert("key1")
	h1.Insert("key2")
	h2 := New[string]()
	h2.Insert("key1")
	h2.Insert("key3")
	assert.False(t, h1.IsSubset(h2))

	h1 = New[string]()
	h1.Insert("key1")
	h1.Insert("key2")
	h2 = New[string]()
	h2.Insert("key1")
	h2.Insert("key2")
	h2.Insert("key3")
	assert.True(t, h1.IsSubset(h2))

	h1 = New[string]()
	h1.Insert("key1")
	h1.Insert("key2")


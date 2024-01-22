package rwmtx

import (
	"sync"
)

type Mtx[T any] struct {
	sync.Mutex
	v T
}

func NewMtx[T any](v T) Mtx[T] {
	return Mtx[T]{v: v}
}

func (m *Mtx[T]) Val() *T {
	return &m.v
}

func (m *Mtx[T]) Get() T {
	m.Lock()
	defer m.Unlock()
	return m.v
}

func (m *Mtx[T]) Set(v T) {
	m.Lock()
	defer m.Unlock()
	m.v = v
}

func (m *Mtx[T]) With(clb func(v *T)) {
	_ = m.WithE(func(tx *T) error {
		clb(tx)
		return nil
	})
}

func (m *Mtx[T]) WithE(clb func(v *T) error) error {
	m.Lock()
	defer m.Unlock()
	return clb(&m.v)
}

//----------------------

type RWMtx[T any] struct {
	sync.RWMutex
	v T
}

func New[T any](v T) RWMtx[T] {
	return RWMtx[T]{v: v}
}

func (m *RWMtx[T]) Val() *T {
	return &m.v
}

func (m *RWMtx[T]) Get() T {
	m.RLock()
	defer m.RUnlock()
	return m.v
}

func (m *RWMtx[T]) Set(v T) {
	m.Lock()
	defer m.Unlock()
	m.v = v
}

func (m *RWMtx[T]) Replace(newVal T) (old T) {
	m.With(func(v *T) {
		old = *v
		*v = newVal
	})
	return
}

func (m *RWMtx[T]) RWith(clb func(v T)) {
	_ = m.RWithE(func(tx T) error {
		clb(tx)
		return nil
	})
}

func (m *RWMtx[T]) RWithE(clb func(v T) error) error {
	m.RLock()
	defer m.RUnlock()
	return clb(m.v)
}

func (m *RWMtx[T]) With(clb func(v *T)) {
	_ = m.WithE(func(tx *T) error {
		clb(tx)
		return nil
	})
}

func (m *RWMtx[T]) WithE(clb func(v *T) error) error {
	m.Lock()
	defer m.Unlock()
	return clb(&m.v)
}

//----------------------

type RWMtxSlice[T any] struct {
	RWMtx[[]T]
}

func (s *RWMtxSlice[T]) Each(clb func(T)) {
	s.RWith(func(v []T) {
		for _, e := range v {
			clb(e)
		}
	})
}

func (s *RWMtxSlice[T]) Append(els ...T) {
	s.With(func(v *[]T) { *v = append(*v, els...) })
}

func (s *RWMtxSlice[T]) Clone() (out []T) {
	s.RWith(func(v []T) {
		out = make([]T, len(v))
		copy(out, v)
	})
	return
}

//----------------------

type RWMtxUInt64[T ~uint64] struct {
	RWMtx[T]
}

func (s *RWMtxUInt64[T]) Incr(diff T) {
	s.With(func(v *T) { *v += diff })
}

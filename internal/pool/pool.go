package pool

import (
	"sync"
)

// Resettable is an interface for types that can be reset to their initial state.
type Resettable interface {
	Reset()
}

// Pool is a generic container for objects that implement the Resettable interface.
// It leverages sync.Pool for efficient object reuse.
type Pool[T Resettable] struct {
	pool sync.Pool
}

// New creates and returns a new Pool for objects of type T.
// The factory function is used to create new objects when the pool is empty.
func New[T Resettable](factory func() T) *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() any {
				return factory()
			},
		},
	}
}

// Get retrieves an object from the pool.
// If the pool is empty, a new object is created using the factory function.
func (p *Pool[T]) Get() T {
	v := p.pool.Get()
	return v.(T)
}

// Put returns an object to the pool.
// The object's Reset() method is called before it is placed back into the pool.
func (p *Pool[T]) Put(v T) {
	v.Reset()
	p.pool.Put(v)
}

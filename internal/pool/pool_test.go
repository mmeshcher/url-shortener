package pool

import (
	"testing"
)

type mockResettable struct {
	val     int
	isReset bool
}

func (m *mockResettable) Reset() {
	m.val = 0
	m.isReset = true
}

func TestPool(t *testing.T) {
	factory := func() *mockResettable {
		return &mockResettable{val: 1, isReset: false}
	}

	p := New[*mockResettable](factory)

	// Get an object from the pool (newly created)
	obj := p.Get()
	if obj.val != 1 || obj.isReset {
		t.Errorf("expected val 1 and isReset false, got val %d and isReset %v", obj.val, obj.isReset)
	}

	// Modify the object and put it back in the pool
	obj.val = 42
	p.Put(obj)

	// Put() should call Reset()
	if !obj.isReset || obj.val != 0 {
		t.Errorf("expected isReset true and val 0 after Put(), got isReset %v and val %d", obj.isReset, obj.val)
	}

	// Get the same object (hopefully) from the pool
	obj2 := p.Get()
	if !obj2.isReset || obj2.val != 0 {
		t.Errorf("expected isReset true and val 0 after Get() from pool, got isReset %v and val %d", obj2.isReset, obj2.val)
	}
}

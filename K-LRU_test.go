package ihe_lru

import "testing"

func TestGeneralLRUUse(t *testing.T) {
	l := NewGeneralLRU(1)
	// get un-exists item
	v, ok := l.Get("hello")
	if ok {
		t.Log(v)
	}

	// add hello world
	l.Add("hello", "world")

	// get exists
	v, ok = l.Get("hello")
	if ok {
		t.Log(v)
	}

	// add exceeded item to ensure evict fine
	l.Add("good", "kangkang")
	v, ok = l.Get("hello")
	if ok {
		t.Log(v)
	}
	v, ok = l.Get("good")
	if ok {
		t.Log(v)
	}
}

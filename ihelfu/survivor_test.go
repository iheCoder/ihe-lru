package ihelfu

import "testing"

func TestSurvivorBasicUse(t *testing.T) {
	s := NewConcurrentSurvivor(2)
	s.Append("a")
	s.Append("b")
	s.Append("c")
	if s.pos != backPos || !s.front.IsEmpty() {
		t.Fatal("err after first full")
	}
	s.Append("d")
	s.Append("e")
	if s.pos != frontPos || !s.back.IsEmpty() {
		t.Fatal("err after second full")
	}
}

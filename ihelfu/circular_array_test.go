package ihelfu

import "testing"

func TestBasicUseCA(t *testing.T) {
	ca := NewCircularArray(3)
	ca.Append("a")
	if ca.items[0] != "a" {
		t.Fatal("not append a")
	}

	ca.Append("b")
	ca.Append("c")
	if !ca.IsFull() {
		t.Fatal("should full with abc")
	}

	x1 := ca.Remove(2)
	if len(x1) != 2 || x1[0] != "a" || x1[1] != "b" {
		t.Fatalf("fail to rm %v", x1)
	}

	ca.Append("d")
	ca.Append("e")
	if !ca.IsFull() {
		t.Fatal("should full with cde")
	}

	x2 := ca.Remove(2)
	if len(x2) != 2 || x2[0] != "c" || x2[1] != "d" {
		t.Fatalf("fail to rm %v", x1)
	}
}

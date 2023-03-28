package ihelfu

import (
	"learn/tool/fuzz"
	"sync"
	"testing"
	"time"
)

func TestGetCSTLimit(t *testing.T) {
	x1 := int64(10)
	y1 := getCSTLimit(x1)
	if y1 != 64 {
		t.Fatalf("y1 %d", y1)
	}
	x2 := int64(512)
	y2 := getCSTLimit(x2)
	if y2 != 64 {
		t.Fatalf("y2 %d", y2)
	}
	x3 := int64(513)
	y3 := getCSTLimit(x3)
	if y3 != 2*64 {
		t.Fatalf("y3 %d", y3)
	}
}

func TestSEEBasicUse(t *testing.T) {
	ie, err := NewIheEvict(100, make(chan []string))
	if err != nil {
		t.Fatal(err)
	}

	ie.UpdateAccessCount("a")
	ie.UpdateAccessCount("a")
	ie.UpdateAccessCount("b")
	ie.UpdateAccessCount("c")
	ie.UpdateAccessCount("a")

	time.Sleep(10 * time.Second)
}

func TestBenchSEE(t *testing.T) {
	count := 1000000
	ie, err := NewIheEvict(100, make(chan []string))
	if err != nil {
		t.Fatal(err)
	}
	wg := &sync.WaitGroup{}
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			ie.UpdateAccessCount(fuzz.GenerateRandStringFixedNumber(1))
			wg.Done()
		}()
	}
	wg.Wait()
}

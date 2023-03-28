package ihelfu

import (
	"fmt"
	"learn/tool/fuzz"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSegIheLfuBasicUse(t *testing.T) {
	u, err := NewSegIheLfu(2)
	if err != nil {
		t.Fatal(err)
	}
	_, ok := u.Get("a")
	if ok {
		t.Fatal("no a indeed")
	}
	u.Insert("a", 1)
	v, ok := u.Get("a")
	if !ok || v != 1 {
		t.Fatal("has a indeed")
	}
}

var miss = int64(0)

func TestBenchSegIheLfu(t *testing.T) {
	// 如果size只给100，那么能撑到10000就算胜利
	// 100w居然才80M，这是多么大的进步啊！！！
	// 还是就是for i:= range go func，i可是复用同一个变量，导致可能会有多个goroutine中的i相同
	count := 1000000
	u, err := NewSegIheLfu(100)
	if err != nil {
		t.Fatal(err)
	}

	wg := &sync.WaitGroup{}
	wg.Add(count)
	var ms runtime.MemStats
	for i := 0; i < count; i++ {
		if i%10 != 0 {
			go func() {
				k := fuzz.GenerateRandStringFixedNumber(2)
				if _, ok := u.Get(k); !ok {
					atomic.AddInt64(&miss, 1)
					u.Insert(k, int64(i))
				}

				wg.Done()
			}()
		} else {
			go func() {
				u.Insert(fuzz.GenerateRandStringFixedNumber(2), int64(i))
				wg.Done()
			}()
		}
		if i%1000 == 0 {
			runtime.ReadMemStats(&ms)
			fmt.Printf("i %d mem alloc %d \n", i, byteToMB(ms.Alloc))
			//time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
		}
	}
	wg.Wait()
	fmt.Printf("miss rate %d", miss)
}

func byteToMB(bs uint64) uint64 {
	return bs / 1024 / 1024
}

func TestWillLeakIfTooManyGo(t *testing.T) {
	count := 10000
	wg := &sync.WaitGroup{}
	wg.Add(count)
	mu := &sync.Mutex{}
	for i := 0; i < count; i++ {
		go func() {
			mu.Lock()
			time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
			mu.Unlock()
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestMultipleLockUnlock(t *testing.T) {
	type lockUnlock struct {
		mu *sync.RWMutex
	}
	count := 64
	ls := make([]*lockUnlock, count)
	for i := 0; i < count; i++ {
		ls[i] = &lockUnlock{mu: &sync.RWMutex{}}
	}

	for _, l := range ls {
		l := l
		go func() {
			l.mu.Lock()

		}()
	}
	time.Sleep(10 * time.Second)
}

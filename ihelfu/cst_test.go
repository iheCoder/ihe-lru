package ihelfu

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func check(key string) bool {
	n := rand.Int()
	return n/3 == 0
}

func TestCSTUsePanic(t *testing.T) {
	cst := NewConcurrentSegTable(64, 8, check)
	count := 10000
	wg := &sync.WaitGroup{}
	wg.Add(count)
	timer := time.NewTicker(time.Second)
	go func() {
		for range timer.C {
			cst.Clean()
		}
	}()
	for i := 0; i < count; i++ {
		x := rand.Int63()
		if x/3 == 0 {
			go pushWg(cst, wg, strconv.Itoa(i))
		} else {
			go pushBackWg(cst, wg, strconv.Itoa(i))
		}
	}
	wg.Wait()
}

func pushFrontWg(cst *ConcurrentSegTable, wg *sync.WaitGroup, key string) {
	defer wg.Done()
	err := cst.AddFront(key)
	if err != nil {
		return
	}
}

func pushWg(cst *ConcurrentSegTable, wg *sync.WaitGroup, key string) {
	defer wg.Done()
	err := cst.Add(key)
	if err != nil {
		return
	}
}

func pushBackWg(cst *ConcurrentSegTable, wg *sync.WaitGroup, key string) {
	defer wg.Done()
	err := cst.AddBack(key)
	if err != nil {
		return
	}
}

func TestRanConcurrent(t *testing.T) {

}

var xi = 0

func TestAtomicConcurrent(t *testing.T) {
	count := 10000
	xav := &atomic.Value{}
	xav.Store(0)
	wg := &sync.WaitGroup{}
	wg.Add(count)
	for i := 0; i < count; i++ {
		go updateAV(xav, wg)
	}
	wg.Wait()
}

func updateAV(av *atomic.Value, wg *sync.WaitGroup) {
	xi++
	av.Load()
	av.Store(xi)
	fmt.Println(av.Load())
	wg.Done()
}

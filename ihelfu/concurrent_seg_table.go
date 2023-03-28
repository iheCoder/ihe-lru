package ihelfu

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
)

const (
	maxUint64     = math.MaxUint64
	defaultSegLen = 8
)

var (
	ErrFull = errors.New("table full")
)

type ConcurrentSegTable struct {
	items     []*segTable
	sizeLimit int
	lock      sync.RWMutex
	full      []uint64
	m         func(key string) bool
	segLen    int
}

// limit 64倍数
func NewConcurrentSegTable(limit int, segLen int, m func(key string) bool) *ConcurrentSegTable {
	items := make([]*segTable, limit)
	for i := 0; i < limit; i++ {
		s := newSegTable(segLen, m)
		items[i] = s
	}

	return &ConcurrentSegTable{
		items:     items,
		sizeLimit: limit,
		lock:      sync.RWMutex{},
		full:      make([]uint64, limit>>6),
		segLen:    segLen,
		m:         m,
	}
}

// 如何确保lfu中删除的和evict中删除的一致呢？
func (t *ConcurrentSegTable) Reset() []string {
	items := make([]*segTable, t.sizeLimit)
	for i := 0; i < t.sizeLimit; i++ {
		s := newSegTable(t.segLen, t.m)
		items[i] = s
	}
	t.lock.Lock()
	t.items = items
	t.full = make([]uint64, t.sizeLimit>>6)
	r := t.list()
	t.lock.Unlock()
	return r
}

func (t *ConcurrentSegTable) list() []string {
	r := make([]string, 0, t.segLen*t.sizeLimit)
	var s *segTable
	for _, s = range t.items {
		if !s.isEmpty() {
			for k, _ := range s.items {
				r = append(r, k)
			}
		}
	}
	return r
}

func (t *ConcurrentSegTable) Add(key string) error {
	if t.IsFull() {
		return ErrFull
	}

	var s *segTable
	i := rand.Intn(len(t.items))
	count := 0

	t.lock.RLock()
	defer t.lock.RUnlock()
	for count < len(t.items) {
		s = t.items[i]
		if ok := t.add(s, i, key); ok {
			return nil
		}
		if i == len(t.items)-1 {
			i = 0
		} else {
			i++
		}
		count++
	}

	return ErrFull
}

func (t *ConcurrentSegTable) AddFront(key string) error {
	if t.IsFull() {
		return ErrFull
	}

	var s *segTable
	var i int
	for i, s = range t.items {
		if ok := t.add(s, i, key); ok {
			return nil
		}
	}

	return ErrFull
}

func (t *ConcurrentSegTable) AddBack(key string) error {
	if t.IsFull() {
		return ErrFull
	}

	var s *segTable
	for i := len(t.items) - 1; i >= 0; i-- {
		s = t.items[i]
		if ok := t.add(s, i, key); ok {
			return nil
		}
	}

	return ErrFull
}

func (t *ConcurrentSegTable) add(s *segTable, i int, key string) bool {
	if s.isFull() {
		return false
	}

	s.lock()
	defer s.unlock()

	if s.isFull() {
		return false
	}

	s.state.Store(adding)
	defer s.state.Store(normal)

	s.add(key)
	if s.isFull() {
		t.setFull(i)
	}
	return true
}

func (t *ConcurrentSegTable) setFull(i int) {
	ix := i >> 6
	iy := i - ix
	t.lock.Lock()
	f := t.full[ix]
	f = f | (1 << iy)
	t.lock.Unlock()
}

func (t *ConcurrentSegTable) setEmpty(i int) {
	ix := i >> 6
	iy := i - ix
	t.lock.Lock()
	f := t.full[ix]
	f = f & (^(1 << iy))
	t.lock.Unlock()
}

func (t *ConcurrentSegTable) Clean() {
	var i int
	var s *segTable
	for i, s = range t.items {
		s := s
		i := i
		// 这里还能出现goroutine泄露!!!
		if !s.isEmpty() && s.state.Load().(int) != cleaning {
			go func() {
				t.lock.RLock()
				s.state.Store(cleaning)
				s.lock()
				s.clean()
				if s.isEmpty() {
					t.setEmpty(i)
				}
				s.unlock()
				s.state.Store(normal)
				t.lock.RUnlock()
			}()
		}
	}
}

func (t *ConcurrentSegTable) IsFull() bool {
	t.lock.RLock()
	defer t.lock.RUnlock()

	for _, f := range t.full {
		if f != maxUint64 {
			return false
		}
	}
	return true
}

const (
	normal   = 0
	adding   = 1
	cleaning = 2
)

type segTable struct {
	// 似乎没什么必要将items设定为list.List。更改为[]string也是可以的吧。或者map更棒
	items     map[string]struct{}
	sizeLimit int
	// 难道是因为之前的mu是非指针类型导致panic，而waitGroup Done无法执行出现这么多goroutine etc嘛
	mu    *sync.RWMutex
	m     func(key string) bool
	state *atomic.Value
}

func newSegTable(limit int, m func(key string) bool) *segTable {
	av := &atomic.Value{}
	av.Store(normal)
	return &segTable{
		items:     make(map[string]struct{}, limit),
		sizeLimit: limit,
		mu:        &sync.RWMutex{},
		m:         m,
		state:     av,
	}
}

func (s *segTable) isFull() bool {
	return len(s.items) >= s.sizeLimit
}

func (s *segTable) isEmpty() bool {
	return len(s.items) == 0
}

func (s *segTable) lock() {
	s.mu.Lock()
}

func (s *segTable) rlock() {
	s.mu.RLock()
}

func (s *segTable) rUnlock() {
	s.mu.RUnlock()
}

func (s *segTable) unlock() {
	s.mu.Unlock()
}

func (s *segTable) add(key string) {
	s.items[key] = struct{}{}
}

func (s *segTable) remove(key string) {
	delete(s.items, key)
}

func (s *segTable) clean() {
	var key string
	for key, _ = range s.items {
		if s.m(key) {
			s.remove(key)
		}
	}
}

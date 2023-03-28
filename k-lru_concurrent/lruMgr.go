package k_lru_concurrent

import (
	"container/list"
	"learn/tool/timeCost"
	"sync"
	"sync/atomic"
	"time"
)

type opType int

const (
	add opType = iota
	moveToFront
	evict
)

type state int

// evict状态没有及时更新这会导致items变得无限大，所以miss rate 82/100 000也是可以理解的
const (
	running state = iota
	idle
)

type evictState atomic.Value

func (s *evictState) IsIdle() bool {
	return (*atomic.Value)(s).Load() == idle
}

func (s *evictState) SetState(x state) {
	(*atomic.Value)(s).Store(x)
}

type lruOp struct {
	eop opType
	e   *list.Element
	v   interface{}
	key string
}

type lruMgr struct {
	ops           chan *lruOp
	threshold     int
	items         map[string]*list.Element
	evictList     *list.List
	mu            sync.RWMutex
	es            evictState
	safeThreshold int
}

func NewLRUMgr(threshold, safeThreshold, optsSize int) *lruMgr {
	es := evictState{}
	es.SetState(idle)
	m := &lruMgr{
		ops:           make(chan *lruOp, optsSize),
		threshold:     threshold,
		items:         make(map[string]*list.Element),
		evictList:     list.New(),
		mu:            sync.RWMutex{},
		es:            es,
		safeThreshold: safeThreshold,
	}

	go m.handleOp()
	return m
}

func (m *lruMgr) Get(key string) (*item, bool) {
	m.mu.RLock()
	i, ok := m.items[key]
	m.mu.RUnlock()
	if !ok {
		return nil, false
	}

	return i.Value.(*item), true
}

func (m *lruMgr) TryUpdate(key string, i *item) bool {
	m.mu.RLock()
	e, ok := m.items[key]
	m.mu.RUnlock()
	if ok {
		e.Value = i
		return true
	}

	return false
}

func (m *lruMgr) NotifyAdd(key string, i *item) {
	m.notifyAdd(key, i)
}

func (m *lruMgr) notifyAdd(key string, i *item) {
	defer timeCost.DefaultTimeCostAnalyzer.DeferModuleTimeCost(timeCost.AddItem)()
	op := &lruOp{
		eop: add,
		key: key,
		v:   i,
	}
	m.ops <- op
}

func (m *lruMgr) NotifyEvict() {
	if m.evictList.Len() > m.threshold && m.es.IsIdle() {
		m.es.SetState(running)
		m.notifyEvict()
	}
}

func (m *lruMgr) notifyEvict() {
	defer timeCost.DefaultTimeCostAnalyzer.DeferModuleTimeCost(timeCost.EvictUnusedItem)()
	op := &lruOp{
		eop: evict,
	}
	m.ops <- op
}

func (m *lruMgr) NotifyMoveToFront(key string) {
	m.notifyMoveToFront(key)
}

func (m *lruMgr) notifyMoveToFront(key string) {
	defer timeCost.DefaultTimeCostAnalyzer.DeferModuleTimeCost(timeCost.NotifyPushFront)()
	m.mu.RLock()
	v, ok := m.items[key]
	m.mu.RUnlock()
	if !ok {
		return
	}
	op := &lruOp{
		eop: moveToFront,
		e:   v,
	}
	m.ops <- op
}

func (m *lruMgr) handleOp() {
	var now time.Time
	for op := range m.ops {
		switch op.eop {
		case add:
			now = time.Now()
			e := m.evictList.PushBack(op.v)
			m.mu.Lock()
			m.items[op.key] = e
			m.mu.Unlock()
			timeCost.DefaultTimeCostAnalyzer.RecordTimeCost(timeCost.AddItem, now)
		case moveToFront:
			m.evictList.MoveToFront(op.e)
		case evict:
			now = time.Now()
			for m.evictList.Len() > m.safeThreshold {
				back := m.evictList.Back()
				m.evictList.Remove(back)
				m.mu.Lock()
				delete(m.items, back.Value.(*item).key)
				m.mu.Unlock()
			}
			m.es.SetState(idle)
			timeCost.DefaultTimeCostAnalyzer.RecordTimeCost(timeCost.EvictUnusedItem, now)
		}
	}
}

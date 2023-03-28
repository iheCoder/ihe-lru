package ihelfu

import "sync"

type segIheLfu struct {
	ie          *iheEvict
	cache       map[string]int64
	lock        *sync.RWMutex
	evictNotify chan []string
}

const (
	defaultEvictRatio = 4
)

func (s *segIheLfu) Get(key string) (int64, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	v, ok := s.cache[key]
	if !ok {
		return -1, false
	}
	s.ie.UpdateAccessCount(key)
	return v, true
}

func (s *segIheLfu) Insert(key string, value int64) {
	// 似乎是这个地方堵死，导致goroutine一直创建却没有回收
	s.lock.RLock()

	_, ok := s.cache[key]
	if ok {
		s.lock.RUnlock()
		return
	}

	s.lock.RUnlock()
	if s.ie.UpdateAccessCount(key) {
		// 如果更新不成功，岂不是永远解锁不了啊
		s.lock.Lock()
		s.cache[key] = value
		s.lock.Unlock()
	}
}

func (s *segIheLfu) evict() {
	for keys := range s.evictNotify {
		s.lock.Lock()
		for _, key := range keys {
			delete(s.cache, key)
		}
		s.lock.Unlock()
	}
}

func NewSegIheLfu(size int) (*segIheLfu, error) {
	en := make(chan []string, size/10+1)
	ie, err := NewIheEvict(int64(size*defaultEvictRatio), en)
	if err != nil {
		return nil, err
	}
	u := &segIheLfu{
		ie:          ie,
		cache:       make(map[string]int64, size),
		lock:        &sync.RWMutex{},
		evictNotify: en,
	}

	go u.evict()
	return u, nil
}

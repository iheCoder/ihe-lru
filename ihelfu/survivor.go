package ihelfu

import "sync"

const (
	frontPos = uint8(0)
	backPos  = uint8(0xff)
)

type concurrentSurvivor struct {
	front *circularArray
	back  *circularArray
	pos   uint8
	mu    *sync.Mutex
}

func NewConcurrentSurvivor(size int) *concurrentSurvivor {
	return &concurrentSurvivor{
		front: NewCircularArray(size),
		back:  NewCircularArray(size),
		pos:   frontPos,
		mu:    &sync.Mutex{},
	}
}

// Append 添加元素，若满自动执行清理。并返回要清理元素
func (s *concurrentSurvivor) Append(key string) (isFull bool, r []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ca := s.getCurrentCA()

	if ca.IsFull() {
		s.pos = ^s.pos
		r = make([]string, ca.size)
		copy(r, ca.items)
		isFull = true
		ca.Reset()
		ca = s.getCurrentCA()
	}

	ca.Append(key)
	return
}

func (s *concurrentSurvivor) getCurrentCA() *circularArray {
	ca := s.front
	if s.pos == backPos {
		ca = s.back
	}
	return ca
}

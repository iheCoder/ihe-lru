package k_lru_concurrent

import (
	"learn/tool/timeCost"
)

type clru struct {
	mgr  *lruMgr
	size int
	ch   chan string
}

func NewCLRU(size int, ch chan string) *clru {
	m := NewLRUMgr(size, size-size/4, size*10)
	l := &clru{
		mgr:  m,
		size: size,
		ch:   ch,
	}

	return l
}

func (l *clru) Get(key string) (string, bool) {
	defer timeCost.DefaultTimeCostAnalyzer.DeferModuleTimeCost(timeCost.GetItem)()
	// 1. 查看是否在缓存中存在
	i, ok := l.mgr.Get(key)
	if !ok {
		return "", false
	}

	// 2. 通知将该元素访问次数增加
	l.notifyPushFront(key)

	// 3. 返回查出来的元素
	return i.value, true
}

func (l *clru) Add(key, value string) {
	defer timeCost.DefaultTimeCostAnalyzer.DeferModuleTimeCost(timeCost.AddItem)()

	i := &item{
		key:   key,
		value: value,
	}
	// 1. 判断是否存在该key，若存在更新，并将其访问次数加1
	ok := l.mgr.TryUpdate(key, i)
	if ok {
		return
	}

	// 2. 若元素不存在该key，则添加该元素至栈底，并将其访问次数加1
	l.mgr.NotifyAdd(key, i)

	// 3. 清理
	l.mgr.NotifyEvict()
}

func (l *clru) notifyPushFront(key string) {
	l.ch <- key
}

func (l *clru) MoveToFront(key string) {
	l.mgr.NotifyMoveToFront(key)
}

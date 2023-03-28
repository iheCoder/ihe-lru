package ihelfu

//import (
//	"container/list"
//	"sync"
//	"sync/atomic"
//	"time"
//)
//
//type iheEvict struct {
//}
//
//type evictZone struct {
//	lock      *sync.RWMutex
//	total     *atomic.Value
//	items     *list.List
//	sizeLimit int
//}
//
//type edenZone struct {
//	lock      *sync.RWMutex
//	total     *atomic.Value
//	items     *list.List
//	sizeLimit int
//}
//
//func (e *edenZone) Add(key string) {
//	// 超过限制时，evict
//	if e.items.Len() >= e.sizeLimit {
//
//	}
//}
//
//type adCache struct {
//	sizeLimit int
//	items     *list.List
//	ez        *edenZone
//	total     *atomic.Value
//}
//
//func (a *adCache) Advance(key string) {
//	// 当前进缓存满时，清除小于1/3 total的元素
//	if a.items.Len() >= a.sizeLimit {
//
//	}
//
//	// 放入
//	// 所以什么时候更新到eden，不要说是定时任务
//	// 就是定时任务，只是在
//}
//
//type baCache struct {
//	items     *list.List
//	sizeLimit int
//	total     *atomic.Value
//	lock      sync.RWMutex
//	timer     time.Ticker
//}
//
//
//func (b *baCache) Backward(key string) {
//	// 在短时间有大量元素进来时会导致backward。如果定时backward和阈值backward
//	// 我实在不确定这个缓存是否是更有利还是更坏。如果baCache中突然有元素大量访问，那么会不会出现缓存命中率下降呢
//	// 也许adCache、baCache根本就应该不存在
//	if b.items.Len() >= b.sizeLimit {
//		b.lock.Lock()
//		if b.items.Len() >= b.sizeLimit {
//			b.items = list.New()
//		}
//		b.lock.Unlock()
//		return
//	}
//
//
//}

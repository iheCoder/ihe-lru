package k_lru_concurrent

import (
	"container/list"
	"learn/tool/timeCost"
	"sync"
	"time"
)

// 想达到什么效果，提供什么功能？
// 设想同时有多个请求同时访问、添加lru缓存
// 提供线程安全的lru缓存。方案如下
// a. 读锁访问items; 写锁移动最近访问元素到evictList，添加元素
// b. Get、Add全部采用锁
// c. 假设会有大量访问缓存，缓存能够命中的情况，而少量添加情况。
//   c.1 那么对于获取缓存的情况，可以先将能够获取到的返回。而将置于栈顶放到channel，让另一个goroutine去放
//       对于添加缓存，能获取到仿照上述Get流程；可以设定清理阈值和添加限制阈值（前者小于后者），比如如果size大小8个以内便可以清理，而只要不超过size就可以添加
// 		 那么可以在删除时限制先别添加，在添加时限制先别清理（问题在于如果删除和获取真的发生并发冲突，会发生什么呢？如果添加和获取又发生冲突又会发生什么呢？）
// 		 我并不认为删除和获取真的冲突会发生什么，因为这并不涉及到对list.Element删除或改变，只是在之后会被gc回收
// 		 添加也不认为会发生什么有破坏性的影响

// final: concurrent k-lru
// Get: return item directly if exists. new another goroutine to put front ele
// Add: return if exists. add directly if not exceed addLimit, rm if exceed rmLimit
type item struct {
	key   string
	value string
}

type lruConcurrent struct {
	items          map[string]*list.Element
	evictList      *list.List
	size           int
	ch             chan string
	mu             sync.RWMutex
	evictThreshold int
	safeThreshold  int
	evictCh        chan struct{}
}

func NewConcurrentLRU(size int, ch chan string) *lruConcurrent {
	l := &lruConcurrent{
		items:          make(map[string]*list.Element),
		evictList:      list.New(),
		size:           size,
		mu:             sync.RWMutex{},
		evictThreshold: size,
		ch:             ch,
		safeThreshold:  size - size/4,
		evictCh:        make(chan struct{}, 1),
	}
	go l.evict()
	return l
}

func (l *lruConcurrent) Get(key string) (string, bool) {
	defer timeCost.DefaultTimeCostAnalyzer.DeferModuleTimeCost(timeCost.GetItem)()
	// 1. 查看是否在缓存中存在
	l.mu.RLock()
	i, ok := l.items[key]
	l.mu.RUnlock()
	if !ok {
		return "", false
	}

	// 2. 通知将该元素访问次数增加
	// 问题在于被删了之后的元素难道真的应该保留原来的计数嘛
	defer l.notifyPushFront(key)

	// 3. 返回查出来的元素
	// 查出来后被删了，其实也不是什么大问题
	return i.Value.(*item).value, true
}

func (l *lruConcurrent) Add(key, value string) {
	defer timeCost.DefaultTimeCostAnalyzer.DeferModuleTimeCost(timeCost.AddItem)()
	i := &item{
		key:   key,
		value: value,
	}
	// 1. 判断是否存在该key，若存在更新，并将其访问次数加1
	// 到顶还是到底呢？若是底，刚加就删，似乎不是很好。若是顶，是不是会导致最近添加的挤压掉大量实际多次被访问的呢？那就底吧！
	// 这是最近最少访问，在没有最少的情况下，当然以近为先
	// 并不认为将访问items独立锁出去会更好，因为可能查出来被删了，那么即使如此，更新仍然没问题吧（虽然也有被gc回收的风险）
	// 重点在于好处呢？如果假定不会有这么多更新的话，其实该锁的还是要锁
	rn := time.Now()
	l.mu.RLock()
	e, ok := l.items[key]
	l.mu.RUnlock()
	if ok {
		e.Value = i
		return
	}
	// 2. 若元素不存在该key，则添加该元素至栈底，并将其访问次数加1
	now := time.Now()
	l.mu.Lock()
	timeCost.DefaultTimeCostAnalyzer.RecordTimeCost(timeCost.AcquireAddItemLock, now)
	e = l.evictList.PushBack(i)
	l.items[key] = e
	l.mu.Unlock()
	timeCost.DefaultTimeCostAnalyzer.RecordTimeCost(timeCost.RealAddItem, rn)

	// 既然假定Add发生次数并不多，那么为什么不阻塞呢？那这样甚至根本不需要添加阈值
	// 为什么需要删除阈值呢？就是在确保add足够的快。
	// 或许坚定add、get足够快，而对于添加到evict栈顶、删除等后台操作应该滞后
	// 3. 栈大于阈值，清理
	if l.evictList.Len() > l.evictThreshold && len(l.evictCh) == 0 {
		now = time.Now()
		l.evictCh <- struct{}{}
		timeCost.DefaultTimeCostAnalyzer.RecordTimeCost(timeCost.EvictUnusedItem, now)
	}
}

// 我并不认为为items、evictList分别设置锁，是多么明智的选择，基本上对items的修改都涉及到对evictList的修改
// add 太快时有来不及evict风险
func (l *lruConcurrent) evict() {
	for {
		select {
		case <-l.evictCh:
			now := time.Now()
			l.mu.Lock()
			timeCost.DefaultTimeCostAnalyzer.RecordTimeCost(timeCost.AcquireEvictUnusedItemLock, now)

			for l.evictList.Len() > l.safeThreshold {
				// 若为空，那么不就会panic嘛。这怎么会为空呢？
				back := l.evictList.Back()
				l.evictList.Remove(back)
				delete(l.items, back.Value.(*item).key)
			}
			l.mu.Unlock()
			timeCost.DefaultTimeCostAnalyzer.RecordTimeCost(timeCost.EvictUnusedItem, now)
		}
	}
}

// 当访问次数过多时时，通知更新channel便成为巨大的瓶颈，一时间可能有1000倍于chan的访问量，那么update access count 根本来不及处理
// 1. 批量，让其批量更新
// 2. 与其批量更新不如，提升处理的速度
func (l *lruConcurrent) notifyPushFront(key string) {
	//now := time.Now()
	l.ch <- key
	//timeCost.DefaultTimeCostAnalyzer.RecordTimeCost(timeCost.NotifyPushFront, now)
}

func (l *lruConcurrent) MoveToFront(key string) {
	// 放到栈顶的元素可能会被删掉啦
	// concurrent map read and map write
	// 如果先readLock判断是否存在，之后lock移到顶，那么可能会使得一个item被多次移到顶
	// 可是这有问题嘛？可能会已经被删掉，但仍然能够移到栈顶。被删掉的概率其实并不大，所以rlock先判断，没有多大的益处
	l.mu.RLock()
	v, ok := l.items[key]
	l.mu.RUnlock()
	if !ok {
		return
	}
	l.mu.Lock()
	// pushBack 实际会与moveToFront冲突的
	// 看来只能批量处理了，获取一次move一堆
	l.evictList.MoveToFront(v)
	l.mu.Unlock()
}

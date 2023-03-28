package k_lru_concurrent

import (
	"learn/tool/timeCost"
	"sync"
	"time"
)

// 根据channel传过来的访问key，更新访问次数，在次数到达阈值时，将该key置于栈顶
// 还是应该要定期删counts的，如果机器运行足够久的话，小的占用也变成大的啦
// 删的方式如下
// 1. 记录key以及最近更新时间，定期查看该时间是否超出一定周期
// 好处在于在高峰期不会多次删除，并且也能在系统闲时定期删除，与时间高度相关 坏处就是在高峰期导致内存占用较多
// 时间判断还有一个坏处是时间短的时候，可能缓存效果微乎极微，可能会很少；也有可能会很多很多
// 2. 记录key以及id。（该id全局唯一，并持续递增）若最早id和最晚id差值到达阈值，则删除
// 这怎么知道最早id啊，要明白原来的最早也可能会被更新。那就是总数呗！
// 3. 根据id总数判断是否删除，将总数设置的足够大，比如说lru size的两倍
// 进入死循环了啊。难道记录key、id的map不需要回收嘛？只能将key、id、count放到一起啦

type recentUseUpdater struct {
	// k > 1
	k                  int
	ch                 chan string
	acm                *Acm
	moveToFront        func(key string)
	cleanHighThreshold int
	id                 int64
	cleanCh            chan struct{}
	mu                 sync.Mutex
}

func NewRecentUseUpdater(k int, ch chan string, moveToFront func(key string), lowThreshold, highThreshold int) *recentUseUpdater {
	return &recentUseUpdater{
		k:  k,
		ch: ch,
		acm: &Acm{
			counts: make(map[string]*accessCount),
			k:      highThreshold - lowThreshold,
		},
		moveToFront: moveToFront,
		// assume highThreshold > lowThreshold
		cleanHighThreshold: highThreshold,
		cleanCh:            make(chan struct{}, 1),
		mu:                 sync.Mutex{},
	}
}

func (u *recentUseUpdater) Run() {
	go u.update()
	go u.clean()
}

// ?1 如果chan堵塞了怎么办呢？这难道用k-lru不是最好的嘛？k-lru当然也会堵塞。chan堵塞就等待呗
// ?2 删除缓存元素时难道不应该删除counts嘛？当然应该。但通过什么方式知道那些元素已经被删除了呢？难道remover定期提供一个删除调的元素，让updater去删？
// 可是刚刚删除的，可能后面又添加了啊！
// 难道引入时间，将一定时间没有得到更新的全部删除。或者引入个数limit，将个数limit之外的全部定期删除
// map可没有之后不之后的概念啊？难道全部遍历一个个删掉
// 暂时先不要管这个删除了，在假定key很小的情况下，这就是数量级达到一定程度也就那样

// 如果改成一次获取大量chan元素，可能会导致特定情况下到达批量时间过长。而且我想不到批量真的能够提升很大的速度嘛？除了Lock外其他很难说很快
func (u *recentUseUpdater) update() {
	for key := range u.ch {
		now := time.Now()
		u.mu.Lock()
		timeCost.DefaultTimeCostAnalyzer.RecordTimeCost(timeCost.AcquireUpdateAccessCountLock, now)
		u.id++
		// 1. 不存在key，新建
		if _, ok := u.acm.Get(key); !ok {
			ac := &accessCount{
				count: 1,
				id:    u.id,
			}
			u.acm.Set(key, ac)
			u.mu.Unlock()
			timeCost.DefaultTimeCostAnalyzer.RecordTimeCost(timeCost.UpdateAccessCount, now)
			continue
		}

		if u.acm.CountLen() > u.cleanHighThreshold && len(u.cleanCh) == 0 {
			u.cleanCh <- struct{}{}
		}

		// 2. 更新访问次数
		na := time.Now()
		u.acm.IncreaseAccessCount(key)
		u.acm.SetID(key, u.id)

		// 3. 若访问次数达到阈值，置于栈顶
		// 置于栈顶后应该重置为0
		if u.acm.GetAccessCount(key) > u.k {
			u.acm.ResetAccessCount(key)
			u.moveToFront(key)
		}
		u.mu.Unlock()
		timeCost.DefaultTimeCostAnalyzer.RecordTimeCost(timeCost.AddAccessCount, na)
		timeCost.DefaultTimeCostAnalyzer.RecordTimeCost(timeCost.UpdateAccessCount, now)
	}
}

func (u *recentUseUpdater) clean() {
	for {
		select {
		case <-u.cleanCh:
			now := time.Now()
			u.mu.Lock()
			ks := u.acm.TopKByQuickSort()
			for _, k := range ks {
				u.acm.Delete(k.key)
			}
			u.mu.Unlock()
			timeCost.DefaultTimeCostAnalyzer.RecordTimeCost(timeCost.CleanAccessCount, now)
		}
	}
}

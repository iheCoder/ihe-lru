package ihelfu

import (
	"learn/algo/count_min_sketch"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultWinCleanDuration    = time.Millisecond
	defaultEdenCleanDuration   = 10 * time.Millisecond
	defaultEvictUpdateDuration = 10 * time.Millisecond
	defaultDelayDuration       = time.Minute

	defaultWinSafeRatio  = 0.5
	defaultEdenSafeRatio = 0.7

	winAdvanceRatio   = 1
	edenBackwardRatio = 1
	evictAdvanceRatio = 1.2

	winRatio   = 0.2
	edenRatio  = 0.7
	evictRatio = 0.1

	defaultEpsilon = 0.01
	defaultDelta   = 0.01

	defaultEvictPercentage = 3
)

var (
	total      = int64(0)
	totalCount = int64(0)
)

type iheEvict struct {
	cms          *count_min_sketch.CMSBitVersion
	delay2Ticker *time.Ticker

	winCleanTicker       *time.Ticker
	WinCleanCh           chan struct{}
	winCleanLock         *sync.Mutex
	winLock              *sync.RWMutex
	winSafeSizeThreshold int
	winAdvanceRatio      float64
	winZone              *circularArray

	edenTimeout           time.Duration
	edenCleanTicker       *time.Ticker
	edenCleanCh           chan struct{}
	edenCleanLock         *sync.Mutex
	edenSafeSizeThreshold int
	edenBackwardRatio     float64
	edenZone              *ConcurrentSegTable

	evictUpdateTicker *time.Ticker
	evictAdvanceRatio float64
	evictZone         *ConcurrentSegTable
	evictNotify       chan []string
}

// size should not close int limit
func NewIheEvict(size int64, en chan []string) (*iheEvict, error) {
	cms, err := count_min_sketch.NewCMSBitVersion(defaultEpsilon, defaultDelta)
	if err != nil {
		return nil, err
	}

	winSize := int(float64(size) * winRatio)
	edenSize := int64(float64(size) * edenRatio)
	evictSize := int64(float64(size) * evictRatio)

	ie := &iheEvict{
		cms:          cms,
		delay2Ticker: time.NewTicker(defaultDelayDuration),

		winCleanTicker:       time.NewTicker(defaultWinCleanDuration),
		WinCleanCh:           make(chan struct{}, 1),
		winCleanLock:         &sync.Mutex{},
		winLock:              &sync.RWMutex{},
		winSafeSizeThreshold: int(float64(winSize) * defaultWinSafeRatio),
		winAdvanceRatio:      winAdvanceRatio,
		winZone:              NewCircularArray(winSize),

		edenCleanTicker:       time.NewTicker(defaultEdenCleanDuration),
		edenCleanCh:           make(chan struct{}, 1),
		edenCleanLock:         &sync.Mutex{},
		edenSafeSizeThreshold: int(float64(edenSize) * defaultEdenSafeRatio),
		edenBackwardRatio:     edenBackwardRatio,

		evictAdvanceRatio: evictAdvanceRatio,
		evictNotify:       en,
		evictUpdateTicker: time.NewTicker(defaultEvictUpdateDuration),
	}

	edl := getCSTLimit(edenSize)
	evl := getCSTLimit(evictSize)
	edenZone := NewConcurrentSegTable(edl, defaultSegLen, ie.checkUnderEden)
	evictZone := NewConcurrentSegTable(evl, defaultSegLen, ie.checkReachEvict)
	ie.edenZone = edenZone
	ie.evictZone = evictZone

	go ie.cleanWinZonePeriodically()
	go ie.cleanEdenZonePeriodically()
	go ie.cleanWindowZone()
	go ie.cleanEdenZone()

	return ie, nil
}

func getCSTLimit(size int64) int {
	x := int64(math.Ceil(float64(size) / float64(defaultSegLen)))
	if x < 2 {
		return 64
	}
	l := x - 1
	limit := 0
	for l != 0 {
		l >>= 6
		limit++
	}
	if limit >= 11 {
		panic("too large size")
	}

	return limit * 64
}

// 先一个一个更新吧，将框架先搭起来
func (i *iheEvict) UpdateAccessCount(key string) bool {
	// not exists, insert window zone
	if i.cms.Estimate(key) == 0 {
		i.cms.Update(key, 1)
		if !i.winZone.IsFull() {
			i.winLock.Lock()
			if !i.winZone.IsFull() {
				i.winZone.Append(key)
				i.winLock.Unlock()
				return true
			}

			// 如果真的这么倒霉碰到了满的情况，直接丢弃也许问题不大，毕竟下次还会再来
			i.winLock.Unlock()
			return false
		}
		i.notifyCleanWinZone()
	}

	// exists, just update
	i.cms.Update(key, 1)
	return true
}

func (i *iheEvict) cleanWinZonePeriodically() {
	for range i.winCleanTicker.C {
		i.notifyCleanWinZone()
	}
}

func (i *iheEvict) delay2Periodically() {
	for range i.delay2Ticker.C {
		i.cms.Delay2()
	}
}

func (i *iheEvict) notifyCleanWinZone() {
	// 我觉得这里也是可能导致堵塞的原因，在高并发下，这种check完全不靠谱
	// 双重锁似乎没有任何用
	if len(i.WinCleanCh) == 0 {
		i.winCleanLock.Lock()
		if len(i.WinCleanCh) == 0 {
			i.WinCleanCh <- struct{}{}
		}
		i.winCleanLock.Unlock()
	}
}

func (i *iheEvict) cleanWindowZone() {
	for range i.WinCleanCh {
		n := i.winZone.Size() - i.winSafeSizeThreshold
		if n > 0 {
			i.winLock.Lock()
			arr := i.winZone.Remove(n)
			i.winLock.Unlock()
			// 如果真是0/1，那似乎也没什么问题啊。但1/10有问题
			// 怎么能除以0呢？？？
			var t float64
			count := atomic.LoadInt64(&totalCount)
			if count != 0 {
				t = float64(atomic.LoadInt64(&total)) / float64(atomic.LoadInt64(&totalCount)) * i.winAdvanceRatio
			}
			var sc float64
			for _, s := range arr {
				sc = float64(i.cms.Estimate(s))
				if sc > t {
					// move to eden
					i.advanceIntoEden(s)
				} else {
					// move to evict
					i.backwardIntoEvict(s)
				}
			}
		}
	}
}

func (i *iheEvict) updateEvictZone() {
	for range i.evictUpdateTicker.C {
		i.evictZone.Clean()
	}
}

func (i *iheEvict) advanceIntoEden(key string) {
	atomic.AddInt64(&total, int64(i.cms.Estimate(key)))
	atomic.AddInt64(&totalCount, 1)

	err := i.edenZone.Add(key)
	if err == nil {
		return
	}
	// 如果想要移到eden区，可是eden满了，该怎么办呢？
	// 暂时返回nil，相当于直接丢弃
	// 应该同时等待不为full，
	// handle full
	i.notifyCleanEdenZone()

	// 真这么倒霉就全部丢弃吧
	i.evictNotify <- []string{key}
}

func (i *iheEvict) backwardIntoEvict(key string) {
	if i.evictZone.IsFull() {
		r := i.evictZone.Reset()
		i.evictNotify <- r
	}
	err := i.evictZone.Add(key)
	if err == ErrFull {
		r := i.evictZone.Reset()
		i.evictNotify <- r
	}
}

func (i *iheEvict) cleanEdenZonePeriodically() {
	for range i.edenCleanTicker.C {
		i.notifyCleanEdenZone()
	}
}

func (i *iheEvict) notifyCleanEdenZone() {
	if len(i.edenCleanCh) == 0 {
		i.edenCleanLock.Lock()
		if len(i.edenCleanCh) == 0 {
			i.edenCleanCh <- struct{}{}
		}
		i.edenCleanLock.Unlock()
	}
}

func (i *iheEvict) cleanEdenZone() {
	for range i.edenCleanCh {
		i.edenZone.Clean()
	}
}

func (i *iheEvict) checkUnderEden(key string) bool {
	c := float64(i.cms.Estimate(key))
	count := float64(atomic.LoadInt64(&totalCount))
	var threshold float64
	if count != 0 {
		threshold = float64(atomic.LoadInt64(&total)) / count * i.edenBackwardRatio
	}

	if c < threshold {
		// move to evict
		atomic.AddInt64(&total, int64(-(i.cms.Estimate(key))))
		atomic.AddInt64(&totalCount, -1)
		i.backwardIntoEvict(key)
		return true
	}
	return false
}

func (i *iheEvict) checkReachEvict(key string) bool {
	c := float64(i.cms.Estimate(key))
	var threshold float64
	count := float64(atomic.LoadInt64(&totalCount))
	if count == 0 {
		threshold = 1
	} else {
		threshold = float64(atomic.LoadInt64(&total)) / count * i.evictAdvanceRatio
	}
	if c > threshold {
		// move to eden
		i.advanceIntoEden(key)
		return true
	} else {
		rand.Seed(time.Now().UnixNano())
		x := rand.Intn(defaultEvictPercentage)
		// 如果等于0，那么自然应该删除。可是在其他情况下应该如何删除呢？随机？阈值？时间？
		if c == 0 || x == 1 {
			i.evictNotify <- []string{key}
		}
	}
	return false
}

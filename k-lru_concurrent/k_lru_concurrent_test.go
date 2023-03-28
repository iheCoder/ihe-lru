package k_lru_concurrent

import (
	"fmt"
	"learn/ihe-lru"
	"learn/tool/timeCost"
	"math/rand"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestBasicUseKLru(t *testing.T) {
	size := 2
	ch := make(chan string, size/2)
	l := NewConcurrentLRU(size, ch)
	u := NewRecentUseUpdater(2, ch, l.MoveToFront, size*5, size*10)
	go u.Run()
	// get un-exists item
	v, ok := l.Get("hello")
	if ok {
		t.Fatal("should no match item")
	}

	// add hello world
	l.Add("hello", "world")

	// get exists
	v, ok = l.Get("hello")
	if !ok {
		t.Fatal("should has hello")
	}

	// add exceeded item to ensure evict fine
	l.Add("good", "kangkang")
	l.Add("very", "well")
	v, ok = l.Get("hello")
	if ok {
		t.Fatal("should has no hello")
	}

	// update good to microsoft
	l.Add("good", "microsoft")
	v, ok = l.Get("good")
	if !ok || v != "microsoft" {
		t.Fatal("update failed")
	}

	// access good will result top good
	l.Get("good")
	l.Get("good")
	if l.evictList.Front().Value.(*item).key != "good" {
		t.Fatal("should good top")
	}
}

// 1 BenchmarkWillPanicIfLotsAccess-8   	  622500	      2772 ns/opType
// 2 BenchmarkWillPanicIfLotsAccess-8   	  543459	      3768 ns/opType 当我添加rlock去事先判断是否被删了，然后lock再取，再移到顶部可是效率反而更慢啦
// 3 BenchmarkWillPanicIfLotsAccess-8   	  501883	      2700 ns/opType 而当我rLock判断是否存在，而后lock移到顶，效率稍微改良那么一点
// 4 BenchmarkWillPanicIfLotsAccess-8   	  579994	      3010 ns/opType 当添加了删除counts的时候就变得如此之慢了(update count加锁，删除也会加锁)，即使只有size*10的时候才会delete
// 5 BenchmarkWillPanicIfLotsAccess-8   	  505783	      3385 ns/opType 居然更大了，当我删除了lock之后，采用超过限制同步删除方式
// 6 BenchmarkWillPanicIfLotsAccess-8   	  476786	      2768 ns/opType 基于4，当将update count的chan长度从size/2 -> size*3，效率回到了3
func BenchmarkWillPanicIfLotsAccess(b *testing.B) {
	size := 5
	ch := make(chan string, size*3)
	l := NewConcurrentLRU(size, ch)
	u := NewRecentUseUpdater(2, ch, l.MoveToFront, size*5, size*10)
	go u.Run()

	//wg := &sync.WaitGroup{}
	//wg.Add(b.N)
	for i := 0; i < b.N; i++ {
		go accessKLru(l)
	}
	//
	//wg.Wait()
	//DefaultTimeCostAnalyzer.OutputResult()
}

const times = 1000000

// module: EvictUnusedItem cost: 42m25.095401144s
// module: UpdateAccessCount cost: 527.232291ms
// module: AddItem cost: 2h40m16.484104279s
// module: GetItem cost: 357.456085ms
// 令我感到意外的是完全没有出现cleanAccessCount的情况出现，我猜测可能是设置的counts限制太大了，已经是size的10倍，不太可能达到这个数量级
// 耗时最大的是addItem高达2小时40分钟，次之是evict，高达42m
// 耗时最小的是getItem，仅仅只有357ms，次之是更新accessCount
// 如果将getItem理解为1s的话，那么accessCount就是2s，evict就是2小时，而addItem就是7.6小时
// AddItem, evict实在不知道怎么优化

// module: AcquireAddItemLock cost: 5h4m8.929607165s
// module: RealAddItem cost: 53.767667ms
// module: AddItem cost: 5h4m9.654936771s
// module: GetItem cost: 325.739612ms
// module: AcquireEvictUnusedItemLock cost: 1h8m55.032245518s
// module: RealEvictUnusedItem cost: 20.173374ms
// module: EvictUnusedItem cost: 1h8m55.36512631s
// module: UpdateAccessCount cost: 584.19573ms
// 显然addItem总时间5h4m9.654936771s，可获取锁的时间高达5h4m8.929607165s，也就是几乎所有时间都耗费在获取锁上
// evict也是如此，也可以认为如果getItem没有获取读锁的时间，可能会更快
// 瓶颈在于map与evictList冲突可能性太大了，同一时间只能有一个访问map或者evictList
// 场景是读远大于写，那么rwLock+map已经是读性能最高的情况啦。map无从改起。因为evictList和map是一起改的，于是evictList也不好改
// 我并不认为读有多么远大于写，假设概率为万分之一，add 42m6s，get 123微秒

// module: RealEvictUnusedItem cost: 14.32118ms
// module: EvictUnusedItem cost: 55m41.964662562s
// module: AcquireAddItemLock cost: 4h11m26.903226964s
// module: RealAddItem cost: 89.395218ms
// module: AddItem cost: 4h11m27.402305783s
// module: GetItem cost: 420.405912ms
// module: UpdateAccessCount cost: 526.10756ms
// module: AcquireEvictUnusedItemLock cost: 55m41.801828251s
// 将清理从高区间一次清理到低区间，确实比之前acquire lock快了，但get item同样慢了100ms，这似乎并不划得来

// module: AcquireEvictUnusedItemLock cost: 1h12m23.268515283s
// module: RealEvictUnusedItem cost: 17.935037ms
// module: EvictUnusedItem cost: 1h12m23.829983047s
// module: AcquireAddItemLock cost: 4h13m9.767082868s
// module: RealAddItem cost: 82.731467ms
// module: AddItem cost: 4h13m11.571224939s
// module: GetItem cost: 318.357604ms
// module: UpdateAccessCount cost: 560.509773ms
// 当我将高区间设定为size，低区间设定为75%，次数GetItem甚至比高了75%就清理到低于75%的情况更快，比75%-50%Acquire addItem lock 相差无几，就是acquire evict lock比其他需要时间都更长一些
// 综合来说100%-75%是最合适的，add get都与上面两个最好成绩接近，甚至更好

//  chan默认是没有缓存，所以如果不put的话，永远是0
//	module: RealAddItem cost: 70.271688ms
//	module: GetItem cost: 261.436112ms
//	module: AcquireEvictUnusedItemLock cost: 1h3m18.468266022s
//	module: RealEvictUnusedItem cost: 17.003046ms
//	module: TopKByQuickSort cost: 477.542µs
//	module: CleanAccessCount cost: 515.459443ms
//	module: AcquireAddItemLock cost: 4h2m28.182788299s
//	module: AddItem cost: 4h2m28.516863981s
//	module: UpdateAccessCount cost: 515.99181ms
//	module: EvictUnusedItem cost: 1h3m18.646605254s

// 能rlock就rlock
//module: TopKByQuickSort cost: 8.928493ms
//module: AcquireAddItemLock cost: 26m58.51948119s
//module: AddItem cost: 27m52.598192031s
//module: AcquireUpdateAccessCountLock cost: 11.380461ms
//module: RealEvictUnusedItem cost: 7.696901ms
//module: UpdateAccessCount cost: 398.181982ms
//module: AddAccessCount cost: 336.331749ms
//module: CleanAccessCount cost: 396.613925ms
//module: RealAddItem cost: 27m12.130106216s
//module: GetItem cost: 5.895315817s
//module: AcquireEvictUnusedItemLock cost: 390.28537ms
func TestCostTime(t *testing.T) {
	size := 5
	ch := make(chan string, size*3)
	l := NewConcurrentLRU(size, ch)
	u := NewRecentUseUpdater(2, ch, l.MoveToFront, size*2, size*5)
	go u.Run()

	wg := &sync.WaitGroup{}
	wg.Add(times)
	for i := 0; i < times; i++ {
		go accessKLruWithWG(l, wg)
	}

	wg.Wait()
	timeCost.DefaultTimeCostAnalyzer.OutputResult()
}

func TestMgrCostTime(t *testing.T) {
	size := 5
	ch := make(chan string, size*3)
	l := NewCLRU(size, ch)
	u := NewRecentUseUpdater(2, ch, l.MoveToFront, size*2, size*5)
	go u.Run()

	wg := &sync.WaitGroup{}
	wg.Add(times)
	for i := 0; i < times; i++ {
		go accessKLruWithWG(l, wg)
	}

	wg.Wait()
	timeCost.DefaultTimeCostAnalyzer.OutputResult()
}

var mismatchCount int
var totalCount = times
var mu = &sync.Mutex{}

// init miss rate 54 394/100 000
// fre 90% miss rate 23 957/100 000, fre 80% miss rate 31960, 27340, 33775
// 22539, 18568, 17527, 20499 push back add item

//module: EvictUnusedItem cost: 2.497746678s
//module: UpdateAccessCount cost: 246.004515ms
//module: AcquireAddItemLock cost: 25.857451965s
//module: RealAddItem cost: 6.349838ms
//module: AddItem cost: 25.993634267s
//module: GetItem cost: 28.602852859s
//module: AcquireEvictUnusedItemLock cost: 2.470828843s
//module: RealEvictUnusedItem cost: 3.063365ms
//miss rate 17962--- PASS: TestMissRate (0.27s)

//module: AcquireAddItemLock cost: 14.549312307s
//module: RealAddItem cost: 3.787659ms
//module: AddItem cost: 14.625897864s
//module: GetItem cost: 16.342150924s
//module: AcquireEvictUnusedItemLock cost: 137.191023ms
//module: RealEvictUnusedItem cost: 3.245067ms
//module: UpdateAccessCount cost: 190.14352ms
//miss rate 12907--- PASS: TestMissRate (0.21s)

// 在rlock pushBack, moveToFront之后
//module: AcquireAddItemLock cost: 4.085149ms
//module: UpdateAccessCount cost: 55.571772ms
//module: AddAccessCount cost: 11.261693ms
//module: RealEvictUnusedItem cost: 61.244µs
//module: RealAddItem cost: 4.530342ms
//module: AddItem cost: 12.980582ms
//module: AcquireUpdateAccessCountLock cost: 3.447716ms
//module: GetItem cost: 1.143711746s
//module: AcquireEvictUnusedItemLock cost: 487.641µs

// 为什么上面的能达到AddItem毫秒啊？别说是缓存吧！pushBack, moveToFront会冲突啊
//module: RealAddItem cost: 12.437650552s
//module: UpdateAccessCount cost: 217.227169ms
//module: AddAccessCount cost: 201.176997ms
//module: AcquireAddItemLock cost: 11.668278547s
//module: AcquireUpdateAccessCountLock cost: 1.438374ms
//module: GetItem cost: 21.275062602s
//module: EvictUnusedItem cost: 174.73768ms
//module: AcquireEvictUnusedItemLock cost: 166.901946ms
//module: AddItem cost: 14.41165827s
//miss rate 22171--- PASS: TestMissRate (0.24s)
var rate = 80
var updateCountChRate = 10
var size = 5
var k = 2
var lowThresholdRate = 2
var hightThresholdRate = 5

// 去掉所有导致无尽goroutine后
//module: UpdateAccessCount cost: 251.234275ms
//module: GetItem cost: 2h11m57.343069207s
//module: AcquireEvictUnusedItemLock cost: 107.418638ms
//module: AcquireUpdateAccessCountLock cost: 7.622123ms
//module: AddAccessCount cost: 184.406418ms
//module: EvictUnusedItem cost: 116.663609ms
//module: AcquireAddItemLock cost: 4.725177454s
//module: RealAddItem cost: 5.010907893s
//module: AddItem cost: 5.361781372s
//miss rate 28432--- PASS: TestMissRate (0.32s)
// === RUN   TestMissRate
//module: AddItem cost: 15.217945783s
//module: GetItem cost: 2h9m31.712808302s
//module: EvictUnusedItem cost: 156.951997ms
//module: AcquireUpdateAccessCountLock cost: 6.166269ms
//module: AddAccessCount cost: 180.202979ms
//module: AcquireAddItemLock cost: 12.253989801s
//module: RealAddItem cost: 13.310629625s
//module: AcquireEvictUnusedItemLock cost: 119.647403ms
//module: UpdateAccessCount cost: 251.327833ms
//miss rate 26094--- PASS: TestMissRate (0.32s)

// 去掉add、update通知更新count 总时间快了一些，准确率未下降，甚至可能更好
// module: AcquireAddItemLock cost: 5.886743597s
//module: AddItem cost: 6.487580638s
//module: GetItem cost: 2h18m36.811779903s
//module: AcquireUpdateAccessCountLock cost: 3.970871ms
//module: AcquireEvictUnusedItemLock cost: 137.024565ms
//module: RealAddItem cost: 6.214382568s
//module: UpdateAccessCount cost: 224.212322ms
//module: AddAccessCount cost: 176.644401ms
//module: EvictUnusedItem cost: 144.897955ms
//miss rate 23611--- PASS: TestMissRate (0.27s)
func TestMissRate(t *testing.T) {
	ch := make(chan string, size*updateCountChRate)
	l := NewConcurrentLRU(size, ch)
	u := NewRecentUseUpdater(k, ch, l.MoveToFront, size*lowThresholdRate, size*hightThresholdRate)
	go u.Run()

	initM := genFreqKeyValues(size, rate)
	for k, v := range initM {
		l.Add(k, v)
	}

	wg := &sync.WaitGroup{}
	wg.Add(times)

	var ms runtime.MemStats
	for i := 0; i < times; i++ {
		go accessKLRUWithMissRate(l, wg)
		if i%1000 == 0 {
			runtime.ReadMemStats(&ms)
			fmt.Printf("mem alloc %d \n", byteToMB(ms.Alloc))
		}
	}

	wg.Wait()
	go printMemStatRepeat(ms)
	time.Sleep(10 * time.Second)
	timeCost.DefaultTimeCostAnalyzer.OutputResult()
	fmt.Printf("miss rate %d", mismatchCount)
}

func printMemStatRepeat(ms runtime.MemStats) {
	tk := time.NewTicker(time.Second)
	for range tk.C {
		runtime.ReadMemStats(&ms)
		fmt.Printf("mem alloc %d \n", byteToMB(ms.Alloc))
	}
}

func byteToMB(bs uint64) uint64 {
	return bs / 1024 / 1024
}

//module: AddItem cost: 11m26.439897571s
//module: GetItem cost: 20.361539001s
//module: AcquireUpdateAccessCountLock cost: 4.064429ms
//module: UpdateAccessCount cost: 138.173077ms
//module: AddAccessCount cost: 75.11472ms
//module: EvictUnusedItem cost: 196.795138ms
//module: NotifyPushFront cost: 4m44.752809494s
//miss rate 26834--- PASS: TestMgrMissRate (0.23s)
// mgr 没有任何优势. miss rate、get item、evict差不多，可是addItem却更慢了，而且同样有goroutine泄露风险

// 在添加timeCost rlock以及去掉所有可能导致无尽goroutine之后
// 一夜回到最开始
//module: AddItem cost: 1h24m21.794910484s
//module: AcquireUpdateAccessCountLock cost: 7.380522ms
//module: UpdateAccessCount cost: 306.869252ms
//module: GetItem cost: 11.829896919s
//module: AddAccessCount cost: 219.928464ms
//module: NotifyPushFront cost: 186.613111ms
//module: EvictUnusedItem cost: 125.426584ms
//miss rate 28955--- PASS: TestMgrMissRate (0.40s)

// 去掉add、update通知更新count 总时间快了
// module: AcquireUpdateAccessCountLock cost: 4.695015ms
//module: UpdateAccessCount cost: 234.111062ms
//module: AddAccessCount cost: 184.772342ms
//module: NotifyPushFront cost: 165.74096ms
//module: AddItem cost: 56.697821632s
//module: GetItem cost: 2h23m2.489063715s
//module: EvictUnusedItem cost: 110.952975ms
//miss rate 26745--- PASS: TestMgrMissRate (0.29s)
// 两者变得差不多啦。lock方式add更快，mgr evict更快点
// 令我奇怪的是内存占用居然差不多，mgr 最高38M， lock最高 39M。 实际表现差不多。最后10s全部稳定在40M
func TestMgrMissRate(t *testing.T) {
	ch := make(chan string, size*updateCountChRate)
	l := NewCLRU(size, ch)
	u := NewRecentUseUpdater(k, ch, l.MoveToFront, size*lowThresholdRate, size*hightThresholdRate)
	go u.Run()

	initM := genFreqKeyValues(size, rate)
	for k, v := range initM {
		l.Add(k, v)
	}

	wg := &sync.WaitGroup{}
	wg.Add(times)

	var ms runtime.MemStats
	for i := 0; i < times; i++ {
		go accessKLRUWithMissRate(l, wg)
		if i%1000 == 0 {
			runtime.ReadMemStats(&ms)
			fmt.Printf("mem alloc %d \n", byteToMB(ms.Alloc))
		}
	}

	wg.Wait()
	go printMemStatRepeat(ms)
	time.Sleep(10 * time.Second)
	timeCost.DefaultTimeCostAnalyzer.OutputResult()
	fmt.Printf("miss rate %d", mismatchCount)
}

var value = 0

func accessKLru(l *lruConcurrent) {
	l.Add(generateRandomFixedSizeString(3), strconv.Itoa(value))
	l.Get(generateRandomFixedSizeString(3))
	value++
}

func accessKLruWithWG(l ihe_lru.LRU, wg *sync.WaitGroup) {
	l.Add(generateRandomFixedSizeString(3), strconv.Itoa(value))
	l.Get(generateRandomFixedSizeString(3))
	value++
	wg.Done()
}

func accessKLRUWithMissRate(l ihe_lru.LRU, wg *sync.WaitGroup) {
	if rand.Intn(10)%9 == 1 {
		l.Add(generateFrequentRandomString(1, rate), strconv.Itoa(value))
	}
	x := generateFrequentRandomString(1, rate)
	_, ok := l.Get(x)
	if !ok {
		mu.Lock()
		mismatchCount++
		mu.Unlock()
		l.Add(x, strconv.Itoa(value))
	}
	value++
	wg.Done()
}

func accessKLRUWithMissRateWithoutWg(l ihe_lru.LRU) {
	if rand.Intn(10)%9 == 1 {
		l.Add(generateFrequentRandomString(1, rate), strconv.Itoa(value))
	}
	x := generateFrequentRandomString(1, rate)
	_, ok := l.Get(x)
	if !ok {
		mu.Lock()
		mismatchCount++
		mu.Unlock()
		l.Add(x, strconv.Itoa(value))
	}
	value++
}

func genKeyValues(size int) map[string]string {
	v := 0
	m := make(map[string]string)
	for i := 0; i < size; i++ {
		m[generateRandomFixedSizeString(1)] = strconv.Itoa(v)
		v++
	}
	return m
}

func genFreqKeyValues(size int, rate int) map[string]string {
	v := 0
	m := make(map[string]string)
	for i := 0; i < size; i++ {
		m[generateFrequentRandomString(1, rate)] = strconv.Itoa(v)
		v++
	}
	return m
}

var (
	section         = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n"}
	n               = len(section)
	frequentSection = []string{"a", "b", "c", "d", "e"}
	fn              = len(frequentSection)
)

func generateRandomFixedSizeString(size int) string {
	r := ""
	for i := 0; i < size; i++ {
		r += section[rand.Intn(n)]
	}
	return r
}

func generateFrequentRandomString(size int, rate int) string {
	r := ""
	for i := 0; i < size; i++ {
		x := rand.Intn(100)
		if x < rate {
			r += frequentSection[rand.Intn(fn)]
		} else {
			r += section[rand.Intn(n)]
		}
	}
	return r
}

func TestGoroutineCost(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(times * 2)
	for i := 0; i < times*2; i++ {
		go doMeaningless(wg)
	}
	wg.Wait()
}

func doMeaningless(wg *sync.WaitGroup) {
	m := make(map[string]string)
	m["hello"] = "world"
	wg.Done()
}

// 在没有多线程情况下，benchmark毫无意义。但benchmark能测多线程嘛？
//func BenchmarkMgrWrite(b *testing.B) {
//	ch := make(chan string, size*updateCountChRate)
//	l := NewCLRU(size, ch)
//	u := NewRecentUseUpdater(k, ch, l.MoveToFront, size*lowThresholdRate, size*hightThresholdRate)
//	go u.NewIrpcClient()
//
//	initM := genFreqKeyValues(size, rate)
//	for k, v := range initM {
//		l.Add(k, v)
//	}
//	for i := 0; i < b.N; i++ {
//		accessKLRUWithMissRate(l, wg)
//	}
//
//
//
//	timeCost.DefaultTimeCostAnalyzer.OutputResult()
//	fmt.Printf("miss rate %d", mismatchCount)
//
//}

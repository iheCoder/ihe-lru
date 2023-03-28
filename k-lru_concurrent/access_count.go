package k_lru_concurrent

import "learn/tool/timeCost"

// top-K: 要找到一定区间内小id。
// 1. 堆很棒。删除时相对来说比较快，可是会稍微耽误一点更新时间
// 2. 快排。可
type accessCount struct {
	count int
	id    int64
}

type Acm struct {
	counts map[string]*accessCount
	k      int
}

type compareItem struct {
	id  int64
	key string
}

func (m *Acm) CountLen() int {
	return len(m.counts)
}

func (m *Acm) Get(key string) (ac *accessCount, ok bool) {
	ac, ok = m.counts[key]
	return
}

func (m *Acm) Set(key string, value *accessCount) {
	m.counts[key] = value
}

func (m *Acm) IncreaseAccessCount(key string) {
	m.counts[key].count++
}

func (m *Acm) ResetAccessCount(key string) {
	m.counts[key].count = 0
}

func (m *Acm) SetID(key string, id int64) {
	m.counts[key].id = id
}

func (m *Acm) GetAccessCount(key string) int {
	return m.counts[key].count
}

func (m *Acm) Delete(key string) {
	delete(m.counts, key)
}

func (m *Acm) TopKByQuickSort() []*compareItem {
	defer timeCost.DefaultTimeCostAnalyzer.DeferModuleTimeCost(timeCost.TopKByQuickSort)()
	l := len(m.counts)
	if l <= m.k {
		return nil
	}

	// 1. convert map to list
	results := make([]*compareItem, l)
	var i int
	for s, count := range m.counts {
		results[i] = &compareItem{
			id:  count.id,
			key: s,
		}
		i++
	}

	left := 0
	right := l
	// 2. 将小于首个元素置于左边，大于的置于右边
	index := m.partition(results, left, right)

	for index != m.k {
		// 3. 若选定值位置小于k，让左边界值加1，再进行partition
		if index < m.k {
			left = index + 1
		} else {
			// 4. 若选定值位置大于k，让右边界值大于该index，再进行partition
			right = index
		}
		index = m.partition(results, left, right)
	}

	return results[:m.k]
}

func (m *Acm) partition(items []*compareItem, left int, right int) int {
	pivot := items[left].id
	pos := left

	// 从左往右遍历数组，若元素值小于选定值，则交换到左边
	for i := left; i < right; i++ {
		if items[i].id < pivot {
			pos++
			items[i], items[pos] = items[pos], items[i]
		}
	}

	items[pos], items[left] = items[left], items[pos]
	return pos
}

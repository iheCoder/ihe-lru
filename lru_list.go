package ihe_lru

import "container/list"

// 想达到什么效果，提供什么功能？
// 设想这样的场景，首先从缓存中查，若缓存存在，则将该key置于驱逐栈顶（也就是最后删除）。若不存在，从其他数据源查询，并加入到缓存
// 提供一个固定大小的lru缓存，能够添加缓存（添加的缓存视为最近使用的，驱逐栈溢出时，溢出栈底元素），访问缓存（访问后的置于驱逐栈顶），
type item struct {
	key   string
	value string
}

type lru struct {
	// 难道真的要将key、value全部放到element嘛。(关键在于删除map元素操作需要key，可是list中没有)这也就意味着即使是自己实现的evictList也必须要为element添加key、且value
	// 那能不能element只添加key呢？那value储存在哪里呢？
	items     map[string]*list.Element
	evictList *list.List
	size      int
}

func NewGeneralLRU(size int) LRU {
	return &lru{
		items:     make(map[string]*list.Element),
		evictList: list.New(),
		size:      size,
	}
}

func (l *lru) Get(key string) (string, bool) {
	// 1. 查看是否在缓存中存在
	i, ok := l.items[key]
	if !ok {
		return "", false
	}

	// 2. 对于存在元素置于evict栈顶
	l.evictList.MoveToFront(i)

	// 3. 返回查出来的元素
	return i.Value.(*item).value, true
}

func (l *lru) Add(key, value string) {
	// 1. 若添加元素在里面，则置于栈顶。
	_, exists := l.Get(key)
	if exists {
		return
	}

	// 2. 若添加元素不在，且大小达到限制，则删除栈底元素
	if l.evictList.Len() >= l.size {
		back := l.evictList.Back()
		l.evictList.Remove(back)
		delete(l.items, back.Value.(*item).key)
	}

	// 3. 添加该元素并置于栈顶
	i := &item{
		key:   key,
		value: value,
	}
	e := l.evictList.PushFront(i)
	l.items[key] = e
}

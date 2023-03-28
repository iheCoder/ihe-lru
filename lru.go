package ihe_lru

type LRU interface {
	// Get 返回key对应lru内容，以及是否存在
	Get(key string) (string, bool)

	// Add 添加lru内容
	Add(key, value string)
}

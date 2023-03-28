package ihelfu

type circularArray struct {
	size  int
	items []string
	// head指向第一个，tail指向最后一个的后一个
	head int
	tail int
}

func NewCircularArray(size int) *circularArray {
	return &circularArray{
		size:  size,
		items: make([]string, size),
		head:  0,
		tail:  0,
	}
}

func (c *circularArray) Reset() {
	c.head = 0
	c.tail = 0
	c.items = make([]string, c.size)
}

func (c *circularArray) IsFull() bool {
	return c.tail-c.head >= c.size
}

func (c *circularArray) IsEmpty() bool {
	return c.tail == c.head
}

func (c *circularArray) Size() int {
	return c.tail - c.head
}

// Append check if full before append
func (c *circularArray) Append(key string) {
	c.items[c.tail%c.size] = key
	c.tail++
}

func (c *circularArray) Remove(n int) []string {
	if n > c.tail-c.head {
		n = c.tail - c.head
	}
	start := c.head % c.size
	end := start + n

	c.head += n
	a := make([]string, n)

	if end <= c.size {
		copy(a, c.items[start:start+n])
	} else {
		copy(a, append(c.items[start:], c.items[:end-c.size]...))
	}
	return a
}

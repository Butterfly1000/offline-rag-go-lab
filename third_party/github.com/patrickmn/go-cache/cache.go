package cache

import "time"

type item struct {
	value interface{}
}

type Cache struct {
	data map[string]item
}

func New(_ time.Duration, _ time.Duration) *Cache {
	return &Cache{
		data: make(map[string]item),
	}
}

func (c *Cache) Set(key string, value interface{}, _ time.Duration) {
	c.data[key] = item{value: value}
}

func (c *Cache) Get(key string) (interface{}, bool) {
	it, ok := c.data[key]
	if !ok {
		return nil, false
	}

	return it.value, true
}

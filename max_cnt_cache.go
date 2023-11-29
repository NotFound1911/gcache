package gcache

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

var (
	errOverCapacity = errors.New("gcache 超过容量限制")
)

type MaxCntCache struct {
	*MapCache
	cnt    int32
	maxCnt int32
}

func NewMaxCntCache(c *MapCache, maxCnt int32) *MaxCntCache {
	res := &MaxCntCache{
		MapCache: c,
		maxCnt:   maxCnt,
	}
	origin := c.onEvicted
	res.onEvicted = func(key string, val any) {
		atomic.AddInt32(&res.cnt, -1)
		if origin != nil {
			origin(key, val)
		}
	}
	return res
}
func (c *MaxCntCache) Set(ctx context.Context, key string, val any, expiration time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.data[key]
	if !ok {
		if c.cnt+1 > c.maxCnt {
			// 可以在这里设计复杂的淘汰策略
			return errOverCapacity
		}
		c.cnt++
	}
	return c.set(key, val, expiration)
}

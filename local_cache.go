package gcache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	errKeyNotFound = errors.New("gcache 键不存在")
)

type MapCacheOption func(cache *MapCache)

type MapCache struct {
	data      map[string]*item
	mu        sync.RWMutex
	onEvicted func(key string, val any)
	close     chan struct{}
	maxCnt    int
	closed    bool
}
type item struct {
	val      any
	deadline time.Time // 过期时间
}

// deadlineBefore 是否在时间t前面 用于判断过期
func (i *item) deadlineBefore(t time.Time) bool {
	return !i.deadline.IsZero() && i.deadline.Before(t)
}

func NewMapCache(interval time.Duration, opts ...MapCacheOption) *MapCache {
	cache := &MapCache{
		data:  make(map[string]*item, 128),
		close: make(chan struct{}),
		onEvicted: func(key string, val any) {
		},
		maxCnt: 10000,
	}
	for _, opt := range opts {
		opt(cache)
	}
	go func() {
		ticker := time.NewTicker(interval)
		for {
			select {
			case t := <-ticker.C:
				cache.mu.Lock()
				i := 0
				for key, val := range cache.data {
					// 随机过期
					if i >= cache.maxCnt {
						break
					}
					if val.deadlineBefore(t) {
						cache.delete(key)
					}
					i++
				}
				cache.mu.Unlock()
			case <-cache.close:
				cache.closed = true
				return
			}
		}
	}()
	return cache
}

func (m *MapCache) Set(ctx context.Context, key string, val any, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.set(key, val, expiration)
}
func (m *MapCache) set(key string, val any, expiration time.Duration) error {
	var dl time.Time
	if expiration > 0 {
		dl = time.Now().Add(expiration)
	}
	m.data[key] = &item{
		val:      val,
		deadline: dl,
	}
	return nil
}
func (m *MapCache) Get(ctx context.Context, key string) (any, error) {
	m.mu.RLock()
	res, ok := m.data[key]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w, key: %s", errKeyNotFound, key)
	}
	now := time.Now()
	if res.deadlineBefore(now) { // 过期清理
		m.mu.Lock()
		defer m.mu.Unlock()
		res, ok := m.data[key]
		if !ok { // 已经被清理
			return nil, fmt.Errorf("%w, key: %s", errKeyNotFound, key)
		}
		// 二次确定过期
		if res.deadlineBefore(now) {
			m.delete(key)
			return nil, fmt.Errorf("%w, key: %s", errKeyNotFound, key)
		}
	}
	return res.val, nil
}
func (m *MapCache) delete(key string) {
	itm, ok := m.data[key]
	if !ok {
		return
	}
	delete(m.data, key)
	m.onEvicted(key, itm.val)
}
func (m *MapCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delete(key)
	return nil
}

func (m *MapCache) LoadAndDelete(ctx context.Context, key string) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	val, ok := m.data[key]
	if !ok {
		return nil, fmt.Errorf("%w, key:%s", errKeyNotFound, key)
	}
	m.delete(key)
	return val.val, nil
}
func (m *MapCache) Close() error {
	select {
	case m.close <- struct{}{}:
	default:
		return errors.New("gcache 重复关闭")
	}
	return nil
}
func BuildMapCacheWithEvictedCallback(fn func(key string, val any)) MapCacheOption {
	return func(cache *MapCache) {
		cache.onEvicted = fn
	}
}

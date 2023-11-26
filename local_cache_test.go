package gcache

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestMapCache_Get(t *testing.T) {
	testCases := []struct {
		name    string
		key     string
		cache   func() *MapCache
		wantVal any
		wantErr error
	}{
		{
			name: "key not found",
			key:  "invalid key",
			cache: func() *MapCache {
				return NewMapCache(10 * time.Second)
			},
			wantErr: fmt.Errorf("%w, key: %s", errKeyNotFound, "invalid key"),
		},
		{
			name: "get value",
			key:  "key",
			cache: func() *MapCache {
				res := NewMapCache(10 * time.Second)
				err := res.Set(context.Background(), "key", 456, time.Minute)
				require.NoError(t, err)
				return res
			},
			wantVal: 456,
		},
		{
			name: "time expired",
			key:  "expired key",
			cache: func() *MapCache {
				res := NewMapCache(10 * time.Second)
				err := res.Set(context.Background(), "expired key", 456, time.Second)
				require.NoError(t, err)
				time.Sleep(time.Second * 2)
				return res
			},
			wantErr: fmt.Errorf("%w, key: %s", errKeyNotFound, "expired key"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			val, err := tc.cache().Get(context.Background(), tc.key)
			assert.Equal(t, tc.wantErr, err)
			if err != nil {
				return
			}
			assert.Equal(t, tc.wantVal, val)
		})
	}
}

func TestMapCache_Loop(t *testing.T) {
	cnt := 0
	cache := NewMapCache(time.Second, BuildMapCacheWithEvictedCallback(func(key string, val any) {
		cnt++
	}))
	err := cache.Set(context.Background(), "key", 456, time.Second)
	require.NoError(t, err)
	time.Sleep(time.Second * 3)
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	_, ok := cache.data["key"]
	require.False(t, ok)
	require.Equal(t, 1, cnt)
}

func TestMapCache_Ticker(t *testing.T) {
	cnt := 0
	setMacCnt := func(cache *MapCache) {
		cache.maxCnt = 3
	}
	cache := NewMapCache(2*time.Second, setMacCnt,
		BuildMapCacheWithEvictedCallback(func(key string, val any) {
			cnt++
		}))
	for i := 0; i < cache.maxCnt+1; i++ {
		key := fmt.Sprintf("key_%d", i)
		err := cache.Set(context.Background(), key, struct{}{}, 1*time.Second)
		require.NoError(t, err)
	}
	time.Sleep(time.Second * 3)
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	cntFalse := 0
	for i := 0; i < cache.maxCnt+1; i++ {
		key := fmt.Sprintf("key_%d", i)
		_, ok := cache.data[key]
		if !ok {
			cntFalse++
		}
	}
	require.Equal(t, cache.maxCnt, cnt)
	require.Equal(t, cache.maxCnt, cntFalse)
}

func TestMapCache_Close(t *testing.T) {
	cache := NewMapCache(time.Second)
	require.False(t, cache.closed)
	time.Sleep(time.Second)
	err := cache.Close()
	time.Sleep(time.Second)
	require.NoError(t, err)
	require.True(t, cache.closed)
}

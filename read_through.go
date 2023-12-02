package gcache

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sync/singleflight"
	"time"
)

var (
	errFailedToRefreshCache = errors.New("gcache 刷新缓存失败")
)

type ReadTroughCache struct {
	Cache
	LoadFunc   func(ctx context.Context, key string) (any, error) // 需要初始化
	Expiration time.Duration                                      // 过期时间
}

func (r *ReadTroughCache) Get(ctx context.Context, key string) (any, error) {
	val, err := r.Cache.Get(ctx, key)
	if err == errKeyNotFound { // 未找到
		val, err = r.LoadFunc(ctx, key)
		if err == nil {
			errSet := r.Cache.Set(ctx, key, val, r.Expiration)
			if errSet != nil {
				return val, fmt.Errorf("%w, 原因: %s", errFailedToRefreshCache, errSet.Error())
			}
		}
	}
	return val, err
}

type ReadThroughCacheV1[T any] struct {
	Cache
	LoadFunc   func(ctx context.Context, key string) (T, error)
	Expiration time.Duration
	g          singleflight.Group
}

func (r *ReadThroughCacheV1[T]) Get(ctx context.Context, key string) (T, error) {
	val, err := r.Cache.Get(ctx, key)
	if err == errKeyNotFound {
		val, err = r.LoadFunc(ctx, key)
		if err == nil {
			errSet := r.Cache.Set(ctx, key, val, r.Expiration)
			if errSet != nil {
				return val.(T), fmt.Errorf("%w, 原因: %s", errFailedToRefreshCache, errSet.Error())
			}
		}
	}
	return val.(T), err
}

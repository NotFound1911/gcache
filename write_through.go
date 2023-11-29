package gcache

import (
	"context"
	"time"
)

type WriteThroughCache struct {
	Cache
	StoreFunc func(ctx context.Context, ket string, val any) error
}

func (w *WriteThroughCache) Set(ctx context.Context, key string, val any, expiration time.Duration) error {
	err := w.StoreFunc(ctx, key, val)
	if err != nil {
		return err
	}
	return w.Cache.Set(ctx, key, val, expiration)
}

type WriteThroughCacheV1[T any] struct {
	Cache
	StoreFunc func(ctx context.Context, key string, val T) error
}

func (w *WriteThroughCacheV1[T]) Set(ctx context.Context, key string, val any, expiration time.Duration) error {
	err := w.StoreFunc(ctx, key, val.(T))
	if err != nil {
		return err
	}
	return w.Cache.Set(ctx, key, val, expiration)
}

package gcache

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
	"time"
)

var (
	ErrFailedToPreemptLock = errors.New("redis-lock: 抢锁失败")
	ErrLockNotHold         = errors.New("redis-lock: 没有持有锁")
	ErrNotOwnLock          = errors.New("redis-lock: 不是自己拥有的锁")
	//go:embed lua/unlock.lua
	luaUnlock string

	//go:embed lua/refresh.lua
	luaRefresh string

	//go:embed lua/lock.lua
	luaLock string
)

type Client struct {
	client redis.Cmdable
	g      singleflight.Group
}

func NewClient(client redis.Cmdable) *Client {
	return &Client{
		client: client,
	}
}
func (c *Client) SingleflightLock(ctx context.Context, key string, expiration time.Duration,
	timeout time.Duration, retry RetryStrategy) (*Lock, error) {
	for {
		flag := false // 是否是自己拿锁标记
		resChan := c.g.DoChan(key, func() (interface{}, error) {
			flag = true // 是自己拿到了锁
			return c.Lock(ctx, key, expiration, timeout, retry)
		})
		select {
		case res := <-resChan:
			if flag { // 确实是自己拿到了锁
				c.g.Forget(key)
				if res.Err != nil {
					return nil, res.Err
				}
				return res.Val.(*Lock), nil
			}
		case <-ctx.Done(): // 超时
			return nil, ctx.Err()
		}
	}
}
func (c *Client) Lock(ctx context.Context, key string, expiration time.Duration,
	timeout time.Duration, retry RetryStrategy) (*Lock, error) {
	var timer *time.Timer
	val := uuid.New().String()
	for {
		// 重试
		lctx, cancel := context.WithTimeout(ctx, timeout)
		res, err := c.client.Eval(lctx, luaLock, []string{key}, val, expiration.Seconds()).Result()
		cancel()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) { // 非超时错误
			return nil, err
		}
		if res == "OK" { // 拿到锁
			return &Lock{
				client:     c.client,
				key:        key,
				value:      val,
				expiration: expiration,
				unlockChan: make(chan struct{}, 1),
			}, nil
		}
		interval, ok := retry.Next()
		if !ok {
			return nil, fmt.Errorf("%w, 超出重试限制", ErrFailedToPreemptLock)
		}
		if timer == nil {
			timer = time.NewTimer(interval)
		} else {
			timer.Reset(interval)
		}
		select {
		case <-timer.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, nil
}
func (c *Client) TryLock(ctx context.Context, key string, expiration time.Duration) (*Lock, error) {
	val := uuid.New().String()
	ok, err := c.client.SetNX(ctx, key, val, expiration).Result() // todo SetNX
	if err != nil {
		return nil, err
	}
	if !ok {
		// 锁被其他实例拿走
		return nil, ErrFailedToPreemptLock
	}
	return &Lock{
		client:     c.client,
		key:        key,
		value:      val,
		expiration: expiration,
	}, err
}

type Lock struct {
	client     redis.Cmdable
	key        string
	value      string
	expiration time.Duration
	unlockChan chan struct{}
}

func (l *Lock) Unlock(ctx context.Context) error {
	res, err := l.client.Eval(ctx, luaUnlock, []string{l.key}, l.value).Int64()
	defer func() {
		select {
		case l.unlockChan <- struct{}{}:
		default:
			// 没有调用AutoRefresh
		}
	}()
	if err != nil {
		return err
	}
	if res != 1 {
		return ErrLockNotHold
	}
	return nil
}

func (l *Lock) Refresh(ctx context.Context) error {
	res, err := l.client.Eval(ctx, luaRefresh, []string{l.key}, l.value, l.expiration.Seconds()).Int64()
	if err != nil {
		return err
	}
	if res != 1 {
		return ErrLockNotHold
	}
	return nil
}

func (l *Lock) AutoRefresh(interval time.Duration, timeout time.Duration) error {
	timeoutChan := make(chan struct{}, 1)
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := l.Refresh(ctx)
			cancel()
			if errors.Is(err, context.DeadlineExceeded) {
				timeoutChan <- struct{}{}
				continue
			}
			if err != nil {
				return err
			}
		case <-timeoutChan:
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := l.Refresh(ctx)
			cancel()
			if errors.Is(err, context.DeadlineExceeded) {
				timeoutChan <- struct{}{}
				continue
			}
			if err != nil {
				return err
			}
		case <-l.unlockChan:
			return nil
		}
	}
}

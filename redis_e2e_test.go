//go:build e2e

package gcache

import (
	"context"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestRedisCache_e2e_Set(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	testCases := []struct {
		name  string
		after func(t *testing.T)

		key        string
		value      string
		expiration time.Duration

		wantErr error
	}{
		{
			name: "set value",
			after: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				res, err := rdb.Get(ctx, "key1").Result()
				require.NoError(t, err)
				assert.Equal(t, "val1", res)
				_, err = rdb.Del(ctx, "key1").Result()
				require.NoError(t, err)
			},
			key:        "key1",
			value:      "val1",
			expiration: time.Minute,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewRedisCache(rdb)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
			defer cancel()
			err := c.Set(ctx, tc.key, tc.value, tc.expiration)
			require.NoError(t, err)
			tc.after(t)
		})
	}

}
func TestRedisCache_e2e_Get(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	testCases := []struct {
		name   string
		before func(t *testing.T)
		after  func(t *testing.T)

		key        string
		value      string
		expiration time.Duration

		wantErr error
	}{
		{
			name: "get value",
			before: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
				defer cancel()
				err := rdb.Set(ctx, "key1", "val1", time.Minute).Err()
				require.NoError(t, err)
			},
			after: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
				defer cancel()
				err := rdb.Del(ctx, "key1").Err()
				require.NoError(t, err)
			},
			key:        "key1",
			value:      "val1",
			expiration: time.Minute,
		},
		{
			name: "timeout",
			before: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
				defer cancel()
				err := rdb.Set(ctx, "key2", "val2", time.Minute).Err()
				require.NoError(t, err)
			},
			after: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
				defer cancel()
				err := rdb.Del(ctx, "key2").Err()
				require.NoError(t, err)
			},
			key:        "key2",
			expiration: time.Microsecond,
			wantErr:    context.DeadlineExceeded,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewRedisCache(rdb)
			tc.before(t)
			ctx, cancel := context.WithTimeout(context.Background(), tc.expiration)
			defer cancel()
			time.Sleep(time.Second)
			res, err := c.Get(ctx, tc.key)
			assert.Equal(t, tc.wantErr, err)
			if err == nil {
				assert.Equal(t, tc.value, res)
			}
			tc.after(t)
		})
	}
}

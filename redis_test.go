package gcache

import (
	"context"
	"fmt"
	"github.com/NotFound1911/gcache/mocks"
	"github.com/golang/mock/gomock"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestRedisCache_Set(t *testing.T) {
	testCases := []struct {
		name       string
		mock       func(controller *gomock.Controller) redis.Cmdable
		key        string
		val        string
		expiration time.Duration
		wantErr    error
	}{
		{
			name: "set value",
			mock: func(controller *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(controller)
				status := redis.NewStatusCmd(context.Background())
				status.SetVal("OK")
				cmd.EXPECT().
					Set(context.Background(), "key", "val", time.Second).Return(status)
				return cmd
			},
			key:        "key",
			val:        "val",
			expiration: time.Second,
		},
		{
			name: "timeout",
			mock: func(controller *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(controller)
				status := redis.NewStatusCmd(context.Background())
				status.SetErr(context.DeadlineExceeded)
				cmd.EXPECT().
					Set(context.Background(), "key", "val", time.Second).Return(status)
				return cmd
			},
			key:        "key",
			val:        "val",
			expiration: time.Second,
			wantErr:    context.DeadlineExceeded,
		},
		{
			name: "unexpected name",
			mock: func(controller *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(controller)
				status := redis.NewStatusCmd(context.Background())
				status.SetVal("NO OK")
				cmd.EXPECT().
					Set(context.Background(), "key", "val", time.Second).Return(status)
				return cmd
			},
			key:        "key",
			val:        "val",
			expiration: time.Second,
			wantErr:    fmt.Errorf("%w, 返回信息: %s", errFailedToSetCache, "NO OK"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := NewRedisCache(tc.mock(ctrl))
			err := c.Set(context.Background(), tc.key, tc.val, tc.expiration)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestRedisCache_Get(t *testing.T) {
	testCases := []struct {
		name string

		mock func(controller *gomock.Controller) redis.Cmdable

		key string

		wantErr error
		wantVal string
	}{
		{
			name: "get value",
			mock: func(controller *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(controller)
				str := redis.NewStringCmd(context.Background())
				str.SetVal("val")
				cmd.EXPECT().
					Get(context.Background(), "key").Return(str)
				return cmd
			},
			key:     "key",
			wantVal: "val",
		},
		{
			name: "timeout",
			mock: func(controller *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(controller)
				str := redis.NewStringCmd(context.Background())
				str.SetErr(context.DeadlineExceeded)
				cmd.EXPECT().
					Get(context.Background(), "key").Return(str)
				return cmd
			},
			key:     "key",
			wantErr: context.DeadlineExceeded,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := NewRedisCache(tc.mock(ctrl))
			val, err := c.Get(context.Background(), tc.key)
			assert.Equal(t, tc.wantErr, err)
			if err != nil {
				return
			}
			assert.Equal(t, tc.wantVal, val)
		})
	}
}

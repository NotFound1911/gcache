package gcache

import (
	"context"
	"github.com/NotFound1911/gcache/mocks"
	"github.com/golang/mock/gomock"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestClient_Lock(t *testing.T) {
	testCases := []struct {
		name string
		mock func(ctrl *gomock.Controller) redis.Cmdable

		key string

		wantErr  error
		wantLock *Lock
	}{
		{
			name: "set nx error",
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(ctrl)
				res := redis.NewBoolResult(false, context.DeadlineExceeded)
				cmd.EXPECT().SetNX(context.Background(), "key1", gomock.Any(), time.Minute).Return(res)
				return cmd
			},
			key:     "key1",
			wantErr: context.DeadlineExceeded,
		},
		{
			name: "locker",
			mock: func(controller *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(controller)
				res := redis.NewBoolResult(true, nil)
				cmd.EXPECT().SetNX(context.Background(), "key1", gomock.Any(), time.Minute).Return(res)
				return cmd
			},
			key: "key1",
			wantLock: &Lock{
				key: "key1",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			client := NewClient(tc.mock(ctrl))
			l, err := client.TryLock(context.Background(), tc.key, time.Minute)
			assert.Equal(t, tc.wantErr, err)
			if err != nil {
				return
			}
			assert.Equal(t, tc.wantLock.key, l.key)
		})
	}
}

func TestLock_Unlock(t *testing.T) {
	testCases := []struct {
		name string

		mock  func(controller *gomock.Controller) redis.Cmdable
		key   string
		value string

		lock *Lock

		wantErr error
	}{
		{
			name: "eval error",
			mock: func(controller *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(controller)
				res := redis.NewCmd(context.Background())
				res.SetErr(context.DeadlineExceeded)
				cmd.EXPECT().Eval(context.Background(), luaUnlock, []string{"key1"}, []any{"val1"}).Return(res)
				return cmd
			},
			key:     "key1",
			value:   "val1",
			wantErr: context.DeadlineExceeded,
		},
		{
			name: "lock not hold",
			mock: func(controller *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(controller)
				res := redis.NewCmd(context.Background())
				res.SetVal(int64(0))
				cmd.EXPECT().Eval(context.Background(), luaUnlock, []string{"key1"}, []any{"val1"}).Return(res)
				return cmd
			},
			key:     "key1",
			value:   "val1",
			wantErr: ErrLockNotHold,
		},
		{
			name: "unlocked",
			mock: func(controller *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(controller)
				res := redis.NewCmd(context.Background())
				res.SetVal(int64(1))
				cmd.EXPECT().Eval(context.Background(), luaUnlock, []string{"key1"}, []any{"val1"}).Return(res)
				return cmd
			},
			key:   "key1",
			value: "val1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			lock := &Lock{
				key:    tc.key,
				value:  tc.value,
				client: tc.mock(ctrl),
			}
			err := lock.Unlock(context.Background())
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestLock_Refresh(t *testing.T) {
	testCases := []struct {
		name string

		mock  func(controller *gomock.Controller) redis.Cmdable
		key   string
		value string

		lock       *Lock
		expiration time.Duration
		wantErr    error
	}{
		{
			name: "eval error",
			mock: func(controller *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(controller)
				res := redis.NewCmd(context.Background())
				res.SetErr(context.DeadlineExceeded)
				cmd.EXPECT().Eval(context.Background(), luaRefresh, []string{"key1"}, []any{"val1", float64(60)}).
					Return(res)
				return cmd
			},
			key:        "key1",
			value:      "val1",
			wantErr:    context.DeadlineExceeded,
			expiration: time.Minute,
		},
		{
			name: "lock not hold",
			mock: func(controller *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(controller)
				res := redis.NewCmd(context.Background())
				res.SetVal(int64(0))
				cmd.EXPECT().Eval(context.Background(), luaRefresh, []string{"key1"}, []any{"val1", float64(60)}).
					Return(res)
				return cmd
			},
			key:        "key1",
			value:      "val1",
			wantErr:    ErrLockNotHold,
			expiration: time.Minute,
		},
		{
			name: "unlocked",
			mock: func(controller *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(controller)
				res := redis.NewCmd(context.Background())
				res.SetVal(int64(1))
				cmd.EXPECT().Eval(context.Background(), luaRefresh, []string{"key1"}, []any{"val1", float64(60)}).
					Return(res)
				return cmd
			},
			key:        "key1",
			value:      "val1",
			expiration: time.Minute,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			lock := &Lock{
				key:        tc.key,
				value:      tc.value,
				client:     tc.mock(ctrl),
				expiration: tc.expiration,
			}
			err := lock.Refresh(context.Background())
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

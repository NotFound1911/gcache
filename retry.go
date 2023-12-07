package gcache

import "time"

type RetryStrategy interface {
	Next() (time.Duration, bool) // 重试间隔  是否重试
}

// FixedInterval 固定间隔重试
type FixedInterval struct {
	Interval time.Duration
	MaxCnt   int
	cnt      int
}

func (f *FixedInterval) Next() (time.Duration, bool) {
	if f.cnt >= f.MaxCnt {
		return 0, false
	}
	f.cnt++
	return f.Interval, true
}

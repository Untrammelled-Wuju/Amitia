package agent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type taskLeaseTimer struct {
	mu      sync.Mutex
	timer   *time.Timer
	expires time.Time
	closed  bool
	cancel  context.CancelFunc
}

func (l *taskLeaseTimer) Confirm(duration time.Duration) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if l.closed || duration <= 0 || duration > 5*time.Minute || !l.expires.IsZero() && !now.Before(l.expires) {
		return fmt.Errorf("设备任务租约已到期或关闭，拒绝恢复同一次执行")
	}
	l.expires = now.Add(duration)
	if l.timer == nil {
		l.timer = time.AfterFunc(duration, func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			if !l.closed && !time.Now().Before(l.expires) {
				l.closed = true
				l.cancel()
			}
		})
	} else {
		l.timer.Reset(duration)
	}
	return nil
}

func (l *taskLeaseTimer) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	if l.timer != nil {
		l.timer.Stop()
	}
}

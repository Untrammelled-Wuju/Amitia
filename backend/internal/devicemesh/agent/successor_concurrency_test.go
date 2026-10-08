package agent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentSuccessorFollowSharesOperationAndRespectsCancellation(t *testing.T) {
	handler := &LocalHandler{}
	started, release := make(chan struct{}), make(chan struct{})
	result := errors.New("独立审批尚未完成")
	var calls atomic.Int32
	done := make(chan error, 1)
	go func() {
		done <- handler.followProviderOnce(t.Context(), func(ctx context.Context) error {
			calls.Add(1)
			close(started)
			select {
			case <-release:
				return result
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	<-started
	follower, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if err := handler.followProviderOnce(follower, func(context.Context) error {
		calls.Add(1)
		return nil
	}); !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 1 {
		t.Fatalf("等待者重复启动切换或未遵守取消: %v", err)
	}
	handler.followMu.Lock()
	operation := handler.followOperation
	handler.followMu.Unlock()
	close(release)
	if err := <-done; err != result {
		t.Fatalf("切换结果丢失: %v", err)
	}
	<-operation.done
	if operation.err != result {
		t.Fatal("并发等待者未获得同一操作结果")
	}
	if err := handler.followProviderOnce(t.Context(), func(context.Context) error {
		calls.Add(1)
		return nil
	}); err != nil || calls.Load() != 2 {
		t.Fatal("已结束的审批结果阻止了下一次切换检查")
	}
}

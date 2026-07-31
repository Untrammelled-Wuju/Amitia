package revisioncommit

import (
	"context"
	"time"

	"github.com/u-ai/backend/log"
)

type RecoveryWorker struct {
	bridge   RevisionBridge
	stopCh   chan struct{}
	interval time.Duration
}

func NewRecoveryWorker(bridge RevisionBridge, interval time.Duration) *RecoveryWorker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &RecoveryWorker{
		bridge:   bridge,
		stopCh:   make(chan struct{}),
		interval: interval,
	}
}

func (w *RecoveryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			if err := w.bridge.RecoverPending(ctx); err != nil {
				log.Logger.Errorf("恢复待处理桥接任务失败: %v", err)
			}
		}
	}
}

func (w *RecoveryWorker) Stop() {
	select {
	case <-w.stopCh:
	default:
		close(w.stopCh)
	}
}

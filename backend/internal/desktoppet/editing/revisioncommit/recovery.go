package revisioncommit

import (
	"context"
	"time"

	"github.com/u-ai/backend/log"
)

type RecoveryWorker struct {
	processor *BridgeProcessor
	stopCh    chan struct{}
	interval  time.Duration
}

func NewRecoveryWorker(processor *BridgeProcessor, interval time.Duration) *RecoveryWorker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &RecoveryWorker{
		processor: processor,
		stopCh:    make(chan struct{}),
		interval:  interval,
	}
}

func (w *RecoveryWorker) Start(ctx context.Context) {
	w.runOnce(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *RecoveryWorker) runOnce(ctx context.Context) {
	if err := w.processor.IngestProcessingOutbox(ctx, 50); err != nil {
		log.Logger.Errorf("消费Processing Outbox失败: %v", err)
	}
	if err := w.processor.ProcessPending(ctx, 50); err != nil {
		log.Logger.Errorf("恢复待处理Inbox条目失败: %v", err)
	}
	if err := w.processor.PublishOutbox(ctx, 50); err != nil {
		log.Logger.Errorf("发布待处理Outbox事件失败: %v", err)
	}
}

func (w *RecoveryWorker) Stop() {
	select {
	case <-w.stopCh:
	default:
		close(w.stopCh)
	}
}

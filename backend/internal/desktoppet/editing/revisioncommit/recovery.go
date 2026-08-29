package revisioncommit

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/log"
)

type RecoveryWorker struct {
	processor   *BridgeProcessor
	stopCh      chan struct{}
	interval    time.Duration
	wg          sync.WaitGroup
	lifecycleMu sync.Mutex
	running     bool
	alive       atomic.Bool
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
	w.lifecycleMu.Lock()
	defer w.lifecycleMu.Unlock()
	if w.running {
		return
	}
	w.stopCh = make(chan struct{})
	w.running = true
	w.alive.Store(true)
	w.wg.Add(1)
	go w.run(ctx)
}

func (w *RecoveryWorker) run(ctx context.Context) {
	defer w.wg.Done()
	defer w.alive.Store(false)
	defer func() {
		if r := recover(); r != nil {
			log.Logger.Errorf("revision commit recovery worker panic: %v", r)
		}
	}()
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
	w.lifecycleMu.Lock()
	defer w.lifecycleMu.Unlock()
	if !w.running {
		return
	}
	close(w.stopCh)
	w.wg.Wait()
	w.running = false
	w.alive.Store(false)
}

func (w *RecoveryWorker) IsRunning() bool {
	w.lifecycleMu.Lock()
	defer w.lifecycleMu.Unlock()
	return w.running && w.alive.Load()
}

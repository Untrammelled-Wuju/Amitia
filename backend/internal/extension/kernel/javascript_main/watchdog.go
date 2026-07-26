package javascript_main

import (
	"context"
	"sync"
	"time"
)

type WatchdogReport struct {
	Healthy            bool
	EventLoopLag       time.Duration
	ActiveInvocations  int
	LastResponseAt     time.Time
	QueueDepth         int
	MemoryUsageMB      int
	LogsDropped        int
	Reason             string
}

type Watchdog struct {
	mu             sync.RWMutex
	instanceID     string
	running        bool
	stopCh         chan struct{}
	report         WatchdogReport
	lastReportAt   time.Time
	checkInterval  time.Duration
	maxLag         time.Duration
	consecutiveFailures int
	maxFailures    int
}

func NewWatchdog(instanceID string) *Watchdog {
	return &Watchdog{
		instanceID:    instanceID,
		checkInterval: 5 * time.Second,
		maxLag:        5 * time.Second,
		maxFailures:   3,
		stopCh:        make(chan struct{}),
		report: WatchdogReport{
			Healthy:        true,
			LastResponseAt: time.Now().UTC(),
		},
	}
}

func (w *Watchdog) Start(ctx context.Context, host *PluginHost) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.stopCh = make(chan struct{})
	w.mu.Unlock()

	go w.loop(ctx, host)
}

func (w *Watchdog) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	w.running = false
	close(w.stopCh)
}

func (w *Watchdog) loop(ctx context.Context, host *PluginHost) {
	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			report := w.check(host)
			w.mu.Lock()
			w.report = report
			w.lastReportAt = time.Now().UTC()
			if !report.Healthy {
				w.consecutiveFailures++
				if w.consecutiveFailures >= w.maxFailures {
					host.MarkCrashed(report.Reason)
				}
			} else {
				w.consecutiveFailures = 0
			}
			w.mu.Unlock()
		}
	}
}

func (w *Watchdog) check(host *PluginHost) WatchdogReport {
	report := WatchdogReport{
		Healthy:        true,
		LastResponseAt: time.Now().UTC(),
	}
	if host != nil {
		report.ActiveInvocations = host.Dispatcher().ActiveCount()
		report.QueueDepth = host.Dispatcher().QueuedCount()
	}
	if report.EventLoopLag > w.maxLag {
		report.Healthy = false
		report.Reason = "event loop lag exceeded"
	}
	return report
}

func (w *Watchdog) Report() WatchdogReport {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.report
}

func (w *Watchdog) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

func (w *Watchdog) ConsecutiveFailures() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.consecutiveFailures
}

type ShutdownCoordinator struct {
	mu                  sync.Mutex
	started             bool
	completed           bool
	rejectNewDone       bool
	queueCancelled      bool
	deactivateCalled    bool
	sessionClosed       bool
	stoppedSent         bool
	forceStopped        bool
	startedAt           time.Time
	completedAt         *time.Time
}

func NewShutdownCoordinator() *ShutdownCoordinator {
	return &ShutdownCoordinator{}
}

func (c *ShutdownCoordinator) BeginShutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.started = true
	c.startedAt = time.Now().UTC()
}

func (c *ShutdownCoordinator) MarkRejectNew() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rejectNewDone = true
}

func (c *ShutdownCoordinator) MarkQueueCancelled() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queueCancelled = true
}

func (c *ShutdownCoordinator) MarkDeactivateCalled() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deactivateCalled = true
}

func (c *ShutdownCoordinator) MarkSessionClosed() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionClosed = true
}

func (c *ShutdownCoordinator) MarkStoppedSent() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stoppedSent = true
}

func (c *ShutdownCoordinator) MarkForceStopped() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.forceStopped = true
}

func (c *ShutdownCoordinator) Complete() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.completed = true
	now := time.Now().UTC()
	c.completedAt = &now
}

func (c *ShutdownCoordinator) IsCompleted() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.completed
}

func (c *ShutdownCoordinator) IsForceStopped() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.forceStopped
}

func (c *ShutdownCoordinator) Duration() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.completedAt == nil {
		return 0
	}
	return c.completedAt.Sub(c.startedAt)
}

package task_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type ProcessHostConfig struct {
	Generation  int64  `json:"generation"`
	InstanceID  string `json:"instanceId"`
	TaskRunID   string `json:"taskRunId"`
	ExtensionID string `json:"extensionId"`
	ModuleID    string `json:"moduleId"`
	DefHash     string `json:"defHash"`
	NodePath    string `json:"nodePath"`
	HostPath    string `json:"hostPath"`
	WorkDir     string `json:"workDir"`
	EntryPath   string `json:"entryPath"`
	EntryHash   string `json:"entryHash"`
}

type ProcessCallbacks struct {
	OnRequest             func(context.Context, string, string, json.RawMessage) (json.RawMessage, error)
	OnProgress            func(seq int64, current, total, percentage *float64, stage, message string)
	OnCheckpoint          func(version int64, payload json.RawMessage, hash string)
	OnCheckpointConfirmed func(version int64, payload json.RawMessage, hash string) error
	OnLog                 func(level, message string, fields map[string]interface{})
	OnFinished            func(status string, result json.RawMessage, artifactID string, errCode, errMsg string)
}

type TaskProcessHost struct {
	config ProcessHostConfig

	mu           sync.Mutex
	cancelCh     chan struct{}
	pauseCh      chan struct{}
	readyCh      chan struct{}
	pauseVersion int64
	doneCh       chan struct{}
	state        string
	stop         context.CancelFunc
	exitCode     int
	exitErr      error
}

func NewTaskProcessHost(cfg ProcessHostConfig) (*TaskProcessHost, error) {
	if cfg.Generation == 0 {
		cfg.Generation = 1
	}
	if cfg.Generation < 1 || cfg.Generation > 9007199254740991 {
		return nil, fmt.Errorf("任务运行代次无效")
	}
	return &TaskProcessHost{
		config:   cfg,
		cancelCh: make(chan struct{}, 1),
		pauseCh:  make(chan struct{}, 1),
		readyCh:  make(chan struct{}),
		doneCh:   make(chan struct{}),
		state:    "initialized",
	}, nil
}

func (h *TaskProcessHost) Start(
	ctx context.Context,
	input json.RawMessage,
	checkpoint json.RawMessage,
	deadline *time.Time,
	attempt int,
	maxAttempts int,
	callbacks ProcessCallbacks,
) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.state != "initialized" {
		return fmt.Errorf("host not in initialized state: %s", h.state)
	}

	h.state = "running"

	processCtx, stop := context.WithCancel(ctx)
	h.stop = stop
	go h.runInner(processCtx, input, checkpoint, deadline, attempt, maxAttempts, callbacks)

	return nil
}

func (h *TaskProcessHost) runInner(
	ctx context.Context,
	input json.RawMessage,
	checkpoint json.RawMessage,
	deadline *time.Time,
	attempt int,
	maxAttempts int,
	callbacks ProcessCallbacks) {
	exitCode, err := h.runProcess(ctx, input, checkpoint, deadline, attempt, maxAttempts, callbacks)
	h.mu.Lock()
	h.exitCode, h.exitErr = exitCode, err
	if ctx.Err() != nil || h.state == "cancelling" {
		h.state = "cancelled"
	} else if err != nil {
		h.state = "failed"
	} else if h.state == "pausing" {
		h.state = "paused"
	} else {
		h.state = "finished"
	}
	h.mu.Unlock()
	close(h.doneCh)
}

func (h *TaskProcessHost) Cancel(ctx context.Context, reason string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.state == "initialized" {
		h.state, h.exitCode, h.exitErr = "cancelled", 1, context.Canceled
		close(h.doneCh)
		return nil
	}
	if h.state != "running" && h.state != "pausing" {
		return nil
	}
	h.state = "cancelling"

	select {
	case h.cancelCh <- struct{}{}:
	default:
	}

	return nil
}

func (h *TaskProcessHost) Pause(ctx context.Context) (int64, error) {
	h.mu.Lock()
	if h.state != "running" {
		h.mu.Unlock()
		return 0, fmt.Errorf("任务进程当前状态无法暂停")
	}
	h.state = "pausing"
	h.mu.Unlock()
	select {
	case <-h.readyCh:
	case <-h.doneCh:
		return 0, fmt.Errorf("任务进程尚未就绪即已退出")
	case <-ctx.Done():
		h.ForceStop()
		return 0, ctx.Err()
	}
	select {
	case h.pauseCh <- struct{}{}:
	case <-h.doneCh:
		return 0, fmt.Errorf("任务进程已退出")
	case <-ctx.Done():
		h.ForceStop()
		return 0, ctx.Err()
	}
	select {
	case <-h.doneCh:
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.state != "paused" || h.pauseVersion < 1 {
			return 0, fmt.Errorf("任务进程未确认暂停: %v", h.exitErr)
		}
		return h.pauseVersion, nil
	case <-ctx.Done():
		h.ForceStop()
		return 0, ctx.Err()
	}
}

func (h *TaskProcessHost) ForceStop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.stop != nil {
		h.stop()
	}
	if h.state == "initialized" {
		h.state = "stopped"
		close(h.doneCh)
	}
}

func (h *TaskProcessHost) Wait() (int, error) {
	<-h.doneCh
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.exitCode, h.exitErr
}

func (h *TaskProcessHost) Done() <-chan struct{} {
	return h.doneCh
}

func (h *TaskProcessHost) CancelCh() chan struct{} {
	return h.cancelCh
}

func (h *TaskProcessHost) State() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state
}

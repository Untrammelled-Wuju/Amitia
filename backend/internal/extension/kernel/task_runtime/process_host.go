package task_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type ProcessHostConfig struct {
	InstanceID  string `json:"instanceId"`
	TaskRunID   string `json:"taskRunId"`
	ExtensionID string `json:"extensionId"`
	ModuleID    string `json:"moduleId"`
	DefHash     string `json:"defHash"`
	NodePath    string `json:"nodePath"`
	HostPath    string `json:"hostPath"`
	WorkDir     string `json:"workDir"`
	EntryPath   string `json:"entryPath"`
}

type ProcessCallbacks struct {
	OnProgress   func(seq int64, current, total, percentage *float64, stage, message string)
	OnCheckpoint func(version int64, payload json.RawMessage, hash string)
	OnLog        func(level, message string, fields map[string]interface{})
	OnFinished   func(status string, result json.RawMessage, artifactID string, errCode, errMsg string)
}

type TaskProcessHost struct {
	config ProcessHostConfig

	mu       sync.Mutex
	cancelCh chan struct{}
	doneCh   chan struct{}
	state    string
}

func NewTaskProcessHost(cfg ProcessHostConfig) (*TaskProcessHost, error) {
	return &TaskProcessHost{
		config:   cfg,
		cancelCh: make(chan struct{}, 1),
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

	go h.runInner(ctx, input, checkpoint, deadline, attempt, maxAttempts, callbacks)

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
	defer close(h.doneCh)

	h.mu.Lock()
	h.state = "running"
	h.mu.Unlock()

	select {
	case <-ctx.Done():
		h.mu.Lock()
		h.state = "cancelled"
		h.mu.Unlock()
		return
	case <-h.cancelCh:
		h.mu.Lock()
		h.state = "cancelled"
		h.mu.Unlock()
		return
	case <-time.After(10 * time.Millisecond):
		h.state = "finished"
		if callbacks.OnFinished != nil {
			callbacks.OnFinished("succeeded", []byte("{}"), "", "", "")
		}
		return
	}
}

func (h *TaskProcessHost) Cancel(ctx context.Context, reason string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.state != "running" {
		return nil
	}

	select {
	case h.cancelCh <- struct{}{}:
	default:
	}

	return nil
}

func (h *TaskProcessHost) ForceStop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.state != "initialized" {
		return
	}

	h.state = "stopped"
}

func (h *TaskProcessHost) Wait() (int, error) {
	<-h.doneCh
	return 0, nil
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

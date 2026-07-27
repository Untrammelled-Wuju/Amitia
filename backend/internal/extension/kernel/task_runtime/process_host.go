package task_runtime

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type ProcessHostState string

const (
	ProcessStateCreated   ProcessHostState = "created"
	ProcessStateStarting  ProcessHostState = "starting"
	ProcessStateReady     ProcessHostState = "ready"
	ProcessStateRunning   ProcessHostState = "running"
	ProcessStateStopping  ProcessHostState = "stopping"
	ProcessStateStopped   ProcessHostState = "stopped"
	ProcessStateCrashed   ProcessHostState = "crashed"
	ProcessStateTimedOut  ProcessHostState = "timed_out"
)

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type TaskProcessHost struct {
	mu           sync.Mutex
	state        ProcessHostState
	instanceID   string
	taskRunID    string
	extensionID  string
	moduleID     string
	defHash      string
	nonce        string
	nodePath     string
	hostPath     string
	workDir      string
	entryPath    string

	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdoutPipe io.ReadCloser
	stderrPipe io.ReadCloser
	pid        int
	writeMu    sync.Mutex
	procCtx    context.Context
	procCancel context.CancelFunc

	pending   map[string]chan *rpcMessage
	pendingMu sync.Mutex
	reqCount  int64

	notifiers   map[string]func(json.RawMessage)
	notifiersMu sync.RWMutex

	exitCh     chan int
	exitErr    error
	closeOnce  sync.Once
	done       chan struct{}
	exitCode   int
}

type ProcessHostConfig struct {
	InstanceID  string
	TaskRunID   string
	ExtensionID string
	ModuleID    string
	DefHash     string
	NodePath    string
	HostPath    string
	WorkDir     string
	EntryPath   string
	Input       json.RawMessage
	Checkpoint  json.RawMessage
	Deadline    *time.Time
	Attempt     int
	MaxAttempts int
}

type ProgressCallback func(seq int64, current, total, percentage *float64, stage, message string)
type CheckpointCallback func(version int64, payload json.RawMessage, hash string)
type LogCallback func(level, message string, fields map[string]interface{})
type FinishedCallback func(status string, result json.RawMessage, artifactID string, errCode, errMsg string)

type ProcessCallbacks struct {
	OnProgress   ProgressCallback
	OnCheckpoint CheckpointCallback
	OnLog        LogCallback
	OnFinished   FinishedCallback
}

func NewTaskProcessHost(cfg ProcessHostConfig) (*TaskProcessHost, error) {
	nonce, err := generateNonce()
	if err != nil {
		return nil, fmt.Errorf("task_process_host: generate nonce: %w", err)
	}
	return &TaskProcessHost{
		state:       ProcessStateCreated,
		instanceID:  cfg.InstanceID,
		taskRunID:   cfg.TaskRunID,
		extensionID: cfg.ExtensionID,
		moduleID:    cfg.ModuleID,
		defHash:     cfg.DefHash,
		nonce:       nonce,
		nodePath:    cfg.NodePath,
		hostPath:    cfg.HostPath,
		workDir:     cfg.WorkDir,
		entryPath:   cfg.EntryPath,
		pending:     make(map[string]chan *rpcMessage),
		notifiers:   make(map[string]func(json.RawMessage)),
		exitCh:      make(chan int, 1),
		done:        make(chan struct{}),
	}, nil
}

func (h *TaskProcessHost) Start(ctx context.Context, input json.RawMessage, checkpoint json.RawMessage, deadline *time.Time, attempt, maxAttempts int, callbacks ProcessCallbacks) error {
	h.mu.Lock()
	if h.state != ProcessStateCreated {
		h.mu.Unlock()
		return fmt.Errorf("task_process_host: already started")
	}
	h.state = ProcessStateStarting
	h.mu.Unlock()

	h.notifiersMu.Lock()
	h.notifiers["task.progress"] = func(params json.RawMessage) {
		var p struct {
			TaskRunID  string           `json:"task_run_id"`
			Sequence   int64            `json:"sequence"`
			Current    *float64         `json:"current"`
			Total      *float64         `json:"total"`
			Percentage *float64         `json:"percentage"`
			Stage      string           `json:"stage"`
			Message    string           `json:"message"`
		}
		if json.Unmarshal(params, &p) == nil && callbacks.OnProgress != nil {
			callbacks.OnProgress(p.Sequence, p.Current, p.Total, p.Percentage, p.Stage, p.Message)
		}
	}
	h.notifiers["task.checkpoint"] = func(params json.RawMessage) {
		var p struct {
			TaskRunID string          `json:"task_run_id"`
			Version   int64           `json:"version"`
			Payload   json.RawMessage `json:"payload"`
			Hash      string          `json:"hash"`
		}
		if json.Unmarshal(params, &p) == nil && callbacks.OnCheckpoint != nil {
			callbacks.OnCheckpoint(p.Version, p.Payload, p.Hash)
		}
	}
	h.notifiers["log.write"] = func(params json.RawMessage) {
		var p struct {
			Level   string                 `json:"level"`
			Message string                 `json:"message"`
			Fields  map[string]interface{} `json:"fields"`
		}
		if json.Unmarshal(params, &p) == nil && callbacks.OnLog != nil {
			callbacks.OnLog(p.Level, p.Message, p.Fields)
		}
	}
	h.notifiers["task.finished"] = func(params json.RawMessage) {
		var p struct {
			TaskRunID  string          `json:"task_run_id"`
			Status     string          `json:"status"`
			Result     json.RawMessage `json:"result"`
			ArtifactID string          `json:"artifact_id"`
			ErrorCode  string          `json:"error_code"`
			ErrorMessage string        `json:"error_message"`
		}
		if json.Unmarshal(params, &p) == nil && callbacks.OnFinished != nil {
			callbacks.OnFinished(p.Status, p.Result, p.ArtifactID, p.ErrorCode, p.ErrorMessage)
		}
	}
	h.notifiersMu.Unlock()

	procCtx, procCancel := context.WithCancel(context.Background())
	h.procCtx = procCtx
	h.procCancel = procCancel

	env := []string{}
	env = append(env, os.Environ()...)
	env = append(env,
		"AMITIA_INSTANCE_ID="+h.instanceID,
		"AMITIA_EXTENSION_ID="+h.extensionID,
		"AMITIA_MODULE_ID="+h.moduleID,
		"AMITIA_NONCE="+h.nonce,
		"AMITIA_HOST_API_VERSION=amitia-runtime-rpc/1",
		"AMITIA_DEFINITION_HASH="+h.defHash,
		"AMITIA_TASK_RUN_ID="+h.taskRunID,
		"AMITIA_TASK_ENTRY="+h.entryPath,
		"AMITIA_TASK_INPUT="+string(input),
		"AMITIA_TASK_ATTEMPT="+strconv.Itoa(attempt),
		"AMITIA_TASK_MAX_ATTEMPTS="+strconv.Itoa(maxAttempts),
	)
	if checkpoint != nil {
		env = append(env, "AMITIA_TASK_CHECKPOINT="+string(checkpoint))
	}
	if deadline != nil {
		env = append(env, "AMITIA_TASK_DEADLINE="+strconv.FormatInt(deadline.UnixMilli(), 10))
	}
	if h.workDir != "" {
		env = append(env, "AMITIA_WORKSPACE_PATH="+h.workDir)
	}

	cmd := exec.CommandContext(procCtx, h.nodePath, h.hostPath)
	cmd.Dir = h.workDir
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		procCancel()
		return fmt.Errorf("task_process_host: stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		procCancel()
		return fmt.Errorf("task_process_host: stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		procCancel()
		return fmt.Errorf("task_process_host: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		procCancel()
		return fmt.Errorf("task_process_host: start process: %w", err)
	}

	h.cmd = cmd
	h.stdin = stdin
	h.stdoutPipe = stdoutPipe
	h.stderrPipe = stderrPipe
	h.pid = cmd.Process.Pid

	go h.readLoop()
	go h.readStderr()
	go h.waitExit()

	if err := h.handshake(ctx); err != nil {
		h.ForceStop()
		return fmt.Errorf("task_process_host: handshake: %w", err)
	}

	if err := h.executeTask(ctx, input, checkpoint, deadline, attempt, maxAttempts); err != nil {
		h.ForceStop()
		return fmt.Errorf("task_process_host: execute: %w", err)
	}

	h.mu.Lock()
	h.state = ProcessStateRunning
	h.mu.Unlock()

	return nil
}

func (h *TaskProcessHost) handshake(ctx context.Context) error {
	helloCh := make(chan *rpcMessage, 1)
	h.notifiersMu.Lock()
	h.notifiers["runtime.hello"] = func(params json.RawMessage) {
		select {
		case helloCh <- &rpcMessage{Params: params}:
		default:
		}
	}
	h.notifiersMu.Unlock()

	readyCh := make(chan *rpcMessage, 1)
	h.notifiersMu.Lock()
	h.notifiers["runtime.ready"] = func(params json.RawMessage) {
		select {
		case readyCh <- &rpcMessage{Params: params}:
		default:
		}
	}
	h.notifiersMu.Unlock()

	select {
	case <-helloCh:
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for runtime.hello")
	case <-ctx.Done():
		return ctx.Err()
	}

	welcome := map[string]interface{}{
		"session_id":    h.instanceID,
		"session_token": h.nonce,
		"limits": map[string]interface{}{
			"max_progress_per_second": 5,
			"max_checkpoint_bytes":    1048576,
			"max_inline_result_bytes": 262144,
		},
		"expires_at": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
	}
	welcomeJSON, _ := json.Marshal(welcome)
	if err := h.notify("host.welcome", welcomeJSON); err != nil {
		return fmt.Errorf("send host.welcome: %w", err)
	}

	select {
	case <-readyCh:
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for runtime.ready")
	case <-ctx.Done():
		return ctx.Err()
	}

	h.mu.Lock()
	h.state = ProcessStateReady
	h.mu.Unlock()
	return nil
}

func (h *TaskProcessHost) executeTask(ctx context.Context, input json.RawMessage, checkpoint json.RawMessage, deadline *time.Time, attempt, maxAttempts int) error {
	params := map[string]interface{}{
		"task_run_id":   h.taskRunID,
		"entry":         h.entryPath,
		"input":         json.RawMessage(input),
		"attempt":       attempt,
		"max_attempts":  maxAttempts,
	}
	if checkpoint != nil {
		params["checkpoint"] = json.RawMessage(checkpoint)
	}
	if deadline != nil {
		params["deadline"] = deadline.UnixMilli()
	}
	paramsJSON, _ := json.Marshal(params)
	_, err := h.call(ctx, "task.execute", paramsJSON)
	if err != nil {
		return fmt.Errorf("task.execute: %w", err)
	}
	return nil
}

func (h *TaskProcessHost) Cancel(ctx context.Context, reason string) error {
	h.mu.Lock()
	if h.state == ProcessStateStopped || h.state == ProcessStateCrashed {
		h.mu.Unlock()
		return nil
	}
	h.state = ProcessStateStopping
	h.mu.Unlock()

	params, _ := json.Marshal(map[string]string{
		"task_run_id": h.taskRunID,
		"reason":      reason,
	})
	_ = h.notify("task.cancel", params)

	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case <-h.done:
	case <-timer.C:
		h.ForceStop()
	}
	return nil
}

func (h *TaskProcessHost) ForceStop() {
	h.closeOnce.Do(func() {
		if h.procCancel != nil {
			h.procCancel()
		}
		if h.cmd != nil && h.cmd.Process != nil {
			_ = h.cmd.Process.Kill()
		}
	})
}

func (h *TaskProcessHost) Wait() (int, error) {
	<-h.done
	return h.exitCode, h.exitErr
}

func (h *TaskProcessHost) Done() <-chan struct{} {
	return h.done
}

func (h *TaskProcessHost) PID() int {
	return h.pid
}

func (h *TaskProcessHost) State() ProcessHostState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state
}

func (h *TaskProcessHost) InstanceID() string {
	return h.instanceID
}

func (h *TaskProcessHost) waitExit() {
	err := h.cmd.Wait()
	h.mu.Lock()
	h.exitErr = err
	if h.cmd.ProcessState != nil {
		h.exitCode = h.cmd.ProcessState.ExitCode()
	}
	prevState := h.state
	if h.state != ProcessStateStopping && h.state != ProcessStateStopped {
		if h.exitCode != 0 {
			h.state = ProcessStateCrashed
		} else {
			h.state = ProcessStateStopped
		}
	}
	h.mu.Unlock()

	if err != nil && prevState != ProcessStateStopping {
		_ = err
	}

	h.closeOnce.Do(func() {})
	close(h.done)
	close(h.exitCh)
}

func (h *TaskProcessHost) readLoop() {
	scanner := bufio.NewScanner(h.stdoutPipe)
	scanner.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		if msg.ID != "" && msg.Method == "" {
			h.pendingMu.Lock()
			ch, ok := h.pending[msg.ID]
			if ok {
				delete(h.pending, msg.ID)
			}
			h.pendingMu.Unlock()
			if ok {
				ch <- &msg
			}
		} else if msg.Method != "" {
			h.notifiersMu.RLock()
			fn, ok := h.notifiers[msg.Method]
			h.notifiersMu.RUnlock()
			if ok && fn != nil {
				fn(msg.Params)
			}
		}
	}
}

func (h *TaskProcessHost) readStderr() {
	scanner := bufio.NewScanner(h.stderrPipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
	}
}

func (h *TaskProcessHost) call(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	id := strconv.FormatInt(atomic.AddInt64(&h.reqCount, 1), 10)
	ch := make(chan *rpcMessage, 1)
	h.pendingMu.Lock()
	h.pending[id] = ch
	h.pendingMu.Unlock()
	defer func() {
		h.pendingMu.Lock()
		delete(h.pending, id)
		h.pendingMu.Unlock()
	}()

	msg := rpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	data, _ := json.Marshal(msg)
	if err := h.write(data); err != nil {
		return nil, err
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("rpc timeout: %s", method)
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-h.done:
		return nil, fmt.Errorf("process exited")
	}
}

func (h *TaskProcessHost) notify(method string, params json.RawMessage) error {
	msg := rpcMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	data, _ := json.Marshal(msg)
	return h.write(data)
}

func (h *TaskProcessHost) write(data []byte) error {
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	if h.stdin == nil {
		return fmt.Errorf("stdin closed")
	}
	_, err := h.stdin.Write(append(data, '\n'))
	return err
}

func generateNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func createTaskWorkspace(root, taskRunID string) (string, error) {
	dir := filepath.Join(root, "temp", "tasks", taskRunID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create task workspace: %w", err)
	}
	return dir, nil
}

func cleanupTaskWorkspace(taskRunID string) {
	dir := filepath.Join(os.TempDir(), "amitia-tasks", taskRunID)
	_ = os.RemoveAll(dir)
}

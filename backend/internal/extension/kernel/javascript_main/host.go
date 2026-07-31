package javascript_main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/jsonrpc"
	"github.com/u-ai/backend/internal/extension/kernel/runtime"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type HostState string

const (
	HostStateCreated   HostState = "created"
	HostStateStarting  HostState = "starting"
	HostStateReady     HostState = "ready"
	HostStateStopping  HostState = "stopping"
	HostStateStopped   HostState = "stopped"
	HostStateCrashed   HostState = "crashed"
	HostStateUnhealthy HostState = "unhealthy"
	HostStateFailed    HostState = "failed"
)

type PluginHost struct {
	mu                  sync.RWMutex
	instanceID          string
	extensionID         string
	moduleID            string
	state               HostState
	spec                runtime.BootstrapSpec
	boundary            runtime.ProcessBoundary
	startedAt           *time.Time
	readyAt             *time.Time
	stoppedAt           *time.Time
	crashCount          int
	lastError           string
	handlers            *HandlerRegistry
	dispatcher          *InvocationDispatcher
	watchdog            *Watchdog
	session             *RuntimeSession
	shutdownCoordinator *ShutdownCoordinator
	rpcVersion          string
	definitionHash      string
	hostAPI             host_api.Gateway

	nodePath       string
	pluginHostPath string
	workDir        string
	env            []string
	networkDisabled bool
	expectedNonce  string
	cmd            *exec.Cmd
	stdin          io.WriteCloser
	stdoutPipe     io.ReadCloser
	stderrPipe     io.ReadCloser
	pid            int
	procCtx        context.Context
	procCancel     context.CancelFunc
	writeMu        sync.Mutex
	pending        map[string]chan *jsonrpc.Response
	pendingMu      sync.Mutex
	reqCounter     int64
	done           chan struct{}
	closeOnce      sync.Once
	exitErr        error
	helloCh        chan *jsonrpc.Notification
	readyCh        chan *jsonrpc.Notification
}

type PluginHostConfig struct {
	InstanceID           string
	ExtensionID          string
	ModuleID             string
	BootstrapSpec        runtime.BootstrapSpec
	ProcessBoundary      runtime.ProcessBoundary
	DefinitionHash       string
	HostAPIVersion       string
	AllowedContributions []AllowedContribution
	NodePath             string
	PluginHostPath       string
	WorkingDirectory     string
	HostAPI              host_api.Gateway
	Env                  []string
	NetworkDisabled      bool
}

type AllowedContribution struct {
	ContributionID string
	EntryType      string
	EntryName      string
}

func NewPluginHost(cfg PluginHostConfig) (*PluginHost, error) {
	if cfg.InstanceID == "" {
		return nil, errors.New("javascript_main: instance id required")
	}
	if cfg.ExtensionID == "" {
		return nil, errors.New("javascript_main: extension id required")
	}
	if cfg.ModuleID == "" {
		return nil, errors.New("javascript_main: module id required")
	}
	host := &PluginHost{
		instanceID:          cfg.InstanceID,
		extensionID:         cfg.ExtensionID,
		moduleID:            cfg.ModuleID,
		state:               HostStateCreated,
		spec:                cfg.BootstrapSpec,
		boundary:            cfg.ProcessBoundary,
		definitionHash:      cfg.DefinitionHash,
		rpcVersion:          cfg.HostAPIVersion,
		hostAPI:             cfg.HostAPI,
		nodePath:            cfg.NodePath,
		pluginHostPath:      cfg.PluginHostPath,
		workDir:             cfg.WorkingDirectory,
		env:                 cfg.Env,
		networkDisabled:     cfg.NetworkDisabled,
		handlers:            NewHandlerRegistry(cfg.AllowedContributions),
		dispatcher:          NewInvocationDispatcher(cfg.BootstrapSpec.ResourceLimits),
		watchdog:            NewWatchdog(cfg.InstanceID),
		shutdownCoordinator: NewShutdownCoordinator(),
		pending:             make(map[string]chan *jsonrpc.Response),
		done:                make(chan struct{}),
	}
	return host, nil
}

func (h *PluginHost) InstanceID() string  { return h.instanceID }
func (h *PluginHost) ExtensionID() string { return h.extensionID }
func (h *PluginHost) ModuleID() string    { return h.moduleID }

func (h *PluginHost) SetHostAPI(gateway host_api.Gateway) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.hostAPI = gateway
}

func (h *PluginHost) PID() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.pid
}

func (h *PluginHost) State() HostState {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.state
}

type StartResult struct {
	Success    bool
	InstanceID string
	Reason     string
	ReadyAt    *time.Time
	Steps      []StartStep
}

type StartStep struct {
	Name   string
	Status string
	Error  string
}

func (h *PluginHost) Start(ctx context.Context) StartResult {
	result := StartResult{InstanceID: h.instanceID}

	h.mu.Lock()
	if h.state != HostStateCreated {
		h.mu.Unlock()
		result.Reason = fmt.Sprintf("host in state %s, expected created", h.state)
		return result
	}
	if h.nodePath == "" {
		h.mu.Unlock()
		result.Reason = "javascript_main: node path required"
		return result
	}
	if h.pluginHostPath == "" {
		h.mu.Unlock()
		result.Reason = "javascript_main: plugin host path required"
		return result
	}
	h.state = HostStateStarting
	now := time.Now().UTC()
	h.startedAt = &now
	h.expectedNonce = generateNonce()
	h.helloCh = make(chan *jsonrpc.Notification, 1)
	h.readyCh = make(chan *jsonrpc.Notification, 1)
	h.mu.Unlock()

	sequence := runtime.DefaultBootstrapSequence()
	for _, step := range sequence.Steps {
		startStep := StartStep{Name: step.Name, Status: "succeeded"}
		if err := h.executeBootstrapStep(ctx, step.Name); err != nil {
			startStep.Status = "failed"
			startStep.Error = err.Error()
			result.Steps = append(result.Steps, startStep)
			result.Reason = fmt.Sprintf("bootstrap step %s failed: %v", step.Name, err)
			h.mu.Lock()
			h.state = HostStateFailed
			h.lastError = err.Error()
			stoppedNow := time.Now().UTC()
			h.stoppedAt = &stoppedNow
			h.mu.Unlock()
			h.cleanupProcess()
			return result
		}
		result.Steps = append(result.Steps, startStep)
	}

	h.mu.Lock()
	h.state = HostStateReady
	readyNow := time.Now().UTC()
	h.readyAt = &readyNow
	h.session = &RuntimeSession{
		InstanceID:     h.instanceID,
		ExtensionID:    h.extensionID,
		ModuleID:       h.moduleID,
		SessionToken:   h.spec.SessionToken,
		DefinitionHash: h.definitionHash,
		State:          runtime.SessionStateReady,
		StartedAt:      h.startedAt.Format(time.RFC3339),
		Ready:          true,
	}
	h.mu.Unlock()

	h.watchdog.Start(ctx, h)

	result.Success = true
	result.ReadyAt = h.readyAt
	return result
}

func (h *PluginHost) executeBootstrapStep(ctx context.Context, stepName string) error {
	switch stepName {
	case "process_start":
		return h.startProcess(ctx)
	case "read_bootstrap_spec":
		if h.spec.InstanceID == "" {
			return errors.New("bootstrap spec missing instance id")
		}
		if h.spec.Entry == "" {
			return errors.New("bootstrap spec missing entry")
		}
		return nil
	case "open_rpc_channel":
		if h.rpcVersion == "" {
			return errors.New("rpc version required")
		}
		return h.waitForHello(ctx)
	case "authenticate_session":
		if h.spec.SessionToken == "" {
			return errors.New("session token required")
		}
		return nil
	case "verify_definition":
		if h.definitionHash == "" {
			return errors.New("definition hash required")
		}
		return nil
	case "initialize_sdk":
		return h.sendInitialize(ctx)
	case "load_entry_module":
		if h.spec.Entry == "" {
			return errors.New("entry module required")
		}
		return nil
	case "call_activate":
		return nil
	case "report_ready":
		return h.waitForReady(ctx)
	}
	return nil
}

func (h *PluginHost) startProcess(ctx context.Context) error {
	procCtx, procCancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(procCtx, h.nodePath, h.pluginHostPath)
	if h.workDir != "" {
		cmd.Dir = h.workDir
	}
	cmd.Env = append(os.Environ(),
		"AMITIA_INSTANCE_ID="+h.instanceID,
		"AMITIA_EXTENSION_ID="+h.extensionID,
		"AMITIA_MODULE_ID="+h.moduleID,
		"AMITIA_NONCE="+h.expectedNonce,
		"AMITIA_HOST_API_VERSION="+h.rpcVersion,
		"AMITIA_DEFINITION_HASH="+h.definitionHash,
	)
	if h.networkDisabled {
		cmd.Env = append(cmd.Env,
			"AMITIA_NETWORK_DISABLED=1",
			"HTTP_PROXY=",
			"HTTPS_PROXY=",
			"NO_PROXY=*",
			"http_proxy=",
			"https_proxy=",
			"no_proxy=*",
			"NODE_OPTIONS=--no-experimental-fetch --disable-network-imports",
		)
	}
	cmd.Env = append(cmd.Env, h.env...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		procCancel()
		return fmt.Errorf("create stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		procCancel()
		return fmt.Errorf("create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		procCancel()
		return fmt.Errorf("create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		procCancel()
		return fmt.Errorf("start subprocess: %w", err)
	}

	h.mu.Lock()
	h.cmd = cmd
	h.stdin = stdin
	h.stdoutPipe = stdoutPipe
	h.stderrPipe = stderrPipe
	h.pid = cmd.Process.Pid
	h.procCtx = procCtx
	h.procCancel = procCancel
	h.mu.Unlock()

	go h.readLoop()
	go h.readStderr()

	return nil
}

func (h *PluginHost) readLoop() {
	h.mu.RLock()
	stdout := h.stdoutPipe
	h.mu.RUnlock()
	if stdout == nil {
		h.markDone()
		return
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		data := scanner.Bytes()
		if len(data) == 0 {
			continue
		}
		env, err := jsonrpc.DecodeEnvelope(data)
		if err != nil {
			continue
		}
		switch env.Kind {
		case jsonrpc.KindResponse, jsonrpc.KindError:
			h.deliverResponse(env.Response)
		case jsonrpc.KindNotification:
			h.handleNotification(env.Notification)
		case jsonrpc.KindRequest:
			h.handleRequest(env.Request)
		}
	}
	h.mu.Lock()
	h.exitErr = scanner.Err()
	h.mu.Unlock()
	h.markDone()
	h.mu.RLock()
	cmd := h.cmd
	h.mu.RUnlock()
	if cmd != nil {
		_ = cmd.Wait()
	}
	h.handleUnexpectedExit()
}

func (h *PluginHost) readStderr() {
	h.mu.RLock()
	stderr := h.stderrPipe
	h.mu.RUnlock()
	if stderr == nil {
		return
	}
	reader := bufio.NewReader(stderr)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			fmt.Fprintf(os.Stderr, "[js-runtime:%s] %s", h.instanceID, line)
		}
		if err != nil {
			return
		}
	}
}

func (h *PluginHost) handleNotification(n *jsonrpc.Notification) {
	switch n.Method {
	case "runtime.hello":
		select {
		case h.helloCh <- n:
		default:
		}
	case "runtime.ready":
		select {
		case h.readyCh <- n:
		default:
		}
	case "log.write":
		var p struct {
			Level   string `json:"level"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(n.Params, &p); err == nil {
			fmt.Fprintf(os.Stderr, "[js-runtime:%s][%s] %s\n", h.instanceID, p.Level, p.Message)
		}
	}
}

func (h *PluginHost) handleRequest(req *jsonrpc.Request) {
	switch req.Method {
	case "host.call":
		h.handleHostCall(req)
	default:
		resp := jsonrpc.EncodeErrorResponse(req.ID, jsonrpc.MethodNotFoundError(req.Method))
		_ = h.writeMessage(resp)
	}
}

type hostCallParams struct {
	Method  string          `json:"method"`
	Version int             `json:"version"`
	Input   json.RawMessage `json:"input"`
}

func (h *PluginHost) handleHostCall(req *jsonrpc.Request) {
	h.mu.RLock()
	gateway := h.hostAPI
	procCtx := h.procCtx
	h.mu.RUnlock()

	if gateway == nil {
		resp := jsonrpc.EncodeErrorResponse(req.ID, jsonrpc.InternalError("host_api: gateway not configured"))
		_ = h.writeMessage(resp)
		return
	}

	if procCtx == nil {
		procCtx = context.Background()
	}

	var p hostCallParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		resp := jsonrpc.EncodeErrorResponse(req.ID, jsonrpc.InternalError(fmt.Sprintf("host.call: decode params: %v", err)))
		_ = h.writeMessage(resp)
		return
	}

	identity := runtime_supervisor.RuntimeIdentity{
		InstanceID:  h.instanceID,
		ExtensionID: domain.ExtensionID(h.extensionID),
		ModuleID:    domain.ModuleID(h.moduleID),
	}

	callReq := host_api.CallRequest{
		CallID:          req.ID.String(),
		RuntimeIdentity: identity,
		Method:          host_api.Method(p.Method),
		Version:         p.Version,
		Input:           p.Input,
	}

	result := gateway.Call(procCtx, callReq)

	if result.Error != nil {
		resp := jsonrpc.EncodeErrorResponse(req.ID, jsonrpc.InternalError(fmt.Sprintf("host.call: %s: %s", result.Error.Code, result.Error.Message)))
		_ = h.writeMessage(resp)
		return
	}

	resp, err := jsonrpc.EncodeResponse(req.ID, result.Output)
	if err != nil {
		errResp := jsonrpc.EncodeErrorResponse(req.ID, jsonrpc.InternalError(fmt.Sprintf("host.call: encode response: %v", err)))
		_ = h.writeMessage(errResp)
		return
	}
	_ = h.writeMessage(resp)
}

func (h *PluginHost) deliverResponse(resp *jsonrpc.Response) {
	h.pendingMu.Lock()
	ch, ok := h.pending[resp.ID.String()]
	if ok {
		delete(h.pending, resp.ID.String())
	}
	h.pendingMu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- resp:
	default:
	}
}

func (h *PluginHost) waitForHello(ctx context.Context) error {
	select {
	case n := <-h.helloCh:
		var p struct {
			InstanceID string `json:"instanceId"`
			Nonce      string `json:"nonce"`
		}
		if err := json.Unmarshal(n.Params, &p); err != nil {
			return fmt.Errorf("decode hello: %w", err)
		}
		if p.InstanceID != h.instanceID {
			return fmt.Errorf("hello instance id mismatch: %s != %s", p.InstanceID, h.instanceID)
		}
		if p.Nonce == "" || p.Nonce != h.expectedNonce {
			return errors.New("hello nonce mismatch")
		}
		return nil
	case <-time.After(10 * time.Second):
		return errors.New("timeout waiting for runtime.hello")
	case <-ctx.Done():
		return ctx.Err()
	case <-h.done:
		h.mu.RLock()
		err := h.exitErr
		h.mu.RUnlock()
		return fmt.Errorf("process exited before hello: %v", err)
	}
}

func (h *PluginHost) sendInitialize(ctx context.Context) error {
	params := map[string]interface{}{
		"instanceId":     h.instanceID,
		"extensionId":    h.extensionID,
		"moduleId":       h.moduleID,
		"entry":          h.spec.Entry,
		"definitionHash": h.definitionHash,
		"hostApiVersion": h.rpcVersion,
		"sessionToken":   h.spec.SessionToken,
	}
	resp, err := h.sendRequest(ctx, "runtime.initialize", params)
	if err != nil {
		return fmt.Errorf("runtime.initialize: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("runtime.initialize error: %s", resp.Error.Message)
	}
	return nil
}

func (h *PluginHost) waitForReady(ctx context.Context) error {
	select {
	case <-h.readyCh:
		return nil
	case <-time.After(15 * time.Second):
		return errors.New("timeout waiting for runtime.ready")
	case <-ctx.Done():
		return ctx.Err()
	case <-h.done:
		h.mu.RLock()
		err := h.exitErr
		h.mu.RUnlock()
		return fmt.Errorf("process exited before ready: %v", err)
	}
}

func (h *PluginHost) sendRequest(ctx context.Context, method string, params interface{}) (*jsonrpc.Response, error) {
	id := atomic.AddInt64(&h.reqCounter, 1)
	reqID := jsonrpc.NewNumberID(id)
	req, err := jsonrpc.EncodeRequest(reqID, method, params)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	ch := make(chan *jsonrpc.Response, 1)
	h.pendingMu.Lock()
	h.pending[reqID.String()] = ch
	h.pendingMu.Unlock()

	if err := h.writeMessage(req); err != nil {
		h.pendingMu.Lock()
		delete(h.pending, reqID.String())
		h.pendingMu.Unlock()
		return nil, fmt.Errorf("write request: %w", err)
	}

	select {
	case resp := <-ch:
		if resp == nil {
			return nil, errors.New("javascript_main: request cancelled: runtime exiting")
		}
		return resp, nil
	case <-ctx.Done():
		h.pendingMu.Lock()
		delete(h.pending, reqID.String())
		h.pendingMu.Unlock()
		_ = h.sendNotification("runtime.cancel", map[string]interface{}{"requestId": reqID.String(), "reason": ctx.Err().Error()})
		return nil, ctx.Err()
	case <-h.done:
		h.pendingMu.Lock()
		delete(h.pending, reqID.String())
		h.pendingMu.Unlock()
		h.mu.RLock()
		err := h.exitErr
		h.mu.RUnlock()
		return nil, fmt.Errorf("javascript_main: process exited: %v", err)
	}
}

func (h *PluginHost) sendNotification(method string, params interface{}) error {
	n, err := jsonrpc.EncodeNotification(method, params)
	if err != nil {
		return err
	}
	return h.writeMessage(n)
}

func (h *PluginHost) writeMessage(msg interface{}) error {
	data, err := jsonrpc.MarshalMessage(msg)
	if err != nil {
		return err
	}
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	if h.stdin == nil {
		return errors.New("javascript_main: stdin closed")
	}
	if _, err := h.stdin.Write(data); err != nil {
		return err
	}
	_, err = h.stdin.Write([]byte("\n"))
	return err
}

func (h *PluginHost) Invoke(ctx context.Context, contributionID string, input interface{}) (interface{}, error) {
	if h.State() != HostStateReady {
		return nil, errors.New("javascript_main: host not ready")
	}
	params := invokeParams{ContributionID: contributionID, Input: input}
	timeout := 30 * time.Second
	if h.spec.ResourceLimits.SingleCallTimeout != "" {
		if d, err := time.ParseDuration(h.spec.ResourceLimits.SingleCallTimeout); err == nil && d > 0 {
			timeout = d
		}
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := h.sendRequest(callCtx, "runtime.invoke", params)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("invoke error: %s", resp.Error.Message)
	}
	var result interface{}
	if len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return nil, fmt.Errorf("decode invoke result: %w", err)
		}
	}
	return result, nil
}

type invokeParams struct {
	ContributionID string      `json:"contributionId"`
	Input          interface{} `json:"input"`
}

func (h *PluginHost) Stop(ctx context.Context, reason string) error {
	h.mu.Lock()
	if h.state == HostStateStopped || h.state == HostStateCrashed || h.state == HostStateFailed {
		h.mu.Unlock()
		return nil
	}
	if h.state != HostStateReady && h.state != HostStateUnhealthy && h.state != HostStateStarting {
		h.mu.Unlock()
		return fmt.Errorf("javascript_main: cannot stop host in state %s", h.state)
	}
	h.state = HostStateStopping
	h.mu.Unlock()

	h.watchdog.Stop()
	h.shutdownCoordinator.BeginShutdown()
	h.dispatcher.RejectNewInvocations()
	h.dispatcher.CancelQueued(reason)

	completed := h.dispatcher.WaitForRunning(ctx, 5*time.Second)
	if !completed {
		h.shutdownCoordinator.MarkForceStopped()
	}

	_ = h.sendNotification("runtime.shutdown", map[string]interface{}{"reason": reason})

	h.shutdownCoordinator.MarkDeactivateCalled()

	h.waitForExit(5 * time.Second)
	h.cleanupProcess()

	h.mu.Lock()
	h.state = HostStateStopped
	now := time.Now().UTC()
	h.stoppedAt = &now
	h.mu.Unlock()

	h.shutdownCoordinator.MarkSessionClosed()
	h.shutdownCoordinator.MarkStoppedSent()
	h.shutdownCoordinator.Complete()

	return nil
}

func (h *PluginHost) waitForExit(timeout time.Duration) {
	h.mu.RLock()
	cmd := h.cmd
	procCancel := h.procCancel
	h.mu.RUnlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-h.done:
		return
	case <-timer.C:
		if procCancel != nil {
			procCancel()
		}
		<-h.done
	}
}

func (h *PluginHost) cleanupProcess() {
	h.writeMu.Lock()
	if h.stdin != nil {
		_ = h.stdin.Close()
		h.stdin = nil
	}
	h.writeMu.Unlock()
	h.mu.Lock()
	procCancel := h.procCancel
	h.cmd = nil
	h.stdoutPipe = nil
	h.stderrPipe = nil
	h.pid = 0
	h.procCancel = nil
	h.mu.Unlock()
	if procCancel != nil {
		procCancel()
	}
	h.pendingMu.Lock()
	for id, ch := range h.pending {
		select {
		case ch <- nil:
		default:
		}
		delete(h.pending, id)
	}
	h.pendingMu.Unlock()
}

func (h *PluginHost) markDone() {
	h.closeOnce.Do(func() {
		close(h.done)
	})
}

func (h *PluginHost) handleUnexpectedExit() {
	h.mu.RLock()
	state := h.state
	exitErr := h.exitErr
	h.mu.RUnlock()
	if state == HostStateStopping || state == HostStateStopped || state == HostStateFailed {
		return
	}
	h.MarkCrashed(fmt.Sprintf("process exited unexpectedly: %v", exitErr))
}

func (h *PluginHost) MarkCrashed(reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state = HostStateCrashed
	h.crashCount++
	h.lastError = reason
	now := time.Now().UTC()
	h.stoppedAt = &now
}

func (h *PluginHost) CrashCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.crashCount
}

func (h *PluginHost) LastError() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastError
}

func (h *PluginHost) Handlers() *HandlerRegistry        { return h.handlers }
func (h *PluginHost) Dispatcher() *InvocationDispatcher { return h.dispatcher }
func (h *PluginHost) Watchdog() *Watchdog               { return h.watchdog }
func (h *PluginHost) Session() *RuntimeSession {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.session
}

type RuntimeSession struct {
	InstanceID     string
	ExtensionID    string
	ModuleID       string
	SessionToken   string
	DefinitionHash string
	State          runtime.SessionState
	StartedAt      string
	Ready          bool
}

func (h *PluginHost) Health() HealthReport {
	h.mu.RLock()
	defer h.mu.RUnlock()
	report := HealthReport{
		InstanceID:  h.instanceID,
		ExtensionID: h.extensionID,
		ModuleID:    h.moduleID,
		State:       h.state,
		CrashCount:  h.crashCount,
	}
	if h.watchdog != nil {
		report.Watchdog = h.watchdog.Report()
	}
	if h.dispatcher != nil {
		report.ActiveInvocations = h.dispatcher.ActiveCount()
		report.QueuedInvocations = h.dispatcher.QueuedCount()
	}
	return report
}

type HealthReport struct {
	InstanceID        string
	ExtensionID       string
	ModuleID          string
	State             HostState
	CrashCount        int
	ActiveInvocations int
	QueuedInvocations int
	Watchdog          WatchdogReport
}

func generateNonce() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

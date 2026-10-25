package javascript_main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/jsonrpc"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/runtime"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	secretpkg "github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/platform/process"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type HostState string

const (
	pluginHostOutboundQueueSize = 512
	pluginHostMaxMessageBytes   = 64 * 1024 * 1024
	pluginHostWriteTimeout      = 60 * time.Second
	pluginHostEnqueueTimeout    = 30 * time.Second
)

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

	nodePath        string
	pluginHostPath  string
	workDir         string
	env             []string
	networkDisabled bool
	hostAccess      PluginHostAccess
	expectedNonce   string
	cmd             *exec.Cmd
	stdin           io.WriteCloser
	stdoutPipe      io.ReadCloser
	stderrPipe      io.ReadCloser
	pid             int
	processTree     process.ProcessTreeHandle
	procCtx         context.Context
	procCancel      context.CancelFunc
	writeMu         sync.Mutex
	outbound        chan []byte
	writerOnce      sync.Once
	writerErr       error
	pending         map[string]chan *jsonrpc.Response
	pendingMu       sync.Mutex
	invocations     map[string]hostInvocationContext
	invocationsMu   sync.Mutex
	reqCounter      int64
	done            chan struct{}
	closeOnce       sync.Once
	exitErr         error
	helloCh         chan *jsonrpc.Notification
	readyCh         chan *jsonrpc.Notification
	stderrMu        sync.Mutex
	stderrTail      []string
	stderrWG        sync.WaitGroup
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
	HostAccess           PluginHostAccess
}

type PluginHostAccess struct {
	FilesystemMode    string
	FilesystemRoots   []string
	FilesystemRootMap map[string]string
	NetworkMode       string
	ProcessMode       string
	MaxProcesses      int
}

type AllowedContribution struct {
	ContributionID string
	EntryType      string
	EntryName      string
}

type hostInvocationContext struct {
	ctx     context.Context
	context capability.ToolInvocationContext
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
	if cfg.NodePath == "" {
		return nil, errors.New("javascript_main: node path required")
	}
	if !filepath.IsAbs(cfg.NodePath) {
		return nil, errors.New("javascript_main: node path must be absolute")
	}
	if cfg.PluginHostPath == "" {
		return nil, errors.New("javascript_main: plugin host path required")
	}
	if !filepath.IsAbs(cfg.PluginHostPath) {
		return nil, errors.New("javascript_main: plugin host path must be absolute")
	}
	if !isHostEntryExt(cfg.PluginHostPath) {
		return nil, fmt.Errorf("javascript_main: unsupported plugin host entry: %s", filepath.Ext(cfg.PluginHostPath))
	}
	if cfg.WorkingDirectory == "" {
		return nil, errors.New("javascript_main: working directory required")
	}
	if !filepath.IsAbs(cfg.WorkingDirectory) {
		return nil, errors.New("javascript_main: working directory must be absolute")
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
		hostAccess:          cfg.HostAccess,
		handlers:            NewHandlerRegistry(cfg.AllowedContributions),
		dispatcher:          NewInvocationDispatcher(cfg.BootstrapSpec.ResourceLimits),
		watchdog:            NewWatchdog(cfg.InstanceID),
		shutdownCoordinator: NewShutdownCoordinator(),
		pending:             make(map[string]chan *jsonrpc.Response),
		invocations:         make(map[string]hostInvocationContext),
		outbound:            make(chan []byte, pluginHostOutboundQueueSize),
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
	cmd := exec.CommandContext(procCtx, h.nodePath, h.nodeLaunchArgs()...)
	if h.workDir != "" {
		cmd.Dir = h.workDir
	}

	cmd.Env = h.buildProcessEnvironment()

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

	process.ConfigureProcess(cmd)
	limits := process.ResourceLimits{}
	if h.hostAccess.MaxProcesses > 0 {
		limits.MaxProcesses = uint32(h.hostAccess.MaxProcesses + 1)
	}
	tree, err := process.StartProcessTreeWithLimits(cmd, limits)
	if err != nil {
		procCancel()
		return fmt.Errorf("start subprocess: %w", err)
	}

	h.mu.Lock()
	h.cmd = cmd
	h.stdin = stdin
	h.stdoutPipe = stdoutPipe
	h.stderrPipe = stderrPipe
	h.pid = cmd.Process.Pid
	h.processTree = tree
	h.procCtx = procCtx
	h.procCancel = procCancel
	h.mu.Unlock()

	go h.readLoop()
	h.startWriter()
	h.stderrWG.Add(1)
	go func() {
		defer h.stderrWG.Done()
		h.readStderr()
	}()

	return nil
}

func (h *PluginHost) nodeLaunchArgs() []string {
	args := []string{"--permission"}
	seenRead := map[string]struct{}{}
	addRead := func(root string) {
		root = strings.TrimSpace(root)
		if root == "" {
			return
		}
		if _, exists := seenRead[root]; exists {
			return
		}
		seenRead[root] = struct{}{}
		args = append(args, "--allow-fs-read="+root)
	}
	addRead(filepath.Dir(h.pluginHostPath))
	if h.spec.Entry != "" {
		addRead(filepath.Dir(h.spec.Entry))
	}
	mode := strings.ToLower(strings.TrimSpace(h.hostAccess.FilesystemMode))
	if mode == "read" || mode == "read_write" {
		for _, root := range h.hostAccess.FilesystemRoots {
			addRead(root)
			if mode == "read_write" {
				args = append(args, "--allow-fs-write="+root)
			}
		}
	}
	if strings.EqualFold(strings.TrimSpace(h.hostAccess.ProcessMode), "direct") {
		args = append(args, "--allow-child-process")
	}
	return append(args, h.pluginHostPath)
}

// startWriter decouples stdout reading from stdin writing. A dedicated
// goroutine drains the outbound queue so the read loop can never stall behind
// a slow or wedged plugin process.
func (h *PluginHost) startWriter() {
	h.writerOnce.Do(func() {
		go func() {
			for {
				select {
				case <-h.done:
					return
				case payload := <-h.outbound:
					if err := h.writeToProcess(payload); err != nil {
						h.mu.Lock()
						if h.writerErr == nil {
							h.writerErr = err
						}
						h.mu.Unlock()
						fmt.Fprintf(os.Stderr, "[js-runtime:%s] stdin write failed: %v\n", h.instanceID, err)
						h.terminateProcess("stdin write failed")
						return
					}
				}
			}
		}()
	})
}

func (h *PluginHost) writeToProcess(payload []byte) error {
	h.mu.RLock()
	stdin := h.stdin
	h.mu.RUnlock()
	if stdin == nil {
		return errors.New("javascript_main: stdin closed")
	}

	h.writeMu.Lock()
	defer h.writeMu.Unlock()

	written := make(chan error, 1)
	go func() {
		_, err := stdin.Write(payload)
		written <- err
	}()

	select {
	case err := <-written:
		return err
	case <-time.After(pluginHostWriteTimeout):
		return fmt.Errorf("javascript_main: stdin write timeout after %s", pluginHostWriteTimeout)
	}
}

func (h *PluginHost) terminateProcess(reason string) {
	h.mu.RLock()
	proc := h.cmd
	cancel := h.procCancel
	tree := h.processTree
	h.mu.RUnlock()

	if reason != "" {
		fmt.Fprintf(os.Stderr, "[js-runtime:%s] terminating runtime: %s\n", h.instanceID, reason)
	}
	if proc != nil && proc.Process != nil {
		if tree != 0 {
			_ = process.TerminateProcessTree(proc.Process.Pid, tree)
		} else {
			_ = proc.Process.Kill()
		}
	}
	if cancel != nil {
		cancel()
	}
}

func (h *PluginHost) buildProcessEnvironment() []string {
	env := process.NewEnvironmentBuilder().
		SetRuntimeInstance(h.instanceID).
		SetExtensionID(h.extensionID).
		SetModuleID(h.moduleID).
		Set("AMITIA_NONCE", h.expectedNonce).
		Set("AMITIA_HOST_API_VERSION", h.rpcVersion).
		Set("AMITIA_DEFINITION_HASH", h.definitionHash)

	if h.networkDisabled {
		env.Set("AMITIA_NETWORK_DISABLED", "1")
	}
	env.Set("AMITIA_HOST_ACCESS_VERSION", "1")
	env.Set("AMITIA_HOST_FILESYSTEM_MODE", defaultHostAccessMode(h.hostAccess.FilesystemMode, "none"))
	env.Set("AMITIA_HOST_FILESYSTEM_ROOTS", strings.Join(h.hostAccess.FilesystemRoots, string(os.PathListSeparator)))
	if encoded, err := json.Marshal(h.hostAccess.FilesystemRootMap); err == nil {
		env.Set("AMITIA_HOST_FILESYSTEM_ROOTS_JSON", string(encoded))
	}
	env.Set("AMITIA_HOST_NETWORK_MODE", defaultHostAccessMode(h.hostAccess.NetworkMode, "none"))
	env.Set("AMITIA_HOST_PROCESS_MODE", defaultHostAccessMode(h.hostAccess.ProcessMode, "none"))
	env.Set("AMITIA_HOST_PROCESS_LIMIT", fmt.Sprintf("%d", h.hostAccess.MaxProcesses))

	for _, e := range h.env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 && !secretpkg.IsSensitiveEnvKey(parts[0]) {
			env.Set(parts[0], parts[1])
		}
	}

	return env.Build()
}

func defaultHostAccessMode(value, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	return value
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
	scanner.Buffer(make([]byte, 0, 1024*1024), pluginHostMaxMessageBytes)
	for scanner.Scan() {
		frame := scanner.Bytes()
		if len(frame) == 0 {
			continue
		}
		// The scanner reuses its buffer, so copy before dispatching work that
		// may outlive this iteration.
		data := make([]byte, len(frame))
		copy(data, frame)
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
			req := env.Request
			go h.handleRequest(req)
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		h.mu.Lock()
		if h.exitErr == nil {
			h.exitErr = scanErr
		}
		h.mu.Unlock()
		fmt.Fprintf(os.Stderr, "[js-runtime:%s] stdout read failed: %v\n", h.instanceID, scanErr)
		h.terminateProcess("stdout read failed")
	}
	h.mu.RLock()
	cmd := h.cmd
	h.mu.RUnlock()
	if cmd != nil {
		waitErr := cmd.Wait()
		h.mu.Lock()
		if waitErr != nil {
			h.exitErr = waitErr
		} else {
			h.exitErr = scanner.Err()
		}
		h.mu.Unlock()
	}
	h.stderrWG.Wait()
	h.markDone()
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
			h.appendStderr(line)
			fmt.Fprintf(os.Stderr, "[js-runtime:%s] %s", h.instanceID, line)
		}
		if err != nil {
			return
		}
	}
}

func (h *PluginHost) appendStderr(line string) {
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return
	}
	h.stderrMu.Lock()
	defer h.stderrMu.Unlock()
	h.stderrTail = append(h.stderrTail, line)
	if len(h.stderrTail) > 50 {
		h.stderrTail = append([]string(nil), h.stderrTail[len(h.stderrTail)-50:]...)
	}
}

func (h *PluginHost) stderrSnapshot() string {
	h.stderrMu.Lock()
	defer h.stderrMu.Unlock()
	return strings.Join(h.stderrTail, "\n")
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
			safeMsg := redactLogLine(p.Message)
			fmt.Fprintf(os.Stderr, "[js-runtime:%s][%s] %s\n", h.instanceID, p.Level, safeMsg)
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
	Method               string                                `json:"method"`
	Version              int                                   `json:"version"`
	Input                json.RawMessage                       `json:"input"`
	ScopeSnapshotID      string                                `json:"scopeSnapshotId"`
	PermissionSnapshotID string                                `json:"permissionSnapshotId"`
	InvocationID         string                                `json:"invocationId"`
	ParentID             string                                `json:"parentId"`
	TraceID              string                                `json:"traceId"`
	ExecutionContext     permission.PermissionExecutionContext `json:"executionContext"`
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
	h.invocationsMu.Lock()
	invocationContext, hasInvocationContext := h.invocations[p.InvocationID]
	h.invocationsMu.Unlock()
	callCtx := procCtx
	if hasInvocationContext {
		if invocationContext.ctx != nil {
			callCtx = invocationContext.ctx
		}
		callCtx = withHostInvocationActor(callCtx, invocationContext.context)
		if p.ScopeSnapshotID == "" {
			p.ScopeSnapshotID = invocationContext.context.ScopeSnapshotID
		}
		if p.PermissionSnapshotID == "" {
			p.PermissionSnapshotID = invocationContext.context.PermissionSnapshotID
		}
		if p.InvocationID == "" {
			p.InvocationID = invocationContext.context.InvocationID
		}
		if p.TraceID == "" {
			p.TraceID = invocationContext.context.TraceID
		}
	}

	identity := runtime_supervisor.RuntimeIdentity{
		InstanceID:  h.instanceID,
		ExtensionID: domain.ExtensionID(h.extensionID),
		ModuleID:    domain.ModuleID(h.moduleID),
	}

	executionContext := p.ExecutionContext.Normalize()
	if hasInvocationContext {
		executionContext.Placement = permission.ExecutionPlacement(invocationContext.context.ExecutionTarget.Placement)
		executionContext.SpaceID = invocationContext.context.ExecutionTarget.SpaceID
		executionContext.DeviceID = invocationContext.context.ExecutionTarget.DeviceID
		executionContext.RuntimeID = invocationContext.context.ExecutionTarget.RuntimeID
		executionContext.RuntimeSessionID = invocationContext.context.ExecutionTarget.RuntimeSessionID
		executionContext.ProviderID = invocationContext.context.ExecutionTarget.ProviderID
		executionContext.ProviderInstanceID = invocationContext.context.ExecutionTarget.ProviderInstanceID
		executionContext.ExtensionID = invocationContext.context.ExtensionID
		executionContext.ModuleID = invocationContext.context.ModuleID
		executionContext.Source = string(invocationContext.context.Source)
	}
	if executionContext.ExtensionID == "" {
		executionContext.ExtensionID = h.extensionID
	}
	if executionContext.ModuleID == "" {
		executionContext.ModuleID = h.moduleID
	}
	if executionContext.Source == "" {
		executionContext.Source = "plugin"
	}
	callReq := host_api.CallRequest{
		CallID:               req.ID.String(),
		RuntimeIdentity:      identity,
		Method:               host_api.Method(p.Method),
		Version:              p.Version,
		Input:                p.Input,
		ScopeSnapshotID:      p.ScopeSnapshotID,
		PermissionSnapshotID: p.PermissionSnapshotID,
		TraceID:              p.TraceID,
		InvocationID:         p.InvocationID,
		ParentID:             p.ParentID,
		ExecutionContext:     executionContext,
	}

	result := gateway.Call(callCtx, callReq)

	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "[host-api] method=%s invocation=%s error=%s: %s\n", p.Method, p.InvocationID, result.Error.Code, result.Error.Message)
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

func withHostInvocationActor(ctx context.Context, invocation capability.ToolInvocationContext) context.Context {
	target := invocation.ExecutionTarget
	spaceID := target.SpaceID
	if spaceID == "" {
		spaceID = runtimeidentity.ParseSpaceID(invocation.SpaceID)
	}
	if spaceID != "" || invocation.CharacterID != "" || invocation.ConversationID != "" {
		ctx = host_api.WithScopeContext(ctx, host_api.ScopeContext{
			SpaceID:        spaceID.String(),
			CharacterID:    invocation.CharacterID,
			ConversationID: invocation.ConversationID,
		})
	}
	if actor, ok := auth.FromContext(ctx); ok && actor != nil {
		return ctx
	}
	if spaceID == "" || target.DeviceID == "" {
		return ctx
	}
	return auth.WithActor(ctx, &auth.ActorContext{
		PrincipalType:    auth.PrincipalTrustedDevice,
		SpaceID:          spaceID,
		DeviceID:         target.DeviceID,
		RuntimeID:        target.RuntimeID,
		SessionID:        invocation.SessionID,
		RuntimeSessionID: target.RuntimeSessionID,
	})
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
		if stderr := h.stderrSnapshot(); stderr != "" {
			return fmt.Errorf("process exited before hello: %v: %s", err, stderr)
		}
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
		if stderr := h.stderrSnapshot(); stderr != "" {
			return fmt.Errorf("process exited before ready: %v: %s", err, stderr)
		}
		return fmt.Errorf("process exited before ready: %v", err)
	}
}

func (h *PluginHost) sendRequest(ctx context.Context, method string, params interface{}) (*jsonrpc.Response, error) {
	return h.sendRequestWithInvocation(ctx, method, params, nil)
}

func (h *PluginHost) sendRequestWithInvocation(ctx context.Context, method string, params interface{}, invocation *capability.ToolInvocationContext) (*jsonrpc.Response, error) {
	id := atomic.AddInt64(&h.reqCounter, 1)
	reqID := jsonrpc.NewStringID(fmt.Sprintf("host-%d", id))
	if invocation != nil {
		h.invocationsMu.Lock()
		h.invocations[reqID.String()] = hostInvocationContext{ctx: ctx, context: *invocation}
		h.invocationsMu.Unlock()
		defer func() {
			h.invocationsMu.Lock()
			delete(h.invocations, reqID.String())
			h.invocationsMu.Unlock()
		}()
	}
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
		_ = h.sendNotification("runtime.cancel", map[string]interface{}{
			"invocationId": reqID.String(),
			"requestId":    reqID.String(),
			"reason":       ctx.Err().Error(),
		})
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
	if len(data) > pluginHostMaxMessageBytes {
		return fmt.Errorf("javascript_main: outbound message too large: %d bytes (limit %d)", len(data), pluginHostMaxMessageBytes)
	}

	payload := make([]byte, 0, len(data)+1)
	payload = append(payload, data...)
	payload = append(payload, '\n')

	select {
	case h.outbound <- payload:
		return nil
	case <-time.After(pluginHostEnqueueTimeout):
		return errors.New("javascript_main: outbound queue saturated")
	case <-h.done:
		return errors.New("javascript_main: runtime exiting")
	}
}

func (h *PluginHost) Invoke(ctx context.Context, contributionID string, input interface{}) (interface{}, error) {
	return h.InvokeWithContext(ctx, contributionID, input, capability.ToolInvocationContext{})
}

func (h *PluginHost) InvokeWithContext(ctx context.Context, contributionID string, input interface{}, invocation capability.ToolInvocationContext) (interface{}, error) {
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
	callCtx, cancel := timeoutpolicy.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := h.sendRequestWithInvocation(callCtx, "runtime.invoke", params, &invocation)
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
	tree := h.processTree
	h.cmd = nil
	h.stdoutPipe = nil
	h.stderrPipe = nil
	h.pid = 0
	h.processTree = 0
	h.procCancel = nil
	h.mu.Unlock()
	if tree != 0 {
		process.CloseProcessTree(tree)
	}
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

func isHostEntryExt(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".mjs", ".cjs":
		return true
	}
	return false
}

func generateNonce() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}

func redactLogLine(line string) string {
	return secretpkg.RedactLogLine(line)
}

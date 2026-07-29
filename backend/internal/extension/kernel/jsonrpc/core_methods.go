package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type MethodGroup string

const (
	GroupRuntime    MethodGroup = "runtime"
	GroupHost       MethodGroup = "host"
	GroupStream     MethodGroup = "stream"
	GroupTask       MethodGroup = "task"
	GroupDiagnostic MethodGroup = "diagnostic"
	GroupLifecycle  MethodGroup = "lifecycle"
)

func methodOf(group MethodGroup, name string) string {
	return string(group) + "." + name
}

type CoreMethods struct {
	mu              sync.RWMutex
	session         *Session
	registry        *MethodRegistry
	streams         *StreamRegistry
	tracker         *RequestTracker
	bp              *BackpressureMeter
	cancelReg       *CancellationRegistry
	healthProvider  HealthProvider
	shutdownHandler ShutdownHandler
	uptime          time.Time
}

type HealthProvider func(ctx context.Context, includeDetails bool) (*HealthResponse, error)
type ShutdownHandler func(ctx context.Context, reason string, grace time.Duration) error

func NewCoreMethods(session *Session, registry *MethodRegistry, streams *StreamRegistry, tracker *RequestTracker, bp *BackpressureMeter, cancelReg *CancellationRegistry) *CoreMethods {
	return &CoreMethods{
		session:   session,
		registry:  registry,
		streams:   streams,
		tracker:   tracker,
		bp:        bp,
		cancelReg: cancelReg,
		uptime:    time.Now().UTC(),
	}
}

func (m *CoreMethods) SetHealthProvider(p HealthProvider) {
	m.mu.Lock()
	m.healthProvider = p
	m.mu.Unlock()
}

func (m *CoreMethods) SetShutdownHandler(h ShutdownHandler) {
	m.mu.Lock()
	m.shutdownHandler = h
	m.mu.Unlock()
}

func (m *CoreMethods) RegisterAll() {
	m.registry.Register(methodOf(GroupRuntime, "invoke"), m.handleInvoke)
	m.registry.Register(methodOf(GroupRuntime, "cancel"), m.handleCancel)
	m.registry.Register(methodOf(GroupRuntime, "health"), m.handleHealth)
	m.registry.Register(methodOf(GroupRuntime, "shutdown"), m.handleShutdown)
	m.registry.Register(methodOf(GroupRuntime, "reload"), m.handleReload)
	m.registry.Register(methodOf(GroupHost, "call"), m.handleHostCall)
	m.registry.Register(methodOf(GroupStream, "open"), m.handleStreamOpen)
	m.registry.Register(methodOf(GroupStream, "chunk"), m.handleStreamChunk)
	m.registry.Register(methodOf(GroupStream, "close"), m.handleStreamClose)
	m.registry.Register(methodOf(GroupStream, "cancel"), m.handleStreamCancel)
	m.registry.Register(methodOf(GroupStream, "credit"), m.handleStreamCredit)
	m.registry.Register(methodOf(GroupTask, "enqueue"), m.handleTaskEnqueue)
	m.registry.Register(methodOf(GroupTask, "execute"), m.handleTaskExecute)
	m.registry.Register(methodOf(GroupTask, "cancel"), m.handleTaskCancel)
	m.registry.Register(methodOf(GroupTask, "checkpoint"), m.handleTaskCheckpoint)
	m.registry.Register(methodOf(GroupDiagnostic, "ping"), m.handlePing)
	m.registry.Register(methodOf(GroupDiagnostic, "stats"), m.handleStats)
	m.registry.Register(methodOf(GroupLifecycle, "activate"), m.handleLifecycleActivate)
	m.registry.Register(methodOf(GroupLifecycle, "deactivate"), m.handleLifecycleDeactivate)
}

type InvokeParams struct {
	Entry          string            `json:"entry"`
	Input          json.RawMessage   `json:"input"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
	StreamID       string            `json:"stream_id,omitempty"`
	Timeout        time.Duration     `json:"timeout,omitempty"`
	Trace          map[string]string `json:"trace,omitempty"`
}

type InvokeResult struct {
	Output   json.RawMessage `json:"output"`
	Duration time.Duration   `json:"duration"`
}

type InvokeHandler func(ctx context.Context, p InvokeParams) (*InvokeResult, error)

var (
	invokeHandlerKey = "jsonrpc:core:invoke:handler"
)

func (m *CoreMethods) SetInvokeHandler(h InvokeHandler) {
	m.registry.Use(func(ctx context.Context, params json.RawMessage, next HandlerFunc) (any, error) {
		return next(context.WithValue(ctx, &invokeHandlerKey, h), params)
	})
}

func (m *CoreMethods) handleInvoke(ctx context.Context, params json.RawMessage) (any, error) {
	var p InvokeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, InvalidParamsError(err.Error())
	}
	h, ok := ctx.Value(&invokeHandlerKey).(InvokeHandler)
	if !ok || h == nil {
		return nil, MethodNotFoundError("runtime.invoke handler not registered")
	}
	return h(ctx, p)
}

func (m *CoreMethods) handleCancel(ctx context.Context, params json.RawMessage) (any, error) {
	var p CancelParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, InvalidParamsError(err.Error())
	}
	if err := m.tracker.Cancel(NewStringID(p.TargetRequestID), p.Reason); err != nil {
		return nil, NewError(ErrCodeRequestNotFound, err.Error(), false, CategoryProtocol)
	}
	if m.cancelReg != nil {
		m.cancelReg.Cancel(p.TargetRequestID, p.Reason)
	}
	return map[string]any{"cancelled": true}, nil
}

func (m *CoreMethods) handleHealth(ctx context.Context, params json.RawMessage) (any, error) {
	var p HealthRequest
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, InvalidParamsError(err.Error())
		}
	}
	m.mu.RLock()
	provider := m.healthProvider
	m.mu.RUnlock()
	if provider != nil {
		return provider(ctx, p.IncludeDetails)
	}
	return &HealthResponse{
		Healthy:     true,
		InstanceID:  m.session.InstanceID,
		Generation:  m.session.Generation,
		Now:         time.Now().UTC(),
		Uptime:      time.Since(m.uptime),
		ActiveCalls: m.tracker.PendingCount(),
	}, nil
}

func (m *CoreMethods) handleShutdown(ctx context.Context, params json.RawMessage) (any, error) {
	var p ShutdownRequest
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, InvalidParamsError(err.Error())
		}
	}
	m.mu.RLock()
	handler := m.shutdownHandler
	m.mu.RUnlock()
	if handler != nil {
		if err := handler(ctx, p.Reason, p.Grace); err != nil {
			return nil, InternalError(err.Error())
		}
	}
	return ShutdownAck{Accepted: true}, nil
}

func (m *CoreMethods) handleReload(ctx context.Context, params json.RawMessage) (any, error) {
	return map[string]any{"accepted": true}, nil
}

type HostCallParams struct {
	Route   string          `json:"route"`
	Method  string          `json:"method"`
	Payload json.RawMessage `json:"payload"`
}

func (m *CoreMethods) handleHostCall(ctx context.Context, params json.RawMessage) (any, error) {
	var p HostCallParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, InvalidParamsError(err.Error())
	}
	return nil, MethodNotFoundError(fmt.Sprintf("host route %s not allowed for this runtime", p.Route))
}

func (m *CoreMethods) handleStreamOpen(ctx context.Context, params json.RawMessage) (any, error) {
	var p StreamOpenRequest
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, InvalidParamsError(err.Error())
	}
	s := NewStream(p.StreamID, p.Method, p.Direction, p.ChunkMax, p.InitialCredit, 16)
	if err := m.streams.Open(s); err != nil {
		return nil, NewError(ErrCodeResourceExhausted, err.Error(), false, CategoryStream)
	}
	return map[string]any{"stream_id": s.ID, "open": true}, nil
}

func (m *CoreMethods) handleStreamChunk(ctx context.Context, params json.RawMessage) (any, error) {
	var p StreamChunk
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, InvalidParamsError(err.Error())
	}
	s, err := m.streams.Get(p.StreamID)
	if err != nil {
		return nil, NewError(ErrCodeStreamClosed, err.Error(), false, CategoryStream)
	}
	if err := s.SendChunk(p.Data); err != nil {
		return nil, NewError(ErrCodeStreamBackpressure, err.Error(), true, CategoryStream)
	}
	return map[string]any{"seq": p.Sequence}, nil
}

func (m *CoreMethods) handleStreamClose(ctx context.Context, params json.RawMessage) (any, error) {
	var p StreamClose
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, InvalidParamsError(err.Error())
	}
	if err := m.streams.Close(p.StreamID, p.Reason); err != nil {
		return nil, NewError(ErrCodeStreamClosed, err.Error(), false, CategoryStream)
	}
	return map[string]any{"closed": true}, nil
}

func (m *CoreMethods) handleStreamCancel(ctx context.Context, params json.RawMessage) (any, error) {
	var p StreamError
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, InvalidParamsError(err.Error())
	}
	s, err := m.streams.Get(p.StreamID)
	if err != nil {
		return nil, NewError(ErrCodeStreamClosed, err.Error(), false, CategoryStream)
	}
	s.Error(NewError(p.Code, p.Message, true, CategoryStream))
	return map[string]any{"cancelled": true}, nil
}

func (m *CoreMethods) handleStreamCredit(ctx context.Context, params json.RawMessage) (any, error) {
	var p StreamCredit
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, InvalidParamsError(err.Error())
	}
	s, err := m.streams.Get(p.StreamID)
	if err != nil {
		return nil, NewError(ErrCodeStreamClosed, err.Error(), false, CategoryStream)
	}
	s.AddCredit(p.Credit)
	if m.bp != nil {
		m.bp.RecordRefill()
	}
	return map[string]any{"credit": p.Credit}, nil
}

type TaskEnqueueParams struct {
	TaskID      string          `json:"task_id"`
	RuntimeType string          `json:"runtime_type"`
	Entry       string          `json:"entry"`
	Input       json.RawMessage `json:"input"`
	Priority    int             `json:"priority"`
}

type TaskResult struct {
	TaskID string          `json:"task_id"`
	Status string          `json:"status"`
	Output json.RawMessage `json:"output,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func (m *CoreMethods) handleTaskEnqueue(ctx context.Context, params json.RawMessage) (any, error) {
	var p TaskEnqueueParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, InvalidParamsError(err.Error())
	}
	return map[string]any{"task_id": p.TaskID, "queued": true}, nil
}

func (m *CoreMethods) handleTaskExecute(ctx context.Context, params json.RawMessage) (any, error) {
	return nil, MethodNotFoundError("task.execute handler not registered")
}

func (m *CoreMethods) handleTaskCancel(ctx context.Context, params json.RawMessage) (any, error) {
	var p struct {
		TaskID string `json:"task_id"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, InvalidParamsError(err.Error())
	}
	return map[string]any{"cancelled": true, "task_id": p.TaskID}, nil
}

func (m *CoreMethods) handleTaskCheckpoint(ctx context.Context, params json.RawMessage) (any, error) {
	var p struct {
		TaskID  string          `json:"task_id"`
		Payload json.RawMessage `json:"payload"`
		Version int             `json:"version"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, InvalidParamsError(err.Error())
	}
	return map[string]any{"saved": true, "task_id": p.TaskID, "version": p.Version}, nil
}

func (m *CoreMethods) handlePing(ctx context.Context, params json.RawMessage) (any, error) {
	return map[string]any{"pong": true, "now": time.Now().UTC()}, nil
}

func (m *CoreMethods) handleStats(ctx context.Context, params json.RawMessage) (any, error) {
	stats := map[string]any{
		"streams":        m.streams.Count(),
		"pending":        m.tracker.PendingCount(),
		"instance_id":    m.session.InstanceID,
		"generation":     m.session.Generation,
		"uptime_seconds": time.Since(m.uptime).Seconds(),
	}
	if m.bp != nil {
		inflight, sent, recv, refills := m.bp.Stats()
		stats["bp_inflight"] = inflight
		stats["bp_sent"] = sent
		stats["bp_recv"] = recv
		stats["bp_refills"] = refills
	}
	return stats, nil
}

func (m *CoreMethods) handleLifecycleActivate(ctx context.Context, params json.RawMessage) (any, error) {
	return map[string]any{"activated": true}, nil
}

func (m *CoreMethods) handleLifecycleDeactivate(ctx context.Context, params json.RawMessage) (any, error) {
	return map[string]any{"deactivated": true}, nil
}

type ProtocolSchema struct {
	Version  string         `json:"version"`
	Methods  []MethodSchema `json:"methods"`
	Errors   []ErrorSchema  `json:"errors"`
	Features []string       `json:"features"`
}

type MethodSchema struct {
	Name      string `json:"name"`
	Direction string `json:"direction"`
	Notify    bool   `json:"notify,omitempty"`
	ParamsRef string `json:"params_ref,omitempty"`
	ResultRef string `json:"result_ref,omitempty"`
}

type ErrorSchema struct {
	Code      string        `json:"code"`
	Category  ErrorCategory `json:"category"`
	Retryable bool          `json:"retryable"`
}

func BuildProtocolSchema() ProtocolSchema {
	methods := []MethodSchema{
		{Name: "runtime.hello", Direction: "runtime->host", Notify: true},
		{Name: "host.welcome", Direction: "host->runtime", Notify: true},
		{Name: "runtime.ready", Direction: "runtime->host", Notify: true},
		{Name: "runtime.invoke", Direction: "host->runtime"},
		{Name: "runtime.cancel", Direction: "host->runtime"},
		{Name: "runtime.health", Direction: "host->runtime"},
		{Name: "runtime.shutdown", Direction: "host->runtime"},
		{Name: "runtime.reload", Direction: "host->runtime"},
		{Name: "host.call", Direction: "runtime->host"},
		{Name: "stream.open", Direction: "bidirectional"},
		{Name: "stream.chunk", Direction: "bidirectional", Notify: true},
		{Name: "stream.close", Direction: "bidirectional", Notify: true},
		{Name: "stream.error", Direction: "bidirectional", Notify: true},
		{Name: "stream.cancel", Direction: "bidirectional"},
		{Name: "stream.credit", Direction: "bidirectional", Notify: true},
		{Name: "task.enqueue", Direction: "host->runtime"},
		{Name: "task.execute", Direction: "host->runtime"},
		{Name: "task.cancel", Direction: "host->runtime"},
		{Name: "task.checkpoint", Direction: "runtime->host", Notify: true},
		{Name: "diagnostic.ping", Direction: "bidirectional"},
		{Name: "diagnostic.stats", Direction: "host->runtime"},
		{Name: "lifecycle.activate", Direction: "host->runtime"},
		{Name: "lifecycle.deactivate", Direction: "host->runtime"},
	}
	errs := []ErrorSchema{
		{Code: string(ErrCodeParseError), Category: CategoryProtocol, Retryable: false},
		{Code: string(ErrCodeInvalidRequest), Category: CategoryProtocol, Retryable: false},
		{Code: string(ErrCodeMethodNotFound), Category: CategoryProtocol, Retryable: false},
		{Code: string(ErrCodeInvalidParams), Category: CategoryProtocol, Retryable: false},
		{Code: string(ErrCodeInternal), Category: CategoryRuntime, Retryable: true},
		{Code: string(ErrCodePermissionDenied), Category: CategoryPermission, Retryable: false},
		{Code: string(ErrCodeResourceExhausted), Category: CategoryResource, Retryable: true},
		{Code: string(ErrCodeTimeout), Category: CategoryTransient, Retryable: false},
		{Code: string(ErrCodeCancelled), Category: CategoryRuntime, Retryable: false},
		{Code: string(ErrCodeProtocol), Category: CategoryProtocol, Retryable: false},
		{Code: string(ErrCodeHandshakeFailed), Category: CategoryProtocol, Retryable: false},
		{Code: string(ErrCodeVersionMismatch), Category: CategoryProtocol, Retryable: false},
		{Code: string(ErrCodeStreamClosed), Category: CategoryStream, Retryable: false},
		{Code: string(ErrCodeStreamBackpressure), Category: CategoryStream, Retryable: true},
		{Code: string(ErrCodeFrameTooLarge), Category: CategoryProtocol, Retryable: false},
		{Code: string(ErrCodeSessionExpired), Category: CategoryPermission, Retryable: false},
		{Code: string(ErrCodeUnauthorized), Category: CategoryPermission, Retryable: false},
	}
	features := []string{
		"streaming", "cancellation", "backpressure", "diagnostics",
		"watchdog", "event_inbox", "checkpoint",
	}
	return ProtocolSchema{Version: RPCVersion, Methods: methods, Errors: errs, Features: features}
}

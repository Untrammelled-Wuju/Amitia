//go:build linux && !android

package terminal

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/platform"
)

const (
	RuntimeIDAndroidLinux = "android-linux"
)

const (
	OpOpen    = "terminal.open"
	OpWrite   = "terminal.write"
	OpRead    = "terminal.read"
	OpResize  = "terminal.resize"
	OpStatus  = "terminal.status"
	OpClose   = "terminal.close"
	OpCancel  = "terminal.cancel"
)

var DefaultShellAllowlist = []string{"/bin/sh", "/bin/bash"}

type AndroidLinuxRequest struct {
	RequestID string         `json:"requestId"`
	Operation string         `json:"operation"`
	SessionID SessionID      `json:"sessionId,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type AndroidLinuxResponse struct {
	RequestID string         `json:"requestId"`
	Status    string         `json:"status"`
	Result    map[string]any `json:"result,omitempty"`
	Error     *AndroidLinuxError `json:"error,omitempty"`
}

type AndroidLinuxError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	DomainCode string `json:"domainCode,omitempty"`
}

type AndroidLinuxProvider interface {
	Execute(ctx context.Context, request AndroidLinuxRequest) AndroidLinuxResponse
	Health(ctx context.Context) HealthStatus
	CloseAll(ctx context.Context) error
}

type AndroidLinuxCancellableProvider interface {
	CancelOp(ctx context.Context, requestID string, reason string) error
}

type HealthStatus string

const (
	HealthReady     HealthStatus = "ready"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthUnknown   HealthStatus = "unknown"
)

type Provider struct {
	manager    *SessionManager
	host       runtimehost.RuntimeHost
	workspace  string
	clock      Clock
}

func IsAndroidLinuxRuntime(host runtimehost.RuntimeHost) bool {
	if host == nil {
		return false
	}

	desc := host.Descriptor()
	if desc.Host != platform.HostPlatformAndroid {
		return false
	}
	if desc.Kind != platform.RuntimeKindProot {
		return false
	}
	if desc.Guest != platform.GuestPlatformLinux {
		return false
	}

	caps := host.Capabilities()
	if caps == nil {
		return false
	}

	if !caps.RequirementSatisfied(runtimehost.CapabilityRequirement{
		ID:      runtimehost.CapProcessSpawn,
		Minimum: runtimehost.SupportSupported,
	}) {
		return false
	}

	if !caps.RequirementSatisfied(runtimehost.CapabilityRequirement{
		ID:      runtimehost.CapRuntimeSandboxedExec,
		Minimum: runtimehost.SupportLimited,
	}) {
		return false
	}

	return true
}

func NewProvider(host runtimehost.RuntimeHost, workspaceRoot string, policy Policy) (*Provider, error) {
	if !IsAndroidLinuxRuntime(host) {
		return nil, ErrNotAvailable("not android linux runtime")
	}

	clock := defaultClock{}

	manager := &SessionManager{
		host:     host,
		sessions: make(map[SessionID]*Session),
		policy:   policy,
		clock:    clock,
		done:     make(chan struct{}),
	}

	return &Provider{
		manager:   manager,
		host:      host,
		workspace: workspaceRoot,
		clock:     clock,
	}, nil
}

func (p *Provider) Execute(ctx context.Context, request AndroidLinuxRequest) AndroidLinuxResponse {
	resp := AndroidLinuxResponse{RequestID: request.RequestID}

	switch request.Operation {
	case OpOpen:
		result, err := p.handleOpen(ctx, request)
		if err != nil {
			resp.Status = "error"
			resp.Error = toAndroidLinuxError(err)
			return resp
		}
		resp.Status = "success"
		resp.Result = result
	case OpWrite:
		result, err := p.handleWrite(ctx, request)
		if err != nil {
			resp.Status = "error"
			resp.Error = toAndroidLinuxError(err)
			return resp
		}
		resp.Status = "success"
		resp.Result = result
	case OpRead:
		result, err := p.handleRead(ctx, request)
		if err != nil {
			resp.Status = "error"
			resp.Error = toAndroidLinuxError(err)
			return resp
		}
		resp.Status = "success"
		resp.Result = result
	case OpResize:
		result, err := p.handleResize(ctx, request)
		if err != nil {
			resp.Status = "error"
			resp.Error = toAndroidLinuxError(err)
			return resp
		}
		resp.Status = "success"
		resp.Result = result
	case OpStatus:
		result, err := p.handleStatus(ctx, request)
		if err != nil {
			resp.Status = "error"
			resp.Error = toAndroidLinuxError(err)
			return resp
		}
		resp.Status = "success"
		resp.Result = result
	case OpClose:
		result, err := p.handleClose(ctx, request)
		if err != nil {
			resp.Status = "error"
			resp.Error = toAndroidLinuxError(err)
			return resp
		}
		resp.Status = "success"
		resp.Result = result
	case OpCancel:
		result, err := p.handleCancel(ctx, request)
		if err != nil {
			resp.Status = "error"
			resp.Error = toAndroidLinuxError(err)
			return resp
		}
		resp.Status = "success"
		resp.Result = result
	default:
		resp.Status = "error"
		resp.Error = &AndroidLinuxError{
			Code:    "invalid_operation",
			Message: "unknown operation: " + request.Operation,
		}
	}

	return resp
}

func (p *Provider) Health(ctx context.Context) HealthStatus {
	if !IsAndroidLinuxRuntime(p.host) {
		return HealthUnhealthy
	}

	if err := probePTY(); err != nil {
		return HealthUnhealthy
	}

	if _, err := os.Stat("/bin/sh"); err != nil {
		return HealthUnhealthy
	}

	if p.workspace == "" {
		return HealthUnhealthy
	}

	return HealthReady
}

func (p *Provider) CloseAll(ctx context.Context) error {
	return p.manager.CloseAll(ctx)
}

func (p *Provider) handleOpen(ctx context.Context, req AndroidLinuxRequest) (map[string]any, error) {
	payload := req.Payload

	shell := "/bin/sh"
	if s, ok := payload["shell"].(string); ok && s != "" {
		shell = s
	}

	cwd := p.workspace
	if c, ok := payload["cwd"].(string); ok && c != "" {
		cwd = c
	}

	rows := uint16(DefaultInitialRows)
	if r, ok := payload["rows"].(float64); ok {
		rows = uint16(r)
	}

	cols := uint16(DefaultInitialCols)
	if c, ok := payload["cols"].(float64); ok {
		cols = uint16(c)
	}

	owner := extractOwner(payload)

	sessID, state, finalRows, finalCols, err := p.manager.Open(ctx, OpenParams{
		Owner:       owner,
		Shell:       shell,
		WorkingDir:  cwd,
		Rows:        rows,
		Cols:        cols,
		Workspace:   p.workspace,
		InvocationID: req.RequestID,
	})

	if err != nil {
		return nil, err
	}

	return map[string]any{
		"sessionId": string(sessID),
		"state":     string(state),
		"rows":      finalRows,
		"cols":      finalCols,
	}, nil
}

type OpenParams struct {
	Owner        SessionOwner
	Shell        string
	WorkingDir   string
	Rows         uint16
	Cols         uint16
	Workspace    string
	InvocationID string
}

type WriteParams struct {
	Owner     SessionOwner
	SessionID SessionID
	Text      string
	Data      []byte
}

func (p *Provider) handleWrite(ctx context.Context, req AndroidLinuxRequest) (map[string]any, error) {
	payload := req.Payload

	sessID := SessionID(payload["sessionId"].(string))

	owner := extractOwner(payload)

	var isText bool
	var text string
	var data []byte

	if t, ok := payload["text"].(string); ok {
		text = t
		isText = true
	}

	if d, ok := payload["data"].(string); ok && !isText {
		data = []byte(d)
	}

	if isText {
		data = []byte(text)
	}

	if len(data) > DefaultMaxStdinSize {
		return nil, ErrInputTooLarge(DefaultMaxStdinSize)
	}

	bytesWritten, err := p.manager.Write(ctx, WriteParams{
		Owner:     owner,
		SessionID: sessID,
		Data:      data,
	})

	if err != nil {
		return nil, err
	}

	return map[string]any{
		"accepted":    true,
		"bytesWritten": bytesWritten,
	}, nil
}

type ReadParams struct {
	Owner           SessionID
	SessionID       SessionID
	AfterSequence   uint64
	MaxBytes        int
	WaitMs          int
}

func (p *Provider) handleRead(ctx context.Context, req AndroidLinuxRequest) (map[string]any, error) {
	payload := req.Payload

	sessID := SessionID(payload["sessionId"].(string))
	owner := extractOwner(payload)

	afterSeq := uint64(0)
	if a, ok := payload["afterSequence"].(float64); ok {
		afterSeq = uint64(a)
	}

	maxBytes := DefaultMaxReadOutputSize
	if m, ok := payload["maxBytes"].(float64); ok {
		maxBytes = int(m)
	}

	waitMs := 0
	if w, ok := payload["waitMs"].(float64); ok {
		waitMs = int(w)
	}

	var chunks []OutputChunk
	var nextSeq uint64
	var truncated bool
	var state SessionState

	if waitMs > 0 {
		waitCtx, cancel := context.WithTimeout(ctx, time.Duration(waitMs)*time.Millisecond)
		defer cancel()

		resultCh := make(chan readResult, 1)
		go func() {
			c, ns, t, s, e := p.manager.Read(waitCtx, ReadParams{
				Owner:         owner,
				SessionID:     sessID,
				AfterSequence: afterSeq,
				MaxBytes:      maxBytes,
			})
			resultCh <-readResult{c, ns, t, s, e}
		}()

		select {
		case <-waitCtx.Done():
			if waitCtx.Err() == context.DeadlineExceeded {
				c, ns, t, s, _ := p.manager.Read(ctx, ReadParams{
					Owner:         owner,
					SessionID:     sessID,
					AfterSequence: afterSeq,
					MaxBytes:      maxBytes,
				})
				chunks = c
				nextSeq = ns
				truncated = t
				state = s
			} else {
				return nil, ErrCancelled()
			}
		case result := <-resultCh:
			if result.err != nil {
				return nil, result.err
			}
			chunks = result.chunks
			nextSeq = result.nextSeq
			truncated = result.truncated
			state = result.state
		}
	} else {
		c, ns, t, s, err := p.manager.Read(ctx, ReadParams{
			Owner:         owner,
			SessionID:     sessID,
			AfterSequence: afterSeq,
			MaxBytes:      maxBytes,
		})
		if err != nil {
			return nil, err
		}
		chunks = c
		nextSeq = ns
		truncated = t
		state = s
	}

	return map[string]any{
		"sessionId":     string(sessID),
		"chunks":        chunksToAny(chunks),
		"nextSequence":  nextSeq,
		"truncated":     truncated,
		"state":         string(state),
	}, nil
}

type readResult struct {
	chunks    []OutputChunk
	nextSeq   uint64
	truncated bool
	state     SessionState
	err       error
}

func chunksToAny(chunks []OutputChunk) []map[string]any {
	result := make([]map[string]any, 0, len(chunks))
	for _, c := range chunks {
		result = append(result, map[string]any{
			"sequence": c.Sequence,
			"stream":   string(c.Stream),
			"data":     c.Data,
		})
	}
	return result
}

func (p *Provider) handleResize(ctx context.Context, req AndroidLinuxRequest) (map[string]any, error) {
	payload := req.Payload

	sessID := SessionID(payload["sessionId"].(string))
	owner := extractOwner(payload)

	rows := uint16(DefaultInitialRows)
	if r, ok := payload["rows"].(float64); ok {
		rows = uint16(r)
	}

	cols := uint16(DefaultInitialCols)
	if c, ok := payload["cols"].(float64); ok {
		cols = uint16(c)
	}

	if err := p.manager.Resize(ctx, owner, sessID, rows, cols); err != nil {
		return nil, err
	}

	return map[string]any{
		"rows": rows,
		"cols": cols,
	}, nil
}

func (p *Provider) handleStatus(ctx context.Context, req AndroidLinuxRequest) (map[string]any, error) {
	payload := req.Payload

	sessID := SessionID(payload["sessionId"].(string))
	owner := extractOwner(payload)

	status, err := p.manager.Status(ctx, owner, sessID)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"sessionId":    string(status.SessionID),
		"state":        string(status.State),
		"exitCode":     status.ExitCode,
		"createdAt":    status.CreatedAt,
		"lastActivity": status.LastActivity,
		"rows":         status.Rows,
		"cols":         status.Cols,
	}, nil
}

func (p *Provider) handleClose(ctx context.Context, req AndroidLinuxRequest) (map[string]any, error) {
	payload := req.Payload

	sessID := SessionID(payload["sessionId"].(string))
	owner := extractOwner(payload)

	state, err := p.manager.Close(ctx, owner, sessID)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"state": string(state),
	}, nil
}

func (p *Provider) handleCancel(ctx context.Context, req AndroidLinuxRequest) (map[string]any, error) {
	payload := req.Payload

	sessID := SessionID(payload["sessionId"].(string))
	owner := extractOwner(payload)

	state, err := p.manager.ForceCancel(ctx, owner, sessID)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"state": string(state),
	}, nil
}

func extractOwner(payload map[string]any) SessionOwner {
	owner := SessionOwner{}

	if v, ok := payload["userId"].(string); ok {
		owner.UserID = v
	}
	if v, ok := payload["characterId"].(string); ok {
		owner.CharacterID = v
	}
	if v, ok := payload["conversationId"].(string); ok {
		owner.ConversationID = v
	}

	return owner
}

func toAndroidLinuxError(err error) *AndroidLinuxError {
	if err == nil {
		return nil
	}

	terr, ok := err.(*Error)
	if ok {
		return &AndroidLinuxError{
			Code:    terr.code,
			Message: terr.message,
		}
	}

	return &AndroidLinuxError{
		Code:    "internal_error",
		Message: err.Error(),
	}
}

var _ AndroidLinuxProvider = (*Provider)(nil)

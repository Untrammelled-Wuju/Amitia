package wasm_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type HostGateway struct {
	mu          sync.RWMutex
	gateway     host_api.Gateway
	sessions    map[string]*gatewaySession
	hostFnSet   *HostFunctionSet
	permCheck   host_api.PermissionChecker
	scopeCheck  host_api.ScopeChecker
}

type gatewaySession struct {
	SessionID   string
	Identity    runtime_supervisor.RuntimeIdentity
	CreatedAt   time.Time
	HostCalls   int
	MaxHostCalls int
}

func NewHostGateway(gateway host_api.Gateway) *HostGateway {
	return &HostGateway{
		gateway:  gateway,
		sessions: make(map[string]*gatewaySession),
		hostFnSet: NewHostFunctionSet(HostFunctionConfig{}),
	}
}

func (g *HostGateway) SetHostFunctionSet(set *HostFunctionSet) {
	g.mu.Lock()
	g.hostFnSet = set
	g.mu.Unlock()
}

func (g *HostGateway) SetPermissionChecker(c host_api.PermissionChecker) {
	g.mu.Lock()
	g.permCheck = c
	g.mu.Unlock()
}

func (g *HostGateway) SetScopeChecker(c host_api.ScopeChecker) {
	g.mu.Lock()
	g.scopeCheck = c
	g.mu.Unlock()
}

func (g *HostGateway) OpenSession(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, maxHostCalls int) (string, error) {
	if g.gateway == nil {
		return "", NewWASMError(ErrCodeHostCallFailed, "host gateway not configured", nil)
	}
	allowedVersions := map[host_api.Method]int{
		host_api.MethodToolExecute:    1,
		host_api.MethodStateGet:       1,
		host_api.MethodStateCAS:       1,
		host_api.MethodResourceRead:   1,
		host_api.MethodEventEmit:      1,
	}
	session, err := g.gateway.OpenSession(ctx, identity, allowedVersions)
	if err != nil {
		return "", NewWASMError(ErrCodeHostCallFailed, fmt.Sprintf("open session: %v", err), err)
	}
	g.mu.Lock()
	g.sessions[session.SessionID] = &gatewaySession{
		SessionID:    session.SessionID,
		Identity:     identity,
		CreatedAt:    time.Now().UTC(),
		MaxHostCalls: maxHostCalls,
	}
	g.mu.Unlock()
	return session.SessionID, nil
}

func (g *HostGateway) CloseSession(ctx context.Context, sessionID string) error {
	if g.gateway == nil {
		return nil
	}
	g.mu.Lock()
	delete(g.sessions, sessionID)
	g.mu.Unlock()
	return g.gateway.CloseSession(ctx, sessionID)
}

func (g *HostGateway) Call(ctx context.Context, sessionID string, method host_api.Method, input json.RawMessage) (json.RawMessage, error) {
	if g.gateway == nil {
		return nil, NewWASMError(ErrCodeHostCallFailed, "host gateway not configured", nil)
	}

	g.mu.RLock()
	session, ok := g.sessions[sessionID]
	g.mu.RUnlock()
	if !ok {
		return nil, NewWASMError(ErrCodeHostCallFailed, "session not found", nil)
	}

	g.mu.Lock()
	if session.MaxHostCalls > 0 && session.HostCalls >= session.MaxHostCalls {
		g.mu.Unlock()
		return nil, NewWASMError(ErrCodeHostCallLimit, "host call limit exceeded", nil)
	}
	session.HostCalls++
	g.mu.Unlock()

	req := host_api.CallRequest{
		CallID:          uuid.NewString(),
		RuntimeIdentity: session.Identity,
		Method:          method,
		Version:         0,
		Input:           input,
		Deadline:        time.Now().UTC().Add(30 * time.Second),
	}

	result := g.gateway.Call(ctx, req)
	if result.Error != nil {
		return nil, NewWASMError(ErrCodeHostCallFailed, fmt.Sprintf("host call %s: %s", method, result.Error.Message), nil)
	}
	return result.Output, nil
}

func (g *HostGateway) CreateHostFunctionSet(identity runtime_supervisor.RuntimeIdentity, maxHostCalls int) *HostFunctionSet {
	sessionID := ""
	return &HostFunctionSet{
		registry: g.createBridgedRegistry(identity, &sessionID, maxHostCalls),
		logger:   func(level, msg string, fields map[string]any) {},
		storage:  nil,
		resource: nil,
		toolHub:  nil,
	}
}

func (g *HostGateway) createBridgedRegistry(identity runtime_supervisor.RuntimeIdentity, sessionID *string, maxHostCalls int) *HostImportRegistry {
	registry := NewHostImportRegistry()
	callCounter := NewHostCallCounter(maxHostCalls)

	registry.Register(ImportLog, func(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
		return g.hostFnSet.handleLog(ctx, hctx, params)
	})

	registry.Register(ImportTime, func(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
		return g.hostFnSet.handleTime(ctx, hctx, params)
	})

	registry.Register(ImportRandom, func(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
		return g.hostFnSet.handleRandom(ctx, hctx, params)
	})

	registry.Register(ImportStorageGet, func(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
		if err := callCounter.Increment(ImportStorageGet); err != nil {
			return nil, err
		}
		return g.callGateway(ctx, identity, host_api.MethodStateGet, params)
	})

	registry.Register(ImportStorageCAS, func(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
		if err := callCounter.Increment(ImportStorageCAS); err != nil {
			return nil, err
		}
		return g.callGateway(ctx, identity, host_api.MethodStateCAS, params)
	})

	registry.Register(ImportResourceRead, func(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
		if err := callCounter.Increment(ImportResourceRead); err != nil {
			return nil, err
		}
		return g.callGateway(ctx, identity, host_api.MethodResourceRead, params)
	})

	registry.Register(ImportToolInvoke, func(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
		if err := callCounter.Increment(ImportToolInvoke); err != nil {
			return nil, err
		}
		return g.callGateway(ctx, identity, host_api.MethodToolExecute, params)
	})

	return registry
}

func (g *HostGateway) callGateway(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, method host_api.Method, input json.RawMessage) (json.RawMessage, error) {
	if g.gateway == nil {
		return nil, NewWASMError(ErrCodeHostCallFailed, "gateway not configured", nil)
	}
	req := host_api.CallRequest{
		CallID:          uuid.NewString(),
		RuntimeIdentity: identity,
		Method:          method,
		Version:         0,
		Input:           input,
		Deadline:        time.Now().UTC().Add(30 * time.Second),
	}
	result := g.gateway.Call(ctx, req)
	if result.Error != nil {
		return nil, NewWASMError(ErrCodeHostCallFailed, fmt.Sprintf("%s: %s", method, result.Error.Message), nil)
	}
	return result.Output, nil
}

func (g *HostGateway) WrapRuntime(runtime *Runtime, identity runtime_supervisor.RuntimeIdentity, maxHostCalls int) {
	bridged := g.createBridgedRegistry(identity, new(string), maxHostCalls)
	existing := runtime.Registry()
	for _, name := range FullAllowedImports {
		if h, ok := bridged.Lookup(name); ok {
			existing.Register(name, h)
		}
	}
}

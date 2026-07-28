package host_api

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type DefaultGateway struct {
	mu         sync.RWMutex
	routes     map[Method]map[int]Route
	sessions   map[string]*Session
	permCheck  PermissionChecker
	scopeCheck ScopeChecker
	audit      AuditWriter
}

func NewDefaultGateway() *DefaultGateway {
	return &DefaultGateway{
		routes:   make(map[Method]map[int]Route),
		sessions: make(map[string]*Session),
	}
}

func (g *DefaultGateway) SetPermissionChecker(c PermissionChecker) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.permCheck = c
}

func (g *DefaultGateway) SetScopeChecker(c ScopeChecker) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.scopeCheck = c
}

func (g *DefaultGateway) SetAuditWriter(w AuditWriter) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.audit = w
}

func (g *DefaultGateway) RegisterRoute(route Route) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	versions, ok := g.routes[route.Method]
	if !ok {
		versions = make(map[int]Route)
		g.routes[route.Method] = versions
	}
	if _, exists := versions[route.Version]; exists {
		return fmt.Errorf("%w: %s v%d", ErrRouteExists, route.Method, route.Version)
	}
	versions[route.Version] = route
	return nil
}

func (g *DefaultGateway) OpenSession(_ context.Context, identity runtime_supervisor.RuntimeIdentity, allowedVersions map[Method]int) (Session, error) {
	if identity.InstanceID == "" {
		return Session{}, fmt.Errorf("%w: missing instance id", ErrIdentityInvalid)
	}
	if identity.ExtensionID == "" {
		return Session{}, fmt.Errorf("%w: missing extension id", ErrIdentityInvalid)
	}
	session := Session{
		SessionID:       uuid.NewString(),
		RuntimeIdentity: identity,
		Generation:      identity.Generation,
		AllowedVersions: allowedVersions,
		CreatedAt:       time.Now().UTC(),
		Active:          true,
	}
	g.mu.Lock()
	g.sessions[session.SessionID] = &session
	g.mu.Unlock()
	return session, nil
}

func (g *DefaultGateway) CloseSession(_ context.Context, sessionID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	session, ok := g.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	session.Active = false
	now := time.Now().UTC()
	session.ExpiresAt = &now
	delete(g.sessions, sessionID)
	return nil
}

func (g *DefaultGateway) Call(ctx context.Context, request CallRequest) CallResult {
	route, ok := g.findRoute(request.Method, request.Version)
	if !ok {
		result := CallResult{
			Status: StatusFailed,
			Error: &Error{
				Code:    ErrorCodeMethodNotFound,
				Message: fmt.Sprintf("method %s v%d not found", request.Method, request.Version),
			},
		}
		g.recordAudit(ctx, request, result)
		return result
	}
	if err := g.checkDeadline(request); err != nil {
		result := CallResult{Status: StatusTimeout, Error: &Error{Code: ErrorCodeTimeout, Message: err.Error()}}
		g.recordAudit(ctx, request, result)
		return result
	}
	if err := ctx.Err(); err != nil {
		result := CallResult{Status: StatusCancelled, Error: &Error{Code: ErrorCodeCancelled, Message: err.Error()}}
		g.recordAudit(ctx, request, result)
		return result
	}
	if g.permCheck == nil {
		result := CallResult{
			Status: StatusRejected,
			Error: &Error{
				Code:    ErrorCodePermissionDenied,
				Message: "host_api: permission checker not wired (fail closed)",
			},
		}
		g.recordAudit(ctx, request, result)
		return result
	}
	if err := g.permCheck.Check(ctx, request.RuntimeIdentity, route.Permission); err != nil {
		result := CallResult{
			Status: StatusRejected,
			Error: &Error{
				Code:    ErrorCodePermissionDenied,
				Message: err.Error(),
			},
		}
		g.recordAudit(ctx, request, result)
		return result
	}
	if g.scopeCheck == nil {
		result := CallResult{
			Status: StatusRejected,
			Error: &Error{
				Code:    ErrorCodeScopeDenied,
				Message: "host_api: scope checker not wired (fail closed)",
			},
		}
		g.recordAudit(ctx, request, result)
		return result
	}
	if err := g.scopeCheck.Check(ctx, request.RuntimeIdentity, request.ScopeSnapshotID, route.ScopePolicy); err != nil {
		result := CallResult{
			Status: StatusRejected,
			Error: &Error{
				Code:    ErrorCodeScopeDenied,
				Message: err.Error(),
			},
		}
		g.recordAudit(ctx, request, result)
		return result
	}
	timeoutCtx := ctx
	if route.Timeout > 0 {
		var cancel context.CancelFunc
		timeoutCtx, cancel = context.WithTimeout(ctx, route.Timeout)
		defer cancel()
	}
	output, err := route.Handler(timeoutCtx, request)
	if err != nil {
		code := ErrorCodeInternal
		status := StatusFailed
		switch {
		case errors.Is(err, ErrTimeout):
			code = ErrorCodeTimeout
			status = StatusTimeout
		case errors.Is(err, ErrCancelled):
			code = ErrorCodeCancelled
			status = StatusCancelled
		case errors.Is(err, ErrRateLimited):
			code = ErrorCodeRateLimited
			status = StatusRateLimit
		case errors.Is(err, ErrInputInvalid):
			code = ErrorCodeInputInvalid
		case errors.Is(err, ErrOutputInvalid):
			code = ErrorCodeOutputInvalid
		}
		output = CallResult{
			Status: status,
			Error: &Error{
				Code:    code,
				Message: err.Error(),
			},
		}
	}
	g.recordAudit(ctx, request, output)
	return output
}

func (g *DefaultGateway) QueryCapability(_ context.Context, method Method) (Route, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	versions, ok := g.routes[method]
	if !ok || len(versions) == 0 {
		return Route{}, false
	}
	maxVersion := -1
	var route Route
	for v, r := range versions {
		if v > maxVersion {
			maxVersion = v
			route = r
		}
	}
	return route, true
}

func (g *DefaultGateway) ListMethods(_ context.Context) []Method {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var methods []Method
	for m := range g.routes {
		methods = append(methods, m)
	}
	return methods
}

func (g *DefaultGateway) findRoute(method Method, version int) (Route, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	versions, ok := g.routes[method]
	if !ok {
		return Route{}, false
	}
	if version == 0 {
		maxVersion := -1
		var route Route
		for v, r := range versions {
			if v > maxVersion {
				maxVersion = v
				route = r
			}
		}
		return route, maxVersion >= 0
	}
	route, ok := versions[version]
	return route, ok
}

func (g *DefaultGateway) checkDeadline(request CallRequest) error {
	if request.Deadline.IsZero() {
		return nil
	}
	if time.Now().UTC().After(request.Deadline) {
		return ErrTimeout
	}
	return nil
}

func (g *DefaultGateway) recordAudit(ctx context.Context, request CallRequest, result CallResult) {
	g.mu.RLock()
	w := g.audit
	g.mu.RUnlock()
	if w != nil {
		w.RecordCall(ctx, request, result)
	}
}

var _ Gateway = (*DefaultGateway)(nil)

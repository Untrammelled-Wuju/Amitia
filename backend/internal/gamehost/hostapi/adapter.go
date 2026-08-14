package hostapi

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ReadyVerifier interface {
	IsReady(connKey string) bool
}

type HostAPIAdapter struct {
	gateway   host_api.Gateway
	mapper    IdentityMapper
	permProv  PermissionSnapshotIDProvider
	scopeProv ScopeSnapshotIDProvider
	ready     ReadyVerifier
	idGen     func() string
	tracker   InvocationTracker
}

type InvocationTracker interface {
	Begin(runtimeID domain.RuntimeInstanceID, callID string, cancel context.CancelFunc) bool
	End(callID string, failed bool)
}

type HostAPIAdapterConfig struct {
	Gateway            host_api.Gateway
	Mapper             IdentityMapper
	PermissionProvider PermissionSnapshotIDProvider
	ScopeProvider      ScopeSnapshotIDProvider
	ReadyVerifier      ReadyVerifier
	IDGenerator        func() string
	InvocationTracker  InvocationTracker
}

func NewHostAPIAdapter(cfg HostAPIAdapterConfig) (*HostAPIAdapter, error) {
	if cfg.Gateway == nil {
		return nil, fmt.Errorf("hostapi: gateway is required")
	}
	if cfg.Mapper == nil {
		return nil, fmt.Errorf("hostapi: identity mapper is required")
	}
	if cfg.PermissionProvider == nil {
		return nil, fmt.Errorf("hostapi: permission snapshot provider is required")
	}
	if cfg.ScopeProvider == nil {
		return nil, fmt.Errorf("hostapi: scope snapshot provider is required")
	}
	if cfg.IDGenerator == nil {
		return nil, fmt.Errorf("hostapi: id generator is required")
	}
	return &HostAPIAdapter{
		gateway:   cfg.Gateway,
		mapper:    cfg.Mapper,
		permProv:  cfg.PermissionProvider,
		scopeProv: cfg.ScopeProvider,
		ready:     cfg.ReadyVerifier,
		idGen:     cfg.IDGenerator,
		tracker:   cfg.InvocationTracker,
	}, nil
}

type Request struct {
	Peer    Peer
	Route   string
	Version int
	Input   json.RawMessage

	ConnKey  string
	Deadline time.Time
}

type Response struct {
	Status string
	Output json.RawMessage
}

func (a *HostAPIAdapter) Call(ctx context.Context, req Request) (Response, error) {
	if req.Route == "" {
		return Response{}, fmt.Errorf("hostapi: route is required")
	}

	if req.ConnKey != "" && a.ready != nil && !a.ready.IsReady(req.ConnKey) {
		return Response{}, ErrNotReady
	}

	identity, err := a.mapper.MapIdentity(ctx, req.Peer)
	if err != nil {
		return Response{}, fmt.Errorf("hostapi: identity mapping failed: %w", err)
	}

	var permSnapID, scopeSnapID string
	if identity.ExtensionID != "" {
		if sid, ok, err := a.permProv.CurrentSnapshotID(ctx, string(identity.ExtensionID), string(identity.ModuleID), identity.Generation); err != nil {
			return Response{}, fmt.Errorf("hostapi: permission snapshot lookup failed: %w", err)
		} else if ok {
			permSnapID = sid
		}

		if sid, ok, err := a.scopeProv.CurrentSnapshotID(ctx, string(identity.ExtensionID), string(identity.ModuleID), identity.Generation); err != nil {
			return Response{}, fmt.Errorf("hostapi: scope snapshot lookup failed: %w", err)
		} else if ok {
			scopeSnapID = sid
		}
	}

	callReq := host_api.CallRequest{
		CallID:               a.idGen(),
		RuntimeIdentity:      identity,
		Method:               host_api.Method(req.Route),
		Version:              req.Version,
		Input:                append(json.RawMessage(nil), req.Input...),
		PermissionSnapshotID: permSnapID,
		ScopeSnapshotID:      scopeSnapID,
	}
	if !req.Deadline.IsZero() {
		callReq.Deadline = req.Deadline
	}

	callCtx, cancel := context.WithCancel(ctx)
	if a.tracker != nil && !a.tracker.Begin(domain.RuntimeInstanceID(identity.InstanceID), callReq.CallID, cancel) {
		cancel()
		return Response{}, fmt.Errorf("hostapi: runtime is not accepting invocations")
	}
	defer cancel()
	if a.tracker != nil {
		defer func() { a.tracker.End(callReq.CallID, false) }()
	}
	result := a.gateway.Call(callCtx, callReq)
	if result.Status == host_api.StatusSuccess {
		return Response{Status: result.Status, Output: result.Output}, nil
	}
	return Response{Status: result.Status}, mapGatewayError(result.Error)
}

func atomicIDGenerator() func() string {
	var counter int64
	return func() string {
		n := atomic.AddInt64(&counter, 1)
		return fmt.Sprintf("gh%d-%d", time.Now().UnixNano(), n)
	}
}

func DefaultIDGenerator() func() string {
	return atomicIDGenerator()
}

package health

import (
	"context"
	"sync"
	"time"
)

type MCPLifecycleReader interface {
	IsInstalled(serverID string) bool
	IsEnabled(serverID string) bool
	RuntimeState(serverID string) string
}

type MCPAuthorizationReader interface {
	AuthorizationState(serverID string) string
	HasCredential(serverID string) bool
}

type MCPProtocolClient interface {
	Probe(ctx context.Context, serverID, endpoint string, headers map[string]string) (HealthProbeResult, error)
}

type MCPHealthStore interface {
	Load(serverID string) (MCPHealthSnapshot, bool)
	Save(snapshot MCPHealthSnapshot)
	LoadGeneration(serverID string) int64
	IncrementGeneration(serverID string) int64
}

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type MCPHealthCoordinator struct {
	lifecycle MCPLifecycleReader
	auth      MCPAuthorizationReader
	client    MCPProtocolClient
	store     MCPHealthStore
	clock     Clock

	mu          sync.Mutex
	generations map[string]int64
	pending     map[string]bool
}

func NewMCPHealthCoordinator(lifecycle MCPLifecycleReader, auth MCPAuthorizationReader, client MCPProtocolClient, store MCPHealthStore) *MCPHealthCoordinator {
	return &MCPHealthCoordinator{
		lifecycle:   lifecycle,
		auth:        auth,
		client:      client,
		store:       store,
		clock:       systemClock{},
		generations: make(map[string]int64),
		pending:     make(map[string]bool),
	}
}

func (c *MCPHealthCoordinator) BeginProbe(serverID string) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	gen := c.store.IncrementGeneration(serverID)
	c.generations[serverID] = gen
	c.pending[serverID] = true
	return gen
}

func (c *MCPHealthCoordinator) Probe(ctx context.Context, serverID, endpoint string, headers map[string]string) MCPHealthSnapshot {
	gen := c.BeginProbe(serverID)
	start := c.clock.Now()

	probeResult, err := c.client.Probe(ctx, serverID, endpoint, headers)
	latency := time.Since(start).Milliseconds()

	snapshot := c.aggregate(serverID, probeResult, err, latency, gen)
	c.EndProbe(serverID)
	return snapshot
}

func (c *MCPHealthCoordinator) EndProbe(serverID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.pending, serverID)
}

func (c *MCPHealthCoordinator) aggregate(serverID string, result HealthProbeResult, probeErr error, latencyMS, generation int64) MCPHealthSnapshot {
	now := c.clock.Now()
	prev, hasPrev := c.store.Load(serverID)

	if hasPrev && prev.ConsecutiveFailures > 0 {
	}

	snapshot := MCPHealthSnapshot{
		ServerID:           serverID,
		Installed:          c.lifecycle.IsInstalled(serverID),
		Enabled:            c.lifecycle.IsEnabled(serverID),
		AuthorizationState: c.auth.AuthorizationState(serverID),
		LastProbeAt:        now,
		LatencyMS:          latencyMS,
	}

	if probeErr != nil || !result.Reachable {
		snapshot.State = MCPHealthUnreachable
		snapshot.Reachability = string(MCPReachUnreachable)
		if hasPrev {
			snapshot.ConsecutiveFailures = prev.ConsecutiveFailures + 1
		} else {
			snapshot.ConsecutiveFailures = 1
		}
		if probeErr != nil {
			snapshot.ErrorCode = "PROBE_FAILED"
			snapshot.ErrorMessage = probeErr.Error()
		} else if result.Error != "" {
			snapshot.ErrorCode = result.Error
			snapshot.ErrorMessage = result.ErrorDetail
		}
		snapshot.RetryAt = c.computeBackoff(snapshot.ConsecutiveFailures, now)
	} else {
		snapshot.Reachability = string(MCPReachReachable)
		snapshot.LastSuccessAt = now
		snapshot.ConsecutiveFailures = 0
		snapshot.ProtocolVersion = result.ProtocolVersion
		snapshot.ServerInfo = result.ServerInfo

		if c.auth.AuthorizationState(serverID) == "authorization_required" {
			snapshot.State = MCPHealthAuthorizationRequired
		} else {
			snapshot.State = MCPHealthReady
		}
	}

	if !snapshot.Installed {
		snapshot.State = MCPHealthInstalling
	} else if !snapshot.Enabled {
		snapshot.State = MCPHealthDisabled
	}

	c.store.Save(snapshot)
	return snapshot
}

func (c *MCPHealthCoordinator) computeBackoff(failures int, now time.Time) *time.Time {
	if failures <= 0 {
		return nil
	}
	delays := []time.Duration{
		30 * time.Second,
		1 * time.Minute,
		2 * time.Minute,
		5 * time.Minute,
		15 * time.Minute,
	}
	idx := failures - 1
	if idx >= len(delays) {
		idx = len(delays) - 1
	}
	t := now.Add(delays[idx])
	return &t
}

func (c *MCPHealthCoordinator) Get(serverID string) (MCPHealthSnapshot, bool) {
	return c.store.Load(serverID)
}

func (c *MCPHealthCoordinator) IsPending(serverID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pending[serverID]
}

func (c *MCPHealthCoordinator) Generation(serverID string) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if g, ok := c.generations[serverID]; ok {
		return g
	}
	return 0
}

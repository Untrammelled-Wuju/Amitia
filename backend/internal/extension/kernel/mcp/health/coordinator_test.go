package health

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

type fakeLifecycle struct {
	installed map[string]bool
	enabled   map[string]bool
	runtime   map[string]string
}

func newFakeLifecycle() *fakeLifecycle {
	return &fakeLifecycle{
		installed: make(map[string]bool),
		enabled:   make(map[string]bool),
		runtime:   make(map[string]string),
	}
}

func (f *fakeLifecycle) IsInstalled(serverID string) bool {
	return f.installed[serverID]
}

func (f *fakeLifecycle) IsEnabled(serverID string) bool {
	return f.enabled[serverID]
}

func (f *fakeLifecycle) RuntimeState(serverID string) string {
	return f.runtime[serverID]
}

type fakeAuth struct {
	authStates map[string]string
	credentials map[string]bool
}

func newFakeAuth() *fakeAuth {
	return &fakeAuth{
		authStates:  make(map[string]string),
		credentials: make(map[string]bool),
	}
}

func (f *fakeAuth) AuthorizationState(serverID string) string {
	return f.authStates[serverID]
}

func (f *fakeAuth) HasCredential(serverID string) bool {
	return f.credentials[serverID]
}

type fakeProtocolClient struct {
	results map[string]HealthProbeResult
	errs    map[string]error
}

func newFakeProtocolClient() *fakeProtocolClient {
	return &fakeProtocolClient{
		results: make(map[string]HealthProbeResult),
		errs:    make(map[string]error),
	}
}

func (f *fakeProtocolClient) Probe(ctx context.Context, serverID, endpoint string, headers map[string]string) (HealthProbeResult, error) {
	if err, ok := f.errs[serverID]; ok {
		return HealthProbeResult{}, err
	}
	if r, ok := f.results[serverID]; ok {
		return r, nil
	}
	return HealthProbeResult{}, nil
}

func TestCoordinator_Probe_Success(t *testing.T) {
	lifecycle := newFakeLifecycle()
	lifecycle.installed["server-1"] = true
	lifecycle.enabled["server-1"] = true

	auth := newFakeAuth()
	auth.authStates["server-1"] = "authorized"

	client := newFakeProtocolClient()
	client.results["server-1"] = HealthProbeResult{
		Reachable:       true,
		LatencyMS:       50,
		ProtocolVersion: "2026-07-28",
		ServerInfo:      MCPServerInfo{Name: "test-server", Version: "1.0"},
	}

	store := NewInMemoryHealthStore()
	coord := NewMCPHealthCoordinator(lifecycle, auth, client, store)

	snap := coord.Probe(context.Background(), "server-1", "https://api.example.com/mcp", nil)

	if snap.State != MCPHealthReady {
		t.Fatalf("expected ready state, got %s", snap.State)
	}
	if snap.ServerID != "server-1" {
		t.Fatalf("server_id mismatch: %s", snap.ServerID)
	}
	if !snap.Installed || !snap.Enabled {
		t.Fatal("expected installed and enabled to be true")
	}
	if snap.Reachability != string(MCPReachReachable) {
		t.Fatalf("reachability mismatch: %s", snap.Reachability)
	}
	if snap.ProtocolVersion != "2026-07-28" {
		t.Fatalf("protocol_version mismatch: %s", snap.ProtocolVersion)
	}
	if snap.ConsecutiveFailures != 0 {
		t.Fatalf("expected 0 failures, got %d", snap.ConsecutiveFailures)
	}
	if snap.RetryAt != nil {
		t.Fatal("RetryAt should be nil on success")
	}
}

func TestCoordinator_Probe_Unreachable(t *testing.T) {
	lifecycle := newFakeLifecycle()
	lifecycle.installed["server-2"] = true
	lifecycle.enabled["server-2"] = true

	auth := newFakeAuth()
	auth.authStates["server-2"] = "authorized"

	client := newFakeProtocolClient()
	client.results["server-2"] = HealthProbeResult{
		Reachable:   false,
		Error:       "TIMEOUT",
		ErrorDetail: "connection timed out",
	}

	store := NewInMemoryHealthStore()
	coord := NewMCPHealthCoordinator(lifecycle, auth, client, store)

	snap := coord.Probe(context.Background(), "server-2", "https://api.example.com/mcp", nil)

	if snap.State != MCPHealthUnreachable {
		t.Fatalf("expected unreachable state, got %s", snap.State)
	}
	if snap.Reachability != string(MCPReachUnreachable) {
		t.Fatalf("reachability mismatch: %s", snap.Reachability)
	}
	if snap.ConsecutiveFailures != 1 {
		t.Fatalf("expected 1 failure, got %d", snap.ConsecutiveFailures)
	}
	if snap.ErrorCode != "TIMEOUT" {
		t.Fatalf("error_code mismatch: %s", snap.ErrorCode)
	}
	if snap.RetryAt == nil {
		t.Fatal("RetryAt should be set after failure")
	}
}

func TestCoordinator_Probe_AuthRequired(t *testing.T) {
	lifecycle := newFakeLifecycle()
	lifecycle.installed["server-3"] = true
	lifecycle.enabled["server-3"] = true

	auth := newFakeAuth()
	auth.authStates["server-3"] = "authorization_required"

	client := newFakeProtocolClient()
	client.results["server-3"] = HealthProbeResult{
		Reachable: true,
	}

	store := NewInMemoryHealthStore()
	coord := NewMCPHealthCoordinator(lifecycle, auth, client, store)

	snap := coord.Probe(context.Background(), "server-3", "https://api.example.com/mcp", nil)

	if snap.State != MCPHealthAuthorizationRequired {
		t.Fatalf("expected authorization_required state, got %s", snap.State)
	}
}

func TestCoordinator_Probe_NotInstalled(t *testing.T) {
	lifecycle := newFakeLifecycle()
	auth := newFakeAuth()
	client := newFakeProtocolClient()
	store := NewInMemoryHealthStore()
	coord := NewMCPHealthCoordinator(lifecycle, auth, client, store)

	snap := coord.Probe(context.Background(), "not-installed", "https://api.example.com/mcp", nil)

	if snap.State != MCPHealthInstalling {
		t.Fatalf("expected installing state for non-installed server, got %s", snap.State)
	}
}

func TestCoordinator_Probe_Disabled(t *testing.T) {
	lifecycle := newFakeLifecycle()
	lifecycle.installed["server-disabled"] = true
	lifecycle.enabled["server-disabled"] = false

	auth := newFakeAuth()
	client := newFakeProtocolClient()
	store := NewInMemoryHealthStore()
	coord := NewMCPHealthCoordinator(lifecycle, auth, client, store)

	snap := coord.Probe(context.Background(), "server-disabled", "https://api.example.com/mcp", nil)

	if snap.State != MCPHealthDisabled {
		t.Fatalf("expected disabled state, got %s", snap.State)
	}
}

func TestCoordinator_Probe_NetworkError(t *testing.T) {
	lifecycle := newFakeLifecycle()
	lifecycle.installed["server-neterr"] = true
	lifecycle.enabled["server-neterr"] = true

	auth := newFakeAuth()
	auth.authStates["server-neterr"] = "authorized"

	client := newFakeProtocolClient()
	client.errs["server-neterr"] = errors.New("network unreachable")

	store := NewInMemoryHealthStore()
	coord := NewMCPHealthCoordinator(lifecycle, auth, client, store)

	snap := coord.Probe(context.Background(), "server-neterr", "https://api.example.com/mcp", nil)

	if snap.State != MCPHealthUnreachable {
		t.Fatalf("expected unreachable state, got %s", snap.State)
	}
	if snap.ErrorCode != "PROBE_FAILED" {
		t.Fatalf("expected PROBE_FAILED error code, got %s", snap.ErrorCode)
	}
	if snap.ErrorMessage != "network unreachable" {
		t.Fatalf("error_message mismatch: %s", snap.ErrorMessage)
	}
}

func TestCoordinator_BackoffIncreases(t *testing.T) {
	lifecycle := newFakeLifecycle()
	lifecycle.installed["server-backoff"] = true
	lifecycle.enabled["server-backoff"] = true

	auth := newFakeAuth()
	client := newFakeProtocolClient()
	client.results["server-backoff"] = HealthProbeResult{Reachable: false}

	store := NewInMemoryHealthStore()
	coord := NewMCPHealthCoordinator(lifecycle, auth, client, store)

	var prevRetry *time.Time
	for i := 1; i <= 5; i++ {
		snap := coord.Probe(context.Background(), "server-backoff", "https://api.example.com/mcp", nil)
		if snap.ConsecutiveFailures != i {
			t.Fatalf("iteration %d: expected %d failures, got %d", i, i, snap.ConsecutiveFailures)
		}
		if snap.RetryAt == nil {
			t.Fatalf("iteration %d: RetryAt should be set", i)
		}
		if prevRetry != nil && !snap.RetryAt.After(*prevRetry) {
			t.Fatalf("iteration %d: RetryAt should increase over time", i)
		}
		prevRetry = snap.RetryAt
	}
}

func TestCoordinator_IsPending(t *testing.T) {
	lifecycle := newFakeLifecycle()
	auth := newFakeAuth()
	client := newFakeProtocolClient()
	store := NewInMemoryHealthStore()
	coord := NewMCPHealthCoordinator(lifecycle, auth, client, store)

	if coord.IsPending("any") {
		t.Fatal("should not be pending initially")
	}

	gen := coord.BeginProbe("server-pending")
	if gen != 1 {
		t.Fatalf("expected generation 1, got %d", gen)
	}
	if !coord.IsPending("server-pending") {
		t.Fatal("should be pending after BeginProbe")
	}

	coord.EndProbe("server-pending")
	if coord.IsPending("server-pending") {
		t.Fatal("should not be pending after EndProbe")
	}
}

func TestCoordinator_Get(t *testing.T) {
	lifecycle := newFakeLifecycle()
	lifecycle.installed["server-get"] = true
	lifecycle.enabled["server-get"] = true

	auth := newFakeAuth()
	auth.authStates["server-get"] = "authorized"

	client := newFakeProtocolClient()
	client.results["server-get"] = HealthProbeResult{
		Reachable: true,
		ServerInfo: MCPServerInfo{Name: "get-test"},
	}

	store := NewInMemoryHealthStore()
	coord := NewMCPHealthCoordinator(lifecycle, auth, client, store)

	coord.Probe(context.Background(), "server-get", "https://api.example.com/mcp", nil)

	snap, ok := coord.Get("server-get")
	if !ok {
		t.Fatal("expected stored snapshot")
	}
	if snap.ServerID != "server-get" {
		t.Fatalf("server_id mismatch: %s", snap.ServerID)
	}

	_, ok = coord.Get("nonexistent")
	if ok {
		t.Fatal("expected not found for nonexistent server")
	}
}

func TestCoordinator_Generation(t *testing.T) {
	lifecycle := newFakeLifecycle()
	auth := newFakeAuth()
	client := newFakeProtocolClient()
	store := NewInMemoryHealthStore()
	coord := NewMCPHealthCoordinator(lifecycle, auth, client, store)

	if coord.Generation("gen-test") != 0 {
		t.Fatal("initial generation should be 0")
	}

	coord.BeginProbe("gen-test")
	if coord.Generation("gen-test") != 1 {
		t.Fatalf("expected generation 1, got %d", coord.Generation("gen-test"))
	}

	coord.BeginProbe("gen-test")
	if coord.Generation("gen-test") != 2 {
		t.Fatalf("expected generation 2, got %d", coord.Generation("gen-test"))
	}
}

func TestInMemoryHealthStore_SaveAndLoad(t *testing.T) {
	store := NewInMemoryHealthStore()

	snap := MCPHealthSnapshot{
		ServerID: "store-test",
		State:    MCPHealthReady,
	}
	store.Save(snap)

	loaded, ok := store.Load("store-test")
	if !ok {
		t.Fatal("expected to load saved snapshot")
	}
	if loaded.ServerID != "store-test" {
		t.Fatalf("server_id mismatch: %s", loaded.ServerID)
	}
	if loaded.State != MCPHealthReady {
		t.Fatalf("state mismatch: %s", loaded.State)
	}
}

func TestInMemoryHealthStore_Generation(t *testing.T) {
	store := NewInMemoryHealthStore()

	if store.LoadGeneration("gen") != 0 {
		t.Fatal("initial generation should be 0")
	}

	gen1 := store.IncrementGeneration("gen")
	if gen1 != 1 {
		t.Fatalf("expected generation 1, got %d", gen1)
	}

	gen2 := store.IncrementGeneration("gen")
	if gen2 != 2 {
		t.Fatalf("expected generation 2, got %d", gen2)
	}

	if store.LoadGeneration("gen") != 2 {
		t.Fatalf("expected loaded generation 2, got %d", store.LoadGeneration("gen"))
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestModern2026Adapter_Era(t *testing.T) {
	client := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{}, nil
	})
	adapter := NewModern2026Adapter(client, 0)
	if adapter.Era() != MCPProtocolEraModern {
		t.Fatalf("expected modern era, got %s", adapter.Era())
	}
}

func TestLegacy2025Adapter_Era(t *testing.T) {
	client := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{}, nil
	})
	adapter := NewLegacy2025Adapter(client, 0)
	if adapter.Era() != MCPProtocolEraLegacy {
		t.Fatalf("expected legacy era, got %s", adapter.Era())
	}
}

func TestAdapter_DefaultTimeout(t *testing.T) {
	client := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{}, nil
	})
	adapter := NewModern2026Adapter(client, 0)
	if adapter.timeout != 15*time.Second {
		t.Fatalf("expected default timeout 15s, got %v", adapter.timeout)
	}
}

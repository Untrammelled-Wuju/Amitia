package builtin

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
)

type fakeQdrantDep struct {
	mu              sync.Mutex
	startCalls      int
	waitCalls       int
	initCalls       int
	ensureCalls     int
	stopCalls       int
	setShuttingDown int
	client          *qdrant.Client
	startErr        error
	waitErr         error
	initErr         error
	ensureErr       error
}

func (f *fakeQdrantDep) StartQdrant() error {
	f.mu.Lock()
	f.startCalls++
	f.mu.Unlock()
	return f.startErr
}
func (f *fakeQdrantDep) WaitForQdrant(port int) error {
	f.mu.Lock()
	f.waitCalls++
	f.mu.Unlock()
	return f.waitErr
}
func (f *fakeQdrantDep) InitClient() error {
	f.mu.Lock()
	f.initCalls++
	f.mu.Unlock()
	return f.initErr
}
func (f *fakeQdrantDep) EnsureCollections() error {
	f.mu.Lock()
	f.ensureCalls++
	f.mu.Unlock()
	return f.ensureErr
}
func (f *fakeQdrantDep) SetQdrantShuttingDown() {
	f.mu.Lock()
	f.setShuttingDown++
	f.mu.Unlock()
}
func (f *fakeQdrantDep) StopQdrant() {
	f.mu.Lock()
	f.stopCalls++
	f.mu.Unlock()
}
func (f *fakeQdrantDep) GetClient() *qdrant.Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.client
}

func TestQdrantProviderFactorySetsDescriptor(t *testing.T) {
	factory := NewQdrantProviderFactoryWithDep(&fakeQdrantDep{})
	ctx := runtimeorchestrator.ProviderBuildContext{
		Config: &config.Config{
			Providers: config.ProvidersConfig{
				VectorStore: config.VectorStoreProviderConfig{
					Enabled:  true,
					Required: false,
					Qdrant: config.QdrantConfig{
						Port: 19178,
					},
				},
			},
		},
	}
	inst, err := factory.Build(ctx)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	desc := inst.Descriptor()
	if desc.ID != "provider.vector-store" {
		t.Fatalf("descriptor ID=%s, want provider.vector-store", desc.ID)
	}
	if desc.Phase != "infrastructure" {
		t.Fatalf("descriptor phase=%s, want infrastructure", desc.Phase)
	}
	if inst.Slot() != "vector-store" {
		t.Fatalf("slot=%s, want vector-store", inst.Slot())
	}
	if inst.ProviderID() != "builtin.qdrant-process" {
		t.Fatalf("provider ID=%s, want builtin.qdrant-process", inst.ProviderID())
	}
}

func TestQdrantProviderStartsInOrder(t *testing.T) {
	dep := &fakeQdrantDep{}
	factory := NewQdrantProviderFactoryWithDep(dep)
	ctx := runtimeorchestrator.ProviderBuildContext{
		Config: &config.Config{
			Providers: config.ProvidersConfig{
				VectorStore: config.VectorStoreProviderConfig{
					Enabled: true,
					Qdrant:  config.QdrantConfig{Port: 19178},
				},
			},
		},
	}
	inst, _ := factory.Build(ctx)
	if err := inst.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if dep.startCalls != 1 || dep.waitCalls != 1 || dep.initCalls != 1 || dep.ensureCalls != 1 {
		t.Fatalf("calls: start=%d wait=%d init=%d ensure=%d", dep.startCalls, dep.waitCalls, dep.initCalls, dep.ensureCalls)
	}
}

func TestQdrantProviderStartFailureCleansUp(t *testing.T) {
	dep := &fakeQdrantDep{waitErr: errors.New("port timeout")}
	factory := NewQdrantProviderFactoryWithDep(dep)
	ctx := runtimeorchestrator.ProviderBuildContext{
		Config: &config.Config{
			Providers: config.ProvidersConfig{
				VectorStore: config.VectorStoreProviderConfig{
					Enabled: true,
					Qdrant:  config.QdrantConfig{Port: 19178},
				},
			},
		},
	}
	inst, _ := factory.Build(ctx)
	err := inst.Start(context.Background())
	if err == nil {
		t.Fatalf("expected start failure")
	}
	if dep.stopCalls == 0 {
		t.Fatalf("expected Qdrant cleanup (StopQdrant) on failure")
	}
}

func TestQdrantProviderStopIsIdempotent(t *testing.T) {
	dep := &fakeQdrantDep{}
	factory := NewQdrantProviderFactoryWithDep(dep)
	ctx := runtimeorchestrator.ProviderBuildContext{
		Config: &config.Config{
			Providers: config.ProvidersConfig{
				VectorStore: config.VectorStoreProviderConfig{
					Enabled: true,
					Qdrant:  config.QdrantConfig{Port: 19178},
				},
			},
		},
	}
	inst, _ := factory.Build(ctx)
	_ = inst.Start(context.Background())
	_ = inst.Stop(context.Background())
	_ = inst.Stop(context.Background())
	if dep.stopCalls != 1 {
		t.Fatalf("stop calls=%d, want 1 (idempotent)", dep.stopCalls)
	}
}

type fakeSurrealDep struct {
	mu              sync.Mutex
	startCalls      int
	waitCalls       int
	clientCalls     int
	svcCalls        int
	monitorCalls    int
	stopMonitorCalls int
	stopCalls       int
	setShuttingDown int
	cfg             config.SurrealConfig
	startErr        error
	waitErr         error
	lastSvc         graph.Service
	restartCb       func()
	healthErr       error
	newClientFn     func(config.SurrealConfig) (*graph.Client, error)
}

func (f *fakeSurrealDep) NewClient(cfg config.SurrealConfig) (*graph.Client, error) {
	f.mu.Lock()
	f.clientCalls++
	fn := f.newClientFn
	f.mu.Unlock()
	if fn != nil {
		return fn(cfg)
	}
	return &graph.Client{}, nil
}

func (f *fakeSurrealDep) StartSurreal() error {
	f.mu.Lock()
	f.startCalls++
	f.mu.Unlock()
	return f.startErr
}
func (f *fakeSurrealDep) WaitForSurreal(port int) error {
	f.mu.Lock()
	f.waitCalls++
	f.mu.Unlock()
	return f.waitErr
}
func (f *fakeSurrealDep) StartSurrealMonitor() {
	f.mu.Lock()
	f.monitorCalls++
	f.mu.Unlock()
}
func (f *fakeSurrealDep) SetSurrealShuttingDown() {
	f.mu.Lock()
	f.setShuttingDown++
	f.mu.Unlock()
}
func (f *fakeSurrealDep) StopSurrealMonitor() {
	f.mu.Lock()
	f.stopMonitorCalls++
	f.mu.Unlock()
}
func (f *fakeSurrealDep) StopSurreal() {
	f.mu.Lock()
	f.stopCalls++
	f.mu.Unlock()
}
func (f *fakeSurrealDep) GetConfig() config.SurrealConfig {
	return f.cfg
}
func (f *fakeSurrealDep) NewGraphService(client *graph.Client) graph.Service {
	f.mu.Lock()
	f.svcCalls++
	if f.lastSvc == nil {
		f.lastSvc = &fakeGraphService{}
	}
	f.mu.Unlock()
	return f.lastSvc
}
func (f *fakeSurrealDep) SetRestartCallback(fn func()) {
	f.mu.Lock()
	f.restartCb = fn
	f.mu.Unlock()
}
func (f *fakeSurrealDep) HealthCheck(ctx context.Context, cfg config.SurrealConfig) error {
	return f.healthErr
}

type fakeGraphService struct{}

func (*fakeGraphService) SyncNode(_, _ string, _ string, _ map[string]interface{}) error { return nil }
func (*fakeGraphService) SyncEdge(_, _ string, _ string, _ float64) error               { return nil }
func (*fakeGraphService) DeleteNode(_ string) error                                      { return nil }
func (*fakeGraphService) DeleteNodeIfOrphan(_ string) error                              { return nil }
func (*fakeGraphService) DeleteNodesByProperty(_, _ string, _ string) error              { return nil }
func (*fakeGraphService) QueryNeighbors(_ string, _ int, _ string) (map[string]interface{}, error) {
	return nil, nil
}
func (*fakeGraphService) FindPaths(_, _ string, _ int) ([]map[string]interface{}, error) {
	return nil, nil
}
func (*fakeGraphService) DeleteOrphanNodes() error                      { return nil }
func (*fakeGraphService) GetStats(_ string) (map[string]interface{}, error) { return nil, nil }
func (*fakeGraphService) GetAllNodes(_ string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (*fakeGraphService) GetAllEdges(_ string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (*fakeGraphService) Name() string { return "fake-graph" }
func (*fakeGraphService) Process(_ context.Context, _ string, _ []map[string]string, _ string) error {
	return nil
}

func TestSurrealProviderStartAndCapability(t *testing.T) {
	dep := &fakeSurrealDep{cfg: config.SurrealConfig{Port: 18000}}
	factory := NewSurrealDBProviderFactoryWithDep(dep)
	ctx := runtimeorchestrator.ProviderBuildContext{
		Config: &config.Config{
			Providers: config.ProvidersConfig{
				GraphStore: config.GraphStoreProviderConfig{
					Enabled:  true,
					Required: false,
					SurrealDB: config.SurrealConfig{
						Port: 18000,
					},
				},
			},
		},
	}
	inst, err := factory.Build(ctx)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := inst.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if dep.startCalls != 1 || dep.waitCalls != 1 || dep.monitorCalls != 1 {
		t.Fatalf("start calls incorrect")
	}
	cap := inst.Capability()
	if cap == nil {
		t.Fatalf("capability should be available after Start")
	}
	svc, ok := cap.(graph.Service)
	if !ok {
		t.Fatalf("capability is not graph.Service")
	}
	if svc.Name() != "fake-graph" {
		t.Fatalf("unexpected service name: %s", svc.Name())
	}
}

func TestSurrealProviderSubscribeCapability(t *testing.T) {
	dep := &fakeSurrealDep{cfg: config.SurrealConfig{Port: 18000}}
	factory := NewSurrealDBProviderFactoryWithDep(dep)
	ctx := runtimeorchestrator.ProviderBuildContext{
		Config: &config.Config{
			Providers: config.ProvidersConfig{
				GraphStore: config.GraphStoreProviderConfig{
					Enabled: true,
					SurrealDB: config.SurrealConfig{
						Port: 18000,
					},
				},
			},
		},
	}
	inst, _ := factory.Build(ctx)
	if err := inst.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	publisher, ok := inst.(runtimeorchestrator.CapabilityPublisher)
	if !ok {
		t.Fatalf("surrealProvider does not implement CapabilityPublisher")
	}

	var got []any
	unsub := publisher.SubscribeCapability(func(v any) { got = append(got, v) })
	dep.mu.Lock()
	cb := dep.restartCb
	dep.mu.Unlock()
	if cb == nil {
		t.Fatalf("restart callback not registered")
	}
	cb()
	if len(got) != 1 {
		t.Fatalf("subscriber not notified on restart, got %d updates", len(got))
	}
	unsub()
	got = nil
	cb()
	if len(got) != 0 {
		t.Fatalf("subscriber should not be notified after unsubscribe")
	}
}

func TestSurrealProviderPanicInSubscriber(t *testing.T) {
	dep := &fakeSurrealDep{cfg: config.SurrealConfig{Port: 18000}}
	factory := NewSurrealDBProviderFactoryWithDep(dep)
	ctx := runtimeorchestrator.ProviderBuildContext{
		Config: &config.Config{
			Providers: config.ProvidersConfig{
				GraphStore: config.GraphStoreProviderConfig{
					Enabled: true,
					SurrealDB: config.SurrealConfig{
						Port: 18000,
					},
				},
			},
		},
	}
	inst, _ := factory.Build(ctx)
	_ = inst.Start(context.Background())
	panicCalled := false
	publisher, ok := inst.(runtimeorchestrator.CapabilityPublisher)
	if !ok {
		t.Fatalf("surreal provider not a publisher")
	}
	publisher.SubscribeCapability(func(v any) { panicCalled = true; panic("sub") })
	publisher.SubscribeCapability(func(v any) {})
	dep.mu.Lock()
	cb := dep.restartCb
	dep.mu.Unlock()
	cb()
	if !panicCalled {
		t.Fatalf("subscriber panic not recovered")
	}
}

func TestSurrealProviderStop(t *testing.T) {
	dep := &fakeSurrealDep{cfg: config.SurrealConfig{Port: 18000}}
	factory := NewSurrealDBProviderFactoryWithDep(dep)
	ctx := runtimeorchestrator.ProviderBuildContext{
		Config: &config.Config{
			Providers: config.ProvidersConfig{
				GraphStore: config.GraphStoreProviderConfig{
					Enabled: true,
					SurrealDB: config.SurrealConfig{
						Port: 18000,
					},
				},
			},
		},
	}
	inst, _ := factory.Build(ctx)
	_ = inst.Start(context.Background())
	_ = inst.Stop(context.Background())
	if dep.stopCalls != 1 || dep.stopMonitorCalls != 1 || dep.setShuttingDown != 1 {
		t.Fatalf("stop not called correctly")
	}
	if inst.Capability() != nil {
		t.Fatalf("capability should be nil after Stop")
	}
}

func TestSurrealProviderStartRetries(t *testing.T) {
	counter := 0
	dep := &fakeSurrealDep{cfg: config.SurrealConfig{Port: 18000}}
	dep.newClientFn = func(cfg config.SurrealConfig) (*graph.Client, error) {
		counter++
		if counter < 2 {
			return nil, errors.New("temporary")
		}
		return &graph.Client{}, nil
	}
	factory := NewSurrealDBProviderFactoryWithDep(dep)
	ctx := runtimeorchestrator.ProviderBuildContext{
		Config: &config.Config{
			Providers: config.ProvidersConfig{
				GraphStore: config.GraphStoreProviderConfig{
					Enabled: true,
					SurrealDB: config.SurrealConfig{
						Port: 18000,
					},
				},
			},
		},
	}
	inst, _ := factory.Build(ctx)
	done := make(chan error, 1)
	go func() { done <- inst.Start(context.Background()) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start retry failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Start retry hung")
	}
}

package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	surrealdbpkg "github.com/u-ai/backend/pkg/database/surrealdb"
)

type SurrealDependency interface {
	StartSurreal() error
	WaitForSurreal(port int) error
	StartSurrealMonitor()
	SetSurrealShuttingDown()
	StopSurrealMonitor()
	StopSurreal()
	GetConfig() config.SurrealConfig
	NewClient(cfg config.SurrealConfig) (*graph.Client, error)
	NewGraphService(client *graph.Client) graph.Service
	SetRestartCallback(fn func())
	HealthCheck(ctx context.Context, cfg config.SurrealConfig) error
}

type defaultSurrealDep struct{}

func (defaultSurrealDep) StartSurreal() error           { return surrealdbpkg.StartSurreal() }
func (defaultSurrealDep) WaitForSurreal(port int) error { return surrealdbpkg.WaitForSurreal(port) }
func (defaultSurrealDep) StartSurrealMonitor()          { surrealdbpkg.StartSurrealMonitor() }
func (defaultSurrealDep) SetSurrealShuttingDown()       { surrealdbpkg.SetSurrealShuttingDown() }
func (defaultSurrealDep) StopSurrealMonitor()           { surrealdbpkg.StopSurrealMonitor() }
func (defaultSurrealDep) StopSurreal()                  { surrealdbpkg.StopSurreal() }
func (defaultSurrealDep) GetConfig() config.SurrealConfig {
	return config.AppCfg.Providers.GraphStore.SurrealDB
}
func (defaultSurrealDep) NewClient(cfg config.SurrealConfig) (*graph.Client, error) {
	return graph.NewClient(cfg)
}
func (defaultSurrealDep) NewGraphService(client *graph.Client) graph.Service {
	return graph.NewService(client)
}
func (defaultSurrealDep) SetRestartCallback(fn func()) {
	surrealdbpkg.SetSurrealRestartCallback(fn)
}
func (defaultSurrealDep) HealthCheck(ctx context.Context, cfg config.SurrealConfig) error {
	url := fmt.Sprintf("ws://%s:%d/rpc", cfg.Host, cfg.Port)
	db, err := surrealdb.New(url)
	if err != nil {
		return err
	}
	defer db.Close(ctx)
	if _, err := db.SignIn(ctx, map[string]string{"user": cfg.Username, "pass": cfg.Password}); err != nil {
		if _, err2 := db.SignIn(ctx, map[string]string{"user": "root", "pass": "root"}); err2 != nil {
			return err2
		}
	}
	if err := db.Use(ctx, cfg.Namespace, cfg.Database); err != nil {
		return err
	}
	_, err = surrealdb.Query[any](ctx, db, "SELECT 1", nil)
	return err
}

type SurrealDBProviderFactory struct {
	dep SurrealDependency
}

func NewSurrealDBProviderFactory() *SurrealDBProviderFactory {
	return &SurrealDBProviderFactory{dep: defaultSurrealDep{}}
}

func NewSurrealDBProviderFactoryWithDep(dep SurrealDependency) *SurrealDBProviderFactory {
	return &SurrealDBProviderFactory{dep: dep}
}

func (f *SurrealDBProviderFactory) ProviderID() string { return "builtin.surrealdb-process" }
func (f *SurrealDBProviderFactory) Slot() runtimeorchestrator.ProviderSlot {
	return runtimeorchestrator.ProviderSlotGraphStore
}
func (f *SurrealDBProviderFactory) Requirements() []runtimehost.CapabilityRequirement {
	return []runtimehost.CapabilityRequirement{
		{ID: runtimehost.CapProcessSpawn, Minimum: runtimehost.SupportSupported},
		{ID: runtimehost.CapProcessTreeControl, Minimum: runtimehost.SupportSupported},
		{ID: runtimehost.CapProcessRestart, Minimum: runtimehost.SupportSupported},
		{ID: runtimehost.CapFilesystemExecutable, Minimum: runtimehost.SupportSupported},
		{ID: runtimehost.CapNetworkLoopback, Minimum: runtimehost.SupportSupported},
	}
}

func (f *SurrealDBProviderFactory) Build(ctx runtimeorchestrator.ProviderBuildContext) (runtimeorchestrator.ProviderInstance, error) {
	if ctx.Config == nil {
		return nil, runtimeorchestrator.DescriptorFailure("", "nil config")
	}
	return &surrealProvider{
		dep:    f.dep,
		config: &ctx.Config.Providers.GraphStore,
		host:   ctx.Host,
	}, nil
}

type surrealProvider struct {
	dep         SurrealDependency
	config      *config.GraphStoreProviderConfig
	host        runtimehost.RuntimeHost
	mu          sync.RWMutex
	capability  graph.Service
	subscribers []func(any)
	started     bool
	stopped     bool
}

func (p *surrealProvider) Descriptor() runtimeorchestrator.ComponentDescriptor {
	return runtimeorchestrator.ComponentDescriptor{
		ID:           runtimeorchestrator.ComponentGraphStore,
		Phase:        runtimeorchestrator.PhaseInfrastructure,
		Enabled:      p.config.Enabled,
		Required:     p.config.Required,
		Capabilities: []string{"storage.graph"},
	}
}

func (p *surrealProvider) Slot() runtimeorchestrator.ProviderSlot {
	return runtimeorchestrator.ProviderSlotGraphStore
}

func (p *surrealProvider) ProviderID() string { return "builtin.surrealdb-process" }

func (p *surrealProvider) Capability() any {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.capability
}

func (p *surrealProvider) SubscribeCapability(fn func(any)) func() {
	p.mu.Lock()
	p.subscribers = append(p.subscribers, fn)
	idx := len(p.subscribers) - 1
	p.mu.Unlock()
	return func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		if idx < len(p.subscribers) {
			p.subscribers[idx] = nil
		}
	}
}

func (p *surrealProvider) Start(ctx context.Context) error {
	if p.started {
		return nil
	}
	if err := p.dep.StartSurreal(); err != nil {
		return p.startFail("StartSurreal", err)
	}
	if err := p.dep.WaitForSurreal(p.config.SurrealDB.Port); err != nil {
		p.dep.StopSurreal()
		return p.startFail("WaitForSurreal", err)
	}
	cfg := p.dep.GetConfig()
	var client *graph.Client
	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			p.dep.StopSurreal()
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
		c, err := p.dep.NewClient(cfg)
		if err == nil {
			client = c
			break
		}
	}
	if client == nil {
		p.dep.StopSurreal()
		return p.startFail("NewClient", context.DeadlineExceeded)
	}
	svc := p.dep.NewGraphService(client)
	p.mu.Lock()
	p.capability = svc
	p.mu.Unlock()

	p.dep.SetRestartCallback(p.handleRestart)
	p.dep.StartSurrealMonitor()
	p.started = true

	p.notifySubscribers(svc)
	return nil
}

func (p *surrealProvider) Ready(ctx context.Context) error {
	p.mu.RLock()
	svc := p.capability
	p.mu.RUnlock()
	if svc == nil {
		return runtimeorchestrator.DescriptorFailure("", "surreal graph service not initialized")
	}
	hctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return p.dep.HealthCheck(hctx, p.dep.GetConfig())
}

func (p *surrealProvider) Stop(ctx context.Context) error {
	if p.stopped {
		return nil
	}
	p.dep.SetSurrealShuttingDown()
	p.dep.SetRestartCallback(nil)
	p.dep.StopSurrealMonitor()
	p.dep.StopSurreal()
	p.mu.Lock()
	p.capability = nil
	var subs []func(any)
	subs = append(subs, p.subscribers...)
	p.subscribers = nil
	p.stopped = true
	p.mu.Unlock()
	for _, fn := range subs {
		if fn != nil {
			func() {
				defer func() { recover() }()
				fn(nil)
			}()
		}
	}
	return nil
}

func (p *surrealProvider) handleRestart() {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	cfg := p.dep.GetConfig()
	var subs []func(any)
	subs = append(subs, p.subscribers...)
	p.mu.Unlock()

	defer func() { recover() }()

	var client *graph.Client
	for i := 0; i < 30; i++ {
		c, err := p.dep.NewClient(cfg)
		if err == nil {
			client = c
			break
		}
		select {
		case <-time.After(1 * time.Second):
		}
	}
	if client == nil {
		return
	}
	svc := p.dep.NewGraphService(client)

	p.mu.Lock()
	p.mu.Unlock()

	for _, fn := range subs {
		if fn == nil {
			continue
		}
		func() {
			defer func() { recover() }()
			fn(svc)
		}()
	}
}

func (p *surrealProvider) notifySubscribers(svc graph.Service) {
	p.mu.RLock()
	var subs []func(any)
	subs = append(subs, p.subscribers...)
	p.mu.RUnlock()
	for _, fn := range subs {
		if fn == nil {
			continue
		}
		func() {
			defer func() { recover() }()
			fn(svc)
		}()
	}
}

func (p *surrealProvider) startFail(stage string, err error) error {
	return &surrealPhaseError{phase: "surrealdb:" + stage, err: err}
}

type surrealPhaseError struct {
	phase string
	err   error
}

func (e *surrealPhaseError) Error() string { return e.phase + ": " + e.err.Error() }
func (e *surrealPhaseError) Unwrap() error { return e.err }

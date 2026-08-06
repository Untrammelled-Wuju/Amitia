package builtin

import (
	"context"
	"sync"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	qdrantpkg "github.com/u-ai/backend/pkg/database/qdrant"
)

type QdrantDependency interface {
	StartQdrant() error
	WaitForQdrant(port int) error
	InitClient() error
	EnsureCollections() error
	SetQdrantShuttingDown()
	StopQdrant()
	GetClient() *qdrant.Client
}

type defaultQdrantDep struct{}

func (defaultQdrantDep) StartQdrant() error           { return qdrantpkg.StartQdrant() }
func (defaultQdrantDep) WaitForQdrant(port int) error { return qdrantpkg.WaitForQdrant(port) }
func (defaultQdrantDep) InitClient() error            { return qdrantpkg.InitClient() }
func (defaultQdrantDep) EnsureCollections() error     { return qdrantpkg.EnsureCollections() }
func (defaultQdrantDep) SetQdrantShuttingDown()       { qdrantpkg.SetQdrantShuttingDown() }
func (defaultQdrantDep) StopQdrant()                  { qdrantpkg.StopQdrant() }
func (defaultQdrantDep) GetClient() *qdrant.Client    { return qdrantpkg.Client }

type QdrantProviderFactory struct {
	dep QdrantDependency
}

func NewQdrantProviderFactory() *QdrantProviderFactory {
	return &QdrantProviderFactory{dep: defaultQdrantDep{}}
}

func NewQdrantProviderFactoryWithDep(dep QdrantDependency) *QdrantProviderFactory {
	return &QdrantProviderFactory{dep: dep}
}

func (f *QdrantProviderFactory) ProviderID() string { return "builtin.qdrant-process" }
func (f *QdrantProviderFactory) Slot() runtimeorchestrator.ProviderSlot {
	return runtimeorchestrator.ProviderSlotVectorStore
}
func (f *QdrantProviderFactory) Requirements() []runtimehost.CapabilityRequirement {
	return []runtimehost.CapabilityRequirement{
		{ID: runtimehost.CapProcessSpawn, Minimum: runtimehost.SupportSupported},
		{ID: runtimehost.CapProcessTreeControl, Minimum: runtimehost.SupportSupported},
		{ID: runtimehost.CapProcessRestart, Minimum: runtimehost.SupportSupported},
		{ID: runtimehost.CapFilesystemExecutable, Minimum: runtimehost.SupportSupported},
		{ID: runtimehost.CapNetworkLoopback, Minimum: runtimehost.SupportSupported},
	}
}

func (f *QdrantProviderFactory) Build(ctx runtimeorchestrator.ProviderBuildContext) (runtimeorchestrator.ProviderInstance, error) {
	if ctx.Config == nil {
		return nil, runtimeorchestrator.DescriptorFailure("", "nil config")
	}
	return &qdrantProvider{
		dep:    f.dep,
		config: &ctx.Config.Providers.VectorStore,
		host:   ctx.Host,
	}, nil
}

type qdrantProvider struct {
	dep          QdrantDependency
	config       *config.VectorStoreProviderConfig
	host         runtimehost.RuntimeHost
	capabilityMu sync.RWMutex
	client       *qdrant.Client
	started      bool
	stopped      bool
}

func (p *qdrantProvider) Descriptor() runtimeorchestrator.ComponentDescriptor {
	return runtimeorchestrator.ComponentDescriptor{
		ID:           runtimeorchestrator.ComponentVectorStore,
		Phase:        runtimeorchestrator.PhaseInfrastructure,
		Enabled:      p.config.Enabled,
		Required:     p.config.Required,
		Capabilities: []string{"storage.vector"},
	}
}

func (p *qdrantProvider) Slot() runtimeorchestrator.ProviderSlot {
	return runtimeorchestrator.ProviderSlotVectorStore
}

func (p *qdrantProvider) ProviderID() string { return "builtin.qdrant-process" }

func (p *qdrantProvider) Capability() any {
	p.capabilityMu.RLock()
	defer p.capabilityMu.RUnlock()
	return p.client
}

func (p *qdrantProvider) Start(ctx context.Context) error {
	if p.started {
		return nil
	}
	if err := p.dep.StartQdrant(); err != nil {
		return p.startFail("StartQdrant", err)
	}
	if err := p.dep.WaitForQdrant(p.config.Qdrant.Port); err != nil {
		p.dep.StopQdrant()
		return p.startFail("WaitForQdrant", err)
	}
	if err := p.dep.InitClient(); err != nil {
		p.dep.StopQdrant()
		return p.startFail("InitClient", err)
	}
	if err := p.dep.EnsureCollections(); err != nil {
		p.dep.StopQdrant()
		return p.startFail("EnsureCollections", err)
	}
	p.started = true
	return nil
}

func (p *qdrantProvider) Ready(ctx context.Context) error {
	client := p.dep.GetClient()
	if client == nil {
		return runtimeorchestrator.DescriptorFailure("", "qdrant client not initialized")
	}
	p.capabilityMu.Lock()
	p.client = client
	p.capabilityMu.Unlock()
	hctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := client.HealthCheck(hctx); err != nil {
		return err
	}
	return nil
}

func (p *qdrantProvider) Stop(ctx context.Context) error {
	if p.stopped {
		return nil
	}
	p.dep.SetQdrantShuttingDown()
	p.dep.StopQdrant()
	p.stopped = true
	return nil
}

func (p *qdrantProvider) startFail(stage string, err error) error {
	return &phaseError{phase: "qdrant:" + stage, err: err}
}

type phaseError struct {
	phase string
	err   error
}

func (e *phaseError) Error() string { return e.phase + ": " + e.err.Error() }
func (e *phaseError) Unwrap() error { return e.err }

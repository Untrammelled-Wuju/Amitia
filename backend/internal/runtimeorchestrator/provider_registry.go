package runtimeorchestrator

import (
	"sync"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
)

type ProviderSlot string

const (
	ProviderSlotScriptRuntime ProviderSlot = "script-runtime"
	ProviderSlotVectorStore   ProviderSlot = "vector-store"
	ProviderSlotGraphStore    ProviderSlot = "graph-store"
)

type ProviderBuildContext struct {
	Config *config.Config
	Host   runtimehost.RuntimeHost
}

type ProviderInstance interface {
	ManagedComponent
	Slot() ProviderSlot
	ProviderID() string
	Capability() any
}

type CapabilityPublisher interface {
	SubscribeCapability(func(any)) func()
}

type ProviderFactory interface {
	ProviderID() string
	Slot() ProviderSlot
	Requirements() []runtimehost.CapabilityRequirement
	Build(ProviderBuildContext) (ProviderInstance, error)
}

type ProviderRegistry struct {
	mu        sync.RWMutex
	factories map[providerKey]ProviderFactory
}

type providerKey struct {
	slot       ProviderSlot
	providerID string
}

func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		factories: make(map[providerKey]ProviderFactory),
	}
}

func (r *ProviderRegistry) Register(factory ProviderFactory) error {
	if factory == nil {
		return invalidDescriptorErr("nil provider factory")
	}
	key := providerKey{slot: factory.Slot(), providerID: factory.ProviderID()}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.factories[key]; exists {
		return providerAlreadyRegisteredErr(string(key.slot), key.providerID)
	}
	r.factories[key] = factory
	return nil
}

func (r *ProviderRegistry) Lookup(slot ProviderSlot, providerID string) (ProviderFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.factories[providerKey{slot: slot, providerID: providerID}]
	return f, ok
}

func (r *ProviderRegistry) Build(slot ProviderSlot, providerID string, ctx ProviderBuildContext) (ProviderInstance, error) {
	r.mu.RLock()
	factory, ok := r.factories[providerKey{slot: slot, providerID: providerID}]
	r.mu.RUnlock()
	if !ok {
		return nil, providerNotFoundErr(string(slot), providerID)
	}
	if factory.Slot() != slot {
		return nil, providerSlotMismatchErr(string(slot), providerID)
	}
	if factory.ProviderID() != providerID {
		return nil, providerNotFoundErr(string(slot), providerID)
	}

	// Check capability requirements
	if ctx.Host != nil {
		hostCaps := ctx.Host.Capabilities()
		for _, req := range factory.Requirements() {
			if !hostCaps.RequirementSatisfied(req) {
				return nil, &runtimehost.CapabilityRequirementUnsatisfiedError{
					ProviderID: factory.ProviderID(),
					Capability: req.ID,
					Required:   req.Minimum,
					Actual:     hostCaps.Support(req.ID),
				}
			}
		}
	}

	instance, err := factory.Build(ctx)
	if err != nil {
		return nil, err
	}
	if instance.Slot() != slot {
		return nil, providerSlotMismatchErr(string(slot), providerID)
	}
	if instance.ProviderID() != providerID {
		return nil, providerNotFoundErr(string(slot), instance.ProviderID())
	}
	return instance, nil
}

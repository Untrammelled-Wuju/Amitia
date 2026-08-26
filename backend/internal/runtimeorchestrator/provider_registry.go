package runtimeorchestrator

import (
	"sync"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

type ProviderSlot string

const (
	ProviderSlotScriptRuntime ProviderSlot = "script-runtime"
	ProviderSlotVectorStore   ProviderSlot = "vector-store"
	ProviderSlotGraphStore    ProviderSlot = "graph-store"

	ProviderSlotIOSSandbox ProviderSlot = "ios-sandbox"
	ProviderSlotIOSNative  ProviderSlot = "ios-native"
)

type ProviderBuildContext struct {
	Config  *config.Config
	Host    runtimehost.RuntimeHost
	Profile runtimeprofile.Profile
}

// ProviderInstance is a runtime-orchestrator managed component instance.
// This ProviderInstance is a runtime-orchestrator managed component instance, not capability.CapabilityProviderInstance.
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

// ProfileAwareProviderFactory is an optional interface that ProviderFactory implementations
// may implement to declare which runtime profiles they support. If a factory implements
// this interface, Build will only be called when the profile is supported.
type ProfileAwareProviderFactory interface {
	SupportedProfiles() []runtimeprofile.Profile
}

// ProviderRegistry owns runtime provider factories used to construct infrastructure/runtime components.
// It is not the capability ProviderRegistry and must not be used to resolve AI capability providers or device execution targets.
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

	if profileAware, ok := factory.(ProfileAwareProviderFactory); ok {
		if !isProfileSupported(profileAware, ctx.Profile) {
			return nil, providerProfileUnsupportedErr(string(slot), providerID, string(ctx.Profile))
		}
	}

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

func isProfileSupported(factory ProfileAwareProviderFactory, profile runtimeprofile.Profile) bool {
	if !profile.IsValid() {
		return false
	}
	for _, supported := range factory.SupportedProfiles() {
		if supported == profile {
			return true
		}
	}
	return false
}

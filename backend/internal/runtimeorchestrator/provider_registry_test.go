package runtimeorchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
)

type fakeProviderInstance struct {
	ComponentDescriptor
	slot       ProviderSlot
	providerID string
	cap        any
}

func (f *fakeProviderInstance) Descriptor() ComponentDescriptor { return f.ComponentDescriptor }
func (f *fakeProviderInstance) Start(ctx context.Context) error  { return nil }
func (f *fakeProviderInstance) Ready(ctx context.Context) error  { return nil }
func (f *fakeProviderInstance) Stop(ctx context.Context) error   { return nil }
func (f *fakeProviderInstance) Slot() ProviderSlot               { return f.slot }
func (f *fakeProviderInstance) ProviderID() string               { return f.providerID }
func (f *fakeProviderInstance) Capability() any                 { return f.cap }

type fakeProviderFactory struct {
	id    string
	slot  ProviderSlot
	build func(ProviderBuildContext) (ProviderInstance, error)
}

func (f *fakeProviderFactory) ProviderID() string                        { return f.id }
func (f *fakeProviderFactory) Slot() ProviderSlot                       { return f.slot }
func (f *fakeProviderFactory) Build(ctx ProviderBuildContext) (ProviderInstance, error) {
	if f.build != nil {
		return f.build(ctx)
	}
	return &fakeProviderInstance{slot: f.slot, providerID: f.id}, nil
}
func (f *fakeProviderFactory) Requirements() []runtimehost.CapabilityRequirement {
	return nil
}

func TestProviderRegistryRegisterAndLookup(t *testing.T) {
	reg := NewProviderRegistry()
	f := &fakeProviderFactory{id: "test.1", slot: ProviderSlotVectorStore}
	if err := reg.Register(f); err != nil {
		t.Fatalf("register: %v", err)
	}
	got, ok := reg.Lookup(ProviderSlotVectorStore, "test.1")
	if !ok {
		t.Fatalf("lookup failed for test.1")
	}
	if got.ProviderID() != "test.1" {
		t.Fatalf("lookup wrong provider")
	}
}

func TestProviderRegistryRejectsDuplicateFactory(t *testing.T) {
	reg := NewProviderRegistry()
	f := &fakeProviderFactory{id: "dup", slot: ProviderSlotVectorStore}
	_ = reg.Register(f)
	err := reg.Register(f)
	if !errors.Is(err, ErrProviderAlreadyRegistered) {
		t.Fatalf("expected ErrProviderAlreadyRegistered, got %v", err)
	}
}

func TestProviderRegistryAllowsSameIDInDifferentSlot(t *testing.T) {
	reg := NewProviderRegistry()
	f1 := &fakeProviderFactory{id: "shared", slot: ProviderSlotVectorStore}
	f2 := &fakeProviderFactory{id: "shared", slot: ProviderSlotGraphStore}
	if err := reg.Register(f1); err != nil {
		t.Fatalf("register f1: %v", err)
	}
	if err := reg.Register(f2); err != nil {
		t.Fatalf("register f2 (different slot): %v", err)
	}
}

func TestProviderRegistryBuildUnknownProvider(t *testing.T) {
	reg := NewProviderRegistry()
	_, err := reg.Build(ProviderSlotVectorStore, "not.exist", ProviderBuildContext{})
	if !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestProviderRegistryBuildSlotMismatch(t *testing.T) {
	reg := NewProviderRegistry()
	mismatch := &fakeProviderFactory{
		id:   "mismatch",
		slot: ProviderSlotVectorStore,
		build: func(ctx ProviderBuildContext) (ProviderInstance, error) {
			return &fakeProviderInstance{slot: ProviderSlotGraphStore, providerID: "mismatch"}, nil
		},
	}
	_ = reg.Register(mismatch)
	_, err := reg.Build(ProviderSlotVectorStore, "mismatch", ProviderBuildContext{})
	if !errors.Is(err, ErrProviderSlotMismatch) {
		t.Fatalf("expected ErrProviderSlotMismatch, got %v", err)
	}
}

func TestProviderRegistryBuildError(t *testing.T) {
	reg := NewProviderRegistry()
	failing := &fakeProviderFactory{
		id:   "failing",
		slot: ProviderSlotVectorStore,
		build: func(ctx ProviderBuildContext) (ProviderInstance, error) {
			return nil, errors.New("build failed")
		},
	}
	_ = reg.Register(failing)
	_, err := reg.Build(ProviderSlotVectorStore, "failing", ProviderBuildContext{})
	if err == nil || err.Error() != "build failed" {
		t.Fatalf("expected 'build failed', got %v", err)
	}
}

func TestProviderRegistryBuildContextPassing(t *testing.T) {
	reg := NewProviderRegistry()
	var captured ProviderBuildContext
	capture := &fakeProviderFactory{
		id:   "capture",
		slot: ProviderSlotVectorStore,
		build: func(ctx ProviderBuildContext) (ProviderInstance, error) {
			captured = ctx
			return &fakeProviderInstance{slot: ProviderSlotVectorStore, providerID: "capture"}, nil
		},
	}
	_ = reg.Register(capture)
	ctx := ProviderBuildContext{Config: &config.Config{}}
	inst, bErr := reg.Build(ProviderSlotVectorStore, "capture", ctx)
	_ = inst
	_ = bErr
	if captured.Config == nil {
		t.Fatalf("build context not passed through")
	}
}

func TestProviderRegistryDoesNotAutoSelectProvider(t *testing.T) {
	reg := NewProviderRegistry()
	_ = reg.Register(&fakeProviderFactory{id: "a", slot: ProviderSlotVectorStore})
	_ = reg.Register(&fakeProviderFactory{id: "b", slot: ProviderSlotVectorStore})
	_, ok := reg.Lookup(ProviderSlotVectorStore, "auto.selected")
	if ok {
		t.Fatalf("registry must not auto-select providers")
	}
}

package display

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/androidnative/virtualdisplay"
)

func newTestStore() (*DisplayStore, *DisplayClassifier) {
	classifier := NewDisplayClassifier()
	store := NewDisplayStore(classifier)
	return store, classifier
}

func TestResolver_ResolveByDisplayID(t *testing.T) {
	store, _ := newTestStore()
	store.Put(DisplayInfo{DisplayID: 0, Generation: 1, IsDefault: true, Width: 1080, Height: 2400, DensityDPI: 420, State: string(DisplayStateOn), IsValid: true, CoordinateSpace: DisplayCoordinateSpace{DisplayID: 0, Width: 1080, Height: 2400}})

	resolver := NewDefaultResolver(store, DefaultSelectionPolicy)
	result, err := resolver.Resolve(context.Background(), DisplayResolveRequest{DisplayID: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Found {
		t.Error("expected found=true")
	}
	if result.Target.DisplayID != 0 {
		t.Errorf("expected display 0, got %d", result.Target.DisplayID)
	}
	if result.Target.Width != 1080 {
		t.Errorf("expected width 1080, got %d", result.Target.Width)
	}
}

func TestResolver_ResolveNotFound(t *testing.T) {
	store, _ := newTestStore()
	resolver := NewDefaultResolver(store, DefaultSelectionPolicy)
	_, err := resolver.Resolve(context.Background(), DisplayResolveRequest{DisplayID: 99})
	if err == nil {
		t.Fatal("expected error for non-existent display")
	}
}

func TestResolver_ResolveByVirtualRef(t *testing.T) {
	store, _ := newTestStore()
	ref := virtualdisplay.VirtualDisplayRef("vd_amitia_001")
	store.Put(DisplayInfo{DisplayID: 4, Generation: 1, Width: 1080, Height: 1920, IsValid: true, ManagedByAmitia: true, VirtualRef: &ref})
	store.SetManagedVirtual(4, &ref)

	resolver := NewDefaultResolver(store, DefaultSelectionPolicy)
	result, err := resolver.Resolve(context.Background(), DisplayResolveRequest{DisplayID: -1, VirtualRef: "vd_amitia_001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Target.DisplayID != 4 {
		t.Errorf("expected display 4, got %d", result.Target.DisplayID)
	}
}

func TestResolver_ResolveByRef(t *testing.T) {
	store, _ := newTestStore()
	store.Put(DisplayInfo{DisplayID: 2, Generation: 3, Ref: "display:2:3", Width: 1920, Height: 1080})

	resolver := NewDefaultResolver(store, DefaultSelectionPolicy)
	result, err := resolver.Resolve(context.Background(), DisplayResolveRequest{DisplayID: -1, Ref: "display:2:3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Target.DisplayID != 2 {
		t.Errorf("expected display 2, got %d", result.Target.DisplayID)
	}
}

func TestResolver_AmbiguousByDefault(t *testing.T) {
	store, _ := newTestStore()
	store.Put(DisplayInfo{DisplayID: 0, IsDefault: true})
	store.Put(DisplayInfo{DisplayID: 1})

	resolver := NewDefaultResolver(store, DefaultSelectionPolicy)
	_, err := resolver.Resolve(context.Background(), DisplayResolveRequest{DisplayID: -1})
	if err == nil {
		t.Fatal("expected ambiguous error")
	}
	displayErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *display.Error, got %T", err)
	}
	if displayErr.Code != ErrDisplayAmbiguous {
		t.Errorf("expected ambiguous error code, got %s", displayErr.Code)
	}
}

func TestResolver_AllowFallback(t *testing.T) {
	store, _ := newTestStore()
	store.Put(DisplayInfo{DisplayID: 0, IsDefault: true})
	store.Put(DisplayInfo{DisplayID: 1})

	policy := DisplaySelectionPolicy{PreferExplicit: true, AllowDefaultFallback: true, RejectAmbiguous: false}
	resolver := NewDefaultResolver(store, policy)
	result, err := resolver.Resolve(context.Background(), DisplayResolveRequest{DisplayID: -1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Target.DisplayID != 0 {
		t.Errorf("expected fallback to display 0, got %d", result.Target.DisplayID)
	}
}

func TestResolver_ValidateGeneration(t *testing.T) {
	store, _ := newTestStore()
	store.Put(DisplayInfo{DisplayID: 0, Generation: 1})

	resolver := NewDefaultResolver(store, DefaultSelectionPolicy)
	if err := resolver.ValidateGeneration(0, 1); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := resolver.ValidateGeneration(0, 2); err == nil {
		t.Error("expected error for stale generation")
	}
}

func TestResolver_ValidateGeneration_Removed(t *testing.T) {
	store, _ := newTestStore()
	store.Put(DisplayInfo{DisplayID: 0, Generation: 1})
	store.Remove(0)

	resolver := NewDefaultResolver(store, DefaultSelectionPolicy)
	if err := resolver.ValidateGeneration(0, 1); err == nil {
		t.Error("expected error for removed display")
	}
}

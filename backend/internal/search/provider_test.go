package search

import (
	"context"
	"testing"
)

type testProvider struct {
	id   string
	caps ProviderCapabilities
	h    ProviderHealth
	err  error
}

func (p *testProvider) ID() string                        { return p.id }
func (p *testProvider) Capabilities() ProviderCapabilities { return p.caps }
func (p *testProvider) Search(_ context.Context, _ SearchRequest) (ProviderSearchResponse, error) {
	return ProviderSearchResponse{}, p.err
}
func (p *testProvider) Health(_ context.Context) ProviderHealth {
	return p.h
}

func TestProviderSet_RegisterAndGet(t *testing.T) {
	set := NewProviderSet("")
	p := &testProvider{id: "fake"}
	set.Register("fake", p)
	got, ok := set.Get("fake")
	if !ok {
		t.Fatal("provider should be found by id")
	}
	if got.ID() != "fake" {
		t.Fatalf("wrong provider retrieved: %s", got.ID())
	}
}

func TestProviderSet_Get_NotExist(t *testing.T) {
	set := NewProviderSet("")
	_, ok := set.Get("missing")
	if ok {
		t.Fatal("should not find missing provider")
	}
}

func TestProviderSet_SetDefault(t *testing.T) {
	set := NewProviderSet("")
	set.Register("a", &testProvider{id: "a"})
	set.Register("b", &testProvider{id: "b"})
	if !set.SetDefault("b") {
		t.Fatal("set default should succeed")
	}
	def, ok := set.Default()
	if !ok {
		t.Fatal("get default should succeed")
	}
	if def.ID() != "b" {
		t.Fatalf("expected default 'b', got %s", def.ID())
	}
}

func TestProviderSet_SetDefault_Unknown(t *testing.T) {
	set := NewProviderSet("")
	if set.SetDefault("nonexistent") {
		t.Fatal("set default for unknown should fail")
	}
}

func TestProviderSet_Default_NoProviders(t *testing.T) {
	set := NewProviderSet("")
	_, ok := set.Default()
	if ok {
		t.Fatal("should not find default when no providers")
	}
}

func TestProviderSet_Has(t *testing.T) {
	set := NewProviderSet("")
	set.Register("p1", &testProvider{id: "p1"})
	if !set.Has("p1") {
		t.Fatal("Has(p1) should be true")
	}
	if set.Has("p2") {
		t.Fatal("Has(p2) should be false")
	}
}

func TestProviderSet_Count(t *testing.T) {
	set := NewProviderSet("")
	if set.Count() != 0 {
		t.Fatal("initial count should be 0")
	}
	set.Register("a", &testProvider{id: "a"})
	set.Register("b", &testProvider{id: "b"})
	if set.Count() != 2 {
		t.Fatalf("expected count 2, got %d", set.Count())
	}
}

func TestProviderSet_All(t *testing.T) {
	set := NewProviderSet("")
	set.Register("a", &testProvider{id: "a"})
	set.Register("b", &testProvider{id: "b"})
	all := set.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(all))
	}
}

func TestProviderSet_DefaultID(t *testing.T) {
	set := NewProviderSet("first")
	set.Register("first", &testProvider{id: "first"})
	if set.DefaultID() != "first" {
		t.Fatalf("expected DefaultID 'first', got %s", set.DefaultID())
	}
	set.SetDefault("first")
	if set.DefaultID() != "first" {
		t.Fatalf("DefaultID should remain 'first'")
	}
}

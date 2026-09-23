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

func (p *testProvider) ID() string                         { return p.id }
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

func TestProviderSet_CandidatesAreDeterministicByPriority(t *testing.T) {
	set := NewProviderSet("")
	caps := ProviderCapabilities{GeneralWeb: true}
	set.RegisterWithPriority("z-low", &testProvider{id: "z-low", caps: caps}, 10)
	set.RegisterWithPriority("b-high", &testProvider{id: "b-high", caps: caps}, 100)
	set.RegisterWithPriority("a-high", &testProvider{id: "a-high", caps: caps}, 100)

	ids := set.CandidateIDs(SearchKindWeb)
	want := []string{"a-high", "b-high", "z-low"}
	if len(ids) != len(want) {
		t.Fatalf("unexpected candidate count: %v", ids)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("candidate[%d]=%q, want %q", i, ids[i], want[i])
		}
	}
}

func TestProviderSet_DefaultProviderPrecedesHigherPriorityFallback(t *testing.T) {
	caps := ProviderCapabilities{GeneralWeb: true}
	set := NewProviderSet("preferred")
	set.RegisterWithPriority("preferred", &testProvider{id: "preferred", caps: caps}, 10)
	set.RegisterWithPriority("fallback", &testProvider{id: "fallback", caps: caps}, 100)
	ids := set.CandidateIDs(SearchKindWeb)
	if len(ids) != 2 || ids[0] != "preferred" || ids[1] != "fallback" {
		t.Fatalf("unexpected provider order: %v", ids)
	}
}

func TestProviderSetUnregisterClearsDefaultAndPriority(t *testing.T) {
	set := NewProviderSet("primary")
	set.RegisterWithPriority("primary", &testProvider{id: "primary"}, 100)
	set.RegisterWithPriority("backup", &testProvider{id: "backup"}, 50)

	if !set.Unregister("primary") {
		t.Fatal("expected unregister to succeed")
	}
	if set.Has("primary") || set.DefaultID() != "" {
		t.Fatalf("provider/default not cleared: has=%v default=%q", set.Has("primary"), set.DefaultID())
	}
	if _, ok := set.Priority("primary"); ok {
		t.Fatal("priority entry should be removed with provider")
	}
	if !set.Has("backup") {
		t.Fatal("unrelated provider should remain registered")
	}
}

func TestProviderSetResolveUsesInstanceOrDefault(t *testing.T) {
	set := NewProviderSet("primary")
	set.RegisterWithPriority("primary", &testProvider{id: "impl-primary"}, 100)
	set.RegisterWithPriority("backup", &testProvider{id: "impl-backup"}, 50)

	resolved, ok := set.Resolve("")
	if !ok || resolved.ID() != "impl-primary" {
		t.Fatalf("default resolve failed: %#v %v", resolved, ok)
	}
	resolved, ok = set.Resolve("backup")
	if !ok || resolved.ID() != "impl-backup" {
		t.Fatalf("instance resolve failed: %#v %v", resolved, ok)
	}
}

func TestProviderSetManifestDefaultsToActualCapabilities(t *testing.T) {
	set := NewProviderSet("")
	provider := &testProvider{id: "plugin", caps: ProviderCapabilities{GeneralWeb: true, SearchKinds: []SearchKind{SearchKindWeb}, DomainFilter: true}}
	if err := set.RegisterWithManifest("plugin_primary", provider, 50, ProviderManifest{NetworkScopes: []string{"https://api.example.com", "https://api.example.com"}}); err != nil {
		t.Fatal(err)
	}
	manifest, ok := set.Manifest("plugin_primary")
	if !ok {
		t.Fatal("manifest not registered")
	}
	if manifest.ID != "plugin_primary" || len(manifest.Capabilities) == 0 || len(manifest.NetworkScopes) != 1 {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
}

func TestProviderSetRejectsMismatchedManifestID(t *testing.T) {
	set := NewProviderSet("")
	err := set.RegisterWithManifest("instance", &testProvider{id: "impl"}, 1, ProviderManifest{ID: "other"})
	if err == nil {
		t.Fatal("expected manifest id mismatch error")
	}
}

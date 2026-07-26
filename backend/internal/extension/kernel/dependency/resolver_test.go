package dependency

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func ver(s string) domain.SemanticVersion {
	v, err := domain.ParseVersion(s)
	if err != nil {
		panic(err)
	}
	return v
}

func TestParseRangeExact(t *testing.T) {
	r, err := ParseRange("=1.2.3")
	if err != nil {
		t.Fatalf("ParseRange: %v", err)
	}
	if !r.Satisfies(ver("1.2.3")) {
		t.Errorf("should satisfy 1.2.3")
	}
	if r.Satisfies(ver("1.2.4")) {
		t.Errorf("should not satisfy 1.2.4")
	}
}

func TestParseRangeCaret(t *testing.T) {
	r, err := ParseRange("^1.2.0")
	if err != nil {
		t.Fatalf("ParseRange: %v", err)
	}
	if !r.Satisfies(ver("1.2.5")) {
		t.Errorf("should satisfy 1.2.5")
	}
	if !r.Satisfies(ver("1.9.0")) {
		t.Errorf("should satisfy 1.9.0")
	}
	if r.Satisfies(ver("2.0.0")) {
		t.Errorf("should not satisfy 2.0.0")
	}
}

func TestParseRangeTilde(t *testing.T) {
	r, err := ParseRange("~1.4.2")
	if err != nil {
		t.Fatalf("ParseRange: %v", err)
	}
	if !r.Satisfies(ver("1.4.5")) {
		t.Errorf("should satisfy 1.4.5")
	}
	if r.Satisfies(ver("1.5.0")) {
		t.Errorf("should not satisfy 1.5.0")
	}
}

func TestParseRangeCompound(t *testing.T) {
	r, err := ParseRange(">=1.2.0,<2.0.0")
	if err != nil {
		t.Fatalf("ParseRange: %v", err)
	}
	if !r.Satisfies(ver("1.5.0")) {
		t.Errorf("should satisfy 1.5.0")
	}
	if r.Satisfies(ver("2.0.0")) {
		t.Errorf("should not satisfy 2.0.0")
	}
	if r.Satisfies(ver("1.1.0")) {
		t.Errorf("should not satisfy 1.1.0")
	}
}

func TestIntersect(t *testing.T) {
	r1, _ := ParseRange(">=1.0.0")
	r2, _ := ParseRange("<2.0.0")
	combined := Intersect(r1, r2)
	if !combined.Satisfies(ver("1.5.0")) {
		t.Errorf("should satisfy 1.5.0")
	}
	if combined.Satisfies(ver("2.5.0")) {
		t.Errorf("should not satisfy 2.5.0")
	}
}

func TestResolveRequired(t *testing.T) {
	provider := CandidateProviderFunc(func(_ context.Context, target string, _ TargetType) ([]Candidate, error) {
		return []Candidate{
			{TargetID: target, Version: ver("1.2.0"), Origin: "installed", Available: true},
			{TargetID: target, Version: ver("1.5.0"), Origin: "system", Available: true},
		}, nil
	})
	r := NewDefaultResolver(provider)
	result := r.Resolve(context.Background(), ResolveRequest{
		SourceID: "com.example/dep-test",
		Phase:    PhaseInstall,
		Requests: []Request{
			{Target: "com.example/required", Type: TargetExtension, VersionRange: "^1.0.0", Required: true},
		},
	})
	if len(result.Resolutions) != 1 {
		t.Fatalf("expected 1 resolution, got %d", len(result.Resolutions))
	}
	if result.Resolutions[0].Status != StatusResolved {
		t.Errorf("expected resolved, got %s", result.Resolutions[0].Status)
	}
	if result.Resolutions[0].Selected == nil {
		t.Fatalf("expected selected candidate")
	}
	if result.Resolutions[0].Selected.Version.Compare(ver("1.5.0")) != 0 {
		t.Errorf("expected highest compatible 1.5.0, got %s", result.Resolutions[0].Selected.Version)
	}
	if len(result.Conflicts) != 0 {
		t.Errorf("expected no conflicts, got %d", len(result.Conflicts))
	}
}

func TestResolveMissingRequired(t *testing.T) {
	provider := CandidateProviderFunc(func(_ context.Context, target string, _ TargetType) ([]Candidate, error) {
		return nil, nil
	})
	r := NewDefaultResolver(provider)
	result := r.Resolve(context.Background(), ResolveRequest{
		SourceID: "com.example/dep-test",
		Phase:    PhaseInstall,
		Requests: []Request{
			{Target: "com.example/missing", Type: TargetExtension, Required: true},
		},
	})
	if result.Resolutions[0].Status != StatusMissing {
		t.Errorf("expected missing, got %s", result.Resolutions[0].Status)
	}
	if len(result.Conflicts) == 0 {
		t.Errorf("expected conflict for missing required")
	}
}

func TestResolveOptionalMissing(t *testing.T) {
	provider := CandidateProviderFunc(func(_ context.Context, _ string, _ TargetType) ([]Candidate, error) {
		return nil, nil
	})
	r := NewDefaultResolver(provider)
	result := r.Resolve(context.Background(), ResolveRequest{
		SourceID: "com.example/dep-test",
		Phase:    PhaseEnable,
		Requests: []Request{
			{Target: "com.example/optional", Type: TargetExtension, Required: false},
		},
	})
	if result.Resolutions[0].Status != StatusDowngraded {
		t.Errorf("expected downgraded, got %s", result.Resolutions[0].Status)
	}
	if len(result.Warnings) == 0 {
		t.Errorf("expected warning for missing optional")
	}
}

func TestResolveVersionConflict(t *testing.T) {
	provider := CandidateProviderFunc(func(_ context.Context, target string, _ TargetType) ([]Candidate, error) {
		return []Candidate{
			{TargetID: target, Version: ver("1.0.0")},
			{TargetID: target, Version: ver("2.0.0")},
		}, nil
	})
	r := NewDefaultResolver(provider)
	result := r.Resolve(context.Background(), ResolveRequest{
		SourceID: "com.example/dep-test",
		Phase:    PhaseInstall,
		Requests: []Request{
			{Target: "com.example/v", Type: TargetExtension, VersionRange: "^3.0.0", Required: true},
		},
	})
	if result.Resolutions[0].Status != StatusMissing {
		t.Errorf("expected missing, got %s", result.Resolutions[0].Status)
	}
	found := false
	for _, c := range result.Conflicts {
		if c.Kind == ConflictVersion {
			found = true
		}
	}
	if !found {
		t.Errorf("expected version_conflict")
	}
}

func TestDetectCycle(t *testing.T) {
	provider := CandidateProviderFunc(func(_ context.Context, _ string, _ TargetType) ([]Candidate, error) {
		return nil, nil
	})
	r := NewDefaultResolver(provider)
	g := Graph{
		Nodes: map[string]Node{
			"a": {ID: "a"},
			"b": {ID: "b"},
			"c": {ID: "c"},
		},
		Edges: []Edge{
			{From: "a", To: "b", Required: true},
			{From: "b", To: "c", Required: true},
			{From: "c", To: "a", Required: true},
		},
	}
	cycle := r.DetectCycle(g)
	if cycle == nil {
		t.Fatalf("expected cycle")
	}
}

func TestTopologicalOrder(t *testing.T) {
	provider := CandidateProviderFunc(func(_ context.Context, _ string, _ TargetType) ([]Candidate, error) {
		return nil, nil
	})
	r := NewDefaultResolver(provider)
	g := Graph{
		Nodes: map[string]Node{
			"a": {ID: "a"},
			"b": {ID: "b"},
			"c": {ID: "c"},
		},
		Edges: []Edge{
			{From: "a", To: "b", Required: true},
			{From: "b", To: "c", Required: true},
		},
	}
	order, err := r.TopologicalOrder(g)
	if err != nil {
		t.Fatalf("TopologicalOrder: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(order))
	}
	pos := map[string]int{}
	for i, id := range order {
		pos[id] = i
	}
	if pos["a"] > pos["b"] {
		t.Errorf("a should come before b")
	}
	if pos["b"] > pos["c"] {
		t.Errorf("b should come before c")
	}
}

func TestAffectedBy(t *testing.T) {
	provider := CandidateProviderFunc(func(_ context.Context, target string, _ TargetType) ([]Candidate, error) {
		return []Candidate{{TargetID: target, Version: ver("1.0.0"), Origin: "installed"}}, nil
	})
	r := NewDefaultResolver(provider)
	r.Resolve(context.Background(), ResolveRequest{
		SourceID: "com.example/dependent",
		Phase:    PhaseInstall,
		Requests: []Request{
			{Target: "com.example/dep", Type: TargetExtension, Required: true},
		},
	})
	affected, err := r.AffectedBy(context.Background(), "com.example/dep")
	if err != nil {
		t.Fatalf("AffectedBy: %v", err)
	}
	if len(affected) != 1 {
		t.Fatalf("expected 1 affected, got %d", len(affected))
	}
	if affected[0].SubjectID != "com.example/dependent" {
		t.Errorf("expected com.example/dependent, got %s", affected[0].SubjectID)
	}
}

func TestSnapshot(t *testing.T) {
	provider := CandidateProviderFunc(func(_ context.Context, target string, _ TargetType) ([]Candidate, error) {
		return []Candidate{{TargetID: target, Version: ver("1.0.0"), Origin: "installed"}}, nil
	})
	r := NewDefaultResolver(provider)
	r.Resolve(context.Background(), ResolveRequest{
		SourceID: "com.example/snap-test",
		Phase:    PhaseStart,
		Requests: []Request{
			{Target: "com.example/dep", Type: TargetExtension, Required: true, VersionRange: "^1.0.0"},
		},
	})
	snap, err := r.Snapshot(context.Background(), "com.example/snap-test")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.SourceID != "com.example/snap-test" {
		t.Errorf("unexpected source id %s", snap.SourceID)
	}
	if len(snap.Resolutions) != 1 {
		t.Errorf("expected 1 resolution, got %d", len(snap.Resolutions))
	}
	if snap.GraphHash == "" {
		t.Errorf("expected graph hash")
	}
}

func TestHostFeatureMissing(t *testing.T) {
	provider := CandidateProviderFunc(func(_ context.Context, target string, _ TargetType) ([]Candidate, error) {
		return []Candidate{{TargetID: target, Version: ver("1.0.0"), Origin: "system"}}, nil
	})
	r := NewDefaultResolver(provider)
	r.SetHostFeatures(map[string]bool{"feature-a": false})
	result := r.Resolve(context.Background(), ResolveRequest{
		SourceID: "com.example/host-test",
		Phase:    PhaseStart,
		Requests: []Request{
			{Target: "feature-a", Type: TargetHostFeature, Required: true},
		},
	})
	if result.Resolutions[0].Status != StatusConflict {
		t.Errorf("expected conflict, got %s", result.Resolutions[0].Status)
	}
	found := false
	for _, c := range result.Conflicts {
		if c.Kind == ConflictHostFeatureMissing {
			found = true
		}
	}
	if !found {
		t.Errorf("expected host_feature_missing conflict")
	}
}

func TestPlatformConflict(t *testing.T) {
	provider := CandidateProviderFunc(func(_ context.Context, target string, _ TargetType) ([]Candidate, error) {
		return []Candidate{{TargetID: target, Version: ver("1.0.0"), Platform: "linux"}}, nil
	})
	r := NewDefaultResolver(provider)
	r.SetPlatform("windows")
	result := r.Resolve(context.Background(), ResolveRequest{
		SourceID: "com.example/plat-test",
		Phase:    PhaseInstall,
		Requests: []Request{
			{Target: "com.example/dep", Type: TargetExtension, Required: true},
		},
	})
	if result.Resolutions[0].Status != StatusConflict {
		t.Errorf("expected conflict, got %s", result.Resolutions[0].Status)
	}
	found := false
	for _, c := range result.Conflicts {
		if c.Kind == ConflictPlatform {
			found = true
		}
	}
	if !found {
		t.Errorf("expected platform_conflict")
	}
}

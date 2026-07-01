package interaction

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type mockLoader struct {
	name       string
	required   bool
	timeout    time.Duration
	loadErr    error
	loadDelay  time.Duration
	loadResult SnapshotField[any]
	cacheKeyFn func(InteractionScope, string) string
	callCount  *atomic.Int32
}

func (m *mockLoader) Name() string { return m.name }

func (m *mockLoader) IsRequired() bool { return m.required }

func (m *mockLoader) Timeout() time.Duration { return m.timeout }

func (m *mockLoader) CacheKey(scope InteractionScope, version string) string {
	if m.cacheKeyFn != nil {
		return m.cacheKeyFn(scope, version)
	}
	return m.name + ":" + version
}

func (m *mockLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	if m.callCount != nil {
		m.callCount.Add(1)
	}
	if m.loadDelay > 0 {
		select {
		case <-time.After(m.loadDelay):
		case <-ctx.Done():
			return SnapshotField[any]{}, ctx.Err()
		}
	}
	if m.loadErr != nil {
		return SnapshotField[any]{}, m.loadErr
	}
	return m.loadResult, nil
}

func TestRegistryRegisterAndLoadAll(t *testing.T) {
	reg := NewContextLoaderRegistry()

	reg.Register(&mockLoader{
		name:     "runtimeProfile",
		required: true,
		timeout:  2 * time.Second,
		loadResult: SnapshotField[any]{
			Value:   RuntimeProfile{PersonalitySource: "test"},
			Source:  "char",
			Status:  LoadStatusReady,
			Version: "v1",
		},
		loadDelay: time.Millisecond,
	})
	reg.Register(&mockLoader{
		name:     "relationship",
		required: false,
		timeout:  2 * time.Second,
		loadResult: SnapshotField[any]{
			Value:   RelationshipState{Trust: 0.9},
			Source:  "rel",
			Status:  LoadStatusReady,
			Version: "v1",
		},
		loadDelay: time.Millisecond,
	})

	ctx := context.Background()
	scope := InteractionScope{UserID: "u1", CharacterID: "c1"}
	snapshot := reg.LoadAll(ctx, scope, "v1")

	if snapshot.Version != "v1" {
		t.Fatalf("expected version v1, got %s", snapshot.Version)
	}
	if snapshot.RuntimeProfile.Value.PersonalitySource != "test" {
		t.Fatalf("expected PersonalitySource test, got %s", snapshot.RuntimeProfile.Value.PersonalitySource)
	}
	if snapshot.Relationship.Value.Trust != 0.9 {
		t.Fatalf("expected Trust 0.9, got %f", snapshot.Relationship.Value.Trust)
	}

	stats := reg.Stats()
	if len(stats) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(stats))
	}
	for _, st := range stats {
		if st.HealthStatus != "ok" {
			t.Fatalf("expected ok for %s, got %s", st.Name, st.HealthStatus)
		}
		if st.Duration == 0 {
			t.Fatalf("expected non-zero duration for %s", st.Name)
		}
	}
}

func TestRequiredLoaderError(t *testing.T) {
	reg := NewContextLoaderRegistry()

	reg.Register(&mockLoader{
		name:     "runtimeProfile",
		required: true,
		timeout:  2 * time.Second,
		loadErr:  errors.New("db down"),
	})

	ctx := context.Background()
	scope := InteractionScope{UserID: "u1", CharacterID: "c1"}
	snapshot := reg.LoadAll(ctx, scope, "v1")

	if snapshot.RuntimeProfile.Status != LoadStatusError {
		t.Fatalf("expected error status, got %s", snapshot.RuntimeProfile.Status)
	}
	if snapshot.RuntimeProfile.Source != "runtimeProfile" {
		t.Fatalf("expected source runtimeProfile, got %s", snapshot.RuntimeProfile.Source)
	}
}

func TestOptionalLoaderError(t *testing.T) {
	reg := NewContextLoaderRegistry()

	reg.Register(&mockLoader{
		name:     "relationship",
		required: false,
		timeout:  2 * time.Second,
		loadErr:  errors.New("timeout"),
	})

	ctx := context.Background()
	scope := InteractionScope{UserID: "u1", CharacterID: "c1"}
	snapshot := reg.LoadAll(ctx, scope, "v1")

	if snapshot.Relationship.Status != LoadStatusUnavailable {
		t.Fatalf("expected unavailable status for optional loader, got %s", snapshot.Relationship.Status)
	}
}

func TestContextCancellation(t *testing.T) {
	reg := NewContextLoaderRegistry()

	var callCount atomic.Int32
	reg.Register(&mockLoader{
		name:      "runtimeProfile",
		required:  true,
		timeout:   5 * time.Second,
		loadDelay: 500 * time.Millisecond,
		loadResult: SnapshotField[any]{
			Value:   RuntimeProfile{PersonalitySource: "test"},
			Source:  "char",
			Status:  LoadStatusReady,
			Version: "v1",
		},
		callCount: &callCount,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	scope := InteractionScope{UserID: "u1", CharacterID: "c1"}
	snapshot := reg.LoadAll(ctx, scope, "v1")

	if snapshot.RuntimeProfile.Status != "" {
		t.Fatalf("expected empty status for cancelled context, got %s", snapshot.RuntimeProfile.Status)
	}

	stats := reg.Stats()
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(stats))
	}
	if stats[0].HealthStatus != "cancelled" {
		t.Fatalf("expected cancelled, got %s", stats[0].HealthStatus)
	}
}

func TestLoaderTimeout(t *testing.T) {
	reg := NewContextLoaderRegistry()

	reg.Register(&mockLoader{
		name:      "life",
		required:  false,
		timeout:   50 * time.Millisecond,
		loadDelay: 200 * time.Millisecond,
	})

	ctx := context.Background()
	scope := InteractionScope{UserID: "u1", CharacterID: "c1"}
	snapshot := reg.LoadAll(ctx, scope, "v1")

	if snapshot.Life.Status != LoadStatusUnavailable {
		t.Fatalf("expected unavailable for timeout, got %s", snapshot.Life.Status)
	}
}

func TestCacheKey(t *testing.T) {
	reg := NewContextLoaderRegistry()

	loader := &mockLoader{
		name:     "conversation",
		required: false,
		timeout:  2 * time.Second,
		cacheKeyFn: func(scope InteractionScope, version string) string {
			return scope.ConversationID + ":" + version
		},
		loadDelay: time.Millisecond,
	}
	reg.Register(loader)
	key := loader.CacheKey(InteractionScope{ConversationID: "conv1"}, "v2")
	if key != "conv1:v2" {
		t.Fatalf("expected conv1:v2, got %s", key)
	}
}

func TestStatsAllHealthModes(t *testing.T) {
	reg := NewContextLoaderRegistry()

	reg.Register(&mockLoader{
		name:     "runtimeProfile",
		required: true,
		timeout:  2 * time.Second,
		loadResult: SnapshotField[any]{
			Value:   RuntimeProfile{PersonalitySource: "ok"},
			Source:  "char",
			Status:  LoadStatusReady,
			Version: "v1",
		},
	})
	reg.Register(&mockLoader{
		name:     "memories",
		required: false,
		timeout:  2 * time.Second,
		loadErr:  errors.New("service unreachable"),
	})

	ctx := context.Background()
	scope := InteractionScope{UserID: "u1", CharacterID: "c1"}
	reg.LoadAll(ctx, scope, "v1")

	stats := reg.Stats()
	if len(stats) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(stats))
	}

	health := map[string]string{}
	for _, st := range stats {
		health[st.Name] = st.HealthStatus
	}
	if health["runtimeProfile"] != "ok" {
		t.Fatalf("expected ok for runtimeProfile, got %s", health["runtimeProfile"])
	}
	if health["memories"] != "error" {
		t.Fatalf("expected error for memories, got %s", health["memories"])
	}
}

func TestParallelLoading(t *testing.T) {
	reg := NewContextLoaderRegistry()

	for i := 0; i < 5; i++ {
		reg.Register(&mockLoader{
			name:      "parallel",
			required:  false,
			timeout:   2 * time.Second,
			loadDelay: 50 * time.Millisecond,
			loadResult: SnapshotField[any]{
				Value:   "ok",
				Source:  "test",
				Status:  LoadStatusReady,
				Version: "v1",
			},
		})
	}

	ctx := context.Background()
	scope := InteractionScope{UserID: "u1", CharacterID: "c1"}
	start := time.Now()
	reg.LoadAll(ctx, scope, "v1")
	elapsed := time.Since(start)

	if elapsed >= 200*time.Millisecond {
		t.Fatalf("parallel loading too slow: %v (expect < 200ms with 5x50ms loaders)", elapsed)
	}
}

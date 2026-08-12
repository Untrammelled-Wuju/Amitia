package uitree

import (
	"context"
	"testing"
	"time"
)

func TestSnapshotCache_PutAndGet(t *testing.T) {
	cache := NewSnapshotCache(DefaultPolicy())

	snapshot := UITreeSnapshot{
		SnapshotID: "uis_test_1",
		Generation: 1,
		Nodes: []UINode{
			{NodeID: "node_1", Text: "Hello"},
			{NodeID: "node_2", Text: "World"},
		},
	}

	cache.Put(snapshot)

	got, ok := cache.Get("uis_test_1")
	if !ok {
		t.Fatal("expected to get snapshot")
	}
	if got.SnapshotID != "uis_test_1" {
		t.Fatalf("expected uis_test_1, got %s", got.SnapshotID)
	}
}

func TestSnapshotCache_GetNode(t *testing.T) {
	cache := NewSnapshotCache(DefaultPolicy())

	snapshot := UITreeSnapshot{
		SnapshotID: "uis_test_1",
		Generation: 1,
		Nodes: []UINode{
			{NodeID: "node_1", Text: "Hello"},
			{NodeID: "node_2", Text: "World"},
		},
	}

	cache.Put(snapshot)

	node, ok := cache.GetNode("uis_test_1", "node_1")
	if !ok {
		t.Fatal("expected to get node")
	}
	if node.Text != "Hello" {
		t.Fatalf("expected Hello, got %s", node.Text)
	}

	_, ok = cache.GetNode("uis_test_1", "node_nonexistent")
	if ok {
		t.Fatal("expected not found for nonexistent node")
	}
}

func TestSnapshotCache_Nonexistent(t *testing.T) {
	cache := NewSnapshotCache(DefaultPolicy())

	_, ok := cache.Get("uis_nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestSnapshotCache_TTLExpiration(t *testing.T) {
	policy := DefaultPolicy()
	policy.SnapshotTTL = 1 * time.Millisecond
	cache := NewSnapshotCache(policy)

	snapshot := UITreeSnapshot{
		SnapshotID: "uis_test_1",
		Generation: 1,
	}
	cache.Put(snapshot)

	time.Sleep(5 * time.Millisecond)

	_, ok := cache.Get("uis_test_1")
	if ok {
		t.Fatal("expected snapshot to be expired")
	}
}

func TestSnapshotCache_MaxSnapshots(t *testing.T) {
	policy := DefaultPolicy()
	policy.MaxSnapshots = 2
	cache := NewSnapshotCache(policy)

	cache.Put(UITreeSnapshot{SnapshotID: "uis_1", Generation: 1})
	cache.Put(UITreeSnapshot{SnapshotID: "uis_2", Generation: 2})
	cache.Put(UITreeSnapshot{SnapshotID: "uis_3", Generation: 3})

	_, ok := cache.Get("uis_1")
	if ok {
		t.Fatal("expected oldest snapshot to be evicted")
	}

	_, ok = cache.Get("uis_3")
	if !ok {
		t.Fatal("expected latest snapshot to exist")
	}
}

func TestSnapshotCache_Latest(t *testing.T) {
	cache := NewSnapshotCache(DefaultPolicy())

	_, ok := cache.Latest()
	if ok {
		t.Fatal("expected no latest snapshot when empty")
	}

	cache.Put(UITreeSnapshot{SnapshotID: "uis_1", Generation: 1})
	time.Sleep(1 * time.Millisecond)
	cache.Put(UITreeSnapshot{SnapshotID: "uis_2", Generation: 2})

	latest, ok := cache.Latest()
	if !ok {
		t.Fatal("expected latest snapshot")
	}
	if latest.SnapshotID != "uis_2" {
		t.Fatalf("expected uis_2, got %s", latest.SnapshotID)
	}
}

func TestSnapshotCache_Invalidate(t *testing.T) {
	cache := NewSnapshotCache(DefaultPolicy())

	cache.Put(UITreeSnapshot{SnapshotID: "uis_1", Generation: 1})
	cache.Invalidate("uis_1")

	_, ok := cache.Get("uis_1")
	if ok {
		t.Fatal("expected snapshot to be invalidated")
	}
}

func TestSnapshotCache_InvalidateAll(t *testing.T) {
	cache := NewSnapshotCache(DefaultPolicy())

	cache.Put(UITreeSnapshot{SnapshotID: "uis_1", Generation: 1})
	cache.Put(UITreeSnapshot{SnapshotID: "uis_2", Generation: 2})
	cache.InvalidateAll()

	_, ok := cache.Get("uis_1")
	if ok {
		t.Fatal("expected all snapshots to be invalidated")
	}
	_, ok = cache.Get("uis_2")
	if ok {
		t.Fatal("expected all snapshots to be invalidated")
	}
}

func TestSnapshotCache_SnapshotResolver(t *testing.T) {
	cache := NewSnapshotCache(DefaultPolicy())
	resolver := cache.SnapshotResolver()

	_, err := resolver.Latest(context.Background())
	if err == nil {
		t.Fatal("expected error for empty cache")
	}

	cache.Put(UITreeSnapshot{SnapshotID: "uis_1", Generation: 1})
	snapshot, err := resolver.Latest(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snapshot.SnapshotID != "uis_1" {
		t.Fatalf("expected uis_1, got %s", snapshot.SnapshotID)
	}

	_, err = resolver.GetSnapshot(context.Background(), "uis_nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent snapshot")
	}
}

func TestSnapshotCache_NodeResolver(t *testing.T) {
	cache := NewSnapshotCache(DefaultPolicy())
	resolver := cache.NodeResolver()

	_, err := resolver.ResolveNode(context.Background(), "uis_1", "node_1")
	if err == nil {
		t.Fatal("expected error for empty cache")
	}

	cache.Put(UITreeSnapshot{
		SnapshotID: "uis_1",
		Generation: 1,
		Nodes: []UINode{
			{NodeID: "node_1", Text: "Hello"},
		},
	})

	resolved, err := resolver.ResolveNode(context.Background(), "uis_1", "node_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Node.NodeID != "node_1" {
		t.Fatalf("expected node_1, got %s", resolved.Node.NodeID)
	}
	if resolved.SnapshotID != "uis_1" {
		t.Fatalf("expected uis_1, got %s", resolved.SnapshotID)
	}
}

func TestService_Status(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{},
		Root:          &mockUIRootSource{},
		ADB:           &mockUIADBSource{},
	}
	service := NewService(sources, DefaultPolicy())

	status := service.Status(context.Background())
	if !status.Available {
		t.Fatal("expected available")
	}
	if status.PreferredSource != string(SourceTypeAccessibility) {
		t.Fatalf("expected accessibility, got %s", status.PreferredSource)
	}
	if !status.AccessibilityReady {
		t.Fatal("expected accessibility ready")
	}
	if status.RootAvailable {
		t.Fatal("expected root not available")
	}
	if status.ADBAvailable {
		t.Fatal("expected adb not available")
	}
}

func TestService_Snapshot(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{},
	}
	service := NewService(sources, DefaultPolicy())

	snapshot, err := service.Snapshot(context.Background(), SnapshotRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snapshot.SnapshotID == "" {
		t.Fatal("expected snapshot ID")
	}
	if snapshot.Source != string(SourceTypeAccessibility) {
		t.Fatalf("expected accessibility source, got %s", snapshot.Source)
	}
	if snapshot.NodeCount == 0 {
		t.Fatal("expected nodes in snapshot")
	}
}

func TestService_Snapshot_ExplicitSource(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{},
		ADB:           &mockUIADBSource{},
	}
	service := NewService(sources, DefaultPolicy())

	_, err := service.Snapshot(context.Background(), SnapshotRequest{Source: SourceADB})
	if err == nil {
		t.Fatal("expected error for unavailable ADB source")
	}
}

func TestService_Snapshot_AutoFallback(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{
			statusFunc: func(ctx context.Context) SourceStatus {
				return SourceStatus{Type: SourceTypeAccessibility, Available: false}
			},
		},
		ADB: &mockUIADBSource{
			statusFunc: func(ctx context.Context) SourceStatus {
				return SourceStatus{Type: SourceTypeADB, Available: true}
			},
			snapshotFunc: func(ctx context.Context, request SnapshotRequest) (RawSnapshot, error) {
				return RawSnapshot{
					Source:     SourceTypeADB,
					Generation: 1,
					CapturedAt: 1000,
					RawNodes: []map[string]any{
						{"nodeId": "adb_node_1", "text": "ADB Node"},
					},
				}, nil
			},
		},
	}
	service := NewService(sources, DefaultPolicy())

	snapshot, err := service.Snapshot(context.Background(), SnapshotRequest{Source: SourceAuto})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snapshot.Source != string(SourceTypeADB) {
		t.Fatalf("expected adb source, got %s", snapshot.Source)
	}
}

func TestService_Find(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{},
	}
	service := NewService(sources, DefaultPolicy())

	_, err := service.Snapshot(context.Background(), SnapshotRequest{})
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}

	result, err := service.Find(context.Background(), FindRequest{
		Text:      "Hello",
		MatchMode: MatchModeContains,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count == 0 {
		t.Fatal("expected at least one match")
	}
}

func TestService_Find_NoSnapshot(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{},
	}
	service := NewService(sources, DefaultPolicy())

	_, err := service.Find(context.Background(), FindRequest{Text: "Hello"})
	if err == nil {
		t.Fatal("expected error for no snapshot")
	}
}

func TestService_Get(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{},
	}
	service := NewService(sources, DefaultPolicy())

	snapshot, err := service.Snapshot(context.Background(), SnapshotRequest{})
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}

	if len(snapshot.Nodes) == 0 {
		t.Fatal("expected nodes")
	}

	result, err := service.Get(context.Background(), GetRequest{
		SnapshotID: snapshot.SnapshotID,
		NodeID:     snapshot.Nodes[0].NodeID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Node.NodeID != snapshot.Nodes[0].NodeID {
		t.Fatalf("expected %s, got %s", snapshot.Nodes[0].NodeID, result.Node.NodeID)
	}
}

func TestService_Get_StaleSnapshot(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{},
	}
	service := NewService(sources, DefaultPolicy())

	_, err := service.Get(context.Background(), GetRequest{
		SnapshotID: "uis_nonexistent",
		NodeID:     "node_1",
	})
	if err == nil {
		t.Fatal("expected error for stale snapshot")
	}
}

func TestSourceSet_SelectSource_Auto(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{},
		ADB:           &mockUIADBSource{},
	}

	source, sourceType, err := sources.SelectSource(SnapshotRequest{Source: SourceAuto}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sourceType != SourceTypeAccessibility {
		t.Fatalf("expected accessibility, got %s", sourceType)
	}
	if source == nil {
		t.Fatal("expected source")
	}
}

func TestSourceSet_SelectSource_ExplicitAccessibility(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{},
	}

	source, sourceType, err := sources.SelectSource(SnapshotRequest{Source: SourceAccessibility}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sourceType != SourceTypeAccessibility {
		t.Fatalf("expected accessibility, got %s", sourceType)
	}
	if source == nil {
		t.Fatal("expected source")
	}
}

func TestSourceSet_SelectSource_ExplicitRoot(t *testing.T) {
	sources := SourceSet{
		Root: &mockUIRootSource{},
	}

	_, _, err := sources.SelectSource(SnapshotRequest{Source: SourceRoot}, false)
	if err == nil {
		t.Fatal("expected error for root without permission")
	}

	_, _, err = sources.SelectSource(SnapshotRequest{Source: SourceRoot}, true)
	if err == nil {
		t.Fatal("expected error for unavailable root")
	}
}

func TestSourceSet_SelectSource_Invalid(t *testing.T) {
	sources := SourceSet{}

	_, _, err := sources.SelectSource(SnapshotRequest{Source: "invalid"}, false)
	if err == nil {
		t.Fatal("expected error for invalid source")
	}
}

func TestSourceSet_AvailableSources(t *testing.T) {
	sources := SourceSet{
		Accessibility: &mockUIAccessibilitySource{},
		Root:          &mockUIRootSource{},
		ADB:           &mockUIADBSource{},
	}

	available := sources.AvailableSources()
	if len(available) != 1 {
		t.Fatalf("expected 1 available source, got %d", len(available))
	}
	if available[0] != string(SourceTypeAccessibility) {
		t.Fatalf("expected accessibility, got %s", available[0])
	}
}

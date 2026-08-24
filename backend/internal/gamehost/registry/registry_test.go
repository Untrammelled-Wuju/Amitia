package registry

import (
	"context"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func newTestPluginDescriptor(id domain.PluginID, extensionID string) domain.PluginDescriptor {
	return domain.PluginDescriptor{
		ID:              id,
		ExtensionID:     extensionID,
		Name:            "Test Plugin " + string(id),
		Version:         "1.0.0",
		ProtocolVersion: "amitia-game-host/1",
		Capabilities: []domain.Capability{
			domain.CapabilityCustomRPC,
		},
		Services: []domain.ServiceDescriptor{
			{
				ID:   domain.ServiceID("svc-1"),
				Name: "Service 1",
				Kind: domain.ServiceKindProcess,
			},
		},
		Channels: []domain.ChannelDescriptor{
			{
				ID:        domain.ChannelID("events"),
				ServiceID: domain.ServiceID("svc-1"),
				Kind:      domain.ChannelKindEvent,
			},
		},
		Metadata: map[string]string{
			"vendor": "test",
		},
	}
}

func TestRegisterPlugin(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	desc := newTestPluginDescriptor("plugin-a", "com.example.game")

	err := r.Register(ctx, desc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !r.Exists(ctx, "plugin-a") {
		t.Error("expected plugin-a to exist after registration")
	}
}

func TestRegisterRejectsInvalidDescriptor(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	desc := domain.PluginDescriptor{
		ID:              "",
		ExtensionID:     "com.example",
		Name:            "Bad Plugin",
		Version:         "1.0.0",
		ProtocolVersion: "amitia-game-host/1",
	}

	err := r.Register(ctx, desc)
	if err == nil {
		t.Fatal("expected error for invalid descriptor")
	}
	if !domain.IsHostError(err, domain.ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestRegisterRejectsDuplicatePluginID(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	desc := newTestPluginDescriptor("plugin-a", "com.example.game")
	r.Register(ctx, desc)

	err := r.Register(ctx, desc)
	if err == nil {
		t.Fatal("expected error for duplicate plugin id")
	}
	if !domain.IsHostError(err, domain.ErrAlreadyExists) {
		t.Errorf("expected already_exists, got %v", err)
	}
}

func TestMultiplePluginsPerExtension(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	descA := newTestPluginDescriptor("plugin-a", "com.example.game-pack")
	descB := newTestPluginDescriptor("plugin-b", "com.example.game-pack")

	err := r.Register(ctx, descA)
	if err != nil {
		t.Fatalf("unexpected error registering plugin-a: %v", err)
	}
	err = r.Register(ctx, descB)
	if err != nil {
		t.Fatalf("unexpected error registering plugin-b: %v", err)
	}

	list, err := r.ListByExtension(ctx, "com.example.game-pack")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(list))
	}

	if list[0].ID != "plugin-a" || list[1].ID != "plugin-b" {
		t.Errorf("expected sorted order [plugin-a, plugin-b], got [%s, %s]", list[0].ID, list[1].ID)
	}
}

func TestListByExtension(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	r.Register(ctx, newTestPluginDescriptor("plugin-a", "com.example.game"))
	r.Register(ctx, newTestPluginDescriptor("plugin-b", "com.example.game"))

	list, err := r.ListByExtension(ctx, "com.example.game")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(list))
	}

	if list[0].ID != "plugin-a" || list[1].ID != "plugin-b" {
		t.Errorf("expected sorted order, got [%s, %s]", list[0].ID, list[1].ID)
	}
}

func TestListByExtensionEmpty(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	list, err := r.ListByExtension(ctx, "nonexistent.extension")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}
}

func TestListByExtensionRejectsEmptyExtensionID(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	_, err := r.ListByExtension(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty extension id")
	}
	if !domain.IsHostError(err, domain.ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestGetUnknownPlugin(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	_, err := r.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown plugin")
	}
	if !domain.IsHostError(err, domain.ErrNotFound) {
		t.Errorf("expected not_found, got %v", err)
	}
}

func TestUnregisterPlugin(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	r.Register(ctx, newTestPluginDescriptor("plugin-a", "com.example.game"))

	err := r.Unregister(ctx, "plugin-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.Exists(ctx, "plugin-a") {
		t.Error("expected plugin-a to not exist after unregister")
	}

	_, err = r.Get(ctx, "plugin-a")
	if err == nil {
		t.Error("expected not_found error after unregister")
	}
}

func TestUnregisterUnknownPlugin(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	err := r.Unregister(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown plugin")
	}
	if !domain.IsHostError(err, domain.ErrNotFound) {
		t.Errorf("expected not_found, got %v", err)
	}
}

func TestUnregisterCleansExtensionIndex(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	r.Register(ctx, newTestPluginDescriptor("plugin-a", "com.example.game"))
	r.Unregister(ctx, "plugin-a")

	list, _ := r.ListByExtension(ctx, "com.example.game")
	if len(list) != 0 {
		t.Errorf("expected 0 plugins in extension index after unregister, got %d", len(list))
	}
}

func TestRegisterClonesDescriptor(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	desc := newTestPluginDescriptor("plugin-a", "com.example.game")
	r.Register(ctx, desc)

	desc.Metadata["vendor"] = "modified"
	desc.Capabilities = append(desc.Capabilities, domain.CapabilityHostAPI)

	stored, err := r.Get(ctx, "plugin-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stored.Metadata["vendor"] != "test" {
		t.Error("modifying original descriptor should not affect stored descriptor")
	}
	if len(stored.Capabilities) != 1 {
		t.Error("modifying original capabilities should not affect stored descriptor")
	}
}

func TestGetReturnsClone(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	r.Register(ctx, newTestPluginDescriptor("plugin-a", "com.example.game"))

	got1, _ := r.Get(ctx, "plugin-a")
	got1.Metadata["vendor"] = "modified"

	got2, _ := r.Get(ctx, "plugin-a")
	if got2.Metadata["vendor"] != "test" {
		t.Error("modifying returned descriptor should not affect registry")
	}
}

func TestListReturnsClone(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	r.Register(ctx, newTestPluginDescriptor("plugin-a", "com.example.game"))

	list1, _ := r.List(ctx)
	list1[0].Metadata["vendor"] = "modified"

	list2, _ := r.List(ctx)
	if list2[0].Metadata["vendor"] != "test" {
		t.Error("modifying returned list should not affect registry")
	}
}

func TestListStableOrder(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	r.Register(ctx, newTestPluginDescriptor("plugin-c", "com.example"))
	r.Register(ctx, newTestPluginDescriptor("plugin-a", "com.example"))
	r.Register(ctx, newTestPluginDescriptor("plugin-b", "com.example"))

	list, _ := r.List(ctx)
	if len(list) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(list))
	}

	expected := []domain.PluginID{"plugin-a", "plugin-b", "plugin-c"}
	for i, plugin := range list {
		if plugin.ID != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], plugin.ID)
		}
	}
}

func TestConcurrentRegisterSamePlugin(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	var mu sync.Mutex
	successCount := 0
	existsCount := 0

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			desc := newTestPluginDescriptor("concurrent-plugin", "com.example")
			err := r.Register(ctx, desc)

			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCount++
			} else if domain.IsHostError(err, domain.ErrAlreadyExists) {
				existsCount++
			}
		}()
	}

	wg.Wait()

	if successCount != 1 {
		t.Errorf("expected exactly 1 success, got %d", successCount)
	}
	if existsCount != goroutines-1 {
		t.Errorf("expected %d already_exists errors, got %d", goroutines-1, existsCount)
	}
}

func TestConcurrentRegistryAccess(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	r.Register(ctx, newTestPluginDescriptor("plugin-a", "com.example"))
	r.Register(ctx, newTestPluginDescriptor("plugin-b", "com.example"))

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 4)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			r.Register(ctx, newTestPluginDescriptor("new-plugin", "com.example"))
		}()

		go func() {
			defer wg.Done()
			r.Get(ctx, "plugin-a")
		}()

		go func() {
			defer wg.Done()
			r.List(ctx)
		}()

		go func() {
			defer wg.Done()
			r.Exists(ctx, "plugin-b")
		}()
	}

	wg.Wait()
}

func TestCancelledContext(t *testing.T) {
	r := NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	desc := newTestPluginDescriptor("plugin-a", "com.example")

	err := r.Register(ctx, desc)
	if err == nil {
		t.Error("expected error for cancelled context on Register")
	}

	_, err = r.Get(ctx, "plugin-a")
	if err == nil {
		t.Error("expected error for cancelled context on Get")
	}

	_, err = r.List(ctx)
	if err == nil {
		t.Error("expected error for cancelled context on List")
	}

	err = r.Unregister(ctx, "plugin-a")
	if err == nil {
		t.Error("expected error for cancelled context on Unregister")
	}
}

func TestExists(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	if r.Exists(ctx, "plugin-a") {
		t.Error("expected Exists to return false for non-existent plugin")
	}

	r.Register(ctx, newTestPluginDescriptor("plugin-a", "com.example"))

	if !r.Exists(ctx, "plugin-a") {
		t.Error("expected Exists to return true for registered plugin")
	}
}

func TestSnapshot(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	r.Register(ctx, newTestPluginDescriptor("plugin-b", "com.example"))
	r.Register(ctx, newTestPluginDescriptor("plugin-a", "com.example"))

	snapshot := r.Snapshot()
	if len(snapshot) != 2 {
		t.Errorf("expected 2 plugins in snapshot, got %d", len(snapshot))
	}

	if snapshot[0].ID != "plugin-a" || snapshot[1].ID != "plugin-b" {
		t.Errorf("expected sorted order in snapshot")
	}
}

func TestCount(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	if r.Count() != 0 {
		t.Errorf("expected count 0, got %d", r.Count())
	}

	r.Register(ctx, newTestPluginDescriptor("plugin-a", "com.example"))
	if r.Count() != 1 {
		t.Errorf("expected count 1, got %d", r.Count())
	}

	r.Register(ctx, newTestPluginDescriptor("plugin-b", "com.example"))
	if r.Count() != 2 {
		t.Errorf("expected count 2, got %d", r.Count())
	}

	r.Unregister(ctx, "plugin-a")
	if r.Count() != 1 {
		t.Errorf("expected count 1 after unregister, got %d", r.Count())
	}
}

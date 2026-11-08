package management

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/runtime"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type behaviorPluginRegistry struct {
	plugin domain.PluginDescriptor
}

func (r behaviorPluginRegistry) Get(context.Context, string) (domain.PluginDescriptor, error) {
	return r.plugin, nil
}

func (r behaviorPluginRegistry) ListByExtension(context.Context, string) ([]domain.PluginDescriptor, error) {
	return []domain.PluginDescriptor{r.plugin}, nil
}

func (r behaviorPluginRegistry) List(context.Context) ([]domain.PluginDescriptor, error) {
	return []domain.PluginDescriptor{r.plugin}, nil
}

type behaviorRuntimeManager struct {
	runtimes []*runtime.RuntimeInstanceRef
}

func (m behaviorRuntimeManager) ListRuntimes() []*runtime.RuntimeInstanceRef { return m.runtimes }
func (m behaviorRuntimeManager) GetRuntime(string) (*runtime.RuntimeInstanceRef, error) {
	return m.runtimes[0], nil
}

type behaviorRPC struct {
	called  bool
	method  string
	payload []byte
}

type mutableBehaviorPluginRegistry struct {
	plugin domain.PluginDescriptor
}

func (r *mutableBehaviorPluginRegistry) Get(context.Context, string) (domain.PluginDescriptor, error) {
	return r.plugin, nil
}

func (r *mutableBehaviorPluginRegistry) ListByExtension(context.Context, string) ([]domain.PluginDescriptor, error) {
	return []domain.PluginDescriptor{r.plugin}, nil
}

func (r *mutableBehaviorPluginRegistry) List(context.Context) ([]domain.PluginDescriptor, error) {
	return []domain.PluginDescriptor{r.plugin}, nil
}

func (r *behaviorRPC) SendCustomRPC(_ context.Context, _, _, method string, payload []byte, _ time.Duration) (*protocol.Envelope, error) {
	r.called = true
	r.method = method
	r.payload = append([]byte(nil), payload...)
	return &protocol.Envelope{Type: protocol.MessageTypeResponse, Payload: json.RawMessage(`{"ok":true}`)}, nil
}

func TestGameBehaviorCatalogIsDynamicAndInvokedByMethod(t *testing.T) {
	catalog := `{
		"version":1,
		"revision":"r7",
		"behaviors":[
			{"id":"pause","name":"暂停","serviceId":"game","method":"game.pause","inputSchema":{"type":"object"},"risk":"low"}
		]
	}`
	plugin := domain.PluginDescriptor{ID: "com.example/rpg/plugin", ExtensionID: "com.example/rpg", Metadata: map[string]string{"behaviors": catalog}}
	rpc := &behaviorRPC{}
	service := NewGameBehaviorService(
		behaviorPluginRegistry{plugin: plugin},
		behaviorRuntimeManager{runtimes: []*runtime.RuntimeInstanceRef{{ID: "rt-1", PluginID: plugin.ID}}},
		rpc,
	)
	list, err := service.List(t.Context(), string(plugin.ID))
	if err != nil || list.CatalogRevision != "r7" || len(list.Behaviors) != 1 || list.Behaviors[0].ID != "pause" {
		t.Fatalf("list behaviors: %+v %v", list, err)
	}
	result, err := service.Invoke(t.Context(), string(plugin.ID), "pause", "", "r7", json.RawMessage(`{"reason":"user"}`))
	if err != nil || !rpc.called || rpc.method != "game.pause" || result.RuntimeID != "rt-1" || string(result.Result) != `{"ok":true}` {
		t.Fatalf("invoke behavior: %+v %v rpc=%+v", result, err, rpc)
	}
	if _, err := service.Invoke(t.Context(), string(plugin.ID), "pause", "", "old", nil); !errors.Is(err, ErrBehaviorRevision) {
		t.Fatalf("stale catalog should fail: %v", err)
	}
	if _, err := service.Invoke(t.Context(), string(plugin.ID), "missing", "", "r7", nil); !errors.Is(err, ErrBehaviorNotFound) {
		t.Fatalf("unknown behavior should fail: %v", err)
	}
}

func TestGameBehaviorCatalogRevisionFencesRemovedAndUnavailableBehaviors(t *testing.T) {
	plugin := domain.PluginDescriptor{
		ID:          "com.example/rpg/plugin",
		ExtensionID: "com.example/rpg",
		Metadata: map[string]string{"behaviors": `{
			"version":1,
			"revision":"r7",
			"behaviors":[
				{"id":"pause","name":"暂停","serviceId":"game","method":"game.pause","inputSchema":{"type":"object"}},
				{"id":"save","name":"保存","serviceId":"game","method":"game.save","inputSchema":{"type":"object"},"available":false}
			]
		}`},
	}
	registry := &mutableBehaviorPluginRegistry{plugin: plugin}
	rpc := &behaviorRPC{}
	service := NewGameBehaviorService(
		registry,
		behaviorRuntimeManager{runtimes: []*runtime.RuntimeInstanceRef{{ID: "rt-1", PluginID: plugin.ID}}},
		rpc,
	)
	if _, err := service.Invoke(t.Context(), string(plugin.ID), "save", "", "r7", nil); !errors.Is(err, ErrBehaviorNotFound) {
		t.Fatalf("unavailable behavior was invoked: %v", err)
	}
	plugin.Metadata = map[string]string{"behaviors": `{
		"version":1,
		"revision":"r8",
		"behaviors":[
			{"id":"resume","name":"继续","serviceId":"game","method":"game.resume","inputSchema":{"type":"object"}}
		]
	}`}
	registry.plugin = plugin
	if _, err := service.Invoke(t.Context(), string(plugin.ID), "pause", "", "r7", nil); !errors.Is(err, ErrBehaviorRevision) {
		t.Fatalf("stale revision invoked removed behavior: %v", err)
	}
	list, err := service.List(t.Context(), string(plugin.ID))
	if err != nil || list.CatalogRevision != "r8" || len(list.Behaviors) != 1 || list.Behaviors[0].ID != "resume" {
		t.Fatalf("runtime catalog update was not observed: %+v %v", list, err)
	}
	if rpc.called {
		t.Fatal("fenced behavior reached the game runtime")
	}
}

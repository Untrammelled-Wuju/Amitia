package integration

import (
	"context"
	"testing"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	gamehostdomain "github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/registry"
)

// mockRegistry 内存实现的PluginRegistry用于测试
type mockRegistry struct {
	plugins map[gamehostdomain.PluginID]gamehostdomain.PluginDescriptor
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		plugins: make(map[gamehostdomain.PluginID]gamehostdomain.PluginDescriptor),
	}
}

func (m *mockRegistry) Register(ctx context.Context, descriptor gamehostdomain.PluginDescriptor) error {
	if _, exists := m.plugins[descriptor.ID]; exists {
		return gamehostdomain.NewHostError(gamehostdomain.ErrAlreadyExists, "plugin already registered")
	}
	m.plugins[descriptor.ID] = descriptor
	return nil
}

func (m *mockRegistry) Unregister(ctx context.Context, pluginID gamehostdomain.PluginID) error {
	if _, exists := m.plugins[pluginID]; !exists {
		return gamehostdomain.NewHostError(gamehostdomain.ErrNotFound, "plugin not found")
	}
	delete(m.plugins, pluginID)
	return nil
}

func (m *mockRegistry) Get(ctx context.Context, pluginID gamehostdomain.PluginID) (gamehostdomain.PluginDescriptor, error) {
	plugin, exists := m.plugins[pluginID]
	if !exists {
		return gamehostdomain.PluginDescriptor{}, gamehostdomain.NewHostError(gamehostdomain.ErrNotFound, "plugin not found")
	}
	return plugin, nil
}

func (m *mockRegistry) List(ctx context.Context) ([]gamehostdomain.PluginDescriptor, error) {
	result := make([]gamehostdomain.PluginDescriptor, 0, len(m.plugins))
	for _, p := range m.plugins {
		result = append(result, p)
	}
	return result, nil
}

func (m *mockRegistry) ListByExtension(ctx context.Context, extensionID string) ([]gamehostdomain.PluginDescriptor, error) {
	result := make([]gamehostdomain.PluginDescriptor, 0)
	for _, p := range m.plugins {
		if p.ExtensionID == extensionID {
			result = append(result, p)
		}
	}
	return result, nil
}

// mockSource 模拟KernelContributionSource
type mockSource struct {
	plugins []KernelGamePlugin
}

func (m *mockSource) ListEnabledGamePlugins(ctx context.Context) ([]KernelGamePlugin, error) {
	return m.plugins, nil
}

func TestFullSyncRegistersEnabledGamePlugins(t *testing.T) {
	reg := newMockRegistry()
	source := &mockSource{
		plugins: []KernelGamePlugin{
			{
				Extension: kerneldomain.ExtensionDefinition{
					ID:      "com.example.game1",
					Name:    kerneldomain.LocalizedText{Default: "Game 1"},
					Version: kerneldomain.SemanticVersion{Major: 1},
					Domain:  kerneldomain.ExtensionDomainGame,
				},
				Contribution: kerneldomain.ContributionDefinition{
					ID:   "main",
					Kind: kerneldomain.ContributionKindGamePlugin,
					Name: kerneldomain.LocalizedText{Default: "Game 1 Plugin"},
					Definition: map[string]any{
						"protocolVersion": "amitia-game-host/1",
						"runtimeModuleId": "runtime",
						"capabilities":    []interface{}{"realtime_control"},
					},
				},
			},
		},
	}

	mapper := NewDefaultGamePluginContributionMapper()
	svc, err := NewGamePluginSyncService(reg, mapper, source)
	if err != nil {
		t.Fatalf("failed to create sync service: %v", err)
	}

	result := svc.FullSync(context.Background())
	if result.Failed > 0 {
		t.Fatalf("full sync failed: %v", result.Errors)
	}
	if result.Registered != 1 {
		t.Errorf("expected 1 registered, got %d", result.Registered)
	}

	// 验证插件确实注册了
	_, err = reg.Get(context.Background(), "com.example.game1/main")
	if err != nil {
		t.Errorf("expected plugin to be registered: %v", err)
	}
}

func TestFullSyncIsIdempotent(t *testing.T) {
	reg := newMockRegistry()
	source := &mockSource{
		plugins: []KernelGamePlugin{
			{
				Extension: kerneldomain.ExtensionDefinition{
					ID:      "com.example.idempotent",
					Name:    kerneldomain.LocalizedText{Default: "Idempotent"},
					Version: kerneldomain.SemanticVersion{Major: 1},
					Domain:  kerneldomain.ExtensionDomainGame,
				},
				Contribution: kerneldomain.ContributionDefinition{
					ID:         "main",
					Kind:       kerneldomain.ContributionKindGamePlugin,
					Name:       kerneldomain.LocalizedText{Default: "Idempotent Plugin"},
					Definition: map[string]any{"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "runtime"},
				},
			},
		},
	}

	mapper := NewDefaultGamePluginContributionMapper()
	svc, err := NewGamePluginSyncService(reg, mapper, source)
	if err != nil {
		t.Fatalf("failed to create sync service: %v", err)
	}

	// 第一次同步
	result1 := svc.FullSync(context.Background())
	if result1.Registered != 1 {
		t.Errorf("first sync: expected 1 registered, got %d", result1.Registered)
	}

	// 第二次同步
	result2 := svc.FullSync(context.Background())
	if result2.Unchanged != 1 {
		t.Errorf("second sync: expected 1 unchanged, got %d", result2.Unchanged)
	}
	if result2.Registered != 0 {
		t.Errorf("second sync: expected 0 registered, got %d", result2.Registered)
	}
}

func TestFullSyncSkipsDesktopPetPlugin(t *testing.T) {
	reg := newMockRegistry()
	source := &mockSource{
		plugins: []KernelGamePlugin{
			{
				Extension: kerneldomain.ExtensionDefinition{
					ID:      "com.example.pet",
					Name:    kerneldomain.LocalizedText{Default: "Pet"},
					Version: kerneldomain.SemanticVersion{Major: 1},
					Domain:  kerneldomain.ExtensionDomainDesktopPet, // 桌宠domain，跳过
				},
				Contribution: kerneldomain.ContributionDefinition{
					ID:   "main",
					Kind: kerneldomain.ContributionKindDesktopPetPlugin,
					Name: kerneldomain.LocalizedText{Default: "Pet Plugin"},
				},
			},
		},
	}

	mapper := NewDefaultGamePluginContributionMapper()
	svc, err := NewGamePluginSyncService(reg, mapper, source)
	if err != nil {
		t.Fatalf("failed to create sync service: %v", err)
	}

	result := svc.FullSync(context.Background())
	if result.Registered != 0 {
		t.Errorf("expected 0 registered for desktop pet plugin, got %d", result.Registered)
	}
}

func TestFullSyncMultiplePluginsPerExtension(t *testing.T) {
	reg := newMockRegistry()
	source := &mockSource{
		plugins: []KernelGamePlugin{
			{
				Extension: kerneldomain.ExtensionDefinition{
					ID:      "com.example.multi",
					Name:    kerneldomain.LocalizedText{Default: "Multi"},
					Version: kerneldomain.SemanticVersion{Major: 1},
					Domain:  kerneldomain.ExtensionDomainGame,
				},
				Contribution: kerneldomain.ContributionDefinition{
					ID:         "plugin1",
					Kind:       kerneldomain.ContributionKindGamePlugin,
					Name:       kerneldomain.LocalizedText{Default: "Plugin 1"},
					Definition: map[string]any{"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "runtime"},
				},
			},
			{
				Extension: kerneldomain.ExtensionDefinition{
					ID:      "com.example.multi",
					Name:    kerneldomain.LocalizedText{Default: "Multi"},
					Version: kerneldomain.SemanticVersion{Major: 1},
					Domain:  kerneldomain.ExtensionDomainGame,
				},
				Contribution: kerneldomain.ContributionDefinition{
					ID:         "plugin2",
					Kind:       kerneldomain.ContributionKindGamePlugin,
					Name:       kerneldomain.LocalizedText{Default: "Plugin 2"},
					Definition: map[string]any{"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "runtime"},
				},
			},
		},
	}

	mapper := NewDefaultGamePluginContributionMapper()
	svc, err := NewGamePluginSyncService(reg, mapper, source)
	if err != nil {
		t.Fatalf("failed to create sync service: %v", err)
	}

	result := svc.FullSync(context.Background())
	if result.Registered != 2 {
		t.Errorf("expected 2 registered, got %d", result.Registered)
	}

	// 验证两个插件都注册了
	_, err = reg.Get(context.Background(), "com.example.multi/plugin1")
	if err != nil {
		t.Errorf("expected plugin1 to be registered: %v", err)
	}
	_, err = reg.Get(context.Background(), "com.example.multi/plugin2")
	if err != nil {
		t.Errorf("expected plugin2 to be registered: %v", err)
	}
}

func TestUnregisterExtension(t *testing.T) {
	reg := registry.NewRegistry()
	source := &mockSource{}
	mapper := NewDefaultGamePluginContributionMapper()
	svc, err := NewGamePluginSyncService(reg, mapper, source)
	if err != nil {
		t.Fatalf("failed to create sync service: %v", err)
	}

	// 先手动注册两个插件
	err = reg.Register(context.Background(), gamehostdomain.PluginDescriptor{
		ID:              "com.example.ext/plugin1",
		ExtensionID:     "com.example.ext",
		Name:            "Plugin 1",
		Version:         "1.0.0",
		ProtocolVersion: "amitia-game-host/1",
	})
	if err != nil {
		t.Fatalf("failed to register plugin1: %v", err)
	}

	err = reg.Register(context.Background(), gamehostdomain.PluginDescriptor{
		ID:              "com.example.ext/plugin2",
		ExtensionID:     "com.example.ext",
		Name:            "Plugin 2",
		Version:         "1.0.0",
		ProtocolVersion: "amitia-game-host/1",
	})
	if err != nil {
		t.Fatalf("failed to register plugin2: %v", err)
	}

	// 执行注销
	result := svc.UnregisterExtension(context.Background(), "com.example.ext")
	if result.Unregistered != 2 {
		t.Errorf("expected 2 unregistered, got %d", result.Unregistered)
	}

	// 验证确实注销了
	_, err = reg.Get(context.Background(), "com.example.ext/plugin1")
	if err == nil {
		t.Error("expected plugin1 to be unregistered")
	}
	_, err = reg.Get(context.Background(), "com.example.ext/plugin2")
	if err == nil {
		t.Error("expected plugin2 to be unregistered")
	}
}

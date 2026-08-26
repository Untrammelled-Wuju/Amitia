package integration

import (
	"context"
	"errors"
	"testing"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	gamehostdomain "github.com/u-ai/backend/internal/gamehost/domain"
)

// errSource 模拟 source 返回错误
type errSource struct {
	err error
}

func (s *errSource) ListEnabledGamePlugins(ctx context.Context) ([]KernelGamePlugin, error) {
	return nil, s.err
}

// selectableMapper 允许配置指定 plugin ID 的 mapping 失败
type selectableMapper struct {
	failOnID gamehostdomain.PluginID
}

func (m *selectableMapper) ToDescriptor(ctx context.Context, ext kerneldomain.ExtensionDefinition, c kerneldomain.ContributionDefinition) (gamehostdomain.PluginDescriptor, error) {
	id := gamehostdomain.PluginID(string(ext.ID) + "/" + string(c.ID))
	if id == m.failOnID {
		return gamehostdomain.PluginDescriptor{}, errors.New("mock mapping failure")
	}
	mapper := NewDefaultGamePluginContributionMapper()
	return mapper.ToDescriptor(ctx, ext, c)
}

func reconcileTestExtension(id kerneldomain.ExtensionID) kerneldomain.ExtensionDefinition {
	return kerneldomain.ExtensionDefinition{
		ID:      id,
		Name:    kerneldomain.LocalizedText{Default: "Test"},
		Version: kerneldomain.SemanticVersion{Major: 1},
		Domain:  kerneldomain.ExtensionDomainGame,
		Modules: []kerneldomain.ModuleDefinition{{
			ID:          "runtime",
			ExtensionID: id,
			Type:        kerneldomain.ModuleTypeService,
			Runtime: &kerneldomain.RuntimeDefinition{
				Type:       kerneldomain.RuntimeTypeService,
				EntryPoint: "bin/runtime",
			},
		}},
	}
}

func reconcileTestContribution(id string) kerneldomain.ContributionDefinition {
	return kerneldomain.ContributionDefinition{
		ID:         kerneldomain.ContributionID(id),
		Kind:       kerneldomain.ContributionKindGamePlugin,
		Name:       kerneldomain.LocalizedText{Default: "Test"},
		Definition: map[string]any{"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "runtime", "network": map[string]any{"mode": "none"}},
	}
}

// TestFullSyncSourceSnapshotFailsRegistryUnchanged 验证 Source snapshot 失败时 Registry 完全不变
func TestFullSyncSourceSnapshotFailsRegistryUnchanged(t *testing.T) {
	reg := newMockRegistry()
	_ = reg.Register(context.Background(), gamehostdomain.PluginDescriptor{
		ID: "plugin-a",
	})
	_ = reg.Register(context.Background(), gamehostdomain.PluginDescriptor{
		ID: "plugin-b",
	})

	source := &errSource{err: errors.New("kernel repository unavailable")}
	mapper := NewDefaultGamePluginContributionMapper()
	svc, err := NewGamePluginSyncService(reg, mapper, source)
	if err != nil {
		t.Fatalf("failed to create sync service: %v", err)
	}

	result := svc.FullSync(context.Background())

	if !result.HasError() {
		t.Fatal("expected FullSync to report error")
	}

	// Registry 必须完全不变
	if _, err := reg.Get(context.Background(), "plugin-a"); err != nil {
		t.Errorf("plugin-a must still exist: %v", err)
	}
	if _, err := reg.Get(context.Background(), "plugin-b"); err != nil {
		t.Errorf("plugin-b must still exist: %v", err)
	}
	if result.Registered != 0 || result.Unregistered != 0 || result.Unchanged != 0 {
		t.Errorf("expected no mutations, got registered=%d unregistered=%d unchanged=%d",
			result.Registered, result.Unregistered, result.Unchanged)
	}
}

// TestFullSyncSinglePluginMappingFailsRegistryUnchanged 验证单插件 Mapping 失败时 Registry 保持原样
func TestFullSyncSinglePluginMappingFailsRegistryUnchanged(t *testing.T) {
	reg := newMockRegistry()
	_ = reg.Register(context.Background(), gamehostdomain.PluginDescriptor{
		ID: "com.example.game-a/main",
	})
	_ = reg.Register(context.Background(), gamehostdomain.PluginDescriptor{
		ID: "com.example.game-b/main",
	})

	source := &mockSource{
		plugins: []KernelGamePlugin{
			{
				Extension:    reconcileTestExtension("com.example.game-a"),
				Contribution: reconcileTestContribution("main"),
			},
			{
				Extension:    reconcileTestExtension("com.example.game-b"),
				Contribution: reconcileTestContribution("main"),
			},
		},
	}

	mapper := &selectableMapper{failOnID: "com.example.game-b/main"}
	svc, err := NewGamePluginSyncService(reg, mapper, source)
	if err != nil {
		t.Fatalf("failed to create sync service: %v", err)
	}

	result := svc.FullSync(context.Background())

	if !result.HasError() {
		t.Fatal("expected FullSync to report error")
	}

	// Registry 必须保持原样
	if _, err := reg.Get(context.Background(), "com.example.game-a/main"); err != nil {
		t.Errorf("plugin-a must still exist: %v", err)
	}
	if _, err := reg.Get(context.Background(), "com.example.game-b/main"); err != nil {
		t.Errorf("plugin-b must still exist: %v", err)
	}
	if result.Registered != 0 || result.Unregistered != 0 {
		t.Errorf("expected no mutations, got registered=%d unregistered=%d",
			result.Registered, result.Unregistered)
	}
}

// TestFullSyncNewPluginMappingFailsExistingPreserved 验证新插件 Mapping 失败时既有插件保留
func TestFullSyncNewPluginMappingFailsExistingPreserved(t *testing.T) {
	reg := newMockRegistry()
	_ = reg.Register(context.Background(), gamehostdomain.PluginDescriptor{
		ID: "com.example.game-a/main",
	})

	source := &mockSource{
		plugins: []KernelGamePlugin{
			{
				Extension:    reconcileTestExtension("com.example.game-a"),
				Contribution: reconcileTestContribution("main"),
			},
			{
				Extension:    reconcileTestExtension("com.example.game-b"),
				Contribution: reconcileTestContribution("main"),
			},
		},
	}

	mapper := &selectableMapper{failOnID: "com.example.game-b/main"}
	svc, err := NewGamePluginSyncService(reg, mapper, source)
	if err != nil {
		t.Fatalf("failed to create sync service: %v", err)
	}

	result := svc.FullSync(context.Background())

	if !result.HasError() {
		t.Fatal("expected FullSync to report error")
	}

	// 既有插件必须保留
	if _, err := reg.Get(context.Background(), "com.example.game-a/main"); err != nil {
		t.Errorf("plugin-a must still exist: %v", err)
	}
	// 新插件不得注册
	if _, err := reg.Get(context.Background(), "com.example.game-b/main"); err == nil {
		t.Error("plugin-b must not be registered")
	}
	// 不得触发 orphan cleanup
	if result.Unregistered != 0 {
		t.Errorf("expected 0 unregistered, got %d", result.Unregistered)
	}
}

// TestFullSyncRealOrphanUnregistered 验证真实 orphan 在完整快照成功后正确 Unregister
func TestFullSyncRealOrphanUnregistered(t *testing.T) {
	reg := newMockRegistry()
	_ = reg.Register(context.Background(), gamehostdomain.PluginDescriptor{
		ID: "com.example.game-a/main",
	})
	_ = reg.Register(context.Background(), gamehostdomain.PluginDescriptor{
		ID: "com.example.game-b/main",
	})

	source := &mockSource{
		plugins: []KernelGamePlugin{
			{
				Extension:    reconcileTestExtension("com.example.game-a"),
				Contribution: reconcileTestContribution("main"),
			},
			// 仅 game-a，game-b 成为 orphan
		},
	}

	mapper := NewDefaultGamePluginContributionMapper()
	svc, err := NewGamePluginSyncService(reg, mapper, source)
	if err != nil {
		t.Fatalf("failed to create sync service: %v", err)
	}

	result := svc.FullSync(context.Background())

	if result.HasError() {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// game-b 被正确 Unregister
	if _, err := reg.Get(context.Background(), "com.example.game-b/main"); err == nil {
		t.Error("plugin-b should be unregistered as orphan")
	}
	// game-a 仍然存在
	if _, err := reg.Get(context.Background(), "com.example.game-a/main"); err != nil {
		t.Errorf("plugin-a must still exist: %v", err)
	}
	if result.Unregistered != 1 {
		t.Errorf("expected 1 unregistered, got %d", result.Unregistered)
	}
}

// TestFullSyncEmptyAuthoritativeSnapshotUnregistersAll 验证空权威快照无错误时 Unregister 全部
func TestFullSyncEmptyAuthoritativeSnapshotUnregistersAll(t *testing.T) {
	reg := newMockRegistry()
	_ = reg.Register(context.Background(), gamehostdomain.PluginDescriptor{
		ID: "com.example.game-a/main",
	})
	_ = reg.Register(context.Background(), gamehostdomain.PluginDescriptor{
		ID: "com.example.game-b/main",
	})

	source := &mockSource{
		plugins: []KernelGamePlugin{}, // 空列表，无错误
	}

	mapper := NewDefaultGamePluginContributionMapper()
	svc, err := NewGamePluginSyncService(reg, mapper, source)
	if err != nil {
		t.Fatalf("failed to create sync service: %v", err)
	}

	result := svc.FullSync(context.Background())

	if result.HasError() {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// 全部 Unregister
	if _, err := reg.Get(context.Background(), "com.example.game-a/main"); err == nil {
		t.Error("plugin-a should be unregistered")
	}
	if _, err := reg.Get(context.Background(), "com.example.game-b/main"); err == nil {
		t.Error("plugin-b should be unregistered")
	}
	if result.Unregistered != 2 {
		t.Errorf("expected 2 unregistered, got %d", result.Unregistered)
	}
}

// TestFullSyncDescriptorStageErrorContainsStageName 验证错误日志包含阶段名称
func TestFullSyncDescriptorStageErrorContainsStageName(t *testing.T) {
	reg := newMockRegistry()
	source := &mockSource{
		plugins: []KernelGamePlugin{
			{
				Extension:    reconcileTestExtension("com.example.game-a"),
				Contribution: reconcileTestContribution("main"),
			},
		},
	}

	mapper := &selectableMapper{failOnID: "com.example.game-a/main"}
	svc, err := NewGamePluginSyncService(reg, mapper, source)
	if err != nil {
		t.Fatalf("failed to create sync service: %v", err)
	}

	result := svc.FullSync(context.Background())

	if !result.HasError() {
		t.Fatal("expected FullSync to report error")
	}

	found := false
	for _, e := range result.Errors {
		if e.Error() != "" && containsString(e.Error(), "stage=descriptor") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error to contain 'stage=descriptor', got: %v", result.Errors)
	}
}

func containsString(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

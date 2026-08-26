package gamehost

import (
	"context"
	"errors"
	"testing"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/gamehost/integration"
)

// mockInstallationLister 模拟 installation 列表
type mockInstallationLister struct {
	installations []kerneldomain.ExtensionInstallation
	err           error
}

func (m *mockInstallationLister) ListInstallations(ctx context.Context) ([]kerneldomain.ExtensionInstallation, error) {
	return m.installations, m.err
}

// mockDefinitionLister 模拟 definition 查询，可配置指定 ID 失败
type mockDefinitionLister struct {
	defs map[kerneldomain.ExtensionID]kerneldomain.ExtensionDefinition
	err  error
}

func (m *mockDefinitionLister) GetExtension(ctx context.Context, id kerneldomain.ExtensionID, version kerneldomain.SemanticVersion) (kerneldomain.ExtensionDefinition, error) {
	if m.err != nil {
		return kerneldomain.ExtensionDefinition{}, m.err
	}
	def, ok := m.defs[id]
	if !ok {
		return kerneldomain.ExtensionDefinition{}, errors.New("definition not found")
	}
	return def, nil
}

// mockContributionLister 模拟 contribution 查询，可配置指定 ID 失败
type mockContributionLister struct {
	contribs map[kerneldomain.ExtensionID][]kerneldomain.ContributionDefinition
	err      error
}

func (m *mockContributionLister) ListContributions(ctx context.Context, extensionID kerneldomain.ExtensionID) ([]kerneldomain.ContributionDefinition, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.contribs[extensionID], nil
}

func makeExtensionInstallation(id kerneldomain.ExtensionID) kerneldomain.ExtensionInstallation {
	return kerneldomain.ExtensionInstallation{
		ExtensionID:      id,
		InstalledVersion: kerneldomain.SemanticVersion{Major: 1},
		InstallationState: kerneldomain.InstallationStateInstalled,
		EnablementState:  kerneldomain.EnablementEnabled,
	}
}

func makeExtensionDefinition(id kerneldomain.ExtensionID) kerneldomain.ExtensionDefinition {
	return kerneldomain.ExtensionDefinition{
		ID:      id,
		Name:    kerneldomain.LocalizedText{Default: "Test"},
		Version: kerneldomain.SemanticVersion{Major: 1},
		Domain:  kerneldomain.ExtensionDomainGame,
	}
}

func makeGamePluginContribution() kerneldomain.ContributionDefinition {
	return kerneldomain.ContributionDefinition{
		ID:   "main",
		Kind: kerneldomain.ContributionKindGamePlugin,
		Name: kerneldomain.LocalizedText{Default: "Test Plugin"},
	}
}

func makeKernelGamePlugin(extID kerneldomain.ExtensionID) integration.KernelGamePlugin {
	return integration.KernelGamePlugin{
		Extension:    makeExtensionDefinition(extID),
		Contribution: makeGamePluginContribution(),
	}
}

// TestListEnabledGamePluginsDefinitionErrorReturnsError 验证定义读取失败时返回错误
func TestListEnabledGamePluginsDefinitionErrorReturnsError(t *testing.T) {
	instRepo := &mockInstallationLister{
		installations: []kerneldomain.ExtensionInstallation{
			makeExtensionInstallation("com.example.game-a"),
			makeExtensionInstallation("com.example.game-b"),
		},
	}
	defRepo := &mockDefinitionLister{
		err: errors.New("repository unavailable"),
	}
	contribRepo := &mockContributionLister{}

	source := newKernelContributionSource(instRepo, defRepo, contribRepo)
	plugins, err := source.ListEnabledGamePlugins(context.Background())

	if err == nil {
		t.Fatal("expected error when definition fails")
	}
	if plugins != nil {
		t.Errorf("expected nil plugins, got %v", plugins)
	}
}

// TestListEnabledGamePluginsContributionErrorReturnsError 验证贡献读取失败时返回错误
func TestListEnabledGamePluginsContributionErrorReturnsError(t *testing.T) {
	instRepo := &mockInstallationLister{
		installations: []kerneldomain.ExtensionInstallation{
			makeExtensionInstallation("com.example.game-a"),
			makeExtensionInstallation("com.example.game-b"),
		},
	}
	defRepo := &mockDefinitionLister{
		defs: map[kerneldomain.ExtensionID]kerneldomain.ExtensionDefinition{
			"com.example.game-a": makeExtensionDefinition("com.example.game-a"),
			"com.example.game-b": makeExtensionDefinition("com.example.game-b"),
		},
	}
	contribRepo := &mockContributionLister{
		err: errors.New("repository unavailable"),
	}

	source := newKernelContributionSource(instRepo, defRepo, contribRepo)
	plugins, err := source.ListEnabledGamePlugins(context.Background())

	if err == nil {
		t.Fatal("expected error when contribution fails")
	}
	if plugins != nil {
		t.Errorf("expected nil plugins, got %v", plugins)
	}
}

// TestListEnabledGamePluginsAllSuccessReturnsCompleteSnapshot 验证全部成功时返回完整快照
func TestListEnabledGamePluginsAllSuccessReturnsCompleteSnapshot(t *testing.T) {
	instRepo := &mockInstallationLister{
		installations: []kerneldomain.ExtensionInstallation{
			makeExtensionInstallation("com.example.game-a"),
			makeExtensionInstallation("com.example.game-b"),
		},
	}
	defRepo := &mockDefinitionLister{
		defs: map[kerneldomain.ExtensionID]kerneldomain.ExtensionDefinition{
			"com.example.game-a": makeExtensionDefinition("com.example.game-a"),
			"com.example.game-b": makeExtensionDefinition("com.example.game-b"),
		},
	}
	contribRepo := &mockContributionLister{
		contribs: map[kerneldomain.ExtensionID][]kerneldomain.ContributionDefinition{
			"com.example.game-a": {makeGamePluginContribution()},
			"com.example.game-b": {makeGamePluginContribution()},
		},
	}

	source := newKernelContributionSource(instRepo, defRepo, contribRepo)
	plugins, err := source.ListEnabledGamePlugins(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plugins) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(plugins))
	}
}

// TestListEnabledGamePluginsInstallationErrorReturnsError 验证安装列表读取失败时返回错误
func TestListEnabledGamePluginsInstallationErrorReturnsError(t *testing.T) {
	instRepo := &mockInstallationLister{
		err: errors.New("installation repository unavailable"),
	}
	defRepo := &mockDefinitionLister{}
	contribRepo := &mockContributionLister{}

	source := newKernelContributionSource(instRepo, defRepo, contribRepo)
	plugins, err := source.ListEnabledGamePlugins(context.Background())

	if err == nil {
		t.Fatal("expected error when installation listing fails")
	}
	if plugins != nil {
		t.Errorf("expected nil plugins, got %v", plugins)
	}
}

// TestListEnabledGamePluginsSnapshotFailureNoPartialResult 验证快照失败时不返回 partial result
func TestListEnabledGamePluginsSnapshotFailureNoPartialResult(t *testing.T) {
	instRepo := &mockInstallationLister{
		installations: []kerneldomain.ExtensionInstallation{
			makeExtensionInstallation("com.example.game-a"),
			makeExtensionInstallation("com.example.game-b"),
		},
	}
	defRepo := &mockDefinitionLister{
		defs: map[kerneldomain.ExtensionID]kerneldomain.ExtensionDefinition{
			"com.example.game-a": makeExtensionDefinition("com.example.game-a"),
			// game-b 的 definition 不存在，会报错
		},
	}
	contribRepo := &mockContributionLister{
		contribs: map[kerneldomain.ExtensionID][]kerneldomain.ContributionDefinition{
			"com.example.game-a": {makeGamePluginContribution()},
		},
	}

	source := newKernelContributionSource(instRepo, defRepo, contribRepo)
	plugins, err := source.ListEnabledGamePlugins(context.Background())

	if err == nil {
		t.Fatal("expected error when definition not found")
	}
	if plugins != nil {
		t.Errorf("expected nil plugins (no partial snapshot), got %d plugins", len(plugins))
	}
}

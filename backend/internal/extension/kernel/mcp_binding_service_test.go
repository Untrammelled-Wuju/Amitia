package kernel

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/secret"
)

func TestMCPBindingDescriptorParsesCanonicalManifest(t *testing.T) {
	installer := NewTypedContributionInstaller(&Container{})
	contrib := domain.ContributionDefinition{
		ID:          "fixture",
		ExtensionID: "example.test/fixture",
		ModuleID:    "main",
		Kind:        domain.ContributionKindMCPServer,
		Name:        domain.LocalizedText{Default: "Fixture MCP"},
		Definition: map[string]any{
			"schemaVersion": 1,
			"transport": map[string]any{
				"type": "stdio",
				"stdio": map[string]any{
					"command": "node",
					"args":    []any{"server.mjs"},
					"env": map[string]any{
						"values": map[string]any{"MODE": "fixture"},
					},
				},
			},
			"capabilities": map[string]any{
				"client": map[string]any{
					"roots":    "required",
					"sampling": "optional",
					"tasks":    "disabled",
				},
			},
		},
	}

	binding, err := installer.mcpBindingDescriptor(context.Background(), contrib, nil, 9)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Transport != "stdio" || binding.Command != "node" || binding.ServerName != "Fixture MCP" {
		t.Fatalf("unexpected stdio binding: %#v", binding)
	}
	if len(binding.Args) != 1 || binding.Args[0] != "server.mjs" || binding.Env["MODE"] != "fixture" {
		t.Fatalf("unexpected stdio arguments or environment: %#v", binding)
	}
	if !binding.Capabilities["roots"] || !binding.Capabilities["sampling"] || binding.Capabilities["tasks"] {
		t.Fatalf("unexpected MCP client capabilities: %#v", binding.Capabilities)
	}
	if binding.ExtensionID != string(contrib.ExtensionID) || binding.ContributionID != string(contrib.ID) || binding.PackageGeneration != 9 {
		t.Fatalf("unexpected package identity: %#v", binding)
	}
}

func TestMCPBindingDescriptorResolvesPackageSecrets(t *testing.T) {
	root := t.TempDir()
	store, err := secret.NewEncryptedFileStore(filepath.Join(root, "secrets.json"), filepath.Join(root, "secrets.key"))
	if err != nil {
		t.Fatal(err)
	}
	broker, err := secret.NewBroker(secret.BrokerConfig{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := broker.Store(context.Background(), "test/mcp", []byte("resolved-secret"))
	if err != nil {
		t.Fatal(err)
	}
	installer := NewTypedContributionInstaller(&Container{SecretBroker: broker})
	contrib := domain.ContributionDefinition{
		ID:          "fixture",
		ExtensionID: "example.test/secret",
		ModuleID:    "main",
		Kind:        domain.ContributionKindMCPServer,
		Name:        domain.LocalizedText{Default: "Secret MCP"},
		Definition: map[string]any{
			"schemaVersion": 1,
			"transport": map[string]any{
				"type": "stdio",
				"stdio": map[string]any{
					"command": "node",
					"env": map[string]any{
						"secrets": map[string]any{"TOKEN": ref.String()},
					},
				},
			},
		},
	}
	binding, err := installer.mcpBindingDescriptor(context.Background(), contrib, nil, 11)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Env["TOKEN"] != "resolved-secret" || binding.AuthType != "stdio_env" {
		t.Fatalf("unexpected resolved environment: %#v", binding)
	}
}

type recordingMCPBindingService struct {
	put         MCPBindingDescriptor
	activated   MCPBindingDescriptor
	deactivated bool
	removed     bool
}

func (s *recordingMCPBindingService) PutContributionBinding(_ context.Context, binding MCPBindingDescriptor) (string, error) {
	s.put = binding
	return "server-1", nil
}

func (s *recordingMCPBindingService) ActivateContributionBinding(_ context.Context, binding MCPBindingDescriptor) (string, error) {
	s.activated = binding
	return "server-1", nil
}

func (s *recordingMCPBindingService) DeactivateContributionBinding(context.Context, string, string) error {
	s.deactivated = true
	return nil
}

func (s *recordingMCPBindingService) RemoveContributionBinding(context.Context, string, string) error {
	s.removed = true
	return nil
}

func (s *recordingMCPBindingService) RemoveExtensionBindings(context.Context, string) error {
	s.removed = true
	return nil
}

func TestMCPContributionLifecycleUsesBindingService(t *testing.T) {
	bindingService := &recordingMCPBindingService{}
	installer := NewTypedContributionInstaller(&Container{MCPBindings: bindingService})
	contrib := domain.ContributionDefinition{
		ID:          "fixture",
		ExtensionID: "example.test/fixture",
		ModuleID:    "main",
		Kind:        domain.ContributionKindMCPServer,
		Name:        domain.LocalizedText{Default: "Fixture MCP"},
		Definition: map[string]any{
			"schemaVersion": 1,
			"transport": map[string]any{
				"type": "stdio",
				"stdio": map[string]any{
					"command": "node",
					"args":    []any{"server.mjs"},
				},
			},
		},
	}
	defData, err := json.Marshal(contrib.Definition)
	if err != nil {
		t.Fatal(err)
	}
	op, err := installer.buildMCPServerOp(context.Background(), contrib, defData, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := op.doInstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	if bindingService.put.Command != "node" || bindingService.put.ExtensionID != string(contrib.ExtensionID) {
		t.Fatalf("install did not use the canonical binding service: %#v", bindingService.put)
	}
	if err := installer.activateSingle(context.Background(), contrib, 4); err != nil {
		t.Fatal(err)
	}
	if bindingService.activated.Command != "node" || bindingService.activated.PackageGeneration != 4 {
		t.Fatalf("activate did not use the canonical binding service: %#v", bindingService.activated)
	}
	if err := installer.deactivateSingle(context.Background(), contrib); err != nil {
		t.Fatal(err)
	}
	if !bindingService.deactivated {
		t.Fatal("deactivate did not use the canonical binding service")
	}
	if err := installer.discardMCPServer(context.Background(), contrib, defData); err != nil {
		t.Fatal(err)
	}
	if !bindingService.removed {
		t.Fatal("discard did not remove the canonical binding")
	}
}

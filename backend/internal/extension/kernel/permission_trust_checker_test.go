package kernel

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

func TestRepositoryPermissionTrustCheckerAllowsDevelopmentInstallation(t *testing.T) {
	ctx := context.Background()
	const extensionID = domain.ExtensionID("com.example/dev-game")
	version := domain.SemanticVersion{Major: 1, Minor: 2, Patch: 3}
	definitions := domain.NewInMemoryDefinitionRepository()
	installations := domain.NewInMemoryInstallationRepository()
	if err := definitions.Put(ctx, domain.ExtensionDefinition{
		ID:      extensionID,
		Version: version,
		Publisher: domain.PublisherReference{
			PublisherID: "example",
			TrustLevel:  "development",
		},
	}); err != nil {
		t.Fatalf("put definition: %v", err)
	}
	if err := installations.PutInstallation(ctx, domain.ExtensionInstallation{
		ExtensionID:       extensionID,
		InstalledVersion:  version,
		InstallationState: domain.InstallationStateInstalled,
		Metadata:          map[string]any{"devOnly": true},
	}); err != nil {
		t.Fatalf("put installation: %v", err)
	}

	checker := newRepositoryPermissionTrustChecker(installations, definitions)
	if !checker.IsTrusted(permission.PermissionSubject{
		Type:        permission.SubjectRuntime,
		ID:          "rt-dev",
		ExtensionID: string(extensionID),
	}) {
		t.Fatal("development installation should be trusted for the explicitly approved runtime path")
	}
}

func TestRepositoryPermissionTrustCheckerRejectsUnknownNonDevelopmentInstallation(t *testing.T) {
	ctx := context.Background()
	const extensionID = domain.ExtensionID("com.example/untrusted-game")
	version := domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}
	definitions := domain.NewInMemoryDefinitionRepository()
	installations := domain.NewInMemoryInstallationRepository()
	if err := definitions.Put(ctx, domain.ExtensionDefinition{
		ID:      extensionID,
		Version: version,
		Publisher: domain.PublisherReference{
			PublisherID: "example",
			TrustLevel:  "unknown",
		},
	}); err != nil {
		t.Fatalf("put definition: %v", err)
	}
	if err := installations.PutInstallation(ctx, domain.ExtensionInstallation{
		ExtensionID:       extensionID,
		InstalledVersion:  version,
		InstallationState: domain.InstallationStateInstalled,
	}); err != nil {
		t.Fatalf("put installation: %v", err)
	}

	checker := newRepositoryPermissionTrustChecker(installations, definitions)
	if checker.IsTrusted(permission.PermissionSubject{
		Type:        permission.SubjectRuntime,
		ID:          "rt-untrusted",
		ExtensionID: string(extensionID),
	}) {
		t.Fatal("unknown non-development installation must remain untrusted")
	}
}

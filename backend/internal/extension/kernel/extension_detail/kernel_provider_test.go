package extension_detail

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/package_distribution"
)

type detailUpdateChecker struct {
	update *package_distribution.ExtensionUpdateMetadata
	err    error
}

func (c detailUpdateChecker) Check(context.Context, string, string) (*package_distribution.ExtensionUpdateMetadata, error) {
	return c.update, c.err
}

type detailVersionReader struct {
	items []VersionDetail
	err   error
}

func (r detailVersionReader) ListVersionDetails(context.Context, string) ([]VersionDetail, error) {
	return r.items, r.err
}

func TestKernelDetailProviderBuildsCanonicalDetail(t *testing.T) {
	definitions := domain.NewInMemoryDefinitionRepository()
	installations := domain.NewInMemoryInstallationRepository()
	extensionID := domain.ExtensionID("com.example/detail-test")
	version := domain.SemanticVersion{Major: 1, Minor: 2, Patch: 3}
	if err := definitions.PutExtension(context.Background(), domain.ExtensionDefinition{
		ID:              extensionID,
		Name:            domain.LocalizedText{Default: "Detail Test"},
		Description:     domain.LocalizedText{Default: "detail"},
		Version:         version,
		ManifestVersion: 1,
		Domain:          domain.ExtensionDomainGeneral,
		Publisher:       domain.PublisherReference{PublisherID: "publisher", TrustLevel: "official"},
		Package:         domain.PackageReference{Signature: domain.SignatureReference{Status: "verified"}},
		Modules: []domain.ModuleDefinition{{
			ID:          "main",
			ExtensionID: extensionID,
			Name:        domain.LocalizedText{Default: "Main"},
			Type:        domain.ModuleTypeService,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := installations.PutInstallation(context.Background(), domain.ExtensionInstallation{
		InstallationID:    "installation",
		ExtensionID:       extensionID,
		InstalledVersion:  version,
		InstallationState: domain.InstallationStateInstalled,
		EnablementState:   domain.EnablementEnabled,
		InstalledAt:       now,
		UpdatedAt:         now,
	}); err != nil {
		t.Fatal(err)
	}
	provider := &KernelDetailProvider{
		Definitions:   definitions,
		Installations: installations,
		Versions: detailVersionReader{items: []VersionDetail{
			{Version: "1.2.3", Current: true},
			{Version: "1.2.2"},
		}},
		Updates: detailUpdateChecker{update: &package_distribution.ExtensionUpdateMetadata{
			ExtensionID: string(extensionID),
			Version:     "1.3.0",
		}},
	}
	detail, err := provider.GetExtension(context.Background(), string(extensionID))
	if err != nil {
		t.Fatal(err)
	}
	if detail.Publisher != "publisher" || detail.Trust != "official" || detail.SignatureStatus != "verified" {
		t.Fatalf("identity fields missing: %+v", detail)
	}
	if len(detail.Modules) != 1 || len(detail.Versions) != 2 || !detail.UpdateAvailable || detail.LatestVersion != "1.3.0" {
		t.Fatalf("detail aggregation incomplete: %+v", detail)
	}
	if len(detail.Actions) == 0 {
		t.Fatal("actions missing")
	}
}

func TestKernelDetailProviderPreservesUpdateError(t *testing.T) {
	definitions := domain.NewInMemoryDefinitionRepository()
	installations := domain.NewInMemoryInstallationRepository()
	extensionID := domain.ExtensionID("com.example/detail-error")
	version := domain.SemanticVersion{Major: 1}
	if err := definitions.PutExtension(context.Background(), domain.ExtensionDefinition{
		ID:              extensionID,
		Name:            domain.LocalizedText{Default: "Detail Error"},
		Version:         version,
		ManifestVersion: 1,
		Domain:          domain.ExtensionDomainGeneral,
		Modules: []domain.ModuleDefinition{{
			ID:          "main",
			ExtensionID: extensionID,
			Name:        domain.LocalizedText{Default: "Main"},
			Type:        domain.ModuleTypeService,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := installations.PutInstallation(context.Background(), domain.ExtensionInstallation{
		ExtensionID:       extensionID,
		InstalledVersion:  version,
		InstallationState: domain.InstallationStateInstalled,
		EnablementState:   domain.EnablementEnabled,
	}); err != nil {
		t.Fatal(err)
	}
	provider := &KernelDetailProvider{
		Definitions:   definitions,
		Installations: installations,
		Updates:       detailUpdateChecker{err: errors.New("registry unavailable: original detail")},
	}
	detail, err := provider.GetExtension(context.Background(), string(extensionID))
	if err != nil {
		t.Fatal(err)
	}
	if detail.UpdateError != "registry unavailable: original detail" {
		t.Fatalf("update error changed: %q", detail.UpdateError)
	}
}

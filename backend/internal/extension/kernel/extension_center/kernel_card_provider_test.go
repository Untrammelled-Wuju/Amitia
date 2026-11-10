package extension_center

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/package_distribution"
)

func TestKernelCardProviderReturnsEmptyWithoutRepositories(t *testing.T) {
	provider := NewKernelCardProvider(nil, nil)
	cards, err := provider.ListCards(context.Background())
	if err != nil {
		t.Fatalf("ListCards returned error: %v", err)
	}
	if len(cards) != 0 {
		t.Fatalf("expected empty cards, got %d", len(cards))
	}
}

type failingCardProvider struct{}

func (failingCardProvider) ListCards(context.Context) ([]ExtensionCard, error) {
	return nil, errors.New("test failure")
}

type emptyCardProvider struct{}

func (emptyCardProvider) ListCards(context.Context) ([]ExtensionCard, error) {
	return []ExtensionCard{}, nil
}

func TestExtensionCenterHandlerPreservesProviderError(t *testing.T) {
	handler := NewHTTPHandler(NewCenterService(failingCardProvider{}))
	mux := http.NewServeMux()
	handler.Register(mux)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/extension-center/view", nil)
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "test failure" {
		t.Fatalf("provider error changed: %q", body["error"])
	}
}

func TestCenterViewSerializesEmptyGroupsAsArrays(t *testing.T) {
	service := NewCenterService(emptyCardProvider{})
	service.SetTargetMode("device-agent")
	view, err := service.GetView(context.Background(), CenterFilter{}, SortByName)
	if err != nil {
		t.Fatal(err)
	}
	if view.TargetMode != "device-agent" {
		t.Fatalf("target mode = %q", view.TargetMode)
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, field := range []string{"installed", "discover", "localImports", "dev", "updates", "needsAction"} {
		if !strings.Contains(text, `"`+field+`":[]`) {
			t.Fatalf("%s must serialize as an empty array: %s", field, text)
		}
	}
}

type fixedUpdateChecker struct {
	update *package_distribution.ExtensionUpdateMetadata
	err    error
}

func (c fixedUpdateChecker) Check(context.Context, string, string) (*package_distribution.ExtensionUpdateMetadata, error) {
	return c.update, c.err
}

func TestKernelCardProviderBuildsUpdateAndTrustState(t *testing.T) {
	definitions := domain.NewInMemoryDefinitionRepository()
	installations := domain.NewInMemoryInstallationRepository()
	extensionID := domain.ExtensionID("com.amitia.builtin.update-test")
	v1 := domain.SemanticVersion{Major: 1}
	v2 := domain.SemanticVersion{Major: 2}
	definition := domain.ExtensionDefinition{
		ID:              extensionID,
		Name:            domain.LocalizedText{Default: "Update Test"},
		Description:     domain.LocalizedText{Default: "update"},
		Version:         v1,
		ManifestVersion: 1,
		Domain:          domain.ExtensionDomainGeneral,
		Publisher:       domain.PublisherReference{PublisherID: "publisher", TrustLevel: "official"},
		Modules: []domain.ModuleDefinition{{
			ID:          "main",
			ExtensionID: extensionID,
			Name:        domain.LocalizedText{Default: "Main"},
			Type:        domain.ModuleTypeService,
		}},
	}
	if err := definitions.PutExtension(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	definition.Version = v2
	if err := definitions.PutExtension(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := installations.PutInstallation(context.Background(), domain.ExtensionInstallation{
		InstallationID:    "installation",
		ExtensionID:       extensionID,
		InstalledVersion:  v1,
		InstallationState: domain.InstallationStateInstalled,
		EnablementState:   domain.EnablementEnabled,
		InstalledAt:       now,
		UpdatedAt:         now,
	}); err != nil {
		t.Fatal(err)
	}

	provider := NewKernelCardProvider(definitions, installations, CardProviderDependencies{
		Updates: fixedUpdateChecker{update: &package_distribution.ExtensionUpdateMetadata{
			ExtensionID: string(extensionID),
			Version:     v2.String(),
		}},
	})
	cards, err := provider.ListCards(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 {
		t.Fatalf("expected one deduplicated card, got %d", len(cards))
	}
	card := cards[0]
	if !card.UpdateAvailable || card.LatestVersion != v2.String() || card.Update == nil {
		t.Fatalf("update state not populated: %+v", card)
	}
	if card.Trust != TrustLevelOfficial {
		t.Fatalf("trust = %q, want official", card.Trust)
	}
	if card.SystemManaged {
		t.Fatal("builtin-looking id without system.managed metadata must not be system managed")
	}
}

func TestKernelCardProviderPreservesUpdateError(t *testing.T) {
	definitions := domain.NewInMemoryDefinitionRepository()
	installations := domain.NewInMemoryInstallationRepository()
	extensionID := domain.ExtensionID("com.example/error-test")
	version := domain.SemanticVersion{Major: 1}
	if err := definitions.PutExtension(context.Background(), domain.ExtensionDefinition{
		ID:              extensionID,
		Name:            domain.LocalizedText{Default: "Error Test"},
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
	now := time.Now().UTC()
	if err := installations.PutInstallation(context.Background(), domain.ExtensionInstallation{
		ExtensionID:       extensionID,
		InstalledVersion:  version,
		InstallationState: domain.InstallationStateInstalled,
		EnablementState:   domain.EnablementEnabled,
		InstalledAt:       now,
		UpdatedAt:         now,
	}); err != nil {
		t.Fatal(err)
	}
	provider := NewKernelCardProvider(definitions, installations, CardProviderDependencies{
		Updates: fixedUpdateChecker{err: errors.New("registry unavailable: signature index")},
	})
	cards, err := provider.ListCards(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || len(cards[0].Diagnostics) != 1 {
		t.Fatalf("diagnostic not preserved: %+v", cards)
	}
	if cards[0].Diagnostics[0].Message != "registry unavailable: signature index" {
		t.Fatalf("error message changed: %q", cards[0].Diagnostics[0].Message)
	}
}

func TestKernelCardProviderTreatsMissingUpdateRegistryAsOptional(t *testing.T) {
	definitions := domain.NewInMemoryDefinitionRepository()
	installations := domain.NewInMemoryInstallationRepository()
	extensionID := domain.ExtensionID("com.example/no-update-registry")
	version := domain.SemanticVersion{Major: 1}
	if err := definitions.PutExtension(context.Background(), domain.ExtensionDefinition{
		ID:              extensionID,
		Name:            domain.LocalizedText{Default: "No Update Registry"},
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
	now := time.Now().UTC()
	if err := installations.PutInstallation(context.Background(), domain.ExtensionInstallation{
		ExtensionID:       extensionID,
		InstalledVersion:  version,
		InstallationState: domain.InstallationStateInstalled,
		EnablementState:   domain.EnablementEnabled,
		InstalledAt:       now,
		UpdatedAt:         now,
	}); err != nil {
		t.Fatal(err)
	}
	provider := NewKernelCardProvider(definitions, installations, CardProviderDependencies{
		Updates: fixedUpdateChecker{err: package_distribution.ErrRegistryNotConfigured},
	})
	cards, err := provider.ListCards(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || len(cards[0].Diagnostics) != 0 {
		t.Fatalf("unconfigured update registry must not mark cards partial: %+v", cards)
	}
}

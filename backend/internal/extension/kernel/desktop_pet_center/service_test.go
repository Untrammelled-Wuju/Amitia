package desktop_pet_center

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

type fakeContainer struct {
	installations  []domain.ExtensionInstallation
	modules        []domain.ModuleDefinition
	contributions  []domain.ContributionDefinition
	grants         []sqlite.PermissionGrant
	listInstErr    error
	getInstErr     error
	listModErr     error
	listContribErr error
	listGrantErr   error
}

func (f *fakeContainer) ListInstallations(ctx context.Context) ([]domain.ExtensionInstallation, error) {
	if f.listInstErr != nil {
		return nil, f.listInstErr
	}
	return f.installations, nil
}

func (f *fakeContainer) GetInstallation(ctx context.Context, id domain.ExtensionID) (domain.ExtensionInstallation, error) {
	if f.getInstErr != nil {
		return domain.ExtensionInstallation{}, f.getInstErr
	}
	for _, inst := range f.installations {
		if inst.ExtensionID == id {
			return inst, nil
		}
	}
	return domain.ExtensionInstallation{}, ErrExtensionNotFound
}

func (f *fakeContainer) ListModules(ctx context.Context, extID domain.ExtensionID) ([]domain.ModuleDefinition, error) {
	if f.listModErr != nil {
		return nil, f.listModErr
	}
	out := make([]domain.ModuleDefinition, 0)
	for _, m := range f.modules {
		if m.ExtensionID == extID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeContainer) ListContributions(ctx context.Context, extID domain.ExtensionID) ([]domain.ContributionDefinition, error) {
	if f.listContribErr != nil {
		return nil, f.listContribErr
	}
	out := make([]domain.ContributionDefinition, 0)
	for _, c := range f.contributions {
		if c.ExtensionID == extID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeContainer) ListGrants(ctx context.Context, extID domain.ExtensionID) ([]sqlite.PermissionGrant, error) {
	if f.listGrantErr != nil {
		return nil, f.listGrantErr
	}
	out := make([]sqlite.PermissionGrant, 0)
	for _, g := range f.grants {
		if g.ExtensionID == extID {
			out = append(out, g)
		}
	}
	return out, nil
}

type fakeRuntime struct {
	installFn   func(ctx context.Context, archivePath string) (kernelInstalledExtension, error)
	updateFn    func(ctx context.Context, archivePath string) (kernelInstalledExtension, error)
	enableFn    func(ctx context.Context, extensionID string) error
	disableFn   func(ctx context.Context, extensionID string) error
	uninstallFn func(ctx context.Context, extensionID string) error
}

func (f *fakeRuntime) Install(ctx context.Context, archivePath string) (kernelInstalledExtension, error) {
	if f.installFn != nil {
		return f.installFn(ctx, archivePath)
	}
	return kernelInstalledExtension{}, nil
}

func (f *fakeRuntime) Update(ctx context.Context, archivePath string) (kernelInstalledExtension, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, archivePath)
	}
	return kernelInstalledExtension{}, nil
}

func (f *fakeRuntime) Enable(ctx context.Context, extensionID string) error {
	if f.enableFn != nil {
		return f.enableFn(ctx, extensionID)
	}
	return nil
}

func (f *fakeRuntime) Disable(ctx context.Context, extensionID string) error {
	if f.disableFn != nil {
		return f.disableFn(ctx, extensionID)
	}
	return nil
}

func (f *fakeRuntime) Uninstall(ctx context.Context, extensionID string) error {
	if f.uninstallFn != nil {
		return f.uninstallFn(ctx, extensionID)
	}
	return nil
}

func makeInstallState(state domain.InstallationState) domain.ExtensionInstallation {
	now := time.Now().UTC()
	prefix := string(state)
	return domain.ExtensionInstallation{
		InstallationID:    "inst-" + prefix,
		ExtensionID:       domain.ExtensionID("com.example/" + prefix),
		InstalledVersion:  domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0},
		PackageID:         "pkg-" + prefix,
		InstallationState: state,
		EnablementState:   domain.EnablementEnabled,
		InstalledAt:       now,
		UpdatedAt:         now,
		Generation:        1,
	}
}

func makePetPluginContrib(extID domain.ExtensionID, contribID string) domain.ContributionDefinition {
	return domain.ContributionDefinition{
		ID:                  domain.ContributionID(contribID),
		ModuleID:            "main",
		ExtensionID:         extID,
		Kind:                domain.ContributionKindDesktopPetPlugin,
		Name:                domain.LocalizedText{Default: "Pet Plugin " + contribID},
		Description:         domain.LocalizedText{Default: "A desktop pet plugin"},
		RequiredPermissions: []string{"display.window", "input.mouse"},
	}
}

func makeGamePluginContrib(extID domain.ExtensionID, contribID string) domain.ContributionDefinition {
	return domain.ContributionDefinition{
		ID:          domain.ContributionID(contribID),
		ModuleID:    "main",
		ExtensionID: extID,
		Kind:        domain.ContributionKindGamePlugin,
		Name:        domain.LocalizedText{Default: "Game Plugin " + contribID},
		Description: domain.LocalizedText{Default: "A game plugin"},
	}
}

func makeToolContrib(extID domain.ExtensionID, contribID string) domain.ContributionDefinition {
	return domain.ContributionDefinition{
		ID:          domain.ContributionID(contribID),
		ModuleID:    "main",
		ExtensionID: extID,
		Kind:        domain.ContributionKindTool,
		Name:        domain.LocalizedText{Default: "Tool " + contribID},
		Description: domain.LocalizedText{Default: "A regular tool"},
	}
}

func TestList_OnlyReturnsDesktopPetPlugins(t *testing.T) {
	container := &fakeContainer{
		installations: []domain.ExtensionInstallation{
			{ExtensionID: "com.example/pet-a", InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}, InstallationState: domain.InstallationStateInstalled, EnablementState: domain.EnablementEnabled},
			{ExtensionID: "com.example/game-b", InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}, InstallationState: domain.InstallationStateInstalled, EnablementState: domain.EnablementEnabled},
			{ExtensionID: "com.example/tool-c", InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}, InstallationState: domain.InstallationStateInstalled, EnablementState: domain.EnablementEnabled},
		},
		contributions: []domain.ContributionDefinition{
			makePetPluginContrib("com.example/pet-a", "pet-overlay"),
			makeGamePluginContrib("com.example/game-b", "game-bridge"),
			makeToolContrib("com.example/tool-c", "useful-tool"),
		},
	}
	svc := NewDesktopPetPluginManagementService(container, &fakeRuntime{}, nil)
	resp, err := svc.List(context.Background(), 1, 20, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Plugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(resp.Plugins))
	}
	if resp.Total != 1 {
		t.Errorf("expected total=1, got %d", resp.Total)
	}
	if len(resp.Plugins) > 0 && resp.Plugins[0].PluginID != "pet-overlay" {
		t.Errorf("expected pet-overlay, got %s", resp.Plugins[0].PluginID)
	}
}

func TestList_MultiContributionInSingleExtension(t *testing.T) {
	container := &fakeContainer{
		installations: []domain.ExtensionInstallation{
			{ExtensionID: "com.example/pet-multi", InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}, InstallationState: domain.InstallationStateInstalled, EnablementState: domain.EnablementEnabled},
		},
		contributions: []domain.ContributionDefinition{
			makePetPluginContrib("com.example/pet-multi", "pet-overlay"),
			makePetPluginContrib("com.example/pet-multi", "pet-interaction"),
		},
	}
	svc := NewDesktopPetPluginManagementService(container, &fakeRuntime{}, nil)
	resp, err := svc.List(context.Background(), 1, 20, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Plugins) != 2 {
		t.Errorf("expected 2 plugins from single extension, got %d", len(resp.Plugins))
	}
	if resp.Total != 2 {
		t.Errorf("expected total=2, got %d", resp.Total)
	}
}

func TestList_PaginationBeforeFilter(t *testing.T) {
	insts := make([]domain.ExtensionInstallation, 0)
	contribs := make([]domain.ContributionDefinition, 0)
	for i := 0; i < 5; i++ {
		extID := domain.ExtensionID("com.example/pet-" + string(rune('a'+i)))
		insts = append(insts, domain.ExtensionInstallation{
			ExtensionID:       extID,
			InstalledVersion:  domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0},
			InstallationState: domain.InstallationStateInstalled,
			EnablementState:   domain.EnablementEnabled,
		})
		contribs = append(contribs, makePetPluginContrib(extID, "plugin-"+string(rune('a'+i))))
	}
	container := &fakeContainer{installations: insts, contributions: contribs}
	svc := NewDesktopPetPluginManagementService(container, &fakeRuntime{}, nil)
	resp, err := svc.List(context.Background(), 1, 3, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if resp.Total != 5 {
		t.Errorf("expected total=5, got %d", resp.Total)
	}
	if len(resp.Plugins) != 3 {
		t.Errorf("expected page size=3, got %d", len(resp.Plugins))
	}
	resp2, err := svc.List(context.Background(), 2, 3, "")
	if err != nil {
		t.Fatalf("List page2: %v", err)
	}
	if len(resp2.Plugins) != 2 {
		t.Errorf("expected page2=2 items, got %d", len(resp2.Plugins))
	}
}

func TestList_SearchFiltersByPluginName(t *testing.T) {
	container := &fakeContainer{
		installations: []domain.ExtensionInstallation{
			{ExtensionID: "com.example/pet-search", InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}, InstallationState: domain.InstallationStateInstalled},
		},
		contributions: []domain.ContributionDefinition{
			{
				ID: petID("alpha-pet"), ModuleID: "main", ExtensionID: "com.example/pet-search",
				Kind: domain.ContributionKindDesktopPetPlugin,
				Name: domain.LocalizedText{Default: "Alpha Pet"},
			},
			{
				ID: petID("beta-pet"), ModuleID: "main", ExtensionID: "com.example/pet-search",
				Kind: domain.ContributionKindDesktopPetPlugin,
				Name: domain.LocalizedText{Default: "Beta Pet"},
			},
		},
	}
	svc := NewDesktopPetPluginManagementService(container, &fakeRuntime{}, nil)
	resp, err := svc.List(context.Background(), 1, 20, "Alpha")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Plugins) != 1 {
		t.Errorf("expected 1 plugin matching Alpha, got %d", len(resp.Plugins))
	}
	if len(resp.Plugins) > 0 && resp.Plugins[0].PluginID != "alpha-pet" {
		t.Errorf("expected alpha-pet, got %s", resp.Plugins[0].PluginID)
	}
}

func TestList_EmptyResultReturnsEmptySlice(t *testing.T) {
	container := &fakeContainer{
		installations: []domain.ExtensionInstallation{
			{ExtensionID: "com.example/tool-only", InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}, InstallationState: domain.InstallationStateInstalled},
		},
		contributions: []domain.ContributionDefinition{
			makeToolContrib("com.example/tool-only", "some-tool"),
		},
	}
	svc := NewDesktopPetPluginManagementService(container, &fakeRuntime{}, nil)
	resp, err := svc.List(context.Background(), 1, 20, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(resp.Plugins))
	}
	if resp.Total != 0 {
		t.Errorf("expected total=0, got %d", resp.Total)
	}
}

func TestList_KernelUnavailable(t *testing.T) {
	svc := NewDesktopPetPluginManagementService(nil, &fakeRuntime{}, nil)
	_, err := svc.List(context.Background(), 1, 20, "")
	if err != ErrKernelUnavailable {
		t.Errorf("expected ErrKernelUnavailable, got %v", err)
	}
}

func TestGet_NotFoundForNonPetPlugin(t *testing.T) {
	container := &fakeContainer{
		installations: []domain.ExtensionInstallation{
			{ExtensionID: "com.example/game-only", InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}, InstallationState: domain.InstallationStateInstalled},
		},
		contributions: []domain.ContributionDefinition{
			makeGamePluginContrib("com.example/game-only", "game-contrib"),
		},
	}
	svc := NewDesktopPetPluginManagementService(container, &fakeRuntime{}, nil)
	_, err := svc.Get(context.Background(), "game-contrib")
	if err != ErrExtensionNotFound {
		t.Errorf("expected ErrExtensionNotFound, got %v", err)
	}
}

func TestGet_ReturnsPetPluginDetail(t *testing.T) {
	container := &fakeContainer{
		installations: []domain.ExtensionInstallation{
			{ExtensionID: "com.example/pet-detail", InstalledVersion: domain.SemanticVersion{Major: 2, Minor: 1, Patch: 0}, InstallationState: domain.InstallationStateInstalled, EnablementState: domain.EnablementDisabled},
		},
		contributions: []domain.ContributionDefinition{
			makePetPluginContrib("com.example/pet-detail", "overlay-1"),
		},
		grants: []sqlite.PermissionGrant{
			{ExtensionID: "com.example/pet-detail", PermissionName: "display.window", State: "granted"},
		},
	}
	svc := NewDesktopPetPluginManagementService(container, &fakeRuntime{}, nil)
	detail, err := svc.Get(context.Background(), "overlay-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if detail.PluginID != "overlay-1" {
		t.Errorf("expected pluginId=overlay-1, got %s", detail.PluginID)
	}
	if detail.ExtensionID != "com.example/pet-detail" {
		t.Errorf("expected extensionId=com.example/pet-detail, got %s", detail.ExtensionID)
	}
	if detail.Enabled {
		t.Errorf("expected enabled=false for disabled install")
	}
	if detail.ManagementTarget != "desktop_pet_center" {
		t.Errorf("expected managementTarget=desktop_pet_center, got %s", detail.ManagementTarget)
	}
	if detail.PermissionSummary == nil {
		t.Fatal("expected permissionSummary to be non-nil")
	}
	if len(detail.PermissionSummary.Declared) != 2 {
		t.Errorf("expected 2 declared permissions, got %d", len(detail.PermissionSummary.Declared))
	}
	if len(detail.PermissionSummary.Granted) != 1 {
		t.Errorf("expected 1 granted permission, got %d", len(detail.PermissionSummary.Granted))
	}
}

func TestGet_EmptyPluginIDReturnsInvalidInput(t *testing.T) {
	container := &fakeContainer{}
	svc := NewDesktopPetPluginManagementService(container, &fakeRuntime{}, nil)
	_, err := svc.Get(context.Background(), "")
	if err != ErrInvalidInput {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestInstall_CallsKernel(t *testing.T) {
	installed := false
	rt := &fakeRuntime{
		installFn: func(ctx context.Context, archivePath string) (kernelInstalledExtension, error) {
			installed = true
			return kernelInstalledExtension{ID: "com.example/new-pet", Name: "New Pet", Version: "1.0.0"}, nil
		},
	}
	svc := NewDesktopPetPluginManagementService(&fakeContainer{}, rt, nil)
	result, err := svc.Install(context.Background(), "/tmp/pkg.zip")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !installed {
		t.Error("expected kernel Install to be called")
	}
	if result.ExtensionID != "com.example/new-pet" {
		t.Errorf("expected extensionId, got %s", result.ExtensionID)
	}
	if result.Version != "1.0.0" {
		t.Errorf("expected version=1.0.0, got %s", result.Version)
	}
}

func TestInstall_EmptyPathReturnsInvalidInput(t *testing.T) {
	svc := NewDesktopPetPluginManagementService(&fakeContainer{}, &fakeRuntime{}, nil)
	_, err := svc.Install(context.Background(), "")
	if err != ErrInvalidInput {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestEnable_RejectsNonPetExtension(t *testing.T) {
	container := &fakeContainer{
		installations: []domain.ExtensionInstallation{
			{ExtensionID: "com.example/game-only", InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}},
		},
		contributions: []domain.ContributionDefinition{
			makeGamePluginContrib("com.example/game-only", "game-contrib"),
		},
	}
	svc := NewDesktopPetPluginManagementService(container, &fakeRuntime{}, nil)
	_, err := svc.Enable(context.Background(), "com.example/game-only")
	if err != ErrNotDesktopPetPlugin {
		t.Errorf("expected ErrNotDesktopPetPlugin, got %v", err)
	}
}

func TestEnable_PetExtensionSucceeds(t *testing.T) {
	enabled := false
	rt := &fakeRuntime{
		enableFn: func(ctx context.Context, extensionID string) error {
			enabled = true
			return nil
		},
	}
	container := &fakeContainer{
		installations: []domain.ExtensionInstallation{
			{ExtensionID: "com.example/pet-enable", InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}},
		},
		contributions: []domain.ContributionDefinition{
			makePetPluginContrib("com.example/pet-enable", "pet-1"),
		},
	}
	svc := NewDesktopPetPluginManagementService(container, rt, nil)
	result, err := svc.Enable(context.Background(), "com.example/pet-enable")
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !enabled {
		t.Error("expected kernel Enable to be called")
	}
	if !result.Success {
		t.Error("expected success=true")
	}
}

func TestDisable_PetExtensionSucceeds(t *testing.T) {
	disabled := false
	rt := &fakeRuntime{
		disableFn: func(ctx context.Context, extensionID string) error {
			disabled = true
			return nil
		},
	}
	container := &fakeContainer{
		installations: []domain.ExtensionInstallation{
			{ExtensionID: "com.example/pet-disable", InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}},
		},
		contributions: []domain.ContributionDefinition{
			makePetPluginContrib("com.example/pet-disable", "pet-1"),
		},
	}
	svc := NewDesktopPetPluginManagementService(container, rt, nil)
	_, err := svc.Disable(context.Background(), "com.example/pet-disable")
	if err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if !disabled {
		t.Error("expected kernel Disable to be called")
	}
}

func TestUninstall_RejectsNonPetExtension(t *testing.T) {
	container := &fakeContainer{
		installations: []domain.ExtensionInstallation{
			{ExtensionID: "com.example/game-only", InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}},
		},
		contributions: []domain.ContributionDefinition{
			makeGamePluginContrib("com.example/game-only", "game-contrib"),
		},
	}
	svc := NewDesktopPetPluginManagementService(container, &fakeRuntime{}, nil)
	_, err := svc.Uninstall(context.Background(), "com.example/game-only")
	if err != ErrNotDesktopPetPlugin {
		t.Errorf("expected ErrNotDesktopPetPlugin, got %v", err)
	}
}

func TestUninstall_PetExtensionSucceeds(t *testing.T) {
	uninstalled := false
	rt := &fakeRuntime{
		uninstallFn: func(ctx context.Context, extensionID string) error {
			uninstalled = true
			return nil
		},
	}
	container := &fakeContainer{
		installations: []domain.ExtensionInstallation{
			{ExtensionID: "com.example/pet-uninstall", InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}},
		},
		contributions: []domain.ContributionDefinition{
			makePetPluginContrib("com.example/pet-uninstall", "pet-1"),
		},
	}
	svc := NewDesktopPetPluginManagementService(container, rt, nil)
	_, err := svc.Uninstall(context.Background(), "com.example/pet-uninstall")
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if !uninstalled {
		t.Error("expected kernel Uninstall to be called")
	}
}

func TestList_DoesNotExposeSecrets(t *testing.T) {
	container := &fakeContainer{
		installations: []domain.ExtensionInstallation{
			{ExtensionID: "com.example/pet-safe", InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}, InstallationState: domain.InstallationStateInstalled},
		},
		contributions: []domain.ContributionDefinition{
			makePetPluginContrib("com.example/pet-safe", "pet-safe"),
		},
	}
	svc := NewDesktopPetPluginManagementService(container, &fakeRuntime{}, nil)
	resp, err := svc.List(context.Background(), 1, 20, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(resp.Plugins))
	}
	p := resp.Plugins[0]
	if p.ExtensionID == "" {
		t.Error("expected extensionId to be non-empty")
	}
	summary := resp.Plugins[0]
	if summary.InstallState == "" {
		t.Error("expected installState to be set")
	}
}

func TestList_CrossCenterIsolation(t *testing.T) {
	container := &fakeContainer{
		installations: []domain.ExtensionInstallation{
			{ExtensionID: "com.example/pet-x", InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}, InstallationState: domain.InstallationStateInstalled},
			{ExtensionID: "com.example/game-y", InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}, InstallationState: domain.InstallationStateInstalled},
			{ExtensionID: "com.example/general-z", InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}, InstallationState: domain.InstallationStateInstalled},
		},
		contributions: []domain.ContributionDefinition{
			makePetPluginContrib("com.example/pet-x", "pet-contrib"),
			makeGamePluginContrib("com.example/game-y", "game-contrib"),
			makeToolContrib("com.example/general-z", "tool-contrib"),
		},
	}
	svc := NewDesktopPetPluginManagementService(container, &fakeRuntime{}, nil)
	resp, err := svc.List(context.Background(), 1, 20, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("expected total=1 (pet only), got %d", resp.Total)
	}
	for _, p := range resp.Plugins {
		if p.ManagementTarget != "desktop_pet_center" {
			t.Errorf("expected all plugins to have desktop_pet_center target, got %s", p.ManagementTarget)
		}
	}
}

func TestInstall_ErrorPropagation(t *testing.T) {
	rt := &fakeRuntime{
		installFn: func(ctx context.Context, archivePath string) (kernelInstalledExtension, error) {
			return kernelInstalledExtension{}, errors.New("manifest validation failed")
		},
	}
	svc := NewDesktopPetPluginManagementService(&fakeContainer{}, rt, nil)
	_, err := svc.Install(context.Background(), "/tmp/bad.zip")
	if err == nil {
		t.Error("expected error from kernel Install")
	}
}

func TestEnable_KernelErrorPropagation(t *testing.T) {
	rt := &fakeRuntime{
		enableFn: func(ctx context.Context, extensionID string) error {
			return errors.New("kernel enable failed")
		},
	}
	container := &fakeContainer{
		installations: []domain.ExtensionInstallation{
			{ExtensionID: "com.example/pet-err", InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}},
		},
		contributions: []domain.ContributionDefinition{
			makePetPluginContrib("com.example/pet-err", "pet-err"),
		},
	}
	svc := NewDesktopPetPluginManagementService(container, rt, nil)
	_, err := svc.Enable(context.Background(), "com.example/pet-err")
	if err == nil {
		t.Error("expected error from kernel Enable")
	}
}

func TestUpdate_CallsKernel(t *testing.T) {
	updated := false
	rt := &fakeRuntime{
		updateFn: func(ctx context.Context, archivePath string) (kernelInstalledExtension, error) {
			updated = true
			return kernelInstalledExtension{ID: "com.example/pet-upd", Name: "Pet Updated", Version: "2.0.0"}, nil
		},
	}
	svc := NewDesktopPetPluginManagementService(&fakeContainer{}, rt, nil)
	result, err := svc.Update(context.Background(), "/tmp/pkg-v2.zip")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !updated {
		t.Error("expected kernel Update to be called")
	}
	if result.ExtensionID != "com.example/pet-upd" {
		t.Errorf("expected extensionId=com.example/pet-upd, got %s", result.ExtensionID)
	}
	if result.Version != "2.0.0" {
		t.Errorf("expected version=2.0.0, got %s", result.Version)
	}
	if result.InstallState != string(PluginInstallStateInstalled) {
		t.Errorf("expected installState=installed, got %s", result.InstallState)
	}
}

func TestUpdate_EmptyPathReturnsInvalidInput(t *testing.T) {
	svc := NewDesktopPetPluginManagementService(&fakeContainer{}, &fakeRuntime{}, nil)
	_, err := svc.Update(context.Background(), "")
	if err != ErrInvalidInput {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestUpdate_ErrorPropagation(t *testing.T) {
	rt := &fakeRuntime{
		updateFn: func(ctx context.Context, archivePath string) (kernelInstalledExtension, error) {
			return kernelInstalledExtension{}, errors.New("update failed: version conflict")
		},
	}
	svc := NewDesktopPetPluginManagementService(&fakeContainer{}, rt, nil)
	_, err := svc.Update(context.Background(), "/tmp/bad.zip")
	if err == nil {
		t.Error("expected error from kernel Update")
	}
}

func TestUpdate_KernelUnavailable(t *testing.T) {
	svc := NewDesktopPetPluginManagementService(&fakeContainer{}, nil, nil)
	_, err := svc.Update(context.Background(), "/tmp/pkg.zip")
	if err != ErrKernelUnavailable {
		t.Errorf("expected ErrKernelUnavailable, got %v", err)
	}
}

func petID(id string) domain.ContributionID {
	return domain.ContributionID(id)
}

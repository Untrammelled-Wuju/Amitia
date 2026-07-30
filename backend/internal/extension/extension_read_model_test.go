package extension

import (
	"context"
	"fmt"
	"testing"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/dependency"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

var dbFaultErr = fmt.Errorf("simulated database connection lost")

type mockInstallationRepo struct {
	inst domain.ExtensionInstallation
	err  error
}

func (r *mockInstallationRepo) PutInstallation(_ context.Context, _ domain.ExtensionInstallation) error {
	return nil
}
func (r *mockInstallationRepo) GetInstallation(_ context.Context, id domain.ExtensionID) (domain.ExtensionInstallation, error) {
	if r.err != nil {
		return domain.ExtensionInstallation{}, r.err
	}
	if r.inst.ExtensionID != "" && r.inst.ExtensionID != id {
		return domain.ExtensionInstallation{}, domain.ErrInvalidExtensionID
	}
	return r.inst, nil
}
func (r *mockInstallationRepo) ListInstallations(_ context.Context) ([]domain.ExtensionInstallation, error) {
	return nil, nil
}
func (r *mockInstallationRepo) DeleteInstallation(_ context.Context, _ domain.ExtensionID) error {
	return nil
}

type mockDefinitionRepo struct {
	defs []domain.ExtensionDefinition
	err  error
}

func (r *mockDefinitionRepo) PutExtension(_ context.Context, _ domain.ExtensionDefinition) error {
	return nil
}
func (r *mockDefinitionRepo) GetExtension(_ context.Context, _ domain.ExtensionID, _ domain.SemanticVersion) (domain.ExtensionDefinition, error) {
	return domain.ExtensionDefinition{}, r.err
}
func (r *mockDefinitionRepo) ListExtensions(_ context.Context) ([]domain.ExtensionDefinition, error) {
	return r.defs, r.err
}
func (r *mockDefinitionRepo) DeleteExtension(_ context.Context, _ domain.ExtensionID, _ domain.SemanticVersion) error {
	return nil
}

type mockDependencyResolver struct {
	subjects []dependency.AffectedSubject
	err      error
}

func (r *mockDependencyResolver) Resolve(_ context.Context, _ dependency.ResolveRequest) dependency.ResolveResult {
	return dependency.ResolveResult{}
}
func (r *mockDependencyResolver) BuildGraph(_ context.Context, _ []string) dependency.Graph {
	return dependency.Graph{}
}
func (r *mockDependencyResolver) Snapshot(_ context.Context, _ string) (dependency.Snapshot, error) {
	return dependency.Snapshot{}, nil
}
func (r *mockDependencyResolver) AffectedBy(_ context.Context, _ string) ([]dependency.AffectedSubject, error) {
	return r.subjects, r.err
}

type mockModuleRepo struct {
	modules []domain.ModuleDefinition
	err     error
}

func (r *mockModuleRepo) PutModule(_ context.Context, _ domain.ModuleDefinition) error { return nil }
func (r *mockModuleRepo) GetModule(_ context.Context, _ domain.ExtensionID, _ domain.ModuleID) (domain.ModuleDefinition, error) {
	return domain.ModuleDefinition{}, r.err
}
func (r *mockModuleRepo) ListModules(_ context.Context, _ domain.ExtensionID) ([]domain.ModuleDefinition, error) {
	return r.modules, r.err
}
func (r *mockModuleRepo) DeleteModules(_ context.Context, _ domain.ExtensionID) error { return nil }
func (r *mockModuleRepo) DeleteModule(_ context.Context, _ domain.ExtensionID, _ domain.ModuleID) error {
	return nil
}

type mockContributionRepo struct {
	contribs []domain.ContributionDefinition
	err      error
}

func (r *mockContributionRepo) PutContribution(_ context.Context, _ domain.ContributionDefinition) error {
	return nil
}
func (r *mockContributionRepo) GetContribution(_ context.Context, _ domain.ExtensionID, _ domain.ContributionID) (domain.ContributionDefinition, error) {
	return domain.ContributionDefinition{}, r.err
}
func (r *mockContributionRepo) ListContributions(_ context.Context, _ domain.ExtensionID) ([]domain.ContributionDefinition, error) {
	return r.contribs, r.err
}
func (r *mockContributionRepo) ListContributionsByModule(_ context.Context, _ domain.ExtensionID, _ domain.ModuleID) ([]domain.ContributionDefinition, error) {
	return nil, nil
}
func (r *mockContributionRepo) DeleteContributions(_ context.Context, _ domain.ExtensionID) error {
	return nil
}
func (r *mockContributionRepo) DeleteContributionsByModule(_ context.Context, _ domain.ExtensionID, _ domain.ModuleID) error {
	return nil
}

func buildTestReadModel(t *testing.T, container *kernelruntime.Container) *ExtensionReadModelService {
	t.Helper()
	runtime, err := kernelruntime.NewRuntime(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runtime.SetContainer(container)
	proxy := NewKernelLifecycleProxy(runtime)
	return NewExtensionReadModelService(proxy, nil)
}

func validTestInstallation() domain.ExtensionInstallation {
	return domain.ExtensionInstallation{
		ExtensionID:       domain.ExtensionID("dev.local.test/ext"),
		InstalledVersion:  domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0},
		InstallationState: domain.InstallationStateInstalled,
		EnablementState:   domain.EnablementEnabled,
	}
}

func TestReadModel_GetInstallation_RepositoryError_Propagates(t *testing.T) {
	container := &kernelruntime.Container{
		InstallationRepository: &mockInstallationRepo{err: dbFaultErr},
	}
	svc := buildTestReadModel(t, container)
	ctx := context.Background()

	t.Run("TryPreviewUninstall", func(t *testing.T) {
		_, ok, err := svc.TryPreviewUninstall(ctx, "dev.local.test/ext")
		if err == nil {
			t.Fatal("Repository Error must propagate as error, got nil")
		}
		if ok {
			t.Fatal("ok must be false when Repository Error occurs")
		}
	})

	t.Run("TryDependencies", func(t *testing.T) {
		_, ok, err := svc.TryDependencies(ctx, "dev.local.test/ext")
		if err == nil {
			t.Fatal("Repository Error must propagate as error, got nil")
		}
		if ok {
			t.Fatal("ok must be false when Repository Error occurs")
		}
	})

	t.Run("TryListVersions", func(t *testing.T) {
		_, ok, err := svc.TryListVersions(ctx, "dev.local.test/ext")
		if err == nil {
			t.Fatal("Repository Error must propagate as error, got nil")
		}
		if ok {
			t.Fatal("ok must be false when Repository Error occurs")
		}
	})

	t.Run("TryCompareVersions", func(t *testing.T) {
		_, ok, err := svc.TryCompareVersions(ctx, "dev.local.test/ext", "1.0.0", "1.1.0")
		if err == nil {
			t.Fatal("Repository Error must propagate as error, got nil")
		}
		if ok {
			t.Fatal("ok must be false when Repository Error occurs")
		}
	})

	t.Run("TryExport", func(t *testing.T) {
		_, ok, err := svc.TryExport(ctx, "dev.local.test/ext", "1.0.0")
		if err == nil {
			t.Fatal("Repository Error must propagate as error, got nil")
		}
		if ok {
			t.Fatal("ok must be false when Repository Error occurs")
		}
	})
}

func TestReadModel_GetInstallation_NotFound_LegacyFallback(t *testing.T) {
	container := &kernelruntime.Container{
		InstallationRepository: &mockInstallationRepo{err: domain.ErrInvalidExtensionID},
	}
	svc := buildTestReadModel(t, container)
	ctx := context.Background()

	t.Run("TryPreviewUninstall", func(t *testing.T) {
		_, ok, err := svc.TryPreviewUninstall(ctx, "dev.local.test/ext")
		if err != nil {
			t.Fatalf("Not Found must not return error, got: %v", err)
		}
		if ok {
			t.Fatal("ok must be false for Not Found (Legacy fallback)")
		}
	})

	t.Run("TryDependencies", func(t *testing.T) {
		_, ok, err := svc.TryDependencies(ctx, "dev.local.test/ext")
		if err != nil {
			t.Fatalf("Not Found must not return error, got: %v", err)
		}
		if ok {
			t.Fatal("ok must be false for Not Found (Legacy fallback)")
		}
	})

	t.Run("TryListVersions", func(t *testing.T) {
		_, ok, err := svc.TryListVersions(ctx, "dev.local.test/ext")
		if err != nil {
			t.Fatalf("Not Found must not return error, got: %v", err)
		}
		if ok {
			t.Fatal("ok must be false for Not Found (Legacy fallback)")
		}
	})

	t.Run("TryCompareVersions", func(t *testing.T) {
		_, ok, err := svc.TryCompareVersions(ctx, "dev.local.test/ext", "1.0.0", "1.1.0")
		if err != nil {
			t.Fatalf("Not Found must not return error, got: %v", err)
		}
		if ok {
			t.Fatal("ok must be false for Not Found (Legacy fallback)")
		}
	})

	t.Run("TryExport", func(t *testing.T) {
		_, ok, err := svc.TryExport(ctx, "dev.local.test/ext", "1.0.0")
		if err != nil {
			t.Fatalf("Not Found must not return error, got: %v", err)
		}
		if ok {
			t.Fatal("ok must be false for Not Found (Legacy fallback)")
		}
	})
}

func TestReadModel_Helper_RepositoryError_Propagates(t *testing.T) {
	ctx := context.Background()

	t.Run("DependencyResolver_fault_via_TryPreviewUninstall", func(t *testing.T) {
		container := &kernelruntime.Container{
			InstallationRepository: &mockInstallationRepo{inst: validTestInstallation()},
			DependencyResolver:     &mockDependencyResolver{err: dbFaultErr},
		}
		svc := buildTestReadModel(t, container)
		_, ok, err := svc.TryPreviewUninstall(ctx, "dev.local.test/ext")
		if err == nil {
			t.Fatal("DependencyResolver fault must propagate as error")
		}
		if ok {
			t.Fatal("ok must be false when helper Repository Error occurs")
		}
	})

	t.Run("DependencyResolver_fault_via_TryDependencies", func(t *testing.T) {
		container := &kernelruntime.Container{
			InstallationRepository: &mockInstallationRepo{inst: validTestInstallation()},
			DependencyResolver:     &mockDependencyResolver{err: dbFaultErr},
		}
		svc := buildTestReadModel(t, container)
		_, ok, err := svc.TryDependencies(ctx, "dev.local.test/ext")
		if err == nil {
			t.Fatal("DependencyResolver fault must propagate as error")
		}
		if ok {
			t.Fatal("ok must be false when helper Repository Error occurs")
		}
	})

	t.Run("ModuleRepository_fault_via_TryDependencies", func(t *testing.T) {
		container := &kernelruntime.Container{
			InstallationRepository: &mockInstallationRepo{inst: validTestInstallation()},
			ModuleRepository:       &mockModuleRepo{err: dbFaultErr},
		}
		svc := buildTestReadModel(t, container)
		_, ok, err := svc.TryDependencies(ctx, "dev.local.test/ext")
		if err == nil {
			t.Fatal("ModuleRepository fault must propagate as error")
		}
		if ok {
			t.Fatal("ok must be false when helper Repository Error occurs")
		}
	})

	t.Run("ContributionRepository_fault_via_TryPreviewUninstall", func(t *testing.T) {
		container := &kernelruntime.Container{
			InstallationRepository: &mockInstallationRepo{inst: validTestInstallation()},
			ContributionRepository: &mockContributionRepo{err: dbFaultErr},
		}
		svc := buildTestReadModel(t, container)
		_, ok, err := svc.TryPreviewUninstall(ctx, "dev.local.test/ext")
		if err == nil {
			t.Fatal("ContributionRepository fault must propagate as error")
		}
		if ok {
			t.Fatal("ok must be false when helper Repository Error occurs")
		}
	})

	t.Run("DefinitionRepository_fault_via_TryListVersions", func(t *testing.T) {
		container := &kernelruntime.Container{
			InstallationRepository: &mockInstallationRepo{inst: validTestInstallation()},
			DefinitionRepository:   &mockDefinitionRepo{err: dbFaultErr},
		}
		svc := buildTestReadModel(t, container)
		_, ok, err := svc.TryListVersions(ctx, "dev.local.test/ext")
		if err == nil {
			t.Fatal("DefinitionRepository fault must propagate as error")
		}
		if ok {
			t.Fatal("ok must be false when DefinitionRepository error occurs")
		}
	})

	t.Run("DefinitionRepository_fault_via_TryCompareVersions", func(t *testing.T) {
		container := &kernelruntime.Container{
			InstallationRepository: &mockInstallationRepo{inst: validTestInstallation()},
			DefinitionRepository:   &mockDefinitionRepo{err: dbFaultErr},
		}
		svc := buildTestReadModel(t, container)
		_, ok, err := svc.TryCompareVersions(ctx, "dev.local.test/ext", "1.0.0", "1.1.0")
		if err == nil {
			t.Fatal("DefinitionRepository fault must propagate as error")
		}
		if ok {
			t.Fatal("ok must be false when DefinitionRepository error occurs")
		}
	})
}

func TestReadModel_LegacyReadCounter_NotIncremented_OnRepositoryError(t *testing.T) {
	container := &kernelruntime.Container{
		InstallationRepository: &mockInstallationRepo{err: dbFaultErr},
	}
	svc := buildTestReadModel(t, container)
	ctx := context.Background()

	before := kernelruntime.GlobalLegacyReadCounter().PackageReadCallsFallbacks()

	_, _, _ = svc.TryPreviewUninstall(ctx, "dev.local.test/ext")
	_, _, _ = svc.TryDependencies(ctx, "dev.local.test/ext")
	_, _, _ = svc.TryListVersions(ctx, "dev.local.test/ext")
	_, _, _ = svc.TryCompareVersions(ctx, "dev.local.test/ext", "1.0.0", "1.1.0")
	_, _, _ = svc.TryExport(ctx, "dev.local.test/ext", "1.0.0")

	after := kernelruntime.GlobalLegacyReadCounter().PackageReadCallsFallbacks()
	if after != before {
		t.Fatalf("legacy_package_read_calls must NOT increment on Repository Error: before=%d after=%d", before, after)
	}
}

func TestReadModel_Helper_GetInstallation_DistinguishesNotFound(t *testing.T) {
	ctx := context.Background()

	t.Run("readReverseDependencies_treats_dependent_not_found_as_not_installed", func(t *testing.T) {
		container := &kernelruntime.Container{
			InstallationRepository: &mockInstallationRepo{inst: validTestInstallation()},
			DependencyResolver: &mockDependencyResolver{
				subjects: []dependency.AffectedSubject{
					{SubjectID: "dev.local.other/ext", Required: true},
				},
			},
		}
		svc := buildTestReadModel(t, container)
		dependents, err := svc.readReverseDependencies(ctx, container, "dev.local.test/ext")
		if err != nil {
			t.Fatalf("dependent Not Found must not cause error, got: %v", err)
		}
		if len(dependents) != 1 {
			t.Fatalf("expected 1 dependent, got %d", len(dependents))
		}
		if dependents[0].Installed {
			t.Fatal("dependent that is Not Found must show Installed=false")
		}
	})
}

func TestReadModel_FaultInjection_AcceptanceCriteria(t *testing.T) {
	ctx := context.Background()

	t.Run("database_failure_returns_error_not_empty_data", func(t *testing.T) {
		container := &kernelruntime.Container{
			InstallationRepository: &mockInstallationRepo{err: dbFaultErr},
		}
		svc := buildTestReadModel(t, container)
		_, ok, err := svc.TryListVersions(ctx, "dev.local.test/ext")
		if err == nil {
			t.Fatal("acceptance: database failure must return error")
		}
		if ok {
			t.Fatal("acceptance: database failure must not return ok=true (no fake success)")
		}
	})

	t.Run("database_failure_does_not_enter_legacy_read", func(t *testing.T) {
		container := &kernelruntime.Container{
			InstallationRepository: &mockInstallationRepo{err: dbFaultErr},
		}
		svc := buildTestReadModel(t, container)
		before := kernelruntime.GlobalLegacyReadCounter().PackageReadCallsFallbacks()
		_, ok, _ := svc.TryPreviewUninstall(ctx, "dev.local.test/ext")
		after := kernelruntime.GlobalLegacyReadCounter().PackageReadCallsFallbacks()
		if ok {
			t.Fatal("acceptance: must not enter Legacy Read on Repository Error")
		}
		if after != before {
			t.Fatalf("acceptance: legacy_package_read_calls must not increment on Repository Error: before=%d after=%d", before, after)
		}
	})

	t.Run("historical_unmigrated_extension_still_compatible", func(t *testing.T) {
		container := &kernelruntime.Container{
			InstallationRepository: &mockInstallationRepo{err: domain.ErrInvalidExtensionID},
		}
		svc := buildTestReadModel(t, container)
		_, ok, err := svc.TryListVersions(ctx, "dev.local.test/ext")
		if err != nil {
			t.Fatalf("acceptance: unmigrated extension must not return error, got: %v", err)
		}
		if ok {
			t.Fatal("acceptance: unmigrated extension must return ok=false to allow Legacy Read")
		}
	})
}

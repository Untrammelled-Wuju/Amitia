//go:build legacy_migration

package extension

import (
	"context"
	"errors"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"gorm.io/gorm"
)

var errLegacyImplMissing = errors.New("legacy migration implementation not registered")

var (
	LegacyInstallPackageImpl           func(s *PackageService, ctx context.Context, request InstallPackageRequest) (PackageOperationResult, error)
	LegacyRollbackPackageImpl          func(s *PackageService, ctx context.Context, extensionID, version, userID, scopeType, scopeID string) (PackageOperationResult, error)
	LegacyUninstallPackageImpl         func(s *PackageService, ctx context.Context, extensionID, userID, scopeType, scopeID string) (PackageOperationResult, error)
	LegacyRecoverOperationsImpl        func(s *PackageService, ctx context.Context) error
	LegacyExportAmitiaxFilesImpl       func(s *PackageService, artifact packageArtifactRecord) (map[string][]byte, error)
	LegacyExportAgentSkillsFilesImpl   func(s *PackageService, artifact packageArtifactRecord, name string) (map[string][]byte, error)
	LegacyDependenciesImpl             func(s *PackageService, ctx context.Context, extensionID, userID, scopeType, scopeID string) (map[string]interface{}, error)
	LegacyPreviewUninstallImpl         func(s *PackageService, ctx context.Context, extensionID, userID, scopeType, scopeID string) (PackageUninstallPreview, error)
	LegacyMigrateLegacyPackageDataImpl func(s *PackageService, ctx context.Context) error
	LegacyMigrateLegacyPackagesImpl    func(s *PackageService, ctx context.Context) error
	LegacyDetectLegacyPackagesImpl     func(s *PackageService, ctx context.Context) (LegacyMigrationReport, error)
)

func (s *PackageService) AttachKernelProxy(proxy *KernelLifecycleProxy) error {
	if proxy == nil {
		return nil
	}
	s.kernelProxy = proxy
	return nil
}

func (s *PackageService) LegacyRepo() *Repository { return s.repository }

func (s *PackageService) LegacyRegistry() *Registry { return s.registry }

func (s *PackageService) LegacyValidator() *SchemaValidator { return s.validator }

func (s *PackageService) LegacyCompiler() *WorkflowCompiler { return s.compiler }

func (s *PackageService) LegacyWorkflowInstaller() *WorkshopInstaller { return s.workflowInstaller }

func (s *PackageService) LegacyAgentSkills() *AgentSkillService { return s.agentSkills }

func (s *PackageService) LegacyLimits() PackageLimits { return s.limits }

func (s *PackageService) LegacyKernel() *kernelruntime.Runtime { return s.kernel }

func (s *PackageService) LegacyDB() *gorm.DB {
	if s.repository == nil {
		return nil
	}
	return s.repository.db
}

func (s *PackageService) LegacyMetric(name string) { s.metric(name) }

func (s *PackageService) LegacyTruncateLegacyMigrationFailure(value string) string {
	return truncateLegacyMigrationFailure(value)
}

func (s *PackageService) LegacyLockExtension(extensionID string) (func(), bool) {
	return s.lockExtension(extensionID)
}

func (s *PackageService) LegacyRecreateService() *PackageService {
	return NewPackageService(s.repository, s.registry, s.validator, s.compiler, s.workflowInstaller, s.agentSkills)
}

func (s *PackageService) LegacyConfigCipher() *configCipher {
	if s.repository == nil {
		return nil
	}
	return s.repository.configCipher
}

func legacyInstallPackageBridge(s *PackageService, ctx context.Context, request InstallPackageRequest) (PackageOperationResult, error) {
	if LegacyInstallPackageImpl != nil {
		return LegacyInstallPackageImpl(s, ctx, request)
	}
	return PackageOperationResult{}, errLegacyImplMissing
}

func legacyRollbackPackageBridge(s *PackageService, ctx context.Context, extensionID, version, userID, scopeType, scopeID string) (PackageOperationResult, error) {
	if LegacyRollbackPackageImpl != nil {
		return LegacyRollbackPackageImpl(s, ctx, extensionID, version, userID, scopeType, scopeID)
	}
	return PackageOperationResult{}, errLegacyImplMissing
}

func legacyUninstallPackageBridge(s *PackageService, ctx context.Context, extensionID, userID, scopeType, scopeID string) (PackageOperationResult, error) {
	if LegacyUninstallPackageImpl != nil {
		return LegacyUninstallPackageImpl(s, ctx, extensionID, userID, scopeType, scopeID)
	}
	return PackageOperationResult{}, errLegacyImplMissing
}

func legacyRecoverOperationsBridge(s *PackageService, ctx context.Context) error {
	if LegacyRecoverOperationsImpl != nil {
		return LegacyRecoverOperationsImpl(s, ctx)
	}
	return errLegacyImplMissing
}

func legacyExportAmitiaxFilesBridge(s *PackageService, artifact packageArtifactRecord) (map[string][]byte, error) {
	if LegacyExportAmitiaxFilesImpl != nil {
		return LegacyExportAmitiaxFilesImpl(s, artifact)
	}
	return nil, errLegacyImplMissing
}

func legacyExportAgentSkillsFilesBridge(s *PackageService, artifact packageArtifactRecord, name string) (map[string][]byte, error) {
	if LegacyExportAgentSkillsFilesImpl != nil {
		return LegacyExportAgentSkillsFilesImpl(s, artifact, name)
	}
	return nil, errLegacyImplMissing
}

func legacyDependenciesBridge(s *PackageService, ctx context.Context, extensionID, userID, scopeType, scopeID string) (map[string]interface{}, error) {
	if LegacyDependenciesImpl != nil {
		return LegacyDependenciesImpl(s, ctx, extensionID, userID, scopeType, scopeID)
	}
	return nil, errLegacyImplMissing
}

func legacyPreviewUninstallBridge(s *PackageService, ctx context.Context, extensionID, userID, scopeType, scopeID string) (PackageUninstallPreview, error) {
	if LegacyPreviewUninstallImpl != nil {
		return LegacyPreviewUninstallImpl(s, ctx, extensionID, userID, scopeType, scopeID)
	}
	return PackageUninstallPreview{}, errLegacyImplMissing
}

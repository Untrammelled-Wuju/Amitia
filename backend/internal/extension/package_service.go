package extension

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
)

type PackageService struct {
	repository        *Repository
	registry          *Registry
	validator         *SchemaValidator
	compiler          *WorkflowCompiler
	workflowInstaller *WorkshopInstaller
	agentSkills       *AgentSkillService
	limits            PackageLimits
	locks             sync.Map
	metrics           sync.Map
	kernel            *kernelruntime.Runtime
	kernelProxy       *KernelLifecycleProxy
	readModel         *ExtensionReadModelService
}

func NewPackageService(repository *Repository, registry *Registry, validator *SchemaValidator, compiler *WorkflowCompiler, workflowInstaller *WorkshopInstaller, agentSkills *AgentSkillService) *PackageService {
	service := &PackageService{repository: repository, registry: registry, validator: validator, compiler: compiler, workflowInstaller: workflowInstaller, agentSkills: agentSkills, limits: DefaultPackageLimits()}
	for _, name := range []string{"extension_package_import_total", "extension_package_import_failure_total", "extension_package_export_total", "extension_package_upgrade_total", "extension_package_upgrade_failure_total", "extension_package_rollback_total", "extension_package_uninstall_total", "extension_package_checksum_failure_total", "extension_package_signature_invalid_total", "extension_package_secret_detected_total", "extension_package_conflict_total", "extension_package_cleanup_failure_total", "package_preview_total", "package_preview_rejected_total", "package_install_total", "package_install_failed_total", "package_operation_requires_recovery", "package_signature_invalid_total", "package_signer_unknown_total", "package_unsigned_confirmed_total", "package_integrity_failed_total", "package_legacy_read_calls", "package_legacy_write_calls", "package_blob_bytes", "package_staging_orphans", "package_artifact_missing", "package_definition_file_mismatch", "legacy_data_detected", "legacy_migration_required", "legacy_write_attempts"} {
		service.metrics.Store(name, new(uint64))
	}
	return service
}

func (s *PackageService) AttachKernel(kernel *kernelruntime.Runtime) error {
	if kernel == nil {
		return nil
	}
	s.kernel = kernel
	s.readModel = NewExtensionReadModelService(kernel, s.repository)
	return kernel.RecoverPackageOperations(context.Background())
}

func (s *PackageService) metric(name string) {
	if value, ok := s.metrics.Load(name); ok {
		atomic.AddUint64(value.(*uint64), 1)
	}
}

func (s *PackageService) Metrics() map[string]uint64 {
	result := map[string]uint64{}
	s.metrics.Range(func(key, value interface{}) bool {
		result[key.(string)] = atomic.LoadUint64(value.(*uint64))
		return true
	})
	result["package_legacy_read_calls"] = uint64(kernelruntime.GlobalLegacyReadCounter().PackageReadCallsFallbacks())
	result["package_legacy_write_calls"] = uint64(kernelruntime.GlobalLegacyCallCounter().PackageWriteCalls())
	result["legacy_write_attempts"] = 0
	if s.repository != nil && s.repository.db != nil && s.repository.db.Migrator().HasTable("extension_package_legacy_migrations") {
		var legacyDetected int64
		s.repository.db.Raw(`SELECT COUNT(*) FROM extension_package_legacy_migrations WHERE migration_status NOT IN ('completed')`).Scan(&legacyDetected)
		result["legacy_data_detected"] = uint64(legacyDetected)
		var migrationRequired int64
		s.repository.db.Raw(`SELECT COUNT(*) FROM extension_package_legacy_migrations WHERE migration_status IN ('manual_required', 'pending_manual_migration')`).Scan(&migrationRequired)
		result["legacy_migration_required"] = uint64(migrationRequired)
	}
	if s.repository != nil && s.repository.db != nil && s.repository.db.Migrator().HasTable("extension_artifacts") {
		var blobBytes int64
		if s.repository.db.Raw(`SELECT COALESCE(SUM(LENGTH(content_blob)), 0) FROM extension_artifacts`).Scan(&blobBytes).Error == nil && blobBytes > 0 {
			result["package_blob_bytes"] = uint64(blobBytes)
		}
	}
	if s.kernel != nil && s.kernel.Container() != nil {
		report := &kernelruntime.FinalGateReport{Metrics: map[string]int64{}, Details: []kernelruntime.FinalGateIssue{}, Errors: []string{}}
		kernelruntime.NewFinalGateProbe(s.kernel.Container()).ProbePackageReleaseGate(context.Background(), report)
		if len(report.Errors) == 0 {
			result["package_operation_requires_recovery"] = uint64(report.Metrics["requires_recovery_operations"])
			result["package_staging_orphans"] = uint64(report.Metrics["orphan_staging_directories"])
			result["package_artifact_missing"] = uint64(report.Metrics["missing_artifact_rows"])
			result["package_definition_file_mismatch"] = uint64(report.Metrics["installation_without_files"] + report.Metrics["files_without_installation"])
		}
	}
	return result
}

func (s *PackageService) lockExtension(id string) (func(), bool) {
	_, loaded := s.locks.LoadOrStore(id, struct{}{})
	if loaded {
		return nil, false
	}
	return func() { s.locks.Delete(id) }, true
}

func sortedPackageCapabilities(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func (s *PackageService) MigrateLegacyPackageData(ctx context.Context) error {
	if err := s.repository.db.WithContext(ctx).Exec(`UPDATE extensions SET owner_user_id = (SELECT user_id FROM extension_agent_skill_metadata WHERE extension_agent_skill_metadata.extension_id = extensions.extension_id), scope_type = COALESCE((SELECT scope_type FROM extension_agent_skill_metadata WHERE extension_agent_skill_metadata.extension_id = extensions.extension_id), scope_type), scope_id = COALESCE((SELECT scope_id FROM extension_agent_skill_metadata WHERE extension_agent_skill_metadata.extension_id = extensions.extension_id), scope_id) WHERE source = 'instructions' AND owner_user_id = ''`).Error; err != nil {
		return err
	}
	if err := s.repository.db.WithContext(ctx).Exec(`UPDATE extensions SET owner_user_id = COALESCE((SELECT ws.user_id FROM extension_artifacts ea JOIN extension_workshop_sessions ws ON ws.id = ea.session_id WHERE ea.extension_id = extensions.extension_id AND ea.extension_version = extensions.current_version LIMIT 1), owner_user_id), scope_type = CASE WHEN COALESCE((SELECT ws.character_id FROM extension_artifacts ea JOIN extension_workshop_sessions ws ON ws.id = ea.session_id WHERE ea.extension_id = extensions.extension_id AND ea.extension_version = extensions.current_version LIMIT 1), '') = '' THEN 'global' ELSE 'character' END, scope_id = COALESCE((SELECT ws.character_id FROM extension_artifacts ea JOIN extension_workshop_sessions ws ON ws.id = ea.session_id WHERE ea.extension_id = extensions.extension_id AND ea.extension_version = extensions.current_version LIMIT 1), scope_id) WHERE source = 'workflow' AND owner_user_id = ''`).Error; err != nil {
		return err
	}
	if err := s.repository.db.WithContext(ctx).Exec(`UPDATE extension_versions SET artifact_id = COALESCE((SELECT artifact_id FROM extension_artifacts WHERE extension_artifacts.extension_id = extension_versions.extension_id AND extension_artifacts.extension_version = extension_versions.version LIMIT 1), artifact_id), artifact_hash = CASE WHEN artifact_hash = '' THEN checksum ELSE artifact_hash END, package_hash = CASE WHEN package_hash = '' THEN checksum ELSE package_hash END, source = CASE WHEN source = '' THEN COALESCE((SELECT source FROM extension_artifacts WHERE extension_artifacts.extension_id = extension_versions.extension_id AND extension_artifacts.extension_version = extension_versions.version LIMIT 1), '') ELSE source END, compatibility_status = CASE WHEN compatibility_status = '' THEN 'compatible' ELSE compatibility_status END, capabilities_json = CASE WHEN capabilities_json = '' THEN '[]' ELSE capabilities_json END, validation_status = CASE WHEN validation_status = '' THEN 'valid' ELSE validation_status END`).Error; err != nil {
		return err
	}
	return nil
}

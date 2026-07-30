package extension

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"gorm.io/gorm"
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
	kernelProxy       *KernelLifecycleProxy
	readModel         *ExtensionReadModelService
}

func NewPackageService(repository *Repository, registry *Registry, validator *SchemaValidator, compiler *WorkflowCompiler, workflowInstaller *WorkshopInstaller, agentSkills *AgentSkillService) *PackageService {
	service := &PackageService{repository: repository, registry: registry, validator: validator, compiler: compiler, workflowInstaller: workflowInstaller, agentSkills: agentSkills, limits: DefaultPackageLimits()}
	for _, name := range []string{"extension_package_import_total", "extension_package_import_failure_total", "extension_package_export_total", "extension_package_upgrade_total", "extension_package_upgrade_failure_total", "extension_package_rollback_total", "extension_package_uninstall_total", "extension_package_checksum_failure_total", "extension_package_signature_invalid_total", "extension_package_secret_detected_total", "extension_package_conflict_total", "extension_package_cleanup_failure_total", "package_preview_total", "package_preview_rejected_total", "package_install_total", "package_install_failed_total", "package_operation_requires_recovery", "package_signature_invalid_total", "package_signer_unknown_total", "package_unsigned_confirmed_total", "package_integrity_failed_total", "package_legacy_read_calls", "package_legacy_write_calls", "package_blob_bytes", "package_staging_orphans", "package_artifact_missing", "package_definition_file_mismatch"} {
		service.metrics.Store(name, new(uint64))
	}
	return service
}

func (s *PackageService) AttachKernelProxy(proxy *KernelLifecycleProxy) error {
	s.kernelProxy = proxy
	if proxy != nil {
		s.readModel = NewExtensionReadModelService(proxy, s.repository)
		return proxy.kernel.RecoverPackageOperations(context.Background())
	}
	return nil
}

func (s *PackageService) Restore(ctx context.Context) error {
	if !s.repository.db.Migrator().HasTable("extension_package_import_sessions") {
		return nil
	}
	if err := s.repository.CleanupPackageSessions(ctx); err != nil {
		s.metric("extension_package_cleanup_failure_total")
		return err
	}
	s.repository.RetryOwnedResourceCleanup(ctx)
	if err := s.repository.db.WithContext(ctx).Exec(`UPDATE extensions SET owner_user_id = (SELECT user_id FROM extension_agent_skill_metadata WHERE extension_agent_skill_metadata.extension_id = extensions.extension_id), scope_type = COALESCE((SELECT scope_type FROM extension_agent_skill_metadata WHERE extension_agent_skill_metadata.extension_id = extensions.extension_id), scope_type), scope_id = COALESCE((SELECT scope_id FROM extension_agent_skill_metadata WHERE extension_agent_skill_metadata.extension_id = extensions.extension_id), scope_id) WHERE source = 'instructions' AND owner_user_id = ''`).Error; err != nil {
		return err
	}
	if err := s.repository.db.WithContext(ctx).Exec(`UPDATE extensions SET owner_user_id = COALESCE((SELECT ws.user_id FROM extension_artifacts ea JOIN extension_workshop_sessions ws ON ws.id = ea.session_id WHERE ea.extension_id = extensions.extension_id AND ea.extension_version = extensions.current_version LIMIT 1), owner_user_id), scope_type = CASE WHEN COALESCE((SELECT ws.character_id FROM extension_artifacts ea JOIN extension_workshop_sessions ws ON ws.id = ea.session_id WHERE ea.extension_id = extensions.extension_id AND ea.extension_version = extensions.current_version LIMIT 1), '') = '' THEN 'global' ELSE 'character' END, scope_id = COALESCE((SELECT ws.character_id FROM extension_artifacts ea JOIN extension_workshop_sessions ws ON ws.id = ea.session_id WHERE ea.extension_id = extensions.extension_id AND ea.extension_version = extensions.current_version LIMIT 1), scope_id) WHERE source = 'workflow' AND owner_user_id = ''`).Error; err != nil {
		return err
	}
	if err := s.repository.db.WithContext(ctx).Exec(`UPDATE extension_versions SET artifact_id = COALESCE((SELECT artifact_id FROM extension_artifacts WHERE extension_artifacts.extension_id = extension_versions.extension_id AND extension_artifacts.extension_version = extension_versions.version LIMIT 1), artifact_id), artifact_hash = CASE WHEN artifact_hash = '' THEN checksum ELSE artifact_hash END, package_hash = CASE WHEN package_hash = '' THEN checksum ELSE package_hash END, source = CASE WHEN source = '' THEN COALESCE((SELECT source FROM extension_artifacts WHERE extension_artifacts.extension_id = extension_versions.extension_id AND extension_artifacts.extension_version = extension_versions.version LIMIT 1), '') ELSE source END, compatibility_status = CASE WHEN compatibility_status = '' THEN 'compatible' ELSE compatibility_status END, capabilities_json = CASE WHEN capabilities_json = '' THEN '[]' ELSE capabilities_json END, validation_status = CASE WHEN validation_status = '' THEN 'valid' ELSE validation_status END`).Error; err != nil {
		return err
	}
	if err := s.recoverPackageOperations(ctx); err != nil {
		return err
	}
	return s.cleanupPackageRecoveryDebris(ctx)
}

func (s *PackageService) PreviewImport(ctx context.Context, request PreviewPackageImportRequest) (preview PackageImportPreview, err error) {
	defer func() {
		if err != nil {
			s.metric("extension_package_import_failure_total")
			ext := asExtensionError(err)
			if strings.Contains(ext.Code, "CHECKSUM") {
				s.metric("extension_package_checksum_failure_total")
			}
			if ext.Code == ErrPackageSignatureInvalid {
				s.metric("extension_package_signature_invalid_total")
			}
		}
	}()
	if strings.TrimSpace(request.UserID) == "" {
		return PackageImportPreview{}, NewExtensionError(ErrSkillPermissionDenied, "缺少导入用户", "", false, nil)
	}
	if request.ScopeType == "" {
		request.ScopeType = string(ScopeGlobal)
	}
	if request.ScopeType != string(ScopeGlobal) && request.ScopeType != string(ScopeCharacter) || request.ScopeType == string(ScopeCharacter) && strings.TrimSpace(request.ScopeID) == "" {
		return PackageImportPreview{}, NewExtensionError(ErrSkillPermissionDenied, "扩展安装作用域无效", request.ScopeType, false, nil)
	}
	if request.ScopeType == string(ScopeCharacter) {
		if err := s.repository.ValidateCharacterScope(ctx, ExecutionScope{UserID: request.UserID, CharacterID: request.ScopeID}); err != nil {
			return PackageImportPreview{}, err
		}
	}
	parsed, err := parsePackageInput(request, s.validator, s.limits)
	if err != nil {
		return PackageImportPreview{}, err
	}
	if parsed.Signature.Fingerprint != "" {
		trusted, trustErr := s.repository.PackageSignerTrusted(ctx, parsed.Signature.Fingerprint)
		if trustErr != nil {
			return PackageImportPreview{}, trustErr
		}
		if trusted {
			parsed.Signature.Status = PackageSignatureTrusted
		}
		var signature packageSignatureDocument
		if json.Unmarshal(parsed.Files["signature.json"], &signature) == nil {
			if err := s.repository.SavePackageSigner(ctx, parsed.Signature, signature.PublicKey); err != nil {
				return PackageImportPreview{}, err
			}
		}
	}
	preview, err = s.buildPackagePreview(ctx, request, parsed)
	if err != nil {
		return PackageImportPreview{}, err
	}
	preview.SessionID = uuid.NewString()
	preview.ExpiresAt = time.Now().UTC().Add(30 * time.Minute)
	if err := s.repository.CreatePackageImportSession(ctx, request.UserID, request.ScopeType, request.ScopeID, request.FileName, parsed, preview); err != nil {
		return PackageImportPreview{}, err
	}
	s.metric("extension_package_import_total")
	return preview, nil
}

func (s *PackageService) buildPackagePreview(ctx context.Context, request PreviewPackageImportRequest, parsed parsedExtensionPackage) (PackageImportPreview, error) {
	preview := PackageImportPreview{Format: parsed.Format, Source: parsed.Source, ScopeType: request.ScopeType, ScopeID: request.ScopeID, PackageHash: parsed.PackageHash, Checksum: PackageChecksumView{Valid: true, PackageHash: parsed.PackageHash}, Signature: parsed.Signature, Files: parsed.FileViews, FileCount: len(parsed.FileViews), Capabilities: []string{}, HighRisk: []string{}, CapabilityConfirmations: []string{}, Dependencies: []PackageDependencyView{}, Risks: []PackageRisk{}, Warnings: append([]string(nil), parsed.Warnings...), Errors: []string{}, AvailableActions: []string{"install"}, TestStatus: "not-provided", Compatible: true}
	for _, file := range parsed.FileViews {
		preview.TotalSize += file.Size
		switch file.Kind {
		case "script-disabled":
			preview.Scripts++
		case "reference":
			preview.References++
		case "asset":
			preview.Assets++
		}
	}
	if parsed.Format == PackageFormatAmitiax {
		manifest := parsed.Manifest
		preview.SkillType = manifest.Entry.Kind
		preview.ID = manifest.Metadata.ID
		preview.Name = manifest.Metadata.Name
		preview.Version = manifest.Metadata.Version
		preview.Description = manifest.Metadata.Description
		preview.License = manifest.Metadata.License
		preview.Triggers = append([]SkillTrigger(nil), manifest.Triggers...)
		preview.Capabilities = append([]string(nil), manifest.Capabilities...)
		preview.Compatibility = manifest.Compatibility.EngineMin
		if compareSemver(s.registry.engineVersion, manifest.Compatibility.EngineMin) < 0 || manifest.Compatibility.EngineMaxExclusive != "" && compareSemver(s.registry.engineVersion, manifest.Compatibility.EngineMaxExclusive) >= 0 {
			preview.Compatible = false
			preview.Errors = append(preview.Errors, ErrPackageEngineIncompatible)
		}
		if parsed.Workflow != nil {
			compiled, issues, compileErr := s.compiler.Compile(ctx, *parsed.Workflow)
			if compileErr != nil {
				return PackageImportPreview{}, NewExtensionError(ErrPackageManifestInvalid, "Workflow 静态分析失败", summarizeIssues(issues), false, compileErr)
			}
			actual := uniqueSortedStrings(compiled.Capabilities)
			declared := uniqueSortedStrings(manifest.Capabilities)
			if !sameStringSets(actual, declared) {
				return PackageImportPreview{}, NewExtensionError(ErrPackageCapabilityMismatch, "Workflow Capability 与实际行为不一致", fmt.Sprintf("声明=%v 实际=%v", declared, actual), false, nil)
			}
			preview.WorkflowSteps = packageStepSummary(parsed.Workflow)
			if request.OperationID != "" {
				if err := s.repository.SetPackageOperationStatus(ctx, request.OperationID, "testing"); err != nil {
					return PackageImportPreview{}, err
				}
			}
			testReport := s.runPackageWorkflowTests(ctx, request, parsed, compiled)
			preview.TestReport = &testReport
			preview.TestStatus = "dry-run-passed"
			if len(parsed.Tests) > 0 {
				preview.TestStatus = "tests-passed"
			}
			if testReport.Status != "passed" {
				preview.TestStatus = "failed"
				preview.Compatible = false
				preview.Errors = append(preview.Errors, ErrPackageTestFailed)
			}
			for _, dependency := range compiled.Dependencies {
				preview.Dependencies = append(preview.Dependencies, s.resolvePackageDependency(ctx, dependency.SkillID, dependency.Version))
			}
		}
		if parsed.AgentSkill != nil {
			if request.OperationID != "" {
				if err := s.repository.SetPackageOperationStatus(ctx, request.OperationID, "testing"); err != nil {
					return PackageImportPreview{}, err
				}
			}
			agentPreview := AgentSkillImportPreview{Definition: parsed.AgentSkill.Definition, Report: parsed.AgentSkill.Report, Files: parsed.AgentSkill.Definition.Resources}
			preview.AgentSkill = &agentPreview
			preview.ScriptsRequired = len(parsed.AgentSkill.Report.RequiredScripts) > 0
			preview.Compatible = preview.Compatible && parsed.AgentSkill.Report.Status != AgentSkillBlocked
			preview.TestStatus = "compatibility-passed"
		}
	} else {
		agent := parsed.AgentSkill
		if agent == nil {
			return PackageImportPreview{}, NewExtensionError(ErrPackageFormatUnsupported, "AgentSkills 内容无效", "", false, nil)
		}
		if request.OperationID != "" {
			if err := s.repository.SetPackageOperationStatus(ctx, request.OperationID, "testing"); err != nil {
				return PackageImportPreview{}, err
			}
		}
		preview.SkillType = "instructions"
		preview.Name = agent.Definition.Name
		preview.Description = agent.Definition.Description
		preview.License = agent.Definition.License
		preview.Compatibility = agent.Definition.Compatibility
		preview.Version = "0.0.0+" + agent.Definition.ContentHash[:12]
		if sourceVersion := agent.Definition.Metadata["version"]; semverPattern.MatchString(sourceVersion) {
			preview.Version = sourceVersion
		}
		preview.ID = localAgentSkillExtensionID(request.UserID, request.ScopeType, request.ScopeID, preview.Name)
		agentPreview := AgentSkillImportPreview{Definition: agent.Definition, Report: agent.Report, Files: agent.Definition.Resources}
		preview.AgentSkill = &agentPreview
		preview.ScriptsRequired = len(agent.Report.RequiredScripts) > 0
		preview.Compatible = agent.Report.Status != AgentSkillBlocked
		preview.TestStatus = "compatibility-passed"
		preview.Triggers = []SkillTrigger{TriggerLLM, TriggerManual}
	}
	for _, capability := range preview.Capabilities {
		if definition, ok := Capability(capability); ok && definition.Risk == "high" {
			preview.HighRisk = append(preview.HighRisk, capability)
			preview.Risks = append(preview.Risks, PackageRisk{Code: "HIGH_RISK_CAPABILITY", Severity: "high", Message: capability})
		}
	}
	if preview.Signature.Status == PackageSignatureUnsigned {
		preview.Risks = append(preview.Risks, PackageRisk{Code: "UNSIGNED_SOURCE", Severity: "medium", Message: "来源未验证"})
		preview.Warnings = append(preview.Warnings, "扩展包未签名")
	}
	if preview.Scripts > 0 {
		preview.Risks = append(preview.Risks, PackageRisk{Code: "SCRIPTS_DISABLED", Severity: "high", Message: "脚本仅作为不可执行资源保留"})
	}
	for _, dependency := range preview.Dependencies {
		if dependency.Required && !dependency.Installed {
			preview.Errors = append(preview.Errors, ErrPackageDependencyMissing+": "+dependency.ID)
		}
	}
	conflict, err := s.packageConflict(ctx, request, preview)
	if err != nil {
		return PackageImportPreview{}, err
	}
	preview.Conflict = conflict
	if conflict == PackageConflictUpgrade {
		if current, getErr := s.registry.Get(ctx, preview.ID); getErr == nil {
			preview.CurrentVersion = current.Definition.Version
			preview.RollbackVersion = current.Definition.Version
			preview.CapabilityConfirmations = stringSetDifference(preview.Capabilities, current.Definition.Capabilities)
			for _, capability := range preview.CapabilityConfirmations {
				preview.Risks = append(preview.Risks, PackageRisk{Code: "CAPABILITY_ADDED", Severity: "high", Message: capability + " 需要重新确认"})
			}
			if currentVersion, versionErr := s.repository.GetPackageVersion(ctx, preview.ID, current.Definition.Version); versionErr == nil {
				preview.UpgradeDiff = s.buildPackageUpgradeDiff(ctx, currentVersion, parsed, preview)
				if currentVersion.SignerFingerprint != preview.Signature.Fingerprint {
					preview.Risks = append(preview.Risks, PackageRisk{Code: "SIGNER_CHANGED", Severity: "high", Message: "升级包签名者与当前版本不同"})
					preview.Warnings = append(preview.Warnings, "升级包签名者发生变化")
				}
				if preview.Scripts > 0 && packageVersionScriptCount(currentVersion, s.limits) == 0 {
					preview.Risks = append(preview.Risks, PackageRisk{Code: "SCRIPTS_ADDED", Severity: "high", Message: "升级包新增 scripts，仍将保持不可执行"})
					preview.Warnings = append(preview.Warnings, "升级包从无 scripts 变为包含 scripts")
				}
				if packageConfigSchemaChanged(ctx, s.repository, currentVersion, parsed) {
					preview.Risks = append(preview.Risks, PackageRisk{Code: "CONFIG_MIGRATION", Severity: "medium", Message: "配置 Schema 发生变化，现有配置将在事务中迁移"})
					preview.Warnings = append(preview.Warnings, "升级需要迁移现有配置")
				}
			}
		}
	}
	switch conflict {
	case PackageConflictSame:
		preview.AvailableActions = []string{"already-installed"}
	case PackageConflictDifferent, PackageConflictID, PackageConflictName:
		preview.AvailableActions = nil
		preview.Errors = append(preview.Errors, string(conflict))
	case PackageConflictUpgrade:
		preview.AvailableActions = []string{"upgrade"}
	case PackageConflictDowngrade:
		preview.AvailableActions = []string{"import-history", "rollback"}
	}
	if !preview.Compatible {
		preview.AvailableActions = nil
	}
	return preview, nil
}

func (s *PackageService) buildPackageUpgradeDiff(ctx context.Context, current packageVersionRecord, parsed parsedExtensionPackage, preview PackageImportPreview) *PackageVersionDiff {
	manifestRaw, _ := json.Marshal(parsed.Manifest)
	capabilitiesRaw, _ := json.Marshal(preview.Capabilities)
	next := packageVersionRecord{ExtensionID: preview.ID, Version: preview.Version, ManifestJSON: string(manifestRaw), CapabilitiesJSON: string(capabilitiesRaw), SignatureStatus: string(preview.Signature.Status), SignerFingerprint: preview.Signature.Fingerprint}
	var currentArtifact packageArtifactRecord
	if s.repository.db.WithContext(ctx).Where("artifact_id = ?", current.ArtifactID).First(&currentArtifact).Error != nil {
		return nil
	}
	nextArtifact := packageArtifactRecord{ManifestJSON: string(manifestRaw), WorkflowJSON: string(parsed.WorkflowRaw), ArtifactKind: "workflow"}
	schemasRaw, _ := json.Marshal(packageSchemas(parsed, parsed.Manifest))
	nextArtifact.SchemasJSON = string(schemasRaw)
	if parsed.AgentSkill != nil {
		nextArtifact.ArtifactKind = "agent-skill"
		nextArtifact.ContentBlob, _ = encodeAgentSkillArtifact(parsed.AgentSkill.Files)
	}
	diff := packageVersionDiffRecords(preview.ID, current, next, &currentArtifact, &nextArtifact)
	var rows []packageDependencyRecord
	_ = s.repository.db.WithContext(ctx).Where("extension_id = ? AND extension_version = ?", preview.ID, current.Version).Find(&rows).Error
	currentDependencies := make([]string, 0, len(rows))
	for _, row := range rows {
		currentDependencies = append(currentDependencies, row.DependencyID+"@"+row.Constraint)
	}
	nextDependencies := make([]string, 0, len(preview.Dependencies))
	for _, dependency := range preview.Dependencies {
		nextDependencies = append(nextDependencies, dependency.ID+"@"+dependency.VersionConstraint)
	}
	diff.Dependencies["added"] = stringSetDifference(nextDependencies, currentDependencies)
	diff.Dependencies["removed"] = stringSetDifference(currentDependencies, nextDependencies)
	diff.Scripts["added"] = stringSetDifference(packageScriptPaths(parsed.FileViews), artifactScriptPaths(currentArtifact))
	diff.Scripts["removed"] = stringSetDifference(artifactScriptPaths(currentArtifact), packageScriptPaths(parsed.FileViews))
	return &diff
}

func packageScriptPaths(files []PackageFileView) []string {
	result := []string{}
	for _, file := range files {
		if file.Kind == "script-disabled" {
			result = append(result, file.Path)
		}
	}
	return uniqueSortedStrings(result)
}

func artifactScriptPaths(artifact packageArtifactRecord) []string {
	if artifact.ArtifactKind != "agent-skill" {
		return []string{}
	}
	files, err := decodeAgentSkillArtifact(artifact.ContentBlob, DefaultAgentSkillLimits())
	if err != nil {
		return []string{}
	}
	result := []string{}
	for name := range files {
		if strings.HasPrefix(strings.ToLower(name), "scripts/") {
			result = append(result, name)
		}
	}
	return uniqueSortedStrings(result)
}

func packageConfigSchemaChanged(ctx context.Context, repository *Repository, version packageVersionRecord, parsed parsedExtensionPackage) bool {
	var artifact packageArtifactRecord
	if repository.db.WithContext(ctx).Where("artifact_id = ?", version.ArtifactID).First(&artifact).Error != nil {
		return false
	}
	var schemas map[string]json.RawMessage
	if json.Unmarshal([]byte(artifact.SchemasJSON), &schemas) != nil {
		return false
	}
	return string(stableJSON(schemas["config"])) != string(stableJSON(parsed.Manifest.ConfigSchema))
}

func packageVersionScriptCount(version packageVersionRecord, limits PackageLimits) int {
	if len(version.PackageBlob) == 0 {
		return 0
	}
	_, files, err := readPackageZIP(version.PackageBlob, limits)
	if err != nil {
		return 0
	}
	count := 0
	for _, file := range files {
		if file.Kind == "script-disabled" {
			count++
		}
	}
	return count
}

func (s *PackageService) resolvePackageDependency(ctx context.Context, id, version string) PackageDependencyView {
	view := PackageDependencyView{ID: id, VersionConstraint: version, Required: true}
	if registered, err := s.registry.Get(ctx, id); err == nil {
		view.Installed = true
		view.Version = registered.Definition.Version
	}
	return view
}

func (s *PackageService) packageConflict(ctx context.Context, request PreviewPackageImportRequest, preview PackageImportPreview) (PackageConflictStatus, error) {
	var existing extensionRecord
	err := s.repository.db.WithContext(ctx).Where("extension_id = ? AND archived_at = ''", preview.ID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if preview.SkillType == "instructions" {
			var count int64
			if err := s.repository.db.WithContext(ctx).Model(&agentSkillMetadataRecord{}).Where("user_id = ? AND name = ? AND scope_type = ? AND scope_id = ? AND removed_at = ''", request.UserID, preview.Name, request.ScopeType, request.ScopeID).Count(&count).Error; err != nil {
				return "", err
			}
			if count > 0 {
				s.metric("extension_package_conflict_total")
				return PackageConflictName, nil
			}
		}
		return PackageConflictNew, nil
	}
	if err != nil {
		return "", err
	}
	var ownership struct {
		OwnerUserID string `gorm:"column:owner_user_id"`
		ScopeType   string `gorm:"column:scope_type"`
		ScopeID     string `gorm:"column:scope_id"`
	}
	if err := s.repository.db.WithContext(ctx).Table("extensions").Select("owner_user_id", "scope_type", "scope_id").Where("extension_id = ?", preview.ID).Take(&ownership).Error; err != nil {
		return "", err
	}
	if ownership.OwnerUserID != "" && (ownership.OwnerUserID != request.UserID || ownership.ScopeType != request.ScopeType || ownership.ScopeID != request.ScopeID) {
		s.metric("extension_package_conflict_total")
		return PackageConflictID, nil
	}
	var version packageVersionRecord
	if err := s.repository.db.WithContext(ctx).Where("extension_id = ? AND version = ?", preview.ID, preview.Version).First(&version).Error; err == nil {
		if version.PackageHash == preview.PackageHash || version.PackageHash == "" && version.Checksum == preview.PackageHash {
			return PackageConflictSame, nil
		}
		s.metric("extension_package_conflict_total")
		return PackageConflictDifferent, nil
	}
	comparison := compareSemver(preview.Version, existing.CurrentVersion)
	if comparison > 0 {
		return PackageConflictUpgrade, nil
	}
	if comparison < 0 {
		return PackageConflictDowngrade, nil
	}
	return PackageConflictDifferent, nil
}

func localAgentSkillExtensionID(userID, scopeType, scopeID, name string) string {
	hash := hashAgentSkillFiles(map[string][]byte{"scope": []byte(userID + "\x00" + scopeType + "\x00" + scopeID)})[:12]
	return "local.agentskill." + hash + "." + name
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
	if s.repository != nil && s.repository.db != nil && s.repository.db.Migrator().HasTable("extension_artifacts") {
		var blobBytes int64
		if s.repository.db.Raw(`SELECT COALESCE(SUM(LENGTH(content_blob)), 0) FROM extension_artifacts`).Scan(&blobBytes).Error == nil && blobBytes > 0 {
			result["package_blob_bytes"] = uint64(blobBytes)
		}
	}
	if s.kernelProxy != nil && s.kernelProxy.ReadContainer() != nil {
		report := &kernelruntime.FinalGateReport{Metrics: map[string]int64{}, Details: []kernelruntime.FinalGateIssue{}, Errors: []string{}}
		kernelruntime.NewFinalGateProbe(s.kernelProxy.ReadContainer()).ProbePackageReleaseGate(context.Background(), report)
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

func decodePackageSignaturePublicKey(parsed parsedExtensionPackage) string {
	var signature packageSignatureDocument
	if json.Unmarshal(parsed.Files["signature.json"], &signature) != nil {
		return ""
	}
	if _, err := base64.StdEncoding.DecodeString(signature.PublicKey); err != nil {
		return ""
	}
	return signature.PublicKey
}

func sortedPackageCapabilities(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

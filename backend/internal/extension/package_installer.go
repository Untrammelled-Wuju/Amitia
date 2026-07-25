// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.

package extension

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type packageConfigMigration struct {
	ID      string
	Secured string
}

func (s *PackageService) Install(ctx context.Context, request InstallPackageRequest) (result PackageOperationResult, err error) {
	if request.ScopeType == string(ScopeCharacter) {
		if err := s.repository.ValidateCharacterScope(ctx, ExecutionScope{UserID: request.UserID, CharacterID: request.ScopeID}); err != nil {
			return PackageOperationResult{}, err
		}
	}
	session, err := s.repository.AcquirePackageImportSession(ctx, request.SessionID, request.UserID, request.ScopeType, request.ScopeID)
	if err != nil {
		return PackageOperationResult{}, err
	}
	defer func() {
		status := "installed"
		if err != nil {
			status = "failed"
		}
		_ = s.repository.FinishPackageImportSession(context.Background(), session.ID, status)
	}()
	operationID := uuid.NewString()
	traceID := uuid.NewString()
	operation := PackageOperationInstall
	record := packageOperationRecord{ID: operationID, Operation: string(operation), PackageHash: session.PackageHash, UserID: request.UserID, ScopeType: request.ScopeType, ScopeID: request.ScopeID, Status: "pending", TraceID: traceID, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := s.repository.CreatePackageOperation(ctx, record); err != nil {
		return PackageOperationResult{}, err
	}
	defer func() {
		if err != nil {
			code := asExtensionError(err).Code
			_ = s.repository.FinishPackageOperation(context.Background(), operationID, "failed", code)
			if operation == PackageOperationUpgrade {
				s.metric("extension_package_upgrade_failure_total")
			}
		}
	}()
	parsed, err := s.reparsePackageSession(session)
	if err != nil {
		return PackageOperationResult{}, err
	}
	if parsed.PackageHash != session.PackageHash {
		return PackageOperationResult{}, NewExtensionError(ErrPackageChecksumMismatch, "正式安装时包 Hash 已变化", "", false, nil)
	}
	extensionID, extensionVersion := packageOperationIdentity(request, parsed)
	previousVersion := ""
	if extensionID != "" {
		if current, getErr := s.repository.GetPackageExtension(ctx, extensionID, request.UserID, request.ScopeType, request.ScopeID); getErr == nil {
			previousVersion = current.CurrentVersion
		}
	}
	identity := PackageImportPreview{ID: extensionID, Version: extensionVersion, Source: parsed.Source, PackageHash: parsed.PackageHash, Signature: parsed.Signature}
	if err := s.repository.UpdatePackageOperationDetails(ctx, operationID, operation, identity, previousVersion); err != nil {
		return PackageOperationResult{}, err
	}
	if err := s.repository.SetPackageOperationStatus(ctx, operationID, "validating"); err != nil {
		return PackageOperationResult{}, err
	}
	preview, err := s.buildPackagePreview(ctx, PreviewPackageImportRequest{UserID: request.UserID, ScopeType: request.ScopeType, ScopeID: request.ScopeID, FileName: session.FileName, OperationID: operationID}, parsed)
	if err != nil {
		return PackageOperationResult{}, err
	}
	if preview.Conflict == PackageConflictUpgrade {
		operation = PackageOperationUpgrade
	}
	if err := s.repository.UpdatePackageOperationDetails(ctx, operationID, operation, preview, previousVersion); err != nil {
		return PackageOperationResult{}, err
	}
	if request.ExpectedExtensionID != "" && preview.ID != request.ExpectedExtensionID {
		return PackageOperationResult{}, NewExtensionError(ErrPackageVersionConflict, "升级包 Extension ID 不匹配", preview.ID, false, nil)
	}
	if len(preview.Errors) > 0 || !preview.Compatible {
		return PackageOperationResult{}, NewExtensionError(ErrPackageInstallFailed, "扩展包不满足安装条件", strings.Join(preview.Errors, "; "), false, nil)
	}
	if parsed.Signature.Status == PackageSignatureUnsigned && !request.ConfirmUnsigned {
		return PackageOperationResult{}, NewExtensionError(ErrPackageHighRiskConfirmationRequired, "未签名来源需要显式确认", "unsigned", false, nil)
	}
	if preview.Scripts > 0 && !request.ConfirmScripts {
		return PackageOperationResult{}, NewExtensionError(ErrPackageHighRiskConfirmationRequired, "需要确认 scripts 始终不可执行", "scripts", false, nil)
	}
	confirmed := map[string]bool{}
	for _, capability := range request.ConfirmedCapabilities {
		confirmed[capability] = true
	}
	for _, capability := range preview.HighRisk {
		if !confirmed[capability] {
			return PackageOperationResult{}, NewExtensionError(ErrPackageHighRiskConfirmationRequired, "高风险 Capability 需要逐项确认", capability, false, nil)
		}
	}
	for _, capability := range preview.CapabilityConfirmations {
		if !confirmed[capability] {
			return PackageOperationResult{}, NewExtensionError(ErrPackageHighRiskConfirmationRequired, "新增 Capability 需要重新确认", capability, false, nil)
		}
	}
	if preview.Conflict == PackageConflictUpgrade && !request.ConfirmVersionChange {
		return PackageOperationResult{}, NewExtensionError(ErrPackageHighRiskConfirmationRequired, "版本变化需要显式确认", preview.Version, false, nil)
	}
	if packagePreviewHasRisk(preview, "SIGNER_CHANGED") && !request.ConfirmSignerChange {
		return PackageOperationResult{}, NewExtensionError(ErrPackageHighRiskConfirmationRequired, "签名者变化需要显式确认", preview.Signature.Fingerprint, false, nil)
	}
	if packagePreviewHasRisk(preview, "CONFIG_MIGRATION") && !request.ConfirmConfigMigration {
		return PackageOperationResult{}, NewExtensionError(ErrPackageHighRiskConfirmationRequired, "配置迁移需要显式确认", preview.Version, false, nil)
	}
	if preview.Conflict == PackageConflictDifferent {
		return PackageOperationResult{}, NewExtensionError(ErrPackageSameVersionDifferentContent, "相同版本内容不同", preview.Version, false, nil)
	}
	if preview.Conflict == PackageConflictID || preview.Conflict == PackageConflictName {
		return PackageOperationResult{}, NewExtensionError(ErrPackageIDConflict, "扩展 ID 或名称冲突", preview.ID, false, nil)
	}
	if preview.Conflict == PackageConflictDowngrade {
		return PackageOperationResult{}, NewExtensionError(ErrPackageVersionConflict, "低版本包不能作为普通安装", preview.Version, false, nil)
	}
	if preview.Conflict == PackageConflictSame {
		if err := s.repository.FinishPackageOperation(ctx, operationID, "succeeded", ""); err != nil {
			return PackageOperationResult{}, err
		}
		return PackageOperationResult{OperationID: operationID, TraceID: traceID, Operation: PackageOperationInstall, ExtensionID: preview.ID, Version: preview.Version, Enabled: false, Status: "succeeded"}, nil
	}
	unlock, ok := s.lockExtension(preview.ID)
	if !ok {
		return PackageOperationResult{}, NewExtensionError(ErrPackageOperationInProgress, "该扩展已有安装类操作进行中", preview.ID, true, nil)
	}
	defer unlock()
	if preview.Conflict == PackageConflictUpgrade {
		s.metric("extension_package_upgrade_total")
	}
	if err := s.repository.SetPackageOperationStatus(ctx, operationID, "staging"); err != nil {
		return PackageOperationResult{}, err
	}
	if restored, restoreErr := s.reinstallArchivedPackage(ctx, request, preview, operationID, traceID); restoreErr != nil {
		return PackageOperationResult{}, restoreErr
	} else if restored != nil {
		if err := s.repository.FinishPackageOperation(ctx, operationID, "succeeded", ""); err != nil {
			return PackageOperationResult{}, err
		}
		return *restored, nil
	}
	if parsed.Workflow != nil {
		result, err = s.installWorkflowPackage(ctx, request, parsed, preview, operationID, traceID)
	} else {
		result, err = s.installInstructionsPackage(ctx, request, parsed, preview, operationID, traceID)
	}
	if err != nil {
		return PackageOperationResult{}, err
	}
	if err := s.repository.FinishPackageOperation(ctx, operationID, "succeeded", ""); err != nil {
		return PackageOperationResult{}, err
	}
	return result, nil
}

func packageOperationIdentity(request InstallPackageRequest, parsed parsedExtensionPackage) (string, string) {
	if parsed.Format == PackageFormatAmitiax {
		return parsed.Manifest.Metadata.ID, parsed.Manifest.Metadata.Version
	}
	if parsed.AgentSkill == nil {
		return "", ""
	}
	version := "0.0.0+" + parsed.AgentSkill.Definition.ContentHash[:12]
	if sourceVersion := parsed.AgentSkill.Definition.Metadata["version"]; semverPattern.MatchString(sourceVersion) {
		version = sourceVersion
	}
	return localAgentSkillExtensionID(request.UserID, request.ScopeType, request.ScopeID, parsed.AgentSkill.Definition.Name), version
}

func packagePreviewHasRisk(preview PackageImportPreview, code string) bool {
	for _, risk := range preview.Risks {
		if risk.Code == code {
			return true
		}
	}
	return false
}

func (s *PackageService) reparsePackageSession(session packageImportSessionRecord) (parsedExtensionPackage, error) {
	request := PreviewPackageImportRequest{UserID: session.UserID, ScopeType: session.ScopeType, ScopeID: session.ScopeID, FileName: session.FileName, Raw: session.PackageBlob}
	if PackageFormat(session.Format) == PackageFormatAgentSkillsDir {
		files, _, err := readPackageZIP(session.PackageBlob, s.limits)
		if err != nil {
			return parsedExtensionPackage{}, err
		}
		parsed, err := parseNativeAgentSkills(files, "", AgentSkillSourceDirectory, PackageFormatAgentSkillsDir, session.PackageBlob)
		if err != nil {
			return parsedExtensionPackage{}, err
		}
		parsed.Source = "local-agentskills-directory"
		return parsed, nil
	}
	parsed, err := parsePackageInput(request, s.validator, s.limits)
	if err != nil {
		return parsedExtensionPackage{}, err
	}
	if parsed.Signature.Fingerprint != "" {
		trusted, trustErr := s.repository.PackageSignerTrusted(context.Background(), parsed.Signature.Fingerprint)
		if trustErr != nil {
			return parsedExtensionPackage{}, trustErr
		}
		if trusted {
			parsed.Signature.Status = PackageSignatureTrusted
		}
	}
	return parsed, nil
}

func (s *PackageService) reinstallArchivedPackage(ctx context.Context, request InstallPackageRequest, preview PackageImportPreview, operationID, traceID string) (*PackageOperationResult, error) {
	var archived extensionRecord
	if err := s.repository.db.WithContext(ctx).Where("extension_id = ? AND archived_at <> ''", preview.ID).First(&archived).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var ownership struct {
		OwnerUserID string `gorm:"column:owner_user_id"`
		ScopeType   string `gorm:"column:scope_type"`
		ScopeID     string `gorm:"column:scope_id"`
	}
	if err := s.repository.db.WithContext(ctx).Table("extensions").Select("owner_user_id", "scope_type", "scope_id").Where("extension_id = ?", preview.ID).Take(&ownership).Error; err != nil {
		return nil, err
	}
	if ownership.OwnerUserID != request.UserID || ownership.ScopeType != request.ScopeType || ownership.ScopeID != request.ScopeID {
		return nil, NewExtensionError(ErrSkillPermissionDenied, "归档扩展作用域不匹配", preview.ID, false, nil)
	}
	version, err := s.repository.GetPackageVersion(ctx, preview.ID, preview.Version)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if version.PackageHash != preview.PackageHash {
		return nil, NewExtensionError(ErrPackageSameVersionDifferentContent, "归档版本内容不同", preview.Version, false, nil)
	}
	var artifact packageArtifactRecord
	if err := s.repository.db.WithContext(ctx).Where("artifact_id = ?", version.ArtifactID).First(&artifact).Error; err != nil {
		return nil, NewExtensionError(ErrPackageArtifactInvalid, "归档 Artifact 缺失", version.ArtifactID, false, err)
	}
	var definition SkillDefinition
	var handler SkillHandler
	if artifact.ArtifactKind == "agent-skill" {
		var manifest Manifest
		if json.Unmarshal([]byte(artifact.ManifestJSON), &manifest) != nil {
			return nil, NewExtensionError(ErrPackageArtifactInvalid, "归档 Manifest 无效", version.ArtifactID, false, nil)
		}
		definition = skillDefinitionFromManifest(manifest, map[string]json.RawMessage{})
		definition.Source = SkillSourceInstructions
	} else {
		definition, handler, err = s.workflowInstaller.definitionFromArtifact(artifact.base())
		if err != nil {
			return nil, err
		}
	}
	definition.Enabled = false
	if err := s.registry.Register(ctx, definition, handler); err != nil {
		return nil, NewExtensionError(ErrPackageInstallFailed, "归档扩展重新注册失败", "", false, err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&extensionRecord{}).Where("extension_id = ?", preview.ID).Updates(map[string]interface{}{"current_version": preview.Version, "manifest_json": string(definition.Manifest), "normalized_manifest_json": string(stableJSON(definition.Manifest)), "enabled": 0, "archived_at": "", "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&packageArtifactRecord{}).Where("extension_id = ?", preview.ID).Update("archived_at", "").Error; err != nil {
			return err
		}
		if artifact.ArtifactKind == "agent-skill" {
			files, decodeErr := decodeAgentSkillArtifact(artifact.ContentBlob, DefaultAgentSkillLimits())
			if decodeErr != nil {
				return decodeErr
			}
			parsed, parseErr := parseAgentSkillFiles(files, "", AgentSkillSourceZIP, DefaultAgentSkillLimits())
			if parseErr != nil {
				return parseErr
			}
			resources, _ := json.Marshal(parsed.Definition.Resources)
			mappings, _ := json.Marshal(parsed.Definition.ToolMappings)
			report, _ := json.Marshal(parsed.Report)
			return tx.Model(&agentSkillMetadataRecord{}).Where("extension_id = ?", preview.ID).Updates(map[string]interface{}{"artifact_id": artifact.ArtifactID, "content_hash": parsed.Definition.ContentHash, "description": parsed.Definition.Description, "resource_index_json": string(resources), "tool_mappings_json": string(mappings), "compatibility_report_json": string(report), "compatibility_status": string(parsed.Report.Status), "enabled": 0, "removed_at": "", "updated_at": now}).Error
		}
		return nil
	}); err != nil {
		_ = s.registry.Unregister(context.Background(), preview.ID)
		return nil, NewExtensionError(ErrPackageInstallFailed, "归档扩展恢复事务失败", "", false, err)
	}
	if s.agentSkills != nil {
		s.agentSkills.invalidateAgentSkillCaches()
	}
	result := PackageOperationResult{OperationID: operationID, TraceID: traceID, Operation: packageOperationForPreview(preview), ExtensionID: preview.ID, Version: preview.Version, Enabled: false, Status: "succeeded"}
	return &result, nil
}

func (s *PackageService) installWorkflowPackage(ctx context.Context, request InstallPackageRequest, parsed parsedExtensionPackage, preview PackageImportPreview, operationID, traceID string) (PackageOperationResult, error) {
	compiled, issues, err := s.compiler.Compile(ctx, *parsed.Workflow)
	if err != nil {
		return PackageOperationResult{}, NewExtensionError(ErrPackageTestFailed, "Workflow Dry Run 编译失败", summarizeIssues(issues), false, err)
	}
	artifactID := uuid.NewString()
	manifest := parsed.Manifest
	manifest.Enabled = false
	manifest.Entry.ArtifactID = artifactID
	manifest.Entry.Path = "workflows/main.json"
	manifestRaw, _ := json.Marshal(manifest)
	schemas := packageSchemas(parsed, manifest)
	schemasRaw, _ := json.Marshal(schemas)
	compiledRaw, _ := json.Marshal(compiled)
	artifact := packageArtifactRecord{ID: uuid.NewString(), ArtifactID: artifactID, ExtensionID: preview.ID, ExtensionVersion: preview.Version, Source: parsed.Source, SessionID: request.SessionID, ManifestJSON: string(manifestRaw), WorkflowJSON: string(parsed.WorkflowRaw), SchemasJSON: string(schemasRaw), CompiledWorkflowJSON: string(compiledRaw), TestsJSON: string(normalizeJSONArray(parsed.Tests)), ReadmeText: string(parsed.Files["docs/README.md"]), SizeBytes: int64(len(parsed.Raw)), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), ArtifactKind: "workflow", ContentBlob: append([]byte(nil), parsed.Raw...), ResourceIndexJSON: "[]", ArtifactStatus: "staged", OperationID: operationID}
	artifact.Checksum = artifactChecksum(artifact.base())
	definition := skillDefinitionFromManifest(manifest, schemas)
	definition.Dependencies = dependencyIDs(compiled.Dependencies)
	definition.Enabled = false
	handler := s.workflowInstaller.workflowHandler(artifact.base(), definition.OutputSchema)
	configMigrations, err := s.preparePackageConfigMigrations(ctx, preview.ID, definition.ConfigSchema, definition.DefaultConfig)
	if err != nil {
		return PackageOperationResult{}, err
	}
	return s.commitPackageVersion(ctx, request, parsed, preview, artifact, definition, handler, compiled.Dependencies, configMigrations, operationID, traceID, nil)
}

func (s *PackageService) installInstructionsPackage(ctx context.Context, request InstallPackageRequest, parsed parsedExtensionPackage, preview PackageImportPreview, operationID, traceID string) (PackageOperationResult, error) {
	if parsed.AgentSkill == nil {
		return PackageOperationResult{}, NewExtensionError(ErrPackageManifestInvalid, "instructions 内容缺失", "", false, nil)
	}
	definition := parsed.AgentSkill.Definition
	definition.ExtensionID = preview.ID
	definition.UserID = request.UserID
	definition.Scope = AgentSkillScope(request.ScopeType)
	definition.ScopeID = request.ScopeID
	definition.ArtifactID = uuid.NewString()
	definition.Enabled = false
	definition.CreatedAt = time.Now().UTC()
	definition.UpdatedAt = definition.CreatedAt
	manifestDefinition := buildAgentSkillManifest(definition, preview.Version)
	if parsed.Format == PackageFormatAmitiax {
		manifest := parsed.Manifest
		manifest.Enabled = false
		manifest.Entry.ArtifactID = definition.ArtifactID
		manifest.Entry.Path = "SKILL.md"
		manifestRaw, _ := json.Marshal(manifest)
		manifestDefinition = skillDefinitionFromManifest(manifest, packageSchemas(parsed, manifest))
		manifestDefinition.Source = SkillSourceInstructions
		manifestDefinition.Manifest = manifestRaw
		manifestDefinition.Entry = manifest.Entry
		manifestDefinition.Enabled = false
	}
	agentArchive, err := encodeAgentSkillArtifact(parsed.AgentSkill.Files)
	if err != nil {
		return PackageOperationResult{}, err
	}
	resources, _ := json.Marshal(definition.Resources)
	artifact := packageArtifactRecord{ID: uuid.NewString(), ArtifactID: definition.ArtifactID, ExtensionID: preview.ID, ExtensionVersion: preview.Version, Source: parsed.Source, SessionID: request.SessionID, ManifestJSON: string(manifestDefinition.Manifest), WorkflowJSON: "{}", SchemasJSON: "{}", CompiledWorkflowJSON: "{}", TestsJSON: "[]", Checksum: definition.ContentHash, SizeBytes: int64(len(agentArchive)), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), ArtifactKind: "agent-skill", ContentBlob: agentArchive, ResourceIndexJSON: string(resources), ArtifactStatus: "staged", OperationID: operationID}
	metadata := buildAgentSkillMetadataRecord(definition, parsed.AgentSkill.Report)
	return s.commitPackageVersion(ctx, request, parsed, preview, artifact, manifestDefinition, nil, nil, nil, operationID, traceID, &metadata)
}

func (s *PackageService) commitPackageVersion(ctx context.Context, request InstallPackageRequest, parsed parsedExtensionPackage, preview PackageImportPreview, artifact packageArtifactRecord, definition SkillDefinition, handler SkillHandler, dependencies []ResolvedSkillDependency, configMigrations []packageConfigMigration, operationID, traceID string, agentMetadata *agentSkillMetadataRecord) (PackageOperationResult, error) {
	if err := s.repository.SetPackageOperationStatus(ctx, operationID, "staging"); err != nil {
		return PackageOperationResult{}, err
	}
	var oldRegistered *RegisteredSkill
	if current, err := s.registry.Get(ctx, preview.ID); err == nil {
		oldRegistered = &current
	}
	installScope := ExecutionScope{UserID: request.UserID}
	if request.ScopeType == string(ScopeCharacter) {
		installScope.CharacterID = request.ScopeID
	}
	previousEnabled := false
	if oldScoped, scopeErr := s.registry.GetScoped(ctx, preview.ID, installScope); scopeErr == nil {
		previousEnabled = oldScoped.Definition.Enabled
	}
	capabilitiesJSON, _ := json.Marshal(sortedPackageCapabilities(definition.Capabilities))
	version := packageVersionRecord{ID: uuid.NewString(), ExtensionID: preview.ID, Version: preview.Version, ManifestJSON: string(definition.Manifest), Checksum: artifact.Checksum, ArtifactID: artifact.ArtifactID, ArtifactHash: artifact.Checksum, PackageHash: preview.PackageHash, Source: preview.Source, SignatureStatus: string(preview.Signature.Status), SignerFingerprint: preview.Signature.Fingerprint, CompatibilityStatus: "compatible", CapabilitiesJSON: string(capabilitiesJSON), InstalledBy: request.UserID, ValidationStatus: "valid", TestStatus: preview.TestStatus, ArtifactStatus: "staged", ActivationStatus: "staged", OperationID: operationID, PackageBlob: append([]byte(nil), parsed.Raw...), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&artifact).Error; err != nil {
			return err
		}
		return tx.Create(&version).Error
	}); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return PackageOperationResult{}, NewExtensionError(ErrPackageSameVersionDifferentContent, "扩展版本已存在", preview.Version, false, err)
		}
		return PackageOperationResult{}, NewExtensionError(ErrPackageInstallFailed, "准备目标版本失败", "", false, err)
	}
	cleanupPrepared := func() {
		_ = s.repository.db.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
			_ = tx.Where("artifact_id = ?", artifact.ArtifactID).Delete(&packageArtifactRecord{}).Error
			return tx.Where("extension_id = ? AND version = ?", preview.ID, preview.Version).Delete(&packageVersionRecord{}).Error
		})
	}
	if oldRegistered != nil {
		_ = s.registry.Unregister(ctx, preview.ID)
	}
	if err := s.repository.SetPackageOperationStatus(ctx, operationID, "registering"); err != nil {
		cleanupPrepared()
		return PackageOperationResult{}, err
	}
	if err := s.registry.Register(ctx, definition, handler); err != nil {
		if oldRegistered != nil {
			_ = s.registry.Register(ctx, oldRegistered.Definition, oldRegistered.Handler)
		}
		cleanupPrepared()
		return PackageOperationResult{}, NewExtensionError(ErrPackageInstallFailed, "Registry 注册目标版本失败", "", false, err)
	}
	if request.ScopeType == string(ScopeCharacter) {
		if err := s.repository.DeleteScopeBinding(ctx, preview.ID, PermissionScope{Type: ScopeGlobal}); err != nil {
			_ = s.registry.Unregister(context.Background(), preview.ID)
			if oldRegistered != nil {
				_ = s.registry.Register(context.Background(), oldRegistered.Definition, oldRegistered.Handler)
			}
			cleanupPrepared()
			return PackageOperationResult{}, NewExtensionError(ErrPackageInstallFailed, "角色作用域绑定失败", "", false, err)
		}
	}
	enabled := previousEnabled
	if err := s.registry.SetScopeEnabled(ctx, preview.ID, installScope, enabled); err != nil {
		_ = s.registry.Unregister(context.Background(), preview.ID)
		if oldRegistered != nil {
			_ = s.registry.Register(context.Background(), oldRegistered.Definition, oldRegistered.Handler)
		}
		cleanupPrepared()
		return PackageOperationResult{}, NewExtensionError(ErrPackageInstallFailed, "作用域状态恢复失败", "", false, err)
	}
	if err := s.repository.SetPackageOperationStatus(ctx, operationID, "committing"); err != nil {
		_ = s.registry.Unregister(context.Background(), preview.ID)
		if oldRegistered != nil {
			_ = s.registry.Register(context.Background(), oldRegistered.Definition, oldRegistered.Handler)
		}
		cleanupPrepared()
		return PackageOperationResult{}, err
	}
	transactionErr := s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		extension := map[string]interface{}{"id": uuid.NewString(), "extension_id": preview.ID, "kind": "Skill", "name": preview.Name, "current_version": preview.Version, "source": string(definition.Source), "enabled": boolNumber(enabled), "manifest_json": string(definition.Manifest), "normalized_manifest_json": string(stableJSON(definition.Manifest)), "owner_user_id": request.UserID, "scope_type": request.ScopeType, "scope_id": request.ScopeID, "created_at": now, "updated_at": now, "archived_at": ""}
		if err := tx.Table("extensions").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "extension_id"}}, DoUpdates: clause.AssignmentColumns([]string{"name", "current_version", "source", "enabled", "manifest_json", "normalized_manifest_json", "owner_user_id", "scope_type", "scope_id", "updated_at", "archived_at"})}).Create(extension).Error; err != nil {
			return err
		}
		if err := tx.Where("extension_id = ? AND extension_version = ?", preview.ID, preview.Version).Delete(&packageDependencyRecord{}).Error; err != nil {
			return err
		}
		for _, dependency := range dependencies {
			row := packageDependencyRecord{ExtensionID: preview.ID, ExtensionVersion: preview.Version, DependencyID: dependency.SkillID, Constraint: dependency.Version, Required: 1, CreatedAt: now}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		for _, migration := range configMigrations {
			if err := tx.Model(&configRecord{}).Where("id = ?", migration.ID).Updates(map[string]interface{}{"config_json": migration.Secured, "config_version": gorm.Expr("config_version + 1"), "updated_at": now}).Error; err != nil {
				return err
			}
		}
		if oldRegistered != nil {
			for _, oldCapability := range oldRegistered.Definition.Capabilities {
				if !containsString(definition.Capabilities, oldCapability) {
					if err := tx.Where("extension_id = ? AND capability = ?", preview.ID, oldCapability).Delete(&grantRecord{}).Error; err != nil {
						return err
					}
				}
			}
		}
		if agentMetadata != nil {
			var existing agentSkillMetadataRecord
			if err := tx.Where("extension_id = ?", preview.ID).First(&existing).Error; err == nil {
				agentMetadata.ID = existing.ID
				agentMetadata.CreatedAt = existing.CreatedAt
				agentMetadata.Enabled = boolNumber(enabled)
			}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "extension_id"}}, DoUpdates: clause.AssignmentColumns([]string{"description", "license", "compatibility", "metadata_json", "allowed_tools", "display_name", "short_description", "default_prompt", "openai_metadata_json", "source", "compatibility_status", "compatibility_report_json", "content_hash", "artifact_id", "raw_frontmatter_json", "extra_frontmatter_json", "resource_index_json", "tool_mappings_json", "scripts_present", "scripts_required", "enabled", "updated_at", "removed_at"})}).Create(agentMetadata).Error; err != nil {
				return err
			}
		}
		if tx.Migrator().HasColumn("extension_versions", "artifact_status") {
			if err := tx.Model(&packageVersionRecord{}).Where("extension_id = ? AND version = ?", preview.ID, preview.Version).Updates(map[string]interface{}{"artifact_status": "active", "activation_status": "active", "failure_code": ""}).Error; err != nil {
				return err
			}
			if err := tx.Model(&packageVersionRecord{}).Where("extension_id = ? AND version <> ? AND activation_status = ?", preview.ID, preview.Version, "active").Updates(map[string]interface{}{"activation_status": "archived", "artifact_status": "archived"}).Error; err != nil {
				return err
			}
		}
		if tx.Migrator().HasColumn("extension_artifacts", "artifact_status") {
			if err := tx.Model(&packageArtifactRecord{}).Where("artifact_id = ?", artifact.ArtifactID).Updates(map[string]interface{}{"artifact_status": "active", "operation_id": operationID}).Error; err != nil {
				return err
			}
			if err := tx.Model(&packageArtifactRecord{}).Where("extension_id = ? AND artifact_id <> ? AND artifact_status = ?", preview.ID, artifact.ArtifactID, "active").Update("artifact_status", "archived").Error; err != nil {
				return err
			}
		}
		return nil
	})
	if transactionErr != nil {
		_ = s.registry.Unregister(context.Background(), preview.ID)
		if oldRegistered != nil {
			_ = s.registry.Register(context.Background(), oldRegistered.Definition, oldRegistered.Handler)
		}
		cleanupPrepared()
		return PackageOperationResult{}, NewExtensionError(ErrPackageInstallFailed, "版本切换事务失败", "", false, transactionErr)
	}
	if s.agentSkills != nil {
		if err := s.repository.SetPackageOperationStatus(ctx, operationID, "refreshing"); err != nil {
			return PackageOperationResult{}, err
		}
		s.agentSkills.invalidateAgentSkillCaches()
	}
	if scoped, scopedErr := s.registry.GetScoped(ctx, preview.ID, ExecutionScope{UserID: request.UserID, CharacterID: request.ScopeID}); scopedErr == nil {
		enabled = scoped.Definition.Enabled
	}
	return PackageOperationResult{OperationID: operationID, TraceID: traceID, Operation: packageOperationForPreview(preview), ExtensionID: preview.ID, Version: preview.Version, Enabled: enabled, Status: "succeeded"}, nil
}

func packageOperationForPreview(preview PackageImportPreview) PackageOperation {
	if preview.Conflict == PackageConflictUpgrade {
		return PackageOperationUpgrade
	}
	return PackageOperationInstall
}

func packageSchemas(parsed parsedExtensionPackage, manifest Manifest) map[string]json.RawMessage {
	result := map[string]json.RawMessage{"input": normalizeJSON(manifest.InputSchema), "output": normalizeJSON(manifest.OutputSchema), "config": normalizeJSON(manifest.ConfigSchema), "defaults": normalizeJSON(manifest.DefaultConfig)}
	for key, value := range parsed.Schemas {
		result[key] = append(json.RawMessage(nil), value...)
	}
	return result
}

func normalizeJSONArray(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`[]`)
	}
	return raw
}

func buildAgentSkillMetadataRecord(definition AgentSkillDefinition, report AgentSkillCompatibilityReport) agentSkillMetadataRecord {
	resources, _ := json.Marshal(definition.Resources)
	mappings, _ := json.Marshal(definition.ToolMappings)
	metadata, _ := json.Marshal(definition.Metadata)
	reportRaw, _ := json.Marshal(report)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	scripts := 0
	for _, resource := range definition.Resources {
		if resource.Kind == AgentSkillResourceScript {
			scripts = 1
		}
	}
	return agentSkillMetadataRecord{ID: uuid.NewString(), ExtensionID: definition.ExtensionID, UserID: definition.UserID, Name: definition.Name, Description: definition.Description, License: definition.License, Compatibility: definition.Compatibility, MetadataJSON: string(metadata), AllowedTools: definition.AllowedTools, DisplayName: definition.DisplayName, ShortDescription: definition.ShortDescription, DefaultPrompt: definition.DefaultPrompt, OpenAIMetadataJSON: string(normalizeJSON(definition.OpenAIMetadata)), ScopeType: string(definition.Scope), ScopeID: definition.ScopeID, Source: string(definition.Source), CompatibilityStatus: string(definition.CompatibilityStatus), CompatibilityReportJSON: string(reportRaw), ContentHash: definition.ContentHash, ArtifactID: definition.ArtifactID, RawFrontmatterJSON: string(normalizeJSON(definition.RawFrontmatter)), ExtraFrontmatterJSON: string(normalizeJSON(definition.ExtraFrontmatter)), ResourceIndexJSON: string(resources), ToolMappingsJSON: string(mappings), ScriptsPresent: scripts, ScriptsRequired: boolNumber(len(report.RequiredScripts) > 0), Enabled: 0, CreatedAt: now, UpdatedAt: now}
}

func (s *PackageService) preparePackageConfigMigrations(ctx context.Context, extensionID string, schema, defaults json.RawMessage) ([]packageConfigMigration, error) {
	if len(schema) == 0 || string(schema) == "{}" {
		return nil, nil
	}
	var records []configRecord
	if err := s.repository.db.WithContext(ctx).Where("extension_id = ? AND archived_at = ''", extensionID).Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]packageConfigMigration, 0, len(records))
	for _, record := range records {
		plain, _, err := s.repository.configCipher.decrypt(record.ConfigJSON)
		if err != nil {
			return nil, NewExtensionError(ErrPackageConfigMigrationFailed, "无法读取现有配置", "", false, err)
		}
		migrated, err := migratePackageConfig(plain, schema, defaults, s.validator)
		if err != nil {
			return nil, err
		}
		secured, err := s.repository.configCipher.encrypt(migrated)
		if err != nil {
			return nil, err
		}
		result = append(result, packageConfigMigration{ID: record.ID, Secured: secured})
	}
	return result, nil
}

func migratePackageConfig(raw, schemaRaw, defaultsRaw json.RawMessage, validator *SchemaValidator) (json.RawMessage, error) {
	var current map[string]interface{}
	var schema struct {
		Properties map[string]struct {
			Default interface{} `json:"default"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	var defaults map[string]interface{}
	if json.Unmarshal(raw, &current) != nil || json.Unmarshal(schemaRaw, &schema) != nil {
		return nil, NewExtensionError(ErrPackageConfigMigrationFailed, "配置或 Config Schema 无效", "", false, nil)
	}
	_ = json.Unmarshal(defaultsRaw, &defaults)
	next := map[string]interface{}{}
	for key, property := range schema.Properties {
		if value, ok := current[key]; ok {
			next[key] = value
		} else if value, ok := defaults[key]; ok {
			next[key] = value
		} else if property.Default != nil {
			next[key] = property.Default
		}
	}
	for _, required := range schema.Required {
		if _, ok := next[required]; !ok {
			return nil, NewExtensionError(ErrPackageConfigMigrationRequired, "新配置包含缺少默认值的必填字段", required, false, nil)
		}
	}
	encoded, _ := json.Marshal(next)
	if err := validator.Validate("package-config-migration", schemaRaw, encoded); err != nil {
		return nil, NewExtensionError(ErrPackageConfigMigrationFailed, "现有配置与新 Schema 不兼容", err.Error(), false, err)
	}
	return encoded, nil
}

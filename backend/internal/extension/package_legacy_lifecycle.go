//go:build legacy_migration

package extension

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *PackageService) exportAmitiaxFiles(artifact packageArtifactRecord) (map[string][]byte, error) {
	if artifact.Checksum == "" || artifact.ArtifactKind != "agent-skill" && artifactChecksum(artifact.base()) != artifact.Checksum {
		return nil, NewExtensionError(ErrPackageArtifactInvalid, "Artifact Checksum 无效", artifact.ArtifactID, false, nil)
	}
	files := map[string][]byte{"manifest.json": []byte(artifact.ManifestJSON)}
	var manifest Manifest
	if json.Unmarshal([]byte(artifact.ManifestJSON), &manifest) != nil {
		return nil, NewExtensionError(ErrPackageArtifactInvalid, "Artifact Manifest 无效", artifact.ArtifactID, false, nil)
	}
	if artifact.ArtifactKind == "agent-skill" {
		manifest.Entry.Path = "instructions/SKILL.md"
		manifestRaw, _ := json.Marshal(manifest)
		files["manifest.json"] = manifestRaw
		agentFiles, err := decodeAgentSkillArtifact(artifact.ContentBlob, DefaultAgentSkillLimits())
		if err != nil {
			return nil, err
		}
		for name, content := range agentFiles {
			files["instructions/"+name] = content
		}
	} else {
		files["workflows/main.json"] = []byte(artifact.WorkflowJSON)
		var schemas map[string]json.RawMessage
		if json.Unmarshal([]byte(artifact.SchemasJSON), &schemas) != nil {
			return nil, NewExtensionError(ErrPackageArtifactInvalid, "Artifact Schema 无效", artifact.ArtifactID, false, nil)
		}
		for key, name := range map[string]string{"input": "input.schema.json", "output": "output.schema.json", "config": "config.schema.json"} {
			if raw := schemas[key]; len(raw) > 0 && string(raw) != "{}" {
				files["schemas/"+name] = raw
			}
		}
		if raw := schemas["defaults"]; len(raw) > 0 && string(raw) != "{}" {
			files["config/defaults.json"] = raw
		}
		if artifact.TestsJSON != "" && artifact.TestsJSON != "[]" && artifact.TestsJSON != "{}" {
			files["tests/report.json"] = []byte(artifact.TestsJSON)
		}
	}
	if artifact.ReadmeText != "" {
		files["docs/README.md"] = []byte(artifact.ReadmeText)
	}
	files["LICENSE"] = []byte(manifest.Metadata.License + "\n")
	sbom, _ := json.Marshal(map[string]interface{}{"spdxVersion": "SPDX-2.3", "dataLicense": "CC0-1.0", "SPDXID": "SPDXRef-DOCUMENT", "name": manifest.Metadata.Name, "documentNamespace": "https://amitia.dev/spdx/" + manifest.Metadata.ID + "/" + manifest.Metadata.Version, "creationInfo": map[string]interface{}{"created": "1970-01-01T00:00:00Z", "creators": []string{"Tool: Amitia"}}, "packages": []interface{}{}})
	files["SBOM.spdx.json"] = sbom
	files["checksums.sha256"] = buildChecksums(files)
	return files, nil
}

func (s *PackageService) exportAgentSkillsFiles(artifact packageArtifactRecord, name string) (map[string][]byte, error) {
	if artifact.ArtifactKind != "agent-skill" {
		return nil, NewExtensionError(ErrPackageExportNotAllowed, "只有 instructions Skill 可导出 AgentSkills ZIP", "", false, nil)
	}
	files, err := decodeAgentSkillArtifact(artifact.ContentBlob, DefaultAgentSkillLimits())
	if err != nil {
		return nil, err
	}
	result := make(map[string][]byte, len(files))
	for fileName, content := range files {
		result[name+"/"+fileName] = content
	}
	return result, nil
}

func scanPackageExportSecrets(files map[string][]byte) error {
	for name, content := range files {
		if secretPattern.Match(content) {
			return NewExtensionError(ErrPackageSecretDetected, "导出内容包含疑似 Secret，请改用 Secret Reference", name, false, nil)
		}
	}
	return nil
}

func (s *PackageService) rollbackLegacyPackage(ctx context.Context, extensionID, version, userID, scopeType, scopeID string) (result PackageOperationResult, err error) {
	if err := s.validatePackageScope(ctx, userID, scopeType, scopeID); err != nil {
		return PackageOperationResult{}, err
	}
	extension, err := s.repository.GetPackageExtension(ctx, extensionID, userID, scopeType, scopeID)
	if err != nil {
		return PackageOperationResult{}, err
	}
	if extension.CurrentVersion == version {
		return PackageOperationResult{OperationID: uuid.NewString(), TraceID: uuid.NewString(), Operation: PackageOperationRollback, ExtensionID: extensionID, Version: version, Enabled: extension.Enabled == 1, Status: "succeeded"}, nil
	}
	unlock, ok := s.lockExtension(extensionID)
	if !ok {
		return PackageOperationResult{}, NewExtensionError(ErrPackageOperationInProgress, "该扩展已有操作进行中", extensionID, true, nil)
	}
	defer unlock()
	target, err := s.repository.GetPackageVersion(ctx, extensionID, version)
	packageValid := len(target.PackageBlob) > 0 && packageHash(target.PackageBlob) == target.PackageHash || len(target.PackageBlob) == 0 && target.Source == "workshop" && target.ArtifactHash != "" && target.ArtifactHash == target.PackageHash
	if err != nil || !packageValid {
		return PackageOperationResult{}, NewExtensionError(ErrPackageArtifactInvalid, "历史版本包或 Hash 无效", version, false, err)
	}
	var artifact packageArtifactRecord
	if err := s.repository.db.WithContext(ctx).Where("artifact_id = ? AND archived_at = ''", target.ArtifactID).First(&artifact).Error; err != nil {
		return PackageOperationResult{}, NewExtensionError(ErrPackageArtifactInvalid, "历史 Artifact 不可用", version, false, err)
	}
	if err := s.revalidateRollbackTarget(ctx, target, artifact, userID, scopeType, scopeID); err != nil {
		return PackageOperationResult{}, err
	}
	var definition SkillDefinition
	var handler SkillHandler
	if artifact.ArtifactKind == "agent-skill" {
		var manifest Manifest
		if json.Unmarshal([]byte(artifact.ManifestJSON), &manifest) != nil {
			return PackageOperationResult{}, NewExtensionError(ErrPackageArtifactInvalid, "历史 Manifest 无效", version, false, nil)
		}
		definition = skillDefinitionFromManifest(manifest, map[string]json.RawMessage{})
		definition.Source = SkillSourceInstructions
	} else {
		definition, handler, err = s.workflowInstaller.definitionFromArtifact(artifact.base())
		if err != nil {
			return PackageOperationResult{}, NewExtensionError(ErrPackageRollbackFailed, "历史 Artifact 校验失败", "", false, err)
		}
	}
	for _, dependency := range definition.Dependencies {
		if _, dependencyErr := s.registry.Get(ctx, dependency); dependencyErr != nil {
			return PackageOperationResult{}, NewExtensionError(ErrPackageDependencyMissing, "回滚目标版本依赖缺失", dependency, false, dependencyErr)
		}
	}
	configMigrations, err := s.preparePackageConfigMigrations(ctx, extensionID, definition.ConfigSchema, definition.DefaultConfig)
	if err != nil {
		return PackageOperationResult{}, err
	}
	current, err := s.registry.Get(ctx, extensionID)
	if err != nil {
		return PackageOperationResult{}, err
	}
	definition.Enabled = current.Definition.Enabled
	_ = s.registry.Unregister(ctx, extensionID)
	if err := s.registry.Register(ctx, definition, handler); err != nil {
		_ = s.registry.Register(ctx, current.Definition, current.Handler)
		return PackageOperationResult{}, NewExtensionError(ErrPackageRollbackFailed, "Registry 回滚失败", "", false, err)
	}
	if err := s.registry.SetEnabled(ctx, extensionID, current.Definition.Enabled); err != nil {
		_ = s.registry.Unregister(ctx, extensionID)
		_ = s.registry.Register(ctx, current.Definition, current.Handler)
		return PackageOperationResult{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&extensionRecord{}).Where("extension_id = ?", extensionID).Updates(map[string]interface{}{"current_version": version, "manifest_json": string(definition.Manifest), "normalized_manifest_json": string(stableJSON(definition.Manifest)), "updated_at": now}).Error; err != nil {
			return err
		}
		if artifact.ArtifactKind == "agent-skill" {
			files, loadErr := decodeAgentSkillArtifact(artifact.ContentBlob, DefaultAgentSkillLimits())
			if loadErr != nil {
				return loadErr
			}
			parsed, parseErr := parseAgentSkillFiles(files, "", AgentSkillSourceZIP, DefaultAgentSkillLimits())
			if parseErr != nil {
				return parseErr
			}
			resources, _ := json.Marshal(parsed.Definition.Resources)
			mappings, _ := json.Marshal(parsed.Definition.ToolMappings)
			report, _ := json.Marshal(parsed.Report)
			return tx.Model(&agentSkillMetadataRecord{}).Where("extension_id = ?", extensionID).Updates(map[string]interface{}{"artifact_id": artifact.ArtifactID, "content_hash": parsed.Definition.ContentHash, "description": parsed.Definition.Description, "resource_index_json": string(resources), "tool_mappings_json": string(mappings), "compatibility_report_json": string(report), "compatibility_status": string(parsed.Report.Status), "updated_at": now}).Error
		}
		for _, migration := range configMigrations {
			if err := tx.Model(&configRecord{}).Where("id = ?", migration.ID).Updates(map[string]interface{}{"config_json": migration.Secured, "config_version": gorm.Expr("config_version + 1"), "updated_at": now}).Error; err != nil {
				return err
			}
		}
		if tx.Migrator().HasColumn("extension_versions", "activation_status") {
			if err := tx.Model(&packageVersionRecord{}).Where("extension_id = ? AND version = ?", extensionID, version).Updates(map[string]interface{}{"artifact_status": "active", "activation_status": "active", "failure_code": ""}).Error; err != nil {
				return err
			}
			if err := tx.Model(&packageVersionRecord{}).Where("extension_id = ? AND version <> ? AND activation_status = ?", extensionID, version, "active").Updates(map[string]interface{}{"activation_status": "archived", "artifact_status": "archived"}).Error; err != nil {
				return err
			}
		}
		if tx.Migrator().HasColumn("extension_artifacts", "artifact_status") {
			if err := tx.Model(&packageArtifactRecord{}).Where("artifact_id = ?", artifact.ArtifactID).Update("artifact_status", "active").Error; err != nil {
				return err
			}
			if err := tx.Model(&packageArtifactRecord{}).Where("extension_id = ? AND artifact_id <> ? AND artifact_status = ?", extensionID, artifact.ArtifactID, "active").Update("artifact_status", "archived").Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		_ = s.registry.Unregister(context.Background(), extensionID)
		_ = s.registry.Register(context.Background(), current.Definition, current.Handler)
		return PackageOperationResult{}, NewExtensionError(ErrPackageRollbackFailed, "数据库回滚失败", "", false, err)
	}
	operationID := uuid.NewString()
	traceID := uuid.NewString()
	record := packageOperationRecord{ID: operationID, ExtensionID: extensionID, ExtensionVersion: version, Operation: string(PackageOperationRollback), Source: target.Source, PackageHash: target.PackageHash, SignatureStatus: target.SignatureStatus, SignerFingerprint: target.SignerFingerprint, PreviousVersion: extension.CurrentVersion, TargetVersion: version, UserID: userID, ScopeType: scopeType, ScopeID: scopeID, Status: "succeeded", TraceID: traceID, CreatedAt: now, CompletedAt: now}
	_ = s.repository.CreatePackageOperation(ctx, record)
	if s.agentSkills != nil {
		s.agentSkills.invalidateAgentSkillCaches()
	}
	s.metric("extension_package_rollback_total")
	if s.kernelProxy != nil {
		_ = s.kernelProxy.NotifyInstall(ctx, extensionID, version)
	}
	return PackageOperationResult{OperationID: operationID, TraceID: traceID, Operation: PackageOperationRollback, ExtensionID: extensionID, Version: version, Enabled: current.Definition.Enabled, Status: "succeeded"}, nil
}

func (s *PackageService) revalidateRollbackTarget(ctx context.Context, target packageVersionRecord, artifact packageArtifactRecord, userID, scopeType, scopeID string) error {
	request := PreviewPackageImportRequest{UserID: userID, ScopeType: scopeType, ScopeID: scopeID, FileName: target.ExtensionID + ".amitiax", Raw: target.PackageBlob}
	if len(target.PackageBlob) > 0 {
		parsed, err := parsePackageInput(request, s.validator, s.limits)
		if err != nil {
			return NewExtensionError(ErrPackageRollbackFailed, "回滚目标包重新解析失败", "", false, err)
		}
		preview, err := s.buildPackagePreview(ctx, request, parsed)
		if err != nil {
			return NewExtensionError(ErrPackageRollbackFailed, "回滚目标包重新验证失败", "", false, err)
		}
		if preview.ID != target.ExtensionID || preview.Version != target.Version || len(preview.Errors) > 0 || !preview.Compatible {
			return NewExtensionError(ErrPackageRollbackFailed, "回滚目标包未通过重新验证", strings.Join(preview.Errors, "; "), false, nil)
		}
		return nil
	}
	if artifact.ArtifactKind != "workflow" {
		return nil
	}
	var manifest Manifest
	var workflow WorkflowDefinition
	var schemas map[string]json.RawMessage
	if json.Unmarshal([]byte(artifact.ManifestJSON), &manifest) != nil || json.Unmarshal([]byte(artifact.WorkflowJSON), &workflow) != nil || json.Unmarshal([]byte(artifact.SchemasJSON), &schemas) != nil {
		return NewExtensionError(ErrPackageRollbackFailed, "回滚目标 Workflow 制品无效", artifact.ArtifactID, false, nil)
	}
	compiled, issues, err := s.compiler.Compile(ctx, workflow)
	if err != nil {
		return NewExtensionError(ErrPackageRollbackFailed, "回滚目标 Workflow 重新编译失败", summarizeIssues(issues), false, err)
	}
	parsed := parsedExtensionPackage{Manifest: manifest, Workflow: &workflow, Tests: json.RawMessage(artifact.TestsJSON), Schemas: schemas}
	report := s.runPackageWorkflowTests(ctx, request, parsed, compiled)
	if report.Status != "passed" {
		return NewExtensionError(ErrPackageRollbackFailed, "回滚目标 Workflow 测试失败", report.Status, false, nil)
	}
	return nil
}

func (s *PackageService) legacyDependencies(ctx context.Context, extensionID, userID, scopeType, scopeID string) (map[string]interface{}, error) {
	extension, err := s.repository.GetPackageExtension(ctx, extensionID, userID, scopeType, scopeID)
	if err != nil {
		return nil, err
	}
	var dependencies []packageDependencyRecord
	if err := s.repository.db.WithContext(ctx).Where("extension_id = ? AND extension_version = ?", extensionID, extension.CurrentVersion).Find(&dependencies).Error; err != nil {
		return nil, err
	}
	direct := make([]PackageDependencyView, 0, len(dependencies))
	for _, dependency := range dependencies {
		direct = append(direct, s.resolvePackageDependency(ctx, dependency.DependencyID, dependency.Constraint))
	}
	reverse, err := s.repository.ReversePackageDependencies(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"dependencies": direct, "dependents": reverse}, nil
}

func (s *PackageService) legacyPreviewUninstall(ctx context.Context, extensionID, userID, scopeType, scopeID string) (PackageUninstallPreview, error) {
	extension, err := s.repository.GetPackageExtension(ctx, extensionID, userID, scopeType, scopeID)
	if err != nil {
		return PackageUninstallPreview{}, err
	}
	dependents, err := s.repository.ReversePackageDependencies(ctx, extensionID)
	if err != nil {
		return PackageUninstallPreview{}, err
	}
	preview := PackageUninstallPreview{ExtensionID: extensionID, CurrentVersion: extension.CurrentVersion, Enabled: extension.Enabled == 1, Dependents: dependents, Grants: []string{}, ArtifactArchived: true, Cleanup: []string{"运行时注册", "Agent Skill 索引", "自有定时任务", "Capability Grant", "当前配置与缓存"}, Preserved: []string{"版本历史", "安装与操作审计", "历史运行记录", "归档 Artifact"}, ReadSource: "legacy"}
	if count, countErr := s.repository.CountOwnedResources(ctx, extensionID, scopeType, scopeID); countErr == nil {
		preview.ScheduleCount = count
	} else {
		return PackageUninstallPreview{}, countErr
	}
	var grants []grantRecord
	_ = s.repository.db.WithContext(ctx).Where("extension_id = ? AND scope_type = ? AND scope_id = ?", extensionID, scopeType, scopeID).Find(&grants).Error
	for _, grant := range grants {
		preview.Grants = append(preview.Grants, grant.Capability+":"+grant.Decision)
	}
	var configCount int64
	_ = s.repository.db.WithContext(ctx).Model(&configRecord{}).Where("extension_id = ? AND scope_type = ? AND scope_id = ?", extensionID, scopeType, scopeID).Count(&configCount).Error
	preview.ConfigPresent = configCount > 0
	_ = s.repository.db.WithContext(ctx).Model(&runRecord{}).Where("extension_id = ? OR skill_id = ?", extensionID, extensionID).Count(&preview.HistoricalRuns).Error
	return preview, nil
}

func (s *PackageService) uninstallLegacyPackage(ctx context.Context, extensionID, userID, scopeType, scopeID string) (PackageOperationResult, error) {
	if err := s.validatePackageScope(ctx, userID, scopeType, scopeID); err != nil {
		return PackageOperationResult{}, err
	}
	extension, err := s.repository.GetPackageExtension(ctx, extensionID, userID, scopeType, scopeID)
	if err != nil {
		return PackageOperationResult{}, err
	}
	if extension.Source != string(SkillSourceWorkflow) && extension.Source != string(SkillSourceInstructions) {
		return PackageOperationResult{}, NewExtensionError(ErrPackageExportNotAllowed, "builtin 或官方 Plugin 不能通过本地包接口卸载", extensionID, false, nil)
	}
	dependents, err := s.repository.ReversePackageDependencies(ctx, extensionID)
	if err != nil {
		return PackageOperationResult{}, err
	}
	if len(dependents) > 0 {
		ids := make([]string, 0, len(dependents))
		for _, dependent := range dependents {
			ids = append(ids, dependent.ID)
		}
		return PackageOperationResult{}, NewExtensionError(ErrPackageDependencyInUse, "扩展仍被其他 workflow 依赖", strings.Join(ids, ", "), false, nil)
	}
	unlock, ok := s.lockExtension(extensionID)
	if !ok {
		return PackageOperationResult{}, NewExtensionError(ErrPackageOperationInProgress, "该扩展已有操作进行中", extensionID, true, nil)
	}
	defer unlock()
	if s.kernelProxy != nil {
		_ = s.kernelProxy.NotifyUninstall(ctx, extensionID)
	}
	if err := s.repository.CleanupOwnedResources(ctx, extensionID, scopeType, scopeID); err != nil {
		return PackageOperationResult{}, err
	}
	registered, err := s.registry.Get(ctx, extensionID)
	if err != nil {
		return PackageOperationResult{}, err
	}
	if err := s.registry.Unregister(ctx, extensionID); err != nil {
		return PackageOperationResult{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	transactionErr := s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&extensionRecord{}).Where("extension_id = ?", extensionID).Updates(map[string]interface{}{"enabled": 0, "archived_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&packageArtifactRecord{}).Where("extension_id = ?", extensionID).Update("archived_at", now).Error; err != nil {
			return err
		}
		if err := tx.Model(&agentSkillMetadataRecord{}).Where("extension_id = ?", extensionID).Updates(map[string]interface{}{"enabled": 0, "removed_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Where("extension_id = ?", extensionID).Delete(&grantRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&configRecord{}).Where("extension_id = ?", extensionID).Update("archived_at", now).Error; err != nil {
			return err
		}
		if tx.Migrator().HasTable("extension_schedules") {
			if err := tx.Table("extension_schedules").Where("plugin_id = ?", extensionID).Delete(nil).Error; err != nil {
				return err
			}
		}
		if tx.Migrator().HasTable(&scopeBindingRecord{}) {
			if err := tx.Where("extension_id = ?", extensionID).Delete(&scopeBindingRecord{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if transactionErr != nil {
		_ = s.registry.Register(context.Background(), registered.Definition, registered.Handler)
		return PackageOperationResult{}, NewExtensionError(ErrPackageUninstallFailed, "卸载事务失败", "", false, transactionErr)
	}
	if s.agentSkills != nil {
		s.agentSkills.clearExtensionFromRounds(extensionID)
		s.agentSkills.invalidateAgentSkillCaches()
	}
	operationID := uuid.NewString()
	traceID := uuid.NewString()
	record := packageOperationRecord{ID: operationID, ExtensionID: extensionID, ExtensionVersion: extension.CurrentVersion, Operation: string(PackageOperationUninstall), Source: extension.Source, PreviousVersion: extension.CurrentVersion, UserID: userID, ScopeType: scopeType, ScopeID: scopeID, Status: "succeeded", TraceID: traceID, CreatedAt: now, CompletedAt: now}
	_ = s.repository.CreatePackageOperation(ctx, record)
	s.metric("extension_package_uninstall_total")
	return PackageOperationResult{OperationID: operationID, TraceID: traceID, Operation: PackageOperationUninstall, ExtensionID: extensionID, Version: extension.CurrentVersion, Enabled: false, Status: "succeeded"}, nil
}

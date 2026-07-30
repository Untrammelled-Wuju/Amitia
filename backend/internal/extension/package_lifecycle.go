package extension

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/trust"
)

func (s *PackageService) Export(ctx context.Context, request ExportPackageRequest) (ExportedPackage, error) {
	if err := s.validatePackageScope(ctx, request.UserID, request.ScopeType, request.ScopeID); err != nil {
		return ExportedPackage{}, err
	}
	versionName := request.Version
	if s.readModel != nil && (request.Format == "" || request.Format == "amitiax") {
		if exported, ok, err := s.readModel.TryExport(ctx, request.ExtensionID, versionName, request.UserID, request.ScopeType, request.ScopeID); err != nil {
			return ExportedPackage{}, NewExtensionError(ErrPackageRepositoryUnavailable, "扩展内核读取失败", request.ExtensionID, true, err)
		} else if ok {
			s.metric("extension_package_export_total")
			return exported, nil
		}
	}
	if s.kernelProxy == nil || !s.kernelProxy.LegacyReadAllowed(ctx, request.ExtensionID) {
		return ExportedPackage{}, NewExtensionError(ErrPackageExportNotAllowed, "扩展不存在于 Kernel，且不处于人工迁移状态", request.ExtensionID, false, nil)
	}
	kernelruntime.GlobalLegacyReadCounter().IncExport()
	kernelruntime.GlobalLegacyReadCounter().IncPackageReadCalls()
	extension, err := s.repository.GetPackageExtension(ctx, request.ExtensionID, request.UserID, request.ScopeType, request.ScopeID)
	if err != nil {
		return ExportedPackage{}, NewExtensionError(ErrPackageExportNotAllowed, "扩展不可导出", request.ExtensionID, false, err)
	}
	if versionName == "" {
		versionName = extension.CurrentVersion
	}
	version, err := s.repository.GetPackageVersion(ctx, request.ExtensionID, versionName)
	if err != nil {
		return ExportedPackage{}, NewExtensionError(ErrPackageExportNotAllowed, "扩展版本缺少可导出 Artifact", versionName, false, err)
	}
	if version.ArtifactID == "" {
		var fallback packageArtifactRecord
		if fallbackErr := s.repository.db.WithContext(ctx).Where("extension_id = ? AND extension_version = ?", request.ExtensionID, versionName).First(&fallback).Error; fallbackErr != nil {
			return ExportedPackage{}, NewExtensionError(ErrPackageExportNotAllowed, "扩展版本缺少可导出 Artifact", versionName, false, fallbackErr)
		}
		version.ArtifactID = fallback.ArtifactID
	}
	var artifact packageArtifactRecord
	if err := s.repository.db.WithContext(ctx).Where("artifact_id = ?", version.ArtifactID).First(&artifact).Error; err != nil {
		return ExportedPackage{}, NewExtensionError(ErrPackageArtifactInvalid, "Artifact 不存在", version.ArtifactID, false, err)
	}
	var files map[string][]byte
	fileName := extension.Name + "-" + versionName + ".amitiax"
	mime := "application/vnd.amitia.extension+zip"
	if artifact.ArtifactKind == "agent-skill" && request.Format == "agentskills-zip" {
		files, err = s.exportAgentSkillsFiles(artifact, extension.Name)
		fileName = extension.Name + "-" + versionName + ".zip"
		mime = "application/zip"
	} else if request.Format == "amitiax" {
		files, err = s.exportAmitiaxFiles(artifact)
	} else {
		return ExportedPackage{}, NewExtensionError(ErrPackageExportNotAllowed, "该 Skill 不支持所选导出格式", request.Format, false, nil)
	}
	if err != nil {
		return ExportedPackage{}, err
	}
	if err := scanPackageExportSecrets(files); err != nil {
		s.metric("extension_package_secret_detected_total")
		return ExportedPackage{}, err
	}
	raw, err := stablePackageZIP(files)
	if err != nil {
		return ExportedPackage{}, err
	}
	exported := ExportedPackage{ExportID: uuid.NewString(), FileName: safePackageFileName(fileName), MIME: mime, Size: int64(len(raw)), Hash: packageHash(raw), Version: versionName, Format: request.Format, SecretScan: "passed", SignatureStatus: version.SignatureStatus, ExpiresAt: time.Now().UTC().Add(15 * time.Minute), Content: raw}
	for name := range files {
		lower := strings.ToLower(name)
		exported.TestsIncluded = exported.TestsIncluded || strings.HasPrefix(lower, "tests/") || strings.Contains(lower, "/tests/")
		exported.ReadmeIncluded = exported.ReadmeIncluded || strings.HasSuffix(lower, "/readme.md") || lower == "readme.md"
		exported.SBOMIncluded = exported.SBOMIncluded || strings.HasSuffix(lower, "sbom.spdx.json")
		exported.ScriptsIncluded = exported.ScriptsIncluded || strings.Contains(lower, "/scripts/") || strings.HasPrefix(lower, "scripts/")
	}
	if err := s.repository.SavePackageExport(ctx, request.UserID, exported, request.ExtensionID); err != nil {
		return ExportedPackage{}, err
	}
	s.metric("extension_package_export_total")
	return exported, nil
}

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

func safePackageFileName(name string) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	var builder strings.Builder
	for _, char := range name {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("._-", char) {
			builder.WriteRune(char)
		} else {
			builder.WriteRune('-')
		}
	}
	return strings.Trim(builder.String(), ".-")
}

func (s *PackageService) GetExport(ctx context.Context, userID, extensionID, exportID string) (ExportedPackage, error) {
	if s.kernelProxy == nil || s.kernelProxy.ReadContainer() == nil {
		return ExportedPackage{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	container := s.kernelProxy.ReadContainer()
	ticket, err := container.PackageRepository.GetExport(ctx, exportID, userID, extensionID)
	if err != nil {
		return ExportedPackage{}, NewExtensionError(ErrPackageExportNotAllowed, "导出凭据不存在", exportID, false, err)
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, ticket.ExpiresAt)
	if err != nil || time.Now().UTC().After(expiresAt) {
		return ExportedPackage{}, NewExtensionError(ErrPackageExportNotAllowed, "导出凭据已过期", exportID, false, err)
	}
	artifact, err := container.PackageRepository.GetArtifact(ctx, ticket.ArtifactID)
	if err != nil {
		return ExportedPackage{}, err
	}
	if _, err := s.kernelProxy.kernel.VerifyStoredPackage(ctx, artifact); err != nil {
		return ExportedPackage{}, NewExtensionError(ErrPackageArtifactInvalid, "导出前制品复验失败", artifact.ArtifactID, false, err)
	}
	return ExportedPackage{ExportID: exportID, FileName: ticket.FileName, MIME: ticket.MIMEType,
		Size: artifact.SizeBytes, Hash: artifact.ArchiveHash, Version: artifact.Version,
		Format: "amitiax", SecretScan: "passed", SignatureStatus: artifact.SignatureStatus,
		ExpiresAt: expiresAt, LocalPath: artifact.ArchivePath}, nil
}

func (s *PackageService) ListVersions(ctx context.Context, extensionID, userID, scopeType, scopeID string) ([]PackageVersionView, error) {
	if err := s.validatePackageScope(ctx, userID, scopeType, scopeID); err != nil {
		return nil, err
	}
	if s.readModel != nil {
		if result, ok, err := s.readModel.TryListVersions(ctx, extensionID); err != nil {
			return nil, NewExtensionError(ErrPackageRepositoryUnavailable, "扩展内核读取失败", extensionID, true, err)
		} else if ok {
			return result, nil
		}
	}
	if s.kernelProxy == nil || !s.kernelProxy.LegacyReadAllowed(ctx, extensionID) {
		return nil, NewExtensionError(ErrPackageRepositoryUnavailable, "扩展不存在于 Kernel，且不允许 Legacy Read", extensionID, false, nil)
	}
	kernelruntime.GlobalLegacyReadCounter().IncListVersions()
	kernelruntime.GlobalLegacyReadCounter().IncPackageReadCalls()
	return s.repository.ListPackageVersions(ctx, extensionID, userID, scopeType, scopeID)
}

func (s *PackageService) CompareVersions(ctx context.Context, extensionID, userID, scopeType, scopeID, fromVersion, toVersion string) (PackageVersionDiff, error) {
	if err := s.validatePackageScope(ctx, userID, scopeType, scopeID); err != nil {
		return PackageVersionDiff{}, err
	}
	if s.readModel != nil {
		if result, ok, err := s.readModel.TryCompareVersions(ctx, extensionID, fromVersion, toVersion); err != nil {
			return PackageVersionDiff{}, NewExtensionError(ErrPackageRepositoryUnavailable, "扩展内核读取失败", extensionID, true, err)
		} else if ok {
			return result, nil
		}
	}
	if s.kernelProxy == nil || !s.kernelProxy.LegacyReadAllowed(ctx, extensionID) {
		return PackageVersionDiff{}, NewExtensionError(ErrPackageRepositoryUnavailable, "扩展不存在于 Kernel，且不允许 Legacy Read", extensionID, false, nil)
	}
	kernelruntime.GlobalLegacyReadCounter().IncCompareVersions()
	kernelruntime.GlobalLegacyReadCounter().IncPackageReadCalls()
	if _, err := s.repository.GetPackageExtension(ctx, extensionID, userID, scopeType, scopeID); err != nil {
		return PackageVersionDiff{}, err
	}
	from, err := s.repository.GetPackageVersion(ctx, extensionID, fromVersion)
	if err != nil {
		return PackageVersionDiff{}, err
	}
	to, err := s.repository.GetPackageVersion(ctx, extensionID, toVersion)
	if err != nil {
		return PackageVersionDiff{}, err
	}
	var fromArtifact, toArtifact packageArtifactRecord
	var fromArtifactPointer, toArtifactPointer *packageArtifactRecord
	if s.repository.db.WithContext(ctx).Where("artifact_id = ?", from.ArtifactID).First(&fromArtifact).Error == nil {
		fromArtifactPointer = &fromArtifact
	}
	if s.repository.db.WithContext(ctx).Where("artifact_id = ?", to.ArtifactID).First(&toArtifact).Error == nil {
		toArtifactPointer = &toArtifact
	}
	return packageVersionDiffRecords(extensionID, from, to, fromArtifactPointer, toArtifactPointer), nil
}

func packageVersionDiffRecords(extensionID string, from, to packageVersionRecord, fromArtifact, toArtifact *packageArtifactRecord) PackageVersionDiff {
	diff := PackageVersionDiff{ExtensionID: extensionID, FromVersion: from.Version, ToVersion: to.Version, Manifest: jsonObjectDiff(from.ManifestJSON, to.ManifestJSON), Schemas: map[string]interface{}{}, Workflow: map[string]interface{}{}, Instructions: map[string]interface{}{}, Capabilities: map[string][]string{}, Signature: map[string]string{"from": from.SignatureStatus, "to": to.SignatureStatus}, Scripts: map[string][]string{"added": {}, "removed": {}}, Dependencies: map[string][]string{"added": {}, "removed": {}}, Trust: map[string]string{"from": from.SignatureStatus, "to": to.SignatureStatus}, Risks: []PackageRisk{}}
	var fromCapabilities, toCapabilities []string
	_ = json.Unmarshal([]byte(from.CapabilitiesJSON), &fromCapabilities)
	_ = json.Unmarshal([]byte(to.CapabilitiesJSON), &toCapabilities)
	diff.Capabilities["added"] = stringSetDifference(toCapabilities, fromCapabilities)
	diff.Capabilities["removed"] = stringSetDifference(fromCapabilities, toCapabilities)
	if len(diff.Capabilities["added"]) > 0 {
		diff.Risks = append(diff.Risks, PackageRisk{Code: "CAPABILITY_ADDED", Severity: "high", Message: strings.Join(diff.Capabilities["added"], ", ")})
	}
	if fromArtifact != nil && toArtifact != nil {
		diff.Schemas = jsonObjectDiff(fromArtifact.SchemasJSON, toArtifact.SchemasJSON)
		if fromArtifact.ArtifactKind == "agent-skill" || toArtifact.ArtifactKind == "agent-skill" {
			diff.Instructions = agentArtifactDiff(*fromArtifact, *toArtifact)
		} else {
			diff.Workflow = jsonObjectDiff(fromArtifact.WorkflowJSON, toArtifact.WorkflowJSON)
		}
	}
	if from.SignerFingerprint != to.SignerFingerprint {
		diff.Signature["fingerprintFrom"] = from.SignerFingerprint
		diff.Signature["fingerprintTo"] = to.SignerFingerprint
		diff.Risks = append(diff.Risks, PackageRisk{Code: "SIGNER_CHANGED", Severity: "high", Message: "签名者发生变化"})
	}
	return diff
}

func jsonObjectDiff(fromRaw, toRaw string) map[string]interface{} {
	var from, to interface{}
	if json.Unmarshal([]byte(fromRaw), &from) != nil || json.Unmarshal([]byte(toRaw), &to) != nil {
		return map[string]interface{}{"changed": fromRaw != toRaw}
	}
	fromJSON, _ := json.Marshal(from)
	toJSON, _ := json.Marshal(to)
	return map[string]interface{}{"changed": string(fromJSON) != string(toJSON), "from": from, "to": to}
}

func agentArtifactDiff(from, to packageArtifactRecord) map[string]interface{} {
	fromFiles, fromErr := decodeAgentSkillArtifact(from.ContentBlob, DefaultAgentSkillLimits())
	toFiles, toErr := decodeAgentSkillArtifact(to.ContentBlob, DefaultAgentSkillLimits())
	if fromErr != nil || toErr != nil {
		return map[string]interface{}{"changed": true, "error": ErrPackageArtifactInvalid}
	}
	fromNames := make([]string, 0, len(fromFiles))
	toNames := make([]string, 0, len(toFiles))
	for name := range fromFiles {
		fromNames = append(fromNames, name)
	}
	for name := range toFiles {
		toNames = append(toNames, name)
	}
	sort.Strings(fromNames)
	sort.Strings(toNames)
	return map[string]interface{}{"skillChanged": string(fromFiles["SKILL.md"]) != string(toFiles["SKILL.md"]), "added": stringSetDifference(toNames, fromNames), "removed": stringSetDifference(fromNames, toNames)}
}

func stringSetDifference(left, right []string) []string {
	rightSet := map[string]bool{}
	for _, value := range right {
		rightSet[value] = true
	}
	result := []string{}
	for _, value := range left {
		if !rightSet[value] {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func (s *PackageService) Rollback(ctx context.Context, extensionID, version, userID, scopeType, scopeID string) (PackageOperationResult, error) {
	if s.kernelProxy == nil {
		return PackageOperationResult{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	if err := s.validatePackageScope(ctx, userID, scopeType, scopeID); err != nil {
		return PackageOperationResult{}, err
	}
	result, err := s.kernelProxy.kernel.ExecutePackageRollback(ctx, extensionID, version, userID, scopeType, scopeID)
	if err != nil {
		return PackageOperationResult{}, NewExtensionError(ErrPackageRollbackFailed, "Extension Kernel 回滚失败", err.Error(), false, err)
	}
	return PackageOperationResult{OperationID: result.OperationID, TraceID: result.TraceID, Operation: PackageOperationRollback, ExtensionID: extensionID, Version: version, Enabled: false, Status: "succeeded"}, nil
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

func (s *PackageService) Dependencies(ctx context.Context, extensionID, userID, scopeType, scopeID string) (map[string]interface{}, error) {
	if err := s.validatePackageScope(ctx, userID, scopeType, scopeID); err != nil {
		return nil, err
	}
	if s.readModel != nil {
		if result, ok, err := s.readModel.TryDependencies(ctx, extensionID); err != nil {
			return nil, NewExtensionError(ErrPackageRepositoryUnavailable, "扩展内核读取失败", extensionID, true, err)
		} else if ok {
			return result, nil
		}
	}
	if s.kernelProxy == nil || !s.kernelProxy.LegacyReadAllowed(ctx, extensionID) {
		return nil, NewExtensionError(ErrPackageRepositoryUnavailable, "扩展不存在于 Kernel，且不允许 Legacy Read", extensionID, false, nil)
	}
	kernelruntime.GlobalLegacyReadCounter().IncDependencies()
	kernelruntime.GlobalLegacyReadCounter().IncPackageReadCalls()
	return s.legacyDependencies(ctx, extensionID, userID, scopeType, scopeID)
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

func (s *PackageService) PreviewUninstall(ctx context.Context, extensionID, userID, scopeType, scopeID string) (PackageUninstallPreview, error) {
	if err := s.validatePackageScope(ctx, userID, scopeType, scopeID); err != nil {
		return PackageUninstallPreview{}, err
	}
	if s.kernelProxy == nil || s.kernelProxy.kernel == nil {
		return PackageUninstallPreview{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	preview, err := s.kernelProxy.kernel.PreviewPackageUninstall(ctx, extensionID, userID, scopeType, scopeID)
	if err != nil {
		return PackageUninstallPreview{}, NewExtensionError(ErrPackageRepositoryUnavailable, "扩展内核读取失败", extensionID, true, err)
	}
	result := PackageUninstallPreview{ExtensionID: preview.ExtensionID, CurrentVersion: preview.CurrentVersion,
		Enabled: preview.Enabled, Dependents: []PackageDependencyView{}, ArtifactArchived: true,
		Cleanup:   []string{"Kernel 定义", "Module", "Contribution", "Installed Tree"},
		Preserved: []string{"Artifact", "Operation", "Rollback Point"}, ReadSource: "kernel"}
	for _, dependent := range preview.Dependents {
		result.Dependents = append(result.Dependents, PackageDependencyView{ID: dependent, Required: true, Installed: true})
	}
	return result, nil
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

func (s *PackageService) Uninstall(ctx context.Context, extensionID, userID, scopeType, scopeID string) (PackageOperationResult, error) {
	if s.kernelProxy == nil {
		return PackageOperationResult{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	if err := s.validatePackageScope(ctx, userID, scopeType, scopeID); err != nil {
		return PackageOperationResult{}, err
	}
	op, err := s.kernelProxy.kernel.ExecutePackageUninstall(ctx, extensionID, userID, scopeType, scopeID)
	if err != nil {
		return PackageOperationResult{}, NewExtensionError(ErrPackageUninstallFailed, "Extension Kernel 卸载失败", err.Error(), false, err)
	}
	return PackageOperationResult{OperationID: op.OperationID, TraceID: op.TraceID, Operation: PackageOperationUninstall, ExtensionID: extensionID, Status: "succeeded"}, nil
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

func (s *PackageService) validatePackageScope(ctx context.Context, userID, scopeType, scopeID string) error {
	if scopeType == string(ScopeCharacter) {
		return s.repository.ValidateCharacterScope(ctx, ExecutionScope{UserID: userID, CharacterID: scopeID})
	}
	if scopeType != "" && scopeType != string(ScopeGlobal) {
		return NewExtensionError(ErrSkillPermissionDenied, "扩展作用域无效", scopeType, false, nil)
	}
	return nil
}

func (s *PackageService) ListOperations(ctx context.Context, userID string, limit int) ([]PackageOperationView, error) {
	if s.kernelProxy == nil || s.kernelProxy.ReadContainer() == nil {
		return nil, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	records, err := s.kernelProxy.ReadContainer().PackageRepository.ListOperations(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	result := make([]PackageOperationView, 0, len(records))
	for _, record := range records {
		result = append(result, kernelPackageOperationView(record, nil))
	}
	return result, nil
}

func (s *PackageService) GetOperation(ctx context.Context, userID, id string) (PackageOperationView, error) {
	if s.kernelProxy == nil || s.kernelProxy.ReadContainer() == nil {
		return PackageOperationView{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	record, steps, err := s.kernelProxy.ReadContainer().PackageRepository.GetOperation(ctx, userID, id)
	if err != nil {
		return PackageOperationView{}, err
	}
	return kernelPackageOperationView(record, steps), nil
}

func kernelPackageOperationView(record kernelruntime.PackageOperationRecord, steps []kernelruntime.PackageOperationStep) PackageOperationView {
	view := PackageOperationView{ID: record.OperationID, Operation: PackageOperation(record.OperationType),
		ExtensionID: record.ExtensionID, TargetVersion: record.TargetVersion, Source: "kernel",
		ScopeType: record.ScopeType, ScopeID: record.ScopeID, Status: record.Status,
		ErrorCode: record.ErrorCode, TraceID: record.TraceID, CreatedAt: record.StartedAt,
		CompletedAt: record.CompletedAt, CurrentStep: record.CurrentStep}
	for _, step := range steps {
		view.Steps = append(view.Steps, PackageOperationStepView{Name: step.StepName, Order: step.StepOrder,
			Status: step.Status, AttemptCount: step.AttemptCount, ResultJSON: step.ResultJSON,
			ErrorCode: step.ErrorCode, StartedAt: step.StartedAt, CompletedAt: step.CompletedAt})
	}
	return view
}

func (s *PackageService) ListSigners(ctx context.Context) ([]PackageSignerView, error) {
	if s.kernelProxy == nil || s.kernelProxy.ReadContainer() == nil {
		return nil, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	records, err := s.kernelProxy.ReadContainer().PackageTrustRepository.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]PackageSignerView, 0, len(records))
	for _, record := range records {
		result = append(result, PackageSignerView{Fingerprint: record.Fingerprint, KeyID: record.KeyID,
			PublisherID: record.PublisherID, Algorithm: "ed25519", DisplayName: record.PublisherID,
			Trusted:     record.TrustLevel == "user_trusted" && record.KeyState == "active",
			TrustSource: record.TrustSource, TrustLevel: record.TrustLevel, KeyState: record.KeyState,
			TrustedAt: record.TrustedAt, RevokedAt: record.RevokedAt})
	}
	return result, nil
}

func (s *PackageService) TrustSigner(ctx context.Context, fingerprint string) error {
	return s.setKernelSignerTrust(ctx, fingerprint, true)
}

func (s *PackageService) RegisterSigner(ctx context.Context, fingerprint, publisherID, keyID string, publicKey []byte) error {
	if s.kernelProxy == nil || s.kernelProxy.ReadContainer() == nil {
		return NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	now := time.Now().UTC()
	key := trust.PublisherKey{KeyID: keyID, PublisherID: publisherID, PublicKey: publicKey,
		Algorithm: trust.AlgorithmEd25519, State: trust.KeyStateActive, CreatedAt: now}
	if err := key.Validate(); err != nil || key.Fingerprint() != fingerprint {
		return NewExtensionError(ErrPackageSignatureInvalid, "发布者密钥无效或指纹不匹配", fingerprint, false, err)
	}
	record := kernelruntime.PackagePublisherKeyRecord{KeyID: keyID, Fingerprint: fingerprint,
		PublicKey: publicKey, PublisherID: publisherID, TrustSource: string(trust.TrustSourceUserDecision),
		TrustLevel: string(trust.TrustLevelUnknown), KeyState: string(trust.KeyStateActive),
		CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano)}
	container := s.kernelProxy.ReadContainer()
	newValue, err := json.Marshal(record)
	if err != nil {
		return err
	}
	coordinator := kernelruntime.NewPackageTrustMutationCoordinator(container)
	_, err = coordinator.Execute(ctx, trust.PolicyMutation{Kind: trust.PolicyMutationPublisherTrust,
		Actor: "authenticated_user", Reason: "register signer", PublisherID: publisherID, KeyID: keyID,
		NewValue: newValue, Restrictive: true})
	return err
}

func (s *PackageService) UntrustSigner(ctx context.Context, fingerprint string) error {
	return s.setKernelSignerTrust(ctx, fingerprint, false)
}

func (s *PackageService) setKernelSignerTrust(ctx context.Context, fingerprint string, trusted bool) error {
	if s.kernelProxy == nil || s.kernelProxy.ReadContainer() == nil {
		return NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	container := s.kernelProxy.ReadContainer()
	record, err := container.PackageTrustRepository.GetByFingerprint(ctx, fingerprint)
	if err != nil {
		return NewExtensionError(ErrPackageSignatureInvalid, "签名密钥不可用", fingerprint, false, err)
	}
	if len(record.PublicKey) != ed25519.PublicKeySize || record.TrustSource == "legacy_fingerprint_only" {
		return NewExtensionError(ErrPackageSignatureInvalid, "签名密钥不可用", fingerprint, false, nil)
	}
	before := record
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if trusted {
		record.TrustLevel = string(trust.TrustLevelUserTrusted)
		record.KeyState = string(trust.KeyStateActive)
		record.TrustedAt = now
		record.RevokedAt = ""
		record.RevocationReason = ""
	} else {
		record.TrustLevel = string(trust.TrustLevelRevoked)
		record.KeyState = string(trust.KeyStateRevoked)
		record.RevokedAt = now
		record.RevocationReason = "user decision"
	}
	record.UpdatedAt = now
	oldValue, err := json.Marshal(before)
	if err != nil {
		return err
	}
	newValue, err := json.Marshal(record)
	if err != nil {
		return err
	}
	reason := "user revoked signer trust"
	if trusted {
		reason = "user trusted signer"
	}
	coordinator := kernelruntime.NewPackageTrustMutationCoordinator(container)
	_, err = coordinator.Execute(ctx, trust.PolicyMutation{Kind: trust.PolicyMutationPublisherTrust,
		Actor: "authenticated_user", Reason: reason, PublisherID: record.PublisherID, KeyID: record.KeyID,
		OldValue: oldValue, NewValue: newValue, Restrictive: !trusted})
	return err
}

func (s *PackageService) PreviewRollback(ctx context.Context, extensionID, version, userID, scopeType, scopeID string) (kernelruntime.PackageRollbackPreviewResult, error) {
	if s.kernelProxy == nil || s.kernelProxy.kernel == nil {
		return kernelruntime.PackageRollbackPreviewResult{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	if err := s.validatePackageScope(ctx, userID, scopeType, scopeID); err != nil {
		return kernelruntime.PackageRollbackPreviewResult{}, err
	}
	return s.kernelProxy.kernel.PreviewPackageRollback(ctx, extensionID, version, userID, scopeType, scopeID)
}

func (s *PackageService) VerifyOperationFinalGate(ctx context.Context, operationID string) (kernelruntime.PackageFinalGateResult, error) {
	if s.kernelProxy == nil || s.kernelProxy.kernel == nil {
		return kernelruntime.PackageFinalGateResult{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	return s.kernelProxy.kernel.VerifyPackageFinalGate(ctx, operationID)
}

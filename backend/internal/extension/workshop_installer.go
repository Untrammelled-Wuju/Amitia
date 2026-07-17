package extension

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WorkshopInstaller struct {
	repository *WorkshopRepository
	registry   *Registry
	compiler   *WorkflowCompiler
	executor   *WorkflowExecutor
	validator  *SchemaValidator
}

func NewWorkshopInstaller(repository *WorkshopRepository, registry *Registry, compiler *WorkflowCompiler, executor *WorkflowExecutor, validator *SchemaValidator) *WorkshopInstaller {
	return &WorkshopInstaller{repository: repository, registry: registry, compiler: compiler, executor: executor, validator: validator}
}

func (i *WorkshopInstaller) Install(ctx context.Context, scope ExecutionScope, sessionID string, revision int64) (WorkshopInstallResult, error) {
	session, sessionRecord, err := i.repository.GetSession(ctx, scope, sessionID)
	if err != nil {
		return WorkshopInstallResult{}, err
	}
	if session.Status == WorkshopInstalled && session.CurrentRevision == revision {
		return WorkshopInstallResult{SessionID: sessionID, SkillID: session.InstalledSkillID, Version: session.InstalledVersion, Enabled: false}, nil
	}
	if session.Status != WorkshopInstalling || revision != session.CurrentRevision {
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopInvalidState, "只有当前且测试通过的修订可以安装", string(session.Status), false, nil)
	}
	view, _, err := i.repository.GetRevision(ctx, scope, sessionID, revision)
	if err != nil {
		return WorkshopInstallResult{}, err
	}
	draft := view.NormalizedDraft
	compiled, issues, err := i.compiler.Compile(ctx, draft.Workflow)
	if err != nil {
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopStaticAnalysisFailed, "安装前工作流重新校验失败", summarizeIssues(issues), false, err)
	}
	if cycleIssues := i.compiler.AnalyzeDependencyCycles(ctx, draft.Metadata.ID, compiled.Dependencies); len(cycleIssues) > 0 {
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopDependencyCycle, "安装前检测到 Skill 调用循环", summarizeIssues(cycleIssues), false, nil)
	}
	if compiled.Checksum != view.WorkflowChecksum {
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopChecksumMismatch, "工作流 Checksum 已变化", "", false, nil)
	}
	if view.Validation == nil || !view.Validation.Valid || view.Validation.WorkflowChecksum != compiled.Checksum {
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopRevisionConflict, "Validation 已失效", "", false, nil)
	}
	if sessionRecord.PermissionRevision != revision || sessionRecord.PermissionChecksum != compiled.Checksum {
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopPermissionStale, "生产权限确认已失效", "", false, nil)
	}
	var confirmation PermissionConfirmation
	if json.Unmarshal([]byte(sessionRecord.PermissionConfirmationJSON), &confirmation) != nil || !confirmation.Production {
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopPermissionRequired, "安装需要独立的生产权限确认", "", false, nil)
	}
	passed, err := i.repository.LatestPassedTest(ctx, sessionID, revision, compiled.Checksum)
	if err != nil {
		return WorkshopInstallResult{}, err
	}
	if passed == nil {
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopTestRequired, "需要当前修订的通过测试", "", false, nil)
	}
	manifestRaw, _ := json.Marshal(draft.Manifest)
	if err := i.validator.ValidateManifest(manifestRaw); err != nil {
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopManifestInvalid, "Manifest 校验失败", err.Error(), false, err)
	}
	if draft.Manifest.Metadata.ID != draft.Metadata.ID || draft.Manifest.Metadata.Version != draft.Metadata.Version || draft.Manifest.Entry.Kind != "workflow" {
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopManifestInvalid, "Manifest 与 Draft 不一致", "", false, nil)
	}
	definition := skillDefinitionFromDraft(draft, compiled)
	artifact, err := buildArtifact(sessionID, revision, draft, compiled, passed.TestRunID)
	if err != nil {
		return WorkshopInstallResult{}, err
	}
	definition.Entry.ArtifactID = artifact.ArtifactID
	definition.Manifest = replaceManifestArtifact(definition.Manifest, artifact.ArtifactID)
	var oldRegistered *RegisteredSkill
	if current, getErr := i.registry.Get(ctx, definition.ID); getErr == nil {
		oldRegistered = &current
		if current.Definition.Version == definition.Version {
			return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopVersionConflict, "该 Skill 版本已存在", definition.Version, false, nil)
		}
		if compareSemver(definition.Version, current.Definition.Version) <= 0 {
			return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopVersionConflict, "更新版本必须高于当前版本", definition.Version, false, nil)
		}
		if _, breaking := suggestWorkshopVersion(current.Definition, draft); len(breaking) > 0 && semverMajor(definition.Version) <= semverMajor(current.Definition.Version) {
			return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopVersionConflict, "不兼容 Schema 变更必须增加 MAJOR 版本", strings.Join(breaking, "; "), false, nil)
		}
	}
	handler := i.workflowHandler(artifact, definition.OutputSchema)
	err = i.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existingArtifact extensionArtifactRecord
		artifactErr := tx.Where("extension_id = ? AND extension_version = ?", definition.ID, definition.Version).First(&existingArtifact).Error
		if artifactErr == nil {
			return NewExtensionError(ErrWorkshopVersionConflict, "制品版本已存在", definition.Version, false, nil)
		}
		if !errors.Is(artifactErr, gorm.ErrRecordNotFound) {
			return artifactErr
		}
		if err := tx.Create(&artifact).Error; err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		record := extensionRecord{ID: uuid.New().String(), ExtensionID: definition.ID, Kind: "Skill", Name: definition.Name, CurrentVersion: definition.Version, Source: string(SkillSourceWorkflow), Enabled: 0, ManifestJSON: string(definition.Manifest), NormalizedManifestJSON: string(stableJSON(definition.Manifest)), CreatedAt: now, UpdatedAt: now}
		if oldRegistered != nil {
			var existing extensionRecord
			if err := tx.Where("extension_id = ?", definition.ID).First(&existing).Error; err != nil {
				return err
			}
			record.ID = existing.ID
			record.CreatedAt = existing.CreatedAt
			record.Enabled = existing.Enabled
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "extension_id"}}, DoUpdates: clause.AssignmentColumns([]string{"name", "current_version", "source", "manifest_json", "normalized_manifest_json", "updated_at"})}).Create(&record).Error; err != nil {
			return err
		}
		version := extensionVersionRecord{ID: uuid.New().String(), ExtensionID: definition.ID, Version: definition.Version, ManifestJSON: string(definition.Manifest), Checksum: artifact.Checksum, CreatedAt: now}
		if err := tx.Create(&version).Error; err != nil {
			return err
		}
		if tx.Migrator().HasColumn("extensions", "owner_user_id") {
			scopeType := string(ScopeGlobal)
			if scope.CharacterID != "" {
				scopeType = string(ScopeCharacter)
			}
			if err := tx.Table("extensions").Where("extension_id = ?", definition.ID).Updates(map[string]interface{}{"owner_user_id": scope.UserID, "scope_type": scopeType, "scope_id": scope.CharacterID}).Error; err != nil {
				return err
			}
			capabilities, _ := json.Marshal(definition.Capabilities)
			if err := tx.Table("extension_versions").Where("extension_id = ? AND version = ?", definition.ID, definition.Version).Updates(map[string]interface{}{"artifact_id": artifact.ArtifactID, "artifact_hash": artifact.Checksum, "package_hash": artifact.Checksum, "source": "workshop", "signature_status": "local-generated", "compatibility_status": "compatible", "capabilities_json": string(capabilities), "installed_by": scope.UserID, "validation_status": "valid", "test_status": "passed"}).Error; err != nil {
				return err
			}
		}
		update := tx.Model(&workshopSessionRecord{}).Where("id = ? AND current_revision = ? AND lock_version = ?", sessionID, revision, sessionRecord.LockVersion).Updates(map[string]interface{}{"status": string(WorkshopInstalled), "installed_skill_id": definition.ID, "installed_version": definition.Version, "lock_version": gorm.Expr("lock_version + 1"), "updated_at": now})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return NewExtensionError(ErrWorkshopRevisionConflict, "安装状态已变化", sessionID, true, nil)
		}
		return insertWorkshopAudit(tx, sessionRecord, scope, WorkshopInstalling, WorkshopInstalled, "revision.installed", revision, compiled.Checksum, "", "", 0)
	})
	if err != nil {
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopInstallFailed, "安装事务失败", err.Error(), false, err)
	}
	if oldRegistered != nil {
		_ = i.registry.Unregister(ctx, definition.ID)
	}
	if err := i.registry.Register(ctx, definition, handler); err != nil {
		if oldRegistered != nil {
			_ = i.registry.Register(ctx, oldRegistered.Definition, oldRegistered.Handler)
		}
		_ = i.repository.db.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
			_ = tx.Where("artifact_id = ?", artifact.ArtifactID).Delete(&extensionArtifactRecord{}).Error
			_ = tx.Where("extension_id = ? AND version = ?", definition.ID, definition.Version).Delete(&extensionVersionRecord{}).Error
			_ = tx.Model(&workshopSessionRecord{}).Where("id = ? AND current_revision = ? AND status = ?", sessionID, revision, string(WorkshopInstalled)).Updates(map[string]interface{}{"status": string(WorkshopTestPassed), "installed_skill_id": session.InstalledSkillID, "installed_version": session.InstalledVersion, "lock_version": gorm.Expr("lock_version + 1"), "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}).Error
			_ = insertWorkshopAudit(tx, sessionRecord, scope, WorkshopInstalled, WorkshopTestPassed, "revision.install.registry_failed", revision, compiled.Checksum, "", ErrWorkshopInstallFailed, 0)
			if oldRegistered != nil {
				return tx.Model(&extensionRecord{}).Where("extension_id = ?", definition.ID).Updates(map[string]interface{}{"current_version": oldRegistered.Definition.Version, "manifest_json": string(oldRegistered.Definition.Manifest), "normalized_manifest_json": string(stableJSON(oldRegistered.Definition.Manifest))}).Error
			}
			return tx.Where("extension_id = ?", definition.ID).Delete(&extensionRecord{}).Error
		})
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopInstallFailed, "Registry 注册失败", err.Error(), false, err)
	}
	_ = i.registry.SetEnabled(ctx, definition.ID, false)
	return WorkshopInstallResult{SessionID: sessionID, SkillID: definition.ID, Version: definition.Version, ArtifactID: artifact.ArtifactID, Enabled: false}, nil
}

func (i *WorkshopInstaller) Restore(ctx context.Context) error {
	artifacts, err := i.repository.CurrentArtifacts(ctx)
	if err != nil {
		return err
	}
	var failures []string
	for _, artifact := range artifacts {
		definition, handler, loadErr := i.definitionFromArtifact(artifact)
		if loadErr != nil {
			_ = i.repository.db.WithContext(ctx).Model(&extensionRecord{}).Where("extension_id = ?", artifact.ExtensionID).Updates(map[string]interface{}{"enabled": 0, "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}).Error
			failures = append(failures, artifact.ExtensionID+": "+loadErr.Error())
			continue
		}
		if _, getErr := i.registry.Get(ctx, definition.ID); getErr == nil {
			_ = i.registry.Unregister(ctx, definition.ID)
		}
		if registerErr := i.registry.Register(ctx, definition, handler); registerErr != nil {
			failures = append(failures, artifact.ExtensionID+": "+registerErr.Error())
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("部分 Workflow Skill 恢复失败: %s", strings.Join(failures, "; "))
	}
	return nil
}

func (i *WorkshopInstaller) Rollback(ctx context.Context, scope ExecutionScope, skillID, version string) (WorkshopInstallResult, error) {
	var artifact extensionArtifactRecord
	if err := i.repository.db.WithContext(ctx).Where("extension_id = ? AND extension_version = ? AND archived_at = ''", skillID, version).First(&artifact).Error; err != nil {
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopRollbackFailed, "历史制品不存在", version, false, err)
	}
	if _, _, err := i.repository.GetSession(ctx, scope, artifact.SessionID); err != nil {
		return WorkshopInstallResult{}, err
	}
	definition, handler, err := i.definitionFromArtifact(artifact)
	if err != nil {
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopRollbackFailed, "历史制品校验失败", err.Error(), false, err)
	}
	current, err := i.registry.Get(ctx, skillID)
	if err != nil {
		return WorkshopInstallResult{}, err
	}
	enabled := current.Definition.Enabled
	if err := i.registry.Unregister(ctx, skillID); err != nil {
		return WorkshopInstallResult{}, err
	}
	definition.Enabled = enabled
	if err := i.registry.Register(ctx, definition, handler); err != nil {
		_ = i.registry.Register(ctx, current.Definition, current.Handler)
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopRollbackFailed, "Registry 回滚失败", err.Error(), false, err)
	}
	if err := i.repository.db.WithContext(ctx).Model(&extensionRecord{}).Where("extension_id = ?", skillID).Updates(map[string]interface{}{"current_version": version, "manifest_json": string(definition.Manifest), "normalized_manifest_json": string(stableJSON(definition.Manifest)), "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}).Error; err != nil {
		_ = i.registry.Unregister(ctx, skillID)
		_ = i.registry.Register(ctx, current.Definition, current.Handler)
		return WorkshopInstallResult{}, NewExtensionError(ErrWorkshopRollbackFailed, "数据库回滚失败", err.Error(), false, err)
	}
	return WorkshopInstallResult{SkillID: skillID, Version: version, ArtifactID: artifact.ArtifactID, Enabled: enabled}, nil
}

func (i *WorkshopInstaller) definitionFromArtifact(artifact extensionArtifactRecord) (SkillDefinition, SkillHandler, error) {
	if artifactChecksum(artifact) != artifact.Checksum {
		return SkillDefinition{}, nil, NewExtensionError(ErrWorkshopChecksumMismatch, "制品 Checksum 不匹配", artifact.ArtifactID, false, nil)
	}
	var manifest Manifest
	var compiled CompiledWorkflow
	if json.Unmarshal([]byte(artifact.ManifestJSON), &manifest) != nil || json.Unmarshal([]byte(artifact.CompiledWorkflowJSON), &compiled) != nil {
		return SkillDefinition{}, nil, fmt.Errorf("制品 JSON 无效")
	}
	if compiled.Checksum == "" || manifest.Entry.ArtifactID != artifact.ArtifactID {
		return SkillDefinition{}, nil, fmt.Errorf("制品入口或编译结果无效")
	}
	schemas := map[string]json.RawMessage{}
	if json.Unmarshal([]byte(artifact.SchemasJSON), &schemas) != nil {
		return SkillDefinition{}, nil, fmt.Errorf("制品 Schema 无效")
	}
	definition := skillDefinitionFromManifest(manifest, schemas)
	definition.Dependencies = dependencyIDs(compiled.Dependencies)
	return definition, i.workflowHandler(artifact, definition.OutputSchema), nil
}
func (i *WorkshopInstaller) workflowHandler(artifact extensionArtifactRecord, outputSchema json.RawMessage) SkillHandler {
	return func(ctx context.Context, request ExecuteSkillRequest) (SkillResult, error) {
		if artifactChecksum(artifact) != artifact.Checksum {
			return SkillResult{}, NewExtensionError(ErrWorkshopChecksumMismatch, "Workflow 制品损坏", artifact.ArtifactID, false, nil)
		}
		var compiled CompiledWorkflow
		if err := json.Unmarshal([]byte(artifact.CompiledWorkflowJSON), &compiled); err != nil {
			return SkillResult{}, NewExtensionError(ErrWorkshopArtifactInvalid, "Workflow 制品无法加载", err.Error(), false, err)
		}
		config, secrets := splitWorkflowConfig(request.Config, artifact.SchemasJSON)
		execution, err := i.executor.Execute(ctx, WorkflowExecutionRequest{Workflow: compiled, Input: request.Input, Config: config, Secrets: secrets, Scope: request.Scope, Mode: WorkflowProduction}, outputSchema)
		result := SkillResult{Output: execution.Output, SideEffects: execution.SideEffects, Status: RunSucceeded}
		if err != nil {
			result.Status = RunFailed
			result.Error = asExtensionError(err)
			return result, result.Error
		}
		return result, nil
	}
}

func splitWorkflowConfig(raw json.RawMessage, schemasJSON string) (json.RawMessage, map[string]string) {
	var schemas map[string]json.RawMessage
	var schema struct {
		Properties map[string]struct {
			Format    string `json:"format"`
			WriteOnly bool   `json:"writeOnly"`
		} `json:"properties"`
	}
	var config map[string]interface{}
	_ = json.Unmarshal([]byte(schemasJSON), &schemas)
	_ = json.Unmarshal(schemas["config"], &schema)
	_ = json.Unmarshal(normalizeJSON(raw), &config)
	if config == nil {
		config = map[string]interface{}{}
	}
	secrets := map[string]string{}
	for key, property := range schema.Properties {
		if !property.WriteOnly && property.Format != "password" && property.Format != "secret" {
			continue
		}
		if value, ok := config[key].(string); ok && value != "" {
			secrets[key] = value
		}
		delete(config, key)
	}
	safe, _ := json.Marshal(config)
	return safe, secrets
}

func buildArtifact(sessionID string, revision int64, draft ExtensionDraft, compiled CompiledWorkflow, testRunID string) (extensionArtifactRecord, error) {
	manifest, _ := json.Marshal(draft.Manifest)
	workflow, _ := json.Marshal(draft.Workflow)
	compiledRaw, _ := json.Marshal(compiled)
	schemas, _ := json.Marshal(map[string]json.RawMessage{"input": draft.InputSchema, "output": draft.OutputSchema, "config": normalizeJSON(draft.ConfigSchema), "defaults": normalizeJSON(draft.DefaultConfig)})
	tests, _ := json.Marshal(map[string]interface{}{"cases": draft.TestCases, "report": testRunID})
	artifact := extensionArtifactRecord{ID: uuid.New().String(), ArtifactID: draft.Manifest.Entry.ArtifactID, ExtensionID: draft.Metadata.ID, ExtensionVersion: draft.Metadata.Version, Source: "workshop", SessionID: sessionID, Revision: revision, ManifestJSON: string(manifest), WorkflowJSON: string(workflow), SchemasJSON: string(schemas), CompiledWorkflowJSON: string(compiledRaw), TestsJSON: string(tests), ReadmeText: fmt.Sprintf("# %s\n\n%s\n", draft.Metadata.Name, draft.Metadata.Description), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	artifact.Checksum = artifactChecksum(artifact)
	artifact.SizeBytes = int64(len(artifact.ManifestJSON) + len(artifact.WorkflowJSON) + len(artifact.SchemasJSON) + len(artifact.CompiledWorkflowJSON) + len(artifact.TestsJSON) + len(artifact.ReadmeText))
	if artifact.SizeBytes > 8*1024*1024 {
		return artifact, NewExtensionError(ErrWorkshopArtifactInvalid, "制品超过大小限制", "", false, nil)
	}
	return artifact, nil
}
func artifactChecksum(artifact extensionArtifactRecord) string {
	raw := strings.Join([]string{artifact.ArtifactID, artifact.ExtensionID, artifact.ExtensionVersion, artifact.ManifestJSON, artifact.WorkflowJSON, artifact.SchemasJSON, artifact.CompiledWorkflowJSON, artifact.TestsJSON, artifact.ReadmeText}, "\x00")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

var modelNamePattern = regexp.MustCompile(`[^a-z0-9_]+`)

func skillDefinitionFromDraft(draft ExtensionDraft, compiled CompiledWorkflow) SkillDefinition {
	manifestRaw, _ := json.Marshal(draft.Manifest)
	modelName := modelNamePattern.ReplaceAllString(strings.ReplaceAll(draft.Metadata.ID, ".", "_"), "_")
	if len(modelName) > 64 {
		modelName = modelName[:64]
	}
	return SkillDefinition{ID: draft.Metadata.ID, ModelName: modelName, Name: draft.Metadata.Name, Description: draft.Metadata.Description, Version: draft.Metadata.Version, Source: SkillSourceWorkflow, Entry: draft.Manifest.Entry, InputSchema: draft.InputSchema, OutputSchema: draft.OutputSchema, ConfigSchema: draft.ConfigSchema, DefaultConfig: normalizeJSON(draft.DefaultConfig), Capabilities: compiled.Capabilities, Dependencies: dependencyIDs(compiled.Dependencies), Triggers: draft.Manifest.Triggers, TimeoutMS: compiled.Limits.MaxExecutionDurationMS, HasSideEffects: compiled.HasSideEffects, Retryable: compiled.Idempotent && !compiled.HasSideEffects, Idempotent: compiled.Idempotent, Enabled: false, Author: draft.Metadata.Author, License: draft.Metadata.License, Manifest: manifestRaw}
}

func dependencyIDs(dependencies []ResolvedSkillDependency) []string {
	result := make([]string, 0, len(dependencies))
	seen := map[string]bool{}
	for _, dependency := range dependencies {
		if dependency.SkillID != "" && !seen[dependency.SkillID] {
			seen[dependency.SkillID] = true
			result = append(result, dependency.SkillID)
		}
	}
	sort.Strings(result)
	return result
}
func skillDefinitionFromManifest(manifest Manifest, schemas map[string]json.RawMessage) SkillDefinition {
	draft := ExtensionDraft{Metadata: DraftMetadata{ID: manifest.Metadata.ID, Name: manifest.Metadata.Name, Version: manifest.Metadata.Version, Description: manifest.Metadata.Description, Author: manifest.Metadata.Author, License: manifest.Metadata.License}, Manifest: manifest, InputSchema: schemas["input"], OutputSchema: schemas["output"], ConfigSchema: schemas["config"], DefaultConfig: schemas["defaults"]}
	compiled := CompiledWorkflow{Capabilities: manifest.Capabilities, Limits: WorkflowLimits{MaxExecutionDurationMS: manifest.Execution.TimeoutMS}, HasSideEffects: manifest.Execution.HasSideEffects, Idempotent: manifest.Execution.Idempotent}
	return skillDefinitionFromDraft(draft, compiled)
}
func replaceManifestArtifact(raw json.RawMessage, artifactID string) json.RawMessage {
	var manifest Manifest
	if json.Unmarshal(raw, &manifest) != nil {
		return raw
	}
	manifest.Entry.ArtifactID = artifactID
	result, _ := json.Marshal(manifest)
	return result
}

func suggestWorkshopVersion(current SkillDefinition, draft ExtensionDraft) (string, []string) {
	breaking := []string{}
	breaking = append(breaking, breakingSchemaChanges(current.InputSchema, draft.InputSchema, "input")...)
	breaking = append(breaking, breakingSchemaChanges(current.OutputSchema, draft.OutputSchema, "output")...)
	breaking = append(breaking, breakingSchemaChanges(current.ConfigSchema, draft.ConfigSchema, "config")...)
	major, minor, patch := semverParts(current.Version)
	if len(breaking) > 0 {
		return fmt.Sprintf("%d.0.0", major+1), breaking
	}
	if !sameStringSets(current.Capabilities, draft.Capabilities) || !jsonEqual(current.InputSchema, draft.InputSchema) || !jsonEqual(current.OutputSchema, draft.OutputSchema) || !jsonEqual(current.ConfigSchema, draft.ConfigSchema) {
		return fmt.Sprintf("%d.%d.0", major, minor+1), nil
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch+1), nil
}

func breakingSchemaChanges(previousRaw, nextRaw json.RawMessage, label string) []string {
	var previous, next struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Type interface{} `json:"type"`
		} `json:"properties"`
	}
	_ = json.Unmarshal(normalizeJSON(previousRaw), &previous)
	_ = json.Unmarshal(normalizeJSON(nextRaw), &next)
	reasons := []string{}
	previousRequired := map[string]bool{}
	for _, field := range previous.Required {
		previousRequired[field] = true
	}
	if label == "input" || label == "config" {
		for _, field := range next.Required {
			if !previousRequired[field] {
				reasons = append(reasons, label+" 新增必填字段 "+field)
			}
		}
	} else {
		for _, field := range previous.Required {
			if _, ok := next.Properties[field]; !ok {
				reasons = append(reasons, label+" 移除必填字段 "+field)
			}
		}
	}
	for field, oldProperty := range previous.Properties {
		if nextProperty, ok := next.Properties[field]; ok && !reflect.DeepEqual(oldProperty.Type, nextProperty.Type) {
			reasons = append(reasons, label+" 改变字段类型 "+field)
		}
	}
	return reasons
}

func semverParts(version string) (int, int, int) {
	parts := strings.SplitN(version, ".", 3)
	values := []int{0, 0, 0}
	for index := range parts {
		text := strings.SplitN(parts[index], "-", 2)[0]
		values[index], _ = strconv.Atoi(text)
	}
	return values[0], values[1], values[2]
}

func semverMajor(version string) int {
	major, _, _ := semverParts(version)
	return major
}

func jsonEqual(left, right json.RawMessage) bool {
	return string(stableJSON(left)) == string(stableJSON(right))
}

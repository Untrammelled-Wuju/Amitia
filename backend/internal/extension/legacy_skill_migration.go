package extension

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	extensionkernel "github.com/u-ai/backend/internal/extension/kernel"
	kernelworkflow "github.com/u-ai/backend/internal/extension/kernel/workflow"
	"gorm.io/gorm"
)

type legacyWorkflowSkillArtifact struct {
	ArtifactID       string `gorm:"column:artifact_id"`
	ExtensionID      string `gorm:"column:extension_id"`
	ExtensionVersion string `gorm:"column:extension_version"`
	ManifestJSON     string `gorm:"column:manifest_json"`
	WorkflowJSON     string `gorm:"column:workflow_json"`
	OwnerSpaceID     string `gorm:"column:owner_space_id"`
	Enabled          int    `gorm:"column:enabled"`
}

type LegacySkillMigrationReport struct {
	Migrated int
	Skipped  int
	Failed   int
	Errors   []string
}

func MigrateLegacyWorkflowSkills(ctx context.Context, db *gorm.DB, container *extensionkernel.Container) (LegacySkillMigrationReport, error) {
	var report LegacySkillMigrationReport
	if db == nil || container == nil || container.WorkflowDefRepo == nil || container.WorkflowRegistry == nil {
		return report, nil
	}
	if !db.Migrator().HasTable("extension_artifacts") || !db.Migrator().HasTable("extensions") || !db.Migrator().HasTable("extension_workshop_sessions") {
		return report, nil
	}
	var rows []legacyWorkflowSkillArtifact
	err := db.WithContext(ctx).
		Table("extension_artifacts AS artifacts").
		Select("artifacts.artifact_id, artifacts.extension_id, artifacts.extension_version, artifacts.manifest_json, artifacts.workflow_json, COALESCE(sessions.space_id, '') AS owner_space_id, extensions.enabled AS enabled").
		Joins("JOIN extensions ON extensions.extension_id = artifacts.extension_id AND extensions.current_version = artifacts.extension_version").
		Joins("LEFT JOIN extension_workshop_sessions AS sessions ON sessions.id = artifacts.session_id").
		Where("artifacts.archived_at = '' AND artifacts.source = ?", "workshop").
		Scan(&rows).Error
	if err != nil {
		return report, fmt.Errorf("scan legacy workflow skills: %w", err)
	}
	for _, row := range rows {
		if row.OwnerSpaceID == "" {
			report.Skipped++
			continue
		}
		if _, err := container.WorkflowDefRepo.Get(ctx, row.ExtensionID); err == nil {
			report.Skipped++
			continue
		} else if !errors.Is(err, sql.ErrNoRows) {
			report.Failed++
			report.Errors = append(report.Errors, row.ExtensionID+": query definition: "+err.Error())
			continue
		}
		var old legacyWorkflowDefinition
		if err := json.Unmarshal([]byte(row.WorkflowJSON), &old); err != nil {
			report.Failed++
			report.Errors = append(report.Errors, row.ExtensionID+": decode workflow: "+err.Error())
			continue
		}
		var manifest Manifest
		if err := json.Unmarshal([]byte(row.ManifestJSON), &manifest); err != nil {
			report.Failed++
			report.Errors = append(report.Errors, row.ExtensionID+": decode manifest: "+err.Error())
			continue
		}
		definition := legacyWorkflowToCallable(
			old,
			row.ExtensionID,
			manifest.Metadata.Name,
			manifest.Metadata.Description,
			"system/amitia-core",
			"legacy-skill-migration",
		)
		definition.Version = row.ExtensionVersion
		definition.Enabled = row.Enabled == 1
		definition.Source = "user"
		if definition.Metadata == nil {
			definition.Metadata = map[string]any{}
		}
		definition.Metadata["ownerSpaceId"] = row.OwnerSpaceID
		definition.Metadata["legacySkillId"] = row.ExtensionID
		if err := container.WorkflowDefRepo.Save(ctx, definition); err != nil {
			report.Failed++
			report.Errors = append(report.Errors, row.ExtensionID+": save definition: "+err.Error())
			continue
		}
		if _, err := container.WorkflowDefRepo.EnsurePublishedRevision(ctx, row.OwnerSpaceID, definition, "旧兼容技能迁移"); err != nil {
			report.Failed++
			report.Errors = append(report.Errors, row.ExtensionID+": save revision: "+err.Error())
			continue
		}
		if err := container.WorkflowRegistry.UpsertContext(ctx, definition); err != nil {
			report.Failed++
			report.Errors = append(report.Errors, row.ExtensionID+": register workflow: "+err.Error())
			continue
		}
		if _, err := container.WorkflowInstallationRepo.EnsureLegacy(ctx, definition, row.OwnerSpaceID, kernelworkflow.WorkflowLocationLocal); err != nil {
			report.Failed++
			report.Errors = append(report.Errors, row.ExtensionID+": save installation: "+err.Error())
			continue
		}
		if definition.CallableByAgent {
			if err := extensionkernel.SyncUserWorkflowAgentTool(ctx, container.ToolRegistry, definition); err != nil {
				report.Failed++
				report.Errors = append(report.Errors, row.ExtensionID+": sync agent tool: "+err.Error())
				continue
			}
		}
		report.Migrated++
	}
	return report, nil
}

type legacyWorkflowLimits struct {
	MaxSteps               int   `json:"maxSteps"`
	MaxExecutionDurationMS int64 `json:"maxExecutionDurationMs"`
	MaxStepDurationMS      int64 `json:"maxStepDurationMs"`
	MaxInputBytes          int64 `json:"maxInputBytes"`
	MaxOutputBytes         int64 `json:"maxOutputBytes"`
	MaxIntermediateBytes   int64 `json:"maxIntermediateBytes"`
	MaxHTTPResponseBytes   int64 `json:"maxHttpResponseBytes"`
	MaxHTTPRedirects       int   `json:"maxHttpRedirects"`
	MaxSkillCallDepth      int   `json:"maxSkillCallDepth"`
	MaxSkillCalls          int   `json:"maxSkillCalls"`
	MaxArrayItems          int   `json:"maxArrayItems"`
	MaxExpressionDepth     int   `json:"maxExpressionDepth"`
	MaxTemplateLength      int   `json:"maxTemplateLength"`
	MaxEventsEmitted       int   `json:"maxEventsEmitted"`
	MaxSchedulesCreated    int   `json:"maxSchedulesCreated"`
	MaxSideEffects         int   `json:"maxSideEffects"`
}

type legacyWorkflowErrorPolicy struct {
	Mode    string          `json:"mode"`
	Default json.RawMessage `json:"default,omitempty"`
}

type legacyConditionExpression struct {
	Op    string                      `json:"op"`
	Args  []legacyConditionExpression `json:"args,omitempty"`
	Left  any                         `json:"left,omitempty"`
	Right any                         `json:"right,omitempty"`
	Value any                         `json:"value,omitempty"`
}

type legacyWorkflowStep struct {
	ID      string                     `json:"id"`
	Type    string                     `json:"type"`
	Input   json.RawMessage            `json:"input"`
	When    *legacyConditionExpression `json:"when,omitempty"`
	OnError legacyWorkflowErrorPolicy  `json:"onError"`
}

type legacyWorkflowDefinition struct {
	SchemaVersion string               `json:"schemaVersion"`
	Steps         []legacyWorkflowStep `json:"steps"`
	Output        json.RawMessage      `json:"output"`
	Limits        legacyWorkflowLimits `json:"limits"`
}

func legacyWorkflowToCallable(wf legacyWorkflowDefinition, id, name, description, extensionID, moduleID string) kernelworkflow.WorkflowDefinition {
	nodes := make([]kernelworkflow.WorkflowNode, 0, len(wf.Steps))
	for _, step := range wf.Steps {
		onError := kernelworkflow.WorkflowOnError{Mode: "fail"}
		if step.OnError.Mode != "" {
			onError.Mode = step.OnError.Mode
			if len(step.OnError.Default) > 0 {
				onError.Default = append(json.RawMessage(nil), step.OnError.Default...)
			}
		}
		node := kernelworkflow.WorkflowNode{ID: step.ID, Type: step.Type, Step: kernelworkflow.WorkflowStepInput{Input: append(json.RawMessage(nil), step.Input...), OnError: onError}}
		if step.When != nil {
			if raw, err := json.Marshal(step.When); err == nil {
				node.Step.When = (*json.RawMessage)(&raw)
			}
		}
		nodes = append(nodes, node)
	}
	outputSchema := wf.Output
	if len(outputSchema) == 0 {
		outputSchema = json.RawMessage(`{}`)
	}
	return kernelworkflow.WorkflowDefinition{
		SchemaVersion:   wf.SchemaVersion,
		ID:              id,
		ExtensionID:     extensionID,
		ModuleID:        moduleID,
		Name:            name,
		Description:     description,
		InputSchema:     json.RawMessage(`{"type":"object","additionalProperties":true}`),
		OutputSchema:    outputSchema,
		Nodes:           nodes,
		CallableByAgent: true,
		Enabled:         true,
		Version:         "1.0.0",
		Source:          "workshop",
		Limits: kernelworkflow.WorkflowLimits{
			MaxSteps:               wf.Limits.MaxSteps,
			MaxExecutionDurationMS: wf.Limits.MaxExecutionDurationMS,
			MaxStepDurationMS:      wf.Limits.MaxStepDurationMS,
			MaxInputBytes:          wf.Limits.MaxInputBytes,
			MaxOutputBytes:         wf.Limits.MaxOutputBytes,
			MaxIntermediateBytes:   wf.Limits.MaxIntermediateBytes,
			MaxHTTPResponseBytes:   wf.Limits.MaxHTTPResponseBytes,
			MaxHTTPRedirects:       wf.Limits.MaxHTTPRedirects,
			MaxSkillCallDepth:      wf.Limits.MaxSkillCallDepth,
			MaxSkillCalls:          wf.Limits.MaxSkillCalls,
			MaxArrayItems:          wf.Limits.MaxArrayItems,
			MaxExpressionDepth:     wf.Limits.MaxExpressionDepth,
			MaxTemplateLength:      wf.Limits.MaxTemplateLength,
			MaxEventsEmitted:       wf.Limits.MaxEventsEmitted,
			MaxSchedulesCreated:    wf.Limits.MaxSchedulesCreated,
			MaxSideEffects:         wf.Limits.MaxSideEffects,
		},
	}
}

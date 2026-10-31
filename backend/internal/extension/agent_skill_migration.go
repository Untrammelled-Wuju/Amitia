package extension

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/observability"
	"gorm.io/gorm"
	amitialog "log"
)

type legacyAgentSkillRow struct {
	ID          string `gorm:"column:id"`
	ExtensionID string `gorm:"column:extension_id"`
	SpaceID     string `gorm:"column:space_id"`
	Name        string `gorm:"column:name"`
	ScopeType   string `gorm:"column:scope_type"`
	ScopeID     string `gorm:"column:scope_id"`
	ArtifactID  string `gorm:"column:artifact_id"`
	Enabled     int    `gorm:"column:enabled"`
}

type legacyAgentSkillActivationRow struct {
	ID                  string `gorm:"column:id"`
	ActivationID        string `gorm:"column:activation_id"`
	ExtensionID         string `gorm:"column:extension_id"`
	SpaceID             string `gorm:"column:space_id"`
	CharacterID         string `gorm:"column:character_id"`
	ConversationID      string `gorm:"column:conversation_id"`
	Channel             string `gorm:"column:channel"`
	TriggerType         string `gorm:"column:trigger_type"`
	Explicit            int    `gorm:"column:explicit"`
	Status              string `gorm:"column:status"`
	LoadedTokens        int    `gorm:"column:loaded_tokens"`
	ResourceReads       int    `gorm:"column:resource_reads"`
	ResourcePathsJSON   string `gorm:"column:resource_paths_json"`
	TraceID             string `gorm:"column:trace_id"`
	ErrorCode           string `gorm:"column:error_code"`
	CreatedAt           string `gorm:"column:created_at"`
	AgentSkillName      string `gorm:"column:agent_skill_name"`
	Source              string `gorm:"column:source"`
	ScopeType           string `gorm:"column:scope_type"`
	CompatibilityStatus string `gorm:"column:compatibility_status"`
	InstructionPosition string `gorm:"column:instruction_position"`
	TokenLimitHit       int    `gorm:"column:token_limit_hit"`
}

func MigrateLegacyAgentSkills(ctx context.Context, db *gorm.DB, runtime *kernel.Runtime) error {
	if db == nil || runtime == nil {
		return fmt.Errorf("agent skill migration unavailable")
	}
	if !db.Migrator().HasTable("extension_agent_skill_cutover") {
		return nil
	}
	var status string
	if err := db.WithContext(ctx).Raw(`SELECT status FROM extension_agent_skill_cutover WHERE id = 1`).Scan(&status).Error; err == nil && status == "completed" {
		return nil
	}
	startedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if err := db.WithContext(ctx).Exec(`INSERT INTO extension_agent_skill_cutover(id,status,started_at) VALUES(1,'running',?) ON CONFLICT(id) DO UPDATE SET status='running',last_error='',started_at=excluded.started_at`, startedAt).Error; err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	fail := func(err error) error {
		if err := db.WithContext(ctx).Exec(`UPDATE extension_agent_skill_cutover SET status='failed',last_error=? WHERE id=1`, err.Error()).Error; err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return err
	}
	hasMetadata := db.Migrator().HasTable("extension_agent_skill_metadata")
	hasActivations := db.Migrator().HasTable("extension_agent_skill_activations")
	if !hasMetadata && !hasActivations {
		if err := markLegacyAgentSkillCutoverComplete(ctx, db, 0, 0); err != nil {
			return fail(err)
		}
		return nil
	}
	rows := []legacyAgentSkillRow{}
	if hasMetadata {
		if err := db.WithContext(ctx).Table("extension_agent_skill_metadata").Where("removed_at = ''").Select("id, extension_id, space_id, name, scope_type, scope_id, artifact_id, enabled").Scan(&rows).Error; err != nil {
			return fail(err)
		}
	}
	migratedSkills := 0
	migratedIDs := map[string]string{}
	for _, row := range rows {
		if strings.TrimSpace(row.SpaceID) == "" || strings.TrimSpace(row.ArtifactID) == "" {
			return fail(fmt.Errorf("legacy agent skill %s has incomplete ownership", row.ExtensionID))
		}
		container := runtime.Container()
		if container == nil {
			return fail(fmt.Errorf("kernel container unavailable"))
		}
		raw, err := loadLegacyAgentSkillArtifact(ctx, db, row.ArtifactID)
		if err != nil {
			return fail(err)
		}
		files, err := decodeLegacyAgentSkillArtifact(raw)
		if err != nil {
			return fail(err)
		}
		built, err := kernel.BuildLocalAgentSkillPackage(ctx, row.SpaceID, row.Name, files)
		if err != nil {
			return fail(fmt.Errorf("migrate skill %s: %w", row.ExtensionID, err))
		}
		migratedIDs[row.ExtensionID] = built.Definition.ExtensionID
		if _, err := container.InstallationRepository.GetInstallation(ctx, kerneldomain.ExtensionID(built.Definition.ExtensionID)); err == nil {
			migratedSkills++
			continue
		}
		scopeType := strings.TrimSpace(row.ScopeType)
		if scopeType == "" {
			scopeType = "global"
		}
		preview, err := runtime.PreviewPackage(ctx, kernel.PackagePreviewRequest{
			SpaceID: row.SpaceID, ScopeType: scopeType, ScopeID: row.ScopeID, FileName: built.FileName, AllowUnsignedLocal: true,
		}, bytes.NewReader(built.Archive))
		if err != nil {
			return fail(fmt.Errorf("preview legacy skill %s: %w", row.ExtensionID, err))
		}
		confirmations := map[string]bool{}
		for _, item := range preview.RequiredConfirmations {
			confirmations[item] = true
		}
		confirmation, err := runtime.ConfirmPackagePreview(ctx, kernel.PackagePreviewConfirmationRequest{
			SessionID: preview.SessionID, SpaceID: row.SpaceID, ScopeType: scopeType, ScopeID: row.ScopeID, Confirmations: confirmations,
		})
		if err != nil {
			return fail(fmt.Errorf("confirm legacy skill %s: %w", row.ExtensionID, err))
		}
		result, err := runtime.ExecutePackageInstall(ctx, kernel.PackageInstallRequest{
			SessionID: preview.SessionID, SpaceID: row.SpaceID, ScopeType: scopeType, ScopeID: row.ScopeID,
			ConfirmationToken: confirmation.ConfirmationToken, IdempotencyKey: "legacy-agent-skill-" + row.ExtensionID,
		})
		if err != nil {
			return fail(fmt.Errorf("install legacy skill %s: %w", row.ExtensionID, err))
		}
		if row.Enabled != 0 {
			if err := runtime.Enable(ctx, result.ExtensionID); err != nil {
				return fail(fmt.Errorf("enable legacy skill %s: %w", row.ExtensionID, err))
			}
		}
		migratedSkills++
	}
	migratedActivations := 0
	if hasActivations {
		activationRows := []legacyAgentSkillActivationRow{}
		if err := db.WithContext(ctx).Table("extension_agent_skill_activations").Order("created_at").Scan(&activationRows).Error; err != nil {
			return fail(err)
		}
		for _, row := range activationRows {
			invocationID := strings.TrimSpace(row.ActivationID)
			if invocationID == "" {
				invocationID = "legacy-agent-skill-" + row.ID
			}
			createdAt, _ := time.Parse(time.RFC3339Nano, row.CreatedAt)
			if createdAt.IsZero() {
				createdAt = time.Now().UTC()
			}
			var resourcePaths []string
			if err := json.Unmarshal([]byte(row.ResourcePathsJSON), &resourcePaths); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			extensionID := row.ExtensionID
			if migratedID := migratedIDs[row.ExtensionID]; migratedID != "" {
				extensionID = migratedID
			}
			statusValue := observability.StatusSucceeded
			if !strings.EqualFold(row.Status, "success") && !strings.EqualFold(row.Status, "succeeded") && !strings.EqualFold(row.Status, "active") {
				statusValue = observability.StatusFailed
			}
			if err := runtime.Container().ObservabilityStore.SaveInvocation(ctx, observability.InvocationRecord{
				InvocationID: invocationID, TraceID: row.TraceID, CapabilityID: "agent_skill", CapabilityType: "agent_skill",
				Source: row.TriggerType, OwnerType: "space", OwnerID: row.SpaceID, ExtensionID: extensionID,
				SpaceID: row.SpaceID, CharacterID: row.CharacterID, ConversationID: row.ConversationID,
				Status: statusValue, RiskLevel: observability.RiskLow, ErrorCode: row.ErrorCode,
				CreatedAt: createdAt, StartedAt: &createdAt, FinishedAt: &createdAt,
				Metadata: map[string]any{"name": row.AgentSkillName, "scope": row.ScopeType, "compatibility": row.CompatibilityStatus,
					"channel": row.Channel, "trigger": row.TriggerType, "explicit": row.Explicit != 0,
					"tokens": row.LoadedTokens, "resourceReads": row.ResourceReads, "resourcePaths": resourcePaths,
					"tokenLimitHit": row.TokenLimitHit != 0, "instructionPosition": row.InstructionPosition},
			}); err != nil {
				return fail(err)
			}
			migratedActivations++
		}
	}
	if err := cleanupLegacyAgentSkills(ctx, db); err != nil {
		return fail(err)
	}
	if err := markLegacyAgentSkillCutoverComplete(ctx, db, migratedSkills, migratedActivations); err != nil {
		return fail(err)
	}
	return nil
}

func markLegacyAgentSkillCutoverComplete(ctx context.Context, db *gorm.DB, skills, activations int) error {
	return db.WithContext(ctx).Exec(`UPDATE extension_agent_skill_cutover SET status='completed',migrated_skills=?,migrated_activations=?,completed_at=?,last_error='' WHERE id=1`, skills, activations, time.Now().UTC().Format(time.RFC3339Nano)).Error
}

func loadLegacyAgentSkillArtifact(ctx context.Context, db *gorm.DB, artifactID string) ([]byte, error) {
	var raw []byte
	if err := db.WithContext(ctx).Table("extension_artifacts").Select("content_blob").Where("artifact_id = ? AND archived_at = ''", artifactID).Row().Scan(&raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("legacy agent skill artifact %s is unavailable", artifactID)
	}
	return raw, nil
}

func decodeLegacyAgentSkillArtifact(raw []byte) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	var total int64
	for _, entry := range reader.File {
		clean := path.Clean(strings.ReplaceAll(entry.Name, "\\", "/"))
		if clean == "." || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
			return nil, fmt.Errorf("legacy agent skill artifact path is invalid")
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("legacy agent skill artifact contains symlink")
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		if entry.UncompressedSize64 > 20<<20 {
			return nil, fmt.Errorf("legacy agent skill artifact entry exceeds limit")
		}
		total += int64(entry.UncompressedSize64)
		if total > 50<<20 || len(files) >= 500 {
			return nil, fmt.Errorf("legacy agent skill artifact exceeds limit")
		}
		rc, err := entry.Open()
		if err != nil {
			return nil, err
		}
		content, readErr := io.ReadAll(io.LimitReader(rc, 20<<20))
		rc.Close()
		if readErr != nil {
			return nil, readErr
		}
		files[clean] = content
	}
	return files, nil
}

func cleanupLegacyAgentSkills(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasTable("extensions") && tx.Migrator().HasTable("extension_agent_skill_metadata") {
			if err := tx.Exec(`DELETE FROM extensions WHERE extension_id IN (SELECT extension_id FROM extension_agent_skill_metadata)`).Error; err != nil {
				return err
			}
		}
		if tx.Migrator().HasTable("extension_artifacts") && tx.Migrator().HasTable("extension_agent_skill_metadata") {
			if err := tx.Exec(`DELETE FROM extension_artifacts WHERE artifact_id IN (SELECT artifact_id FROM extension_agent_skill_metadata)`).Error; err != nil {
				return err
			}
		}
		if err := tx.Exec(`DROP TABLE IF EXISTS extension_agent_skill_activations`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DROP TABLE IF EXISTS extension_agent_skill_metadata`).Error; err != nil {
			return err
		}
		return nil
	})
}

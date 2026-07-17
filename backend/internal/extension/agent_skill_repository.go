package extension

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type agentSkillMetadataRecord struct {
	ID                      string `gorm:"column:id;primaryKey"`
	ExtensionID             string `gorm:"column:extension_id"`
	UserID                  string `gorm:"column:user_id"`
	Name                    string `gorm:"column:name"`
	Description             string `gorm:"column:description"`
	License                 string `gorm:"column:license"`
	Compatibility           string `gorm:"column:compatibility"`
	MetadataJSON            string `gorm:"column:metadata_json"`
	AllowedTools            string `gorm:"column:allowed_tools"`
	DisplayName             string `gorm:"column:display_name"`
	ShortDescription        string `gorm:"column:short_description"`
	DefaultPrompt           string `gorm:"column:default_prompt"`
	OpenAIMetadataJSON      string `gorm:"column:openai_metadata_json"`
	ScopeType               string `gorm:"column:scope_type"`
	ScopeID                 string `gorm:"column:scope_id"`
	Source                  string `gorm:"column:source"`
	CompatibilityStatus     string `gorm:"column:compatibility_status"`
	CompatibilityReportJSON string `gorm:"column:compatibility_report_json"`
	ContentHash             string `gorm:"column:content_hash"`
	ArtifactID              string `gorm:"column:artifact_id"`
	RawFrontmatterJSON      string `gorm:"column:raw_frontmatter_json"`
	ExtraFrontmatterJSON    string `gorm:"column:extra_frontmatter_json"`
	ResourceIndexJSON       string `gorm:"column:resource_index_json"`
	ToolMappingsJSON        string `gorm:"column:tool_mappings_json"`
	ScriptsPresent          int    `gorm:"column:scripts_present"`
	ScriptsRequired         int    `gorm:"column:scripts_required"`
	Enabled                 int    `gorm:"column:enabled"`
	CreatedAt               string `gorm:"column:created_at"`
	UpdatedAt               string `gorm:"column:updated_at"`
	RemovedAt               string `gorm:"column:removed_at"`
}

func (agentSkillMetadataRecord) TableName() string { return "extension_agent_skill_metadata" }

type agentSkillArtifactRecord struct {
	ArtifactID        string `gorm:"column:artifact_id"`
	ExtensionID       string `gorm:"column:extension_id"`
	Checksum          string `gorm:"column:checksum"`
	Content           []byte `gorm:"column:content_blob"`
	ResourceIndexJSON string `gorm:"column:resource_index_json"`
	ArchivedAt        string `gorm:"column:archived_at"`
}

func (agentSkillArtifactRecord) TableName() string { return "extension_artifacts" }

type agentSkillActivationRecord struct {
	ID                  string `gorm:"column:id;primaryKey"`
	ActivationID        string `gorm:"column:activation_id"`
	ExtensionID         string `gorm:"column:extension_id"`
	AgentSkillName      string `gorm:"column:agent_skill_name"`
	Source              string `gorm:"column:source"`
	ScopeType           string `gorm:"column:scope_type"`
	CompatibilityStatus string `gorm:"column:compatibility_status"`
	UserID              string `gorm:"column:user_id"`
	CharacterID         string `gorm:"column:character_id"`
	ConversationID      string `gorm:"column:conversation_id"`
	Channel             string `gorm:"column:channel"`
	TriggerType         string `gorm:"column:trigger_type"`
	Explicit            int    `gorm:"column:explicit"`
	Status              string `gorm:"column:status"`
	LoadedTokens        int    `gorm:"column:loaded_tokens"`
	ResourceReads       int    `gorm:"column:resource_reads"`
	ResourcePathsJSON   string `gorm:"column:resource_paths_json"`
	ScriptsUsed         int    `gorm:"column:scripts_used"`
	ToolMappingsJSON    string `gorm:"column:tool_mappings_json"`
	InstructionPosition string `gorm:"column:instruction_position"`
	TokenLimitHit       int    `gorm:"column:token_limit_hit"`
	TraceID             string `gorm:"column:trace_id"`
	ErrorCode           string `gorm:"column:error_code"`
	CreatedAt           string `gorm:"column:created_at"`
}

func (agentSkillActivationRecord) TableName() string { return "extension_agent_skill_activations" }

func (r *Repository) InstallAgentSkill(ctx context.Context, definition AgentSkillDefinition, report AgentSkillCompatibilityReport, manifest SkillDefinition, files map[string][]byte) error {
	archive, err := encodeAgentSkillArtifact(files)
	if err != nil {
		return err
	}
	resources, _ := json.Marshal(definition.Resources)
	mappings, _ := json.Marshal(definition.ToolMappings)
	metadata, _ := json.Marshal(definition.Metadata)
	reportRaw, _ := json.Marshal(report)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	scriptsPresent := 0
	for _, resource := range definition.Resources {
		if resource.Kind == AgentSkillResourceScript {
			scriptsPresent = 1
			break
		}
	}
	record := agentSkillMetadataRecord{ID: uuid.NewString(), ExtensionID: definition.ExtensionID, UserID: definition.UserID, Name: definition.Name, Description: definition.Description, License: definition.License, Compatibility: definition.Compatibility, MetadataJSON: string(metadata), AllowedTools: definition.AllowedTools, DisplayName: definition.DisplayName, ShortDescription: definition.ShortDescription, DefaultPrompt: definition.DefaultPrompt, OpenAIMetadataJSON: string(normalizeJSON(definition.OpenAIMetadata)), ScopeType: string(definition.Scope), ScopeID: definition.ScopeID, Source: string(definition.Source), CompatibilityStatus: string(definition.CompatibilityStatus), CompatibilityReportJSON: string(reportRaw), ContentHash: definition.ContentHash, ArtifactID: definition.ArtifactID, RawFrontmatterJSON: string(normalizeJSON(definition.RawFrontmatter)), ExtraFrontmatterJSON: string(normalizeJSON(definition.ExtraFrontmatter)), ResourceIndexJSON: string(resources), ToolMappingsJSON: string(mappings), ScriptsPresent: scriptsPresent, ScriptsRequired: boolNumber(len(report.RequiredScripts) > 0), Enabled: 0, CreatedAt: now, UpdatedAt: now}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&agentSkillMetadataRecord{}).Where("user_id = ? AND name = ? AND scope_type = ? AND scope_id = ? AND removed_at = ''", definition.UserID, definition.Name, definition.Scope, definition.ScopeID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return NewExtensionError(ErrAgentSkillNameConflict, "Agent Skill name already exists in this scope", definition.Name, false, nil)
		}
		artifact := map[string]interface{}{"id": uuid.NewString(), "artifact_id": definition.ArtifactID, "extension_id": definition.ExtensionID, "extension_version": manifest.Version, "source": string(definition.Source), "session_id": "", "revision": 0, "manifest_json": string(manifest.Manifest), "workflow_json": "{}", "schemas_json": "{}", "compiled_workflow_json": "{}", "tests_json": "[]", "readme_text": "", "checksum": definition.ContentHash, "size_bytes": len(archive), "created_at": now, "archived_at": "", "artifact_kind": "agent-skill", "content_blob": archive, "resource_index_json": string(resources)}
		if err := tx.Table("extension_artifacts").Create(artifact).Error; err != nil {
			return err
		}
		return tx.Create(&record).Error
	})
}

func (r *Repository) ListAgentSkillRecords(ctx context.Context) ([]agentSkillMetadataRecord, error) {
	var rows []agentSkillMetadataRecord
	err := r.db.WithContext(ctx).Where("removed_at = ''").Order("name ASC, scope_type ASC, scope_id ASC").Find(&rows).Error
	return rows, err
}
func (r *Repository) GetAgentSkillRecord(ctx context.Context, id string) (agentSkillMetadataRecord, error) {
	var row agentSkillMetadataRecord
	err := r.db.WithContext(ctx).Where("extension_id = ? AND removed_at = ''", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, NewExtensionError(ErrAgentSkillNotFound, "Agent Skill not found", id, false, nil)
	}
	return row, err
}

func (r *Repository) LoadAgentSkill(ctx context.Context, id string) (AgentSkillDefinition, AgentSkillCompatibilityReport, map[string][]byte, error) {
	row, err := r.GetAgentSkillRecord(ctx, id)
	if err != nil {
		return AgentSkillDefinition{}, AgentSkillCompatibilityReport{}, nil, err
	}
	var artifact agentSkillArtifactRecord
	if err := r.db.WithContext(ctx).Where("artifact_id = ? AND archived_at = ''", row.ArtifactID).First(&artifact).Error; err != nil {
		return AgentSkillDefinition{}, AgentSkillCompatibilityReport{}, nil, NewExtensionError(ErrAgentSkillArtifactInvalid, "Agent Skill artifact is unavailable", id, false, err)
	}
	files, err := decodeAgentSkillArtifact(artifact.Content, DefaultAgentSkillLimits())
	if err != nil {
		return AgentSkillDefinition{}, AgentSkillCompatibilityReport{}, nil, err
	}
	if hashAgentSkillFiles(files) != row.ContentHash || artifact.Checksum != row.ContentHash {
		return AgentSkillDefinition{}, AgentSkillCompatibilityReport{}, nil, NewExtensionError(ErrAgentSkillChecksumMismatch, "Agent Skill artifact checksum mismatch", id, false, nil)
	}
	definition := agentSkillDefinitionFromRecord(row)
	definition.Body = extractAgentSkillBody(files["SKILL.md"])
	var report AgentSkillCompatibilityReport
	if json.Unmarshal([]byte(row.CompatibilityReportJSON), &report) != nil {
		report.Status = definition.CompatibilityStatus
	}
	return definition, report, files, nil
}

func (r *Repository) SetAgentSkillEnabled(ctx context.Context, id string, enabled bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		result := tx.Model(&agentSkillMetadataRecord{}).Where("extension_id = ? AND removed_at = ''", id).Updates(map[string]interface{}{"enabled": boolNumber(enabled), "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return NewExtensionError(ErrAgentSkillNotFound, "Agent Skill not found", id, false, nil)
		}
		return tx.Model(&extensionRecord{}).Where("extension_id = ?", id).Updates(map[string]interface{}{"enabled": boolNumber(enabled), "updated_at": now}).Error
	})
}
func (r *Repository) RemoveAgentSkill(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row agentSkillMetadataRecord
		if err := tx.Where("extension_id = ? AND removed_at = ''", id).First(&row).Error; err != nil {
			return NewExtensionError(ErrAgentSkillNotFound, "Agent Skill not found", id, false, err)
		}
		if err := tx.Model(&row).Updates(map[string]interface{}{"enabled": 0, "removed_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Table("extension_artifacts").Where("artifact_id = ?", row.ArtifactID).Update("archived_at", now).Error; err != nil {
			return err
		}
		return tx.Model(&extensionRecord{}).Where("extension_id = ?", id).Updates(map[string]interface{}{"enabled": 0, "archived_at": now, "updated_at": now}).Error
	})
}

func (r *Repository) SaveAgentSkillActivation(ctx context.Context, a AgentSkillActivation) error {
	paths, _ := json.Marshal(a.ResourcePaths)
	mappings, _ := json.Marshal(a.ToolMappings)
	row := agentSkillActivationRecord{ID: a.ID, ActivationID: a.ActivationID, ExtensionID: a.ExtensionID, AgentSkillName: a.AgentSkillName, Source: string(a.Source), ScopeType: string(a.Scope), CompatibilityStatus: string(a.CompatibilityStatus), UserID: a.UserID, CharacterID: a.CharacterID, ConversationID: a.ConversationID, Channel: a.Channel, TriggerType: a.TriggerType, Explicit: boolNumber(a.Explicit), Status: a.Status, LoadedTokens: a.LoadedTokens, ResourceReads: a.ResourceReads, ResourcePathsJSON: string(paths), ScriptsUsed: boolNumber(a.ScriptsUsed), ToolMappingsJSON: string(mappings), InstructionPosition: a.InstructionPosition, TokenLimitHit: boolNumber(a.TokenLimitHit), TraceID: a.TraceID, ErrorCode: a.ErrorCode, CreatedAt: a.CreatedAt.UTC().Format(time.RFC3339Nano)}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "activation_id"}}, DoUpdates: clause.AssignmentColumns([]string{"status", "loaded_tokens", "resource_reads", "resource_paths_json", "scripts_used", "tool_mappings_json", "instruction_position", "token_limit_hit", "error_code"})}).Create(&row).Error
}
func (r *Repository) ListAgentSkillActivations(ctx context.Context, extensionID, userID string, limit int) ([]AgentSkillActivation, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var rows []agentSkillActivationRecord
	query := r.db.WithContext(ctx).Where("extension_id = ? AND user_id = ?", extensionID, userID).Order("created_at DESC").Limit(limit)
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]AgentSkillActivation, 0, len(rows))
	for _, row := range rows {
		var paths []string
		var mappings []AgentSkillToolMapping
		_ = json.Unmarshal([]byte(row.ResourcePathsJSON), &paths)
		_ = json.Unmarshal([]byte(row.ToolMappingsJSON), &mappings)
		created, _ := time.Parse(time.RFC3339Nano, row.CreatedAt)
		result = append(result, AgentSkillActivation{ID: row.ID, ActivationID: row.ActivationID, ExtensionID: row.ExtensionID, AgentSkillName: row.AgentSkillName, Source: AgentSkillSource(row.Source), Scope: AgentSkillScope(row.ScopeType), CompatibilityStatus: AgentSkillCompatibilityStatus(row.CompatibilityStatus), UserID: row.UserID, CharacterID: row.CharacterID, ConversationID: row.ConversationID, Channel: row.Channel, TriggerType: row.TriggerType, Explicit: row.Explicit == 1, Status: row.Status, LoadedTokens: row.LoadedTokens, ResourceReads: row.ResourceReads, ResourcePaths: paths, ScriptsUsed: row.ScriptsUsed == 1, ToolMappings: mappings, InstructionPosition: row.InstructionPosition, TokenLimitHit: row.TokenLimitHit == 1, TraceID: row.TraceID, ErrorCode: row.ErrorCode, CreatedAt: created})
	}
	return result, nil
}

func agentSkillDefinitionFromRecord(row agentSkillMetadataRecord) AgentSkillDefinition {
	var metadata map[string]string
	var resources []AgentSkillResource
	var mappings []AgentSkillToolMapping
	_ = json.Unmarshal([]byte(row.MetadataJSON), &metadata)
	_ = json.Unmarshal([]byte(row.ResourceIndexJSON), &resources)
	_ = json.Unmarshal([]byte(row.ToolMappingsJSON), &mappings)
	created, _ := time.Parse(time.RFC3339Nano, row.CreatedAt)
	updated, _ := time.Parse(time.RFC3339Nano, row.UpdatedAt)
	return AgentSkillDefinition{ExtensionID: row.ExtensionID, Name: row.Name, Description: row.Description, License: row.License, Compatibility: row.Compatibility, Metadata: metadata, AllowedTools: row.AllowedTools, DisplayName: row.DisplayName, ShortDescription: row.ShortDescription, DefaultPrompt: row.DefaultPrompt, Source: AgentSkillSource(row.Source), Scope: AgentSkillScope(row.ScopeType), ScopeID: row.ScopeID, UserID: row.UserID, ArtifactID: row.ArtifactID, ContentHash: row.ContentHash, RawFrontmatter: json.RawMessage(row.RawFrontmatterJSON), ExtraFrontmatter: json.RawMessage(row.ExtraFrontmatterJSON), OpenAIMetadata: json.RawMessage(row.OpenAIMetadataJSON), Resources: resources, ToolMappings: mappings, CompatibilityStatus: AgentSkillCompatibilityStatus(row.CompatibilityStatus), Enabled: row.Enabled == 1, CreatedAt: created, UpdatedAt: updated}
}

func encodeAgentSkillArtifact(files map[string][]byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		header := &zip.FileHeader{Name: key, Method: zip.Deflate}
		header.SetMode(0o444)
		header.SetModTime(time.Unix(0, 0).UTC())
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(files[key]); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
func decodeAgentSkillArtifact(raw []byte, limits AgentSkillLimits) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, NewExtensionError(ErrAgentSkillArtifactInvalid, "Agent Skill artifact is invalid", err.Error(), false, err)
	}
	files := map[string][]byte{}
	canonical := map[string]struct{}{}
	var total int64
	for _, file := range reader.File {
		clean, err := validateAgentSkillRelativePath(file.Name, limits)
		if err != nil {
			return nil, err
		}
		mode := file.Mode()
		if mode&os.ModeSymlink != 0 || mode&os.ModeType != 0 {
			return nil, NewExtensionError(ErrAgentSkillArtifactInvalid, "Agent Skill artifact contains link", clean, false, nil)
		}
		key := strings.ToLower(norm.NFC.String(clean))
		if _, exists := canonical[key]; exists {
			return nil, NewExtensionError(ErrAgentSkillArtifactInvalid, "Agent Skill artifact contains duplicate path", clean, false, nil)
		}
		canonical[key] = struct{}{}
		total += int64(file.UncompressedSize64)
		if int64(file.UncompressedSize64) > limits.MaxResourceBytes || total > limits.MaxExpandedBytes {
			return nil, NewExtensionError(ErrAgentSkillArtifactInvalid, "Agent Skill artifact exceeds limit", "", false, nil)
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		content, readErr := io.ReadAll(io.LimitReader(rc, limits.MaxResourceBytes+1))
		rc.Close()
		if readErr != nil {
			return nil, readErr
		}
		if int64(len(content)) > limits.MaxResourceBytes {
			return nil, NewExtensionError(ErrAgentSkillArtifactInvalid, "Agent Skill artifact resource exceeds limit", clean, false, nil)
		}
		files[clean] = content
	}
	return files, nil
}
func extractAgentSkillBody(raw []byte) string {
	normalized := strings.ReplaceAll(string(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})), "\r\n", "\n")
	closing := strings.Index(normalized[4:], "\n---\n")
	if closing < 0 {
		return ""
	}
	return normalized[closing+9:]
}

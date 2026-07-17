package extension

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db              *gorm.DB
	configCipher    *configCipher
	configCipherErr error
}

func (r *Repository) ListVersions(ctx context.Context, extensionID string) ([]ExtensionVersionView, error) {
	var records []extensionVersionRecord
	if err := r.db.WithContext(ctx).Where("extension_id = ?", extensionID).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]ExtensionVersionView, len(records))
	for index, record := range records {
		createdAt, _ := time.Parse(time.RFC3339Nano, record.CreatedAt)
		if createdAt.IsZero() {
			createdAt, _ = time.Parse("2006-01-02 15:04:05", record.CreatedAt)
		}
		result[index] = ExtensionVersionView{Version: record.Version, Checksum: record.Checksum, Manifest: redactJSON(json.RawMessage(record.ManifestJSON)), CreatedAt: createdAt}
	}
	return result, nil
}

type extensionRecord struct {
	ID                     string `gorm:"column:id;primaryKey"`
	ExtensionID            string `gorm:"column:extension_id;uniqueIndex"`
	Kind                   string `gorm:"column:kind"`
	Name                   string `gorm:"column:name"`
	CurrentVersion         string `gorm:"column:current_version"`
	Source                 string `gorm:"column:source"`
	Enabled                int    `gorm:"column:enabled"`
	ManifestJSON           string `gorm:"column:manifest_json"`
	NormalizedManifestJSON string `gorm:"column:normalized_manifest_json"`
	CreatedAt              string `gorm:"column:created_at"`
	UpdatedAt              string `gorm:"column:updated_at"`
	ArchivedAt             string `gorm:"column:archived_at"`
}

func (extensionRecord) TableName() string { return "extensions" }

type extensionVersionRecord struct {
	ID           string `gorm:"column:id;primaryKey"`
	ExtensionID  string `gorm:"column:extension_id"`
	Version      string `gorm:"column:version"`
	ManifestJSON string `gorm:"column:manifest_json"`
	Checksum     string `gorm:"column:checksum"`
	CreatedAt    string `gorm:"column:created_at"`
}

func (extensionVersionRecord) TableName() string { return "extension_versions" }

type grantRecord struct {
	ID          string `gorm:"column:id;primaryKey"`
	ExtensionID string `gorm:"column:extension_id"`
	Capability  string `gorm:"column:capability"`
	Decision    string `gorm:"column:decision"`
	ScopeType   string `gorm:"column:scope_type"`
	ScopeID     string `gorm:"column:scope_id"`
	ExpiresAt   string `gorm:"column:expires_at"`
	ConsumedAt  string `gorm:"column:consumed_at"`
	CreatedAt   string `gorm:"column:created_at"`
	UpdatedAt   string `gorm:"column:updated_at"`
}

func (grantRecord) TableName() string { return "extension_capability_grants" }

type configRecord struct {
	ID            string `gorm:"column:id;primaryKey"`
	ExtensionID   string `gorm:"column:extension_id"`
	ScopeType     string `gorm:"column:scope_type"`
	ScopeID       string `gorm:"column:scope_id"`
	ConfigJSON    string `gorm:"column:config_json"`
	ConfigVersion int64  `gorm:"column:config_version"`
	CreatedAt     string `gorm:"column:created_at"`
	UpdatedAt     string `gorm:"column:updated_at"`
}

func (configRecord) TableName() string { return "extension_configs" }

type runRecord struct {
	RunID            string `gorm:"column:run_id;primaryKey"`
	ExtensionID      string `gorm:"column:extension_id"`
	ExtensionVersion string `gorm:"column:extension_version"`
	SkillID          string `gorm:"column:skill_id"`
	UserID           string `gorm:"column:user_id"`
	CharacterID      string `gorm:"column:character_id"`
	ConversationID   string `gorm:"column:conversation_id"`
	Channel          string `gorm:"column:channel"`
	Trigger          string `gorm:"column:trigger"`
	Status           string `gorm:"column:status"`
	InputSummary     string `gorm:"column:input_summary"`
	OutputSummary    string `gorm:"column:output_summary"`
	SideEffectsJSON  string `gorm:"column:side_effects_json"`
	IdempotencyKey   string `gorm:"column:idempotency_key"`
	StartedAt        string `gorm:"column:started_at"`
	FinishedAt       string `gorm:"column:finished_at"`
	DurationMS       int64  `gorm:"column:duration_ms"`
	ErrorCode        string `gorm:"column:error_code"`
	ErrorDetail      string `gorm:"column:error_detail"`
	TraceID          string `gorm:"column:trace_id"`
	CreatedAt        string `gorm:"column:created_at"`
}

func (runRecord) TableName() string { return "extension_runs" }

func NewRepository(db *gorm.DB) *Repository {
	cipher, err := newConfigCipher(db)
	return &Repository{db: db, configCipher: cipher, configCipherErr: err}
}

func (r *Repository) ResolveEnabled(ctx context.Context, definition SkillDefinition) (bool, error) {
	var record extensionRecord
	err := r.db.WithContext(ctx).Where("extension_id = ?", definition.ID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return definition.Enabled, nil
	}
	if err != nil {
		return false, err
	}
	return record.Enabled == 1, nil
}

func (r *Repository) UpsertDefinition(ctx context.Context, definition SkillDefinition) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	normalized := string(definition.Manifest)
	var value interface{}
	if json.Unmarshal(definition.Manifest, &value) == nil {
		if raw, err := json.Marshal(value); err == nil {
			normalized = string(raw)
		}
	}
	record := extensionRecord{
		ID: uuid.New().String(), ExtensionID: definition.ID, Kind: "Skill", Name: definition.Name,
		CurrentVersion: definition.Version, Source: string(definition.Source), Enabled: boolNumber(definition.Enabled),
		ManifestJSON: string(definition.Manifest), NormalizedManifestJSON: normalized, CreatedAt: now, UpdatedAt: now,
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing extensionRecord
		err := tx.Where("extension_id = ?", definition.ID).First(&existing).Error
		if err == nil {
			record.ID = existing.ID
			record.CreatedAt = existing.CreatedAt
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "extension_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "current_version", "source", "enabled", "manifest_json", "normalized_manifest_json", "updated_at"}),
		}).Create(&record).Error; err != nil {
			return err
		}
		hash := sha256.Sum256(definition.Manifest)
		version := extensionVersionRecord{ID: uuid.New().String(), ExtensionID: definition.ID, Version: definition.Version, ManifestJSON: string(definition.Manifest), Checksum: hex.EncodeToString(hash[:]), CreatedAt: now}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&version).Error
	})
}

func (r *Repository) CreateRun(ctx context.Context, run RunView) error {
	record := runRecordFromView(run)
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *Repository) UpdateRun(ctx context.Context, result SkillResult, outputSummary string) error {
	updates := map[string]interface{}{
		"status": string(result.Status), "output_summary": outputSummary, "finished_at": time.Now().UTC().Format(time.RFC3339Nano),
		"duration_ms": result.DurationMS,
	}
	if sideEffects, err := json.Marshal(result.SideEffects); err == nil {
		updates["side_effects_json"] = string(sideEffects)
	}
	if result.Error != nil {
		updates["error_code"] = result.Error.Code
		updates["error_detail"] = result.Error.Detail
	}
	return r.db.WithContext(ctx).Model(&runRecord{}).Where("run_id = ?", result.RunID).Updates(updates).Error
}

func (r *Repository) SetRunStatus(ctx context.Context, runID string, status RunStatus) error {
	return r.db.WithContext(ctx).Model(&runRecord{}).Where("run_id = ?", runID).Update("status", string(status)).Error
}

func (r *Repository) FindIdempotentRun(ctx context.Context, skillID, characterID, conversationID, key string) (*RunView, error) {
	var record runRecord
	err := r.db.WithContext(ctx).Where("skill_id = ? AND character_id = ? AND conversation_id = ? AND idempotency_key = ?", skillID, characterID, conversationID, key).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	view := runViewFromRecord(record)
	return &view, nil
}

func (r *Repository) ListRuns(ctx context.Context, scope ExecutionScope, filter RunFilter) (RunPage, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	query := r.db.WithContext(ctx).Model(&runRecord{})
	query = applyRunScope(query, scope)
	if filter.SkillID != "" {
		query = query.Where("skill_id = ?", filter.SkillID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.CharacterID != "" {
		if scope.CharacterID != "" && filter.CharacterID != scope.CharacterID {
			return RunPage{}, NewExtensionError(ErrSkillPermissionDenied, "Character scope mismatch", filter.CharacterID, false, nil)
		}
		query = query.Where("character_id = ?", filter.CharacterID)
	}
	if filter.Channel != "" {
		query = query.Where("channel = ?", filter.Channel)
	}
	if filter.Trigger != "" {
		query = query.Where("trigger = ?", filter.Trigger)
	}
	if filter.From != "" {
		query = query.Where("created_at >= ?", filter.From)
	}
	if filter.To != "" {
		query = query.Where("created_at <= ?", filter.To)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return RunPage{}, err
	}
	var records []runRecord
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
		return RunPage{}, err
	}
	items := make([]RunView, 0, len(records))
	for _, record := range records {
		items = append(items, runViewFromRecord(record))
	}
	return RunPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *Repository) GetRun(ctx context.Context, scope ExecutionScope, runID string) (RunView, error) {
	var record runRecord
	query := applyRunScope(r.db.WithContext(ctx), scope).Where("run_id = ?", runID)
	if err := query.First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return RunView{}, NewExtensionError(ErrSkillNotFound, "Run not found", runID, false, nil)
		}
		return RunView{}, err
	}
	return runViewFromRecord(record), nil
}

func (r *Repository) ValidateConversationScope(ctx context.Context, scope ExecutionScope) error {
	if strings.TrimSpace(scope.ConversationID) == "" {
		return nil
	}
	var conversation struct {
		CharacterID string `gorm:"column:character_id"`
		Channel     string `gorm:"column:channel"`
	}
	err := r.db.WithContext(ctx).Table("conversations").Select("character_id", "channel").Where("id = ?", scope.ConversationID).Take(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NewExtensionError(ErrSkillPermissionDenied, "Conversation scope is unavailable", scope.ConversationID, false, nil)
	}
	if err != nil {
		return fmt.Errorf("validate conversation scope: %w", err)
	}
	if conversation.CharacterID != scope.CharacterID || (scope.Channel != "" && conversation.Channel != "" && !strings.EqualFold(conversation.Channel, scope.Channel)) {
		return NewExtensionError(ErrSkillPermissionDenied, "Conversation scope mismatch", scope.ConversationID, false, nil)
	}
	return nil
}

func (r *Repository) LatestRun(ctx context.Context, scope ExecutionScope, skillID string) (*RunView, error) {
	var record runRecord
	query := applyRunScope(r.db.WithContext(ctx), scope).Where("skill_id = ?", skillID).Order("created_at DESC")
	err := query.First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	view := runViewFromRecord(record)
	return &view, nil
}

func (r *Repository) GetConfig(ctx context.Context, skillID string, scope PermissionScope, defaults json.RawMessage) (json.RawMessage, error) {
	var record configRecord
	err := r.db.WithContext(ctx).Where("extension_id = ? AND scope_type = ? AND scope_id = ?", skillID, scope.Type, scope.ID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return append(json.RawMessage(nil), normalizeJSON(defaults)...), nil
	}
	if err != nil {
		return nil, err
	}
	if r.configCipherErr != nil {
		return nil, r.configCipherErr
	}
	plain, encrypted, err := r.configCipher.decrypt(record.ConfigJSON)
	if err != nil {
		return nil, err
	}
	if !encrypted {
		secured, secureErr := r.configCipher.encrypt(plain)
		if secureErr != nil {
			return nil, secureErr
		}
		if updateErr := r.db.WithContext(ctx).Model(&configRecord{}).Where("id = ?", record.ID).Update("config_json", secured).Error; updateErr != nil {
			return nil, updateErr
		}
	}
	return json.RawMessage(plain), nil
}

func (r *Repository) UpdateConfig(ctx context.Context, skillID string, scope PermissionScope, config json.RawMessage) error {
	if err := validateScopeID(scope); err != nil {
		return err
	}
	if r.configCipherErr != nil {
		return r.configCipherErr
	}
	secured, err := r.configCipher.encrypt(config)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := configRecord{ID: uuid.New().String(), ExtensionID: skillID, ScopeType: string(scope.Type), ScopeID: scope.ID, ConfigJSON: secured, ConfigVersion: 1, CreatedAt: now, UpdatedAt: now}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "extension_id"}, {Name: "scope_type"}, {Name: "scope_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"config_json": secured, "config_version": gorm.Expr("config_version + 1"), "updated_at": now}),
	}).Create(&record).Error
}

func (r *Repository) ResetConfig(ctx context.Context, skillID string, scope PermissionScope) error {
	return r.db.WithContext(ctx).Where("extension_id = ? AND scope_type = ? AND scope_id = ?", skillID, scope.Type, scope.ID).Delete(&configRecord{}).Error
}

func (r *Repository) ListGrants(ctx context.Context, skillID string) ([]PermissionGrantView, error) {
	var records []grantRecord
	if err := r.db.WithContext(ctx).Where("extension_id = ?", skillID).Order("capability, scope_type, scope_id").Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]PermissionGrantView, 0, len(records))
	for _, record := range records {
		capability, _ := Capability(record.Capability)
		items = append(items, PermissionGrantView{ID: record.ID, Capability: record.Capability, Risk: capability.Risk, Description: capability.Description, Decision: PermissionDecision(record.Decision), ScopeType: ScopeType(record.ScopeType), ScopeID: record.ScopeID, ExpiresAt: record.ExpiresAt, ConsumedAt: record.ConsumedAt})
	}
	return items, nil
}

func (r *Repository) ReplaceGrants(ctx context.Context, skillID string, grants []PermissionGrantInput) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("extension_id = ?", skillID).Delete(&grantRecord{}).Error; err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		for _, grant := range grants {
			if _, ok := Capability(grant.Capability); !ok {
				return fmt.Errorf("unknown capability: %s", grant.Capability)
			}
			if !validDecision(grant.Decision) || !validScopeType(grant.ScopeType) {
				return fmt.Errorf("invalid permission grant")
			}
			if err := validateScopeID(PermissionScope{Type: grant.ScopeType, ID: grant.ScopeID}); err != nil {
				return err
			}
			record := grantRecord{ID: uuid.New().String(), ExtensionID: skillID, Capability: grant.Capability, Decision: string(grant.Decision), ScopeType: string(grant.ScopeType), ScopeID: grant.ScopeID, ExpiresAt: grant.ExpiresAt, CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) ResolveGrant(ctx context.Context, identity ExtensionIdentity, capability string, scope ExecutionScope, consume bool) (PermissionDecision, bool, error) {
	var records []grantRecord
	if err := r.db.WithContext(ctx).Where("extension_id = ? AND capability = ? AND consumed_at = ''", identity.SkillID, capability).Order("created_at DESC").Find(&records).Error; err != nil {
		return DecisionDeny, false, err
	}
	now := time.Now().UTC()
	for _, record := range records {
		if record.ExpiresAt != "" {
			expires, err := time.Parse(time.RFC3339Nano, record.ExpiresAt)
			if err == nil && !expires.After(now) {
				continue
			}
		}
		if !grantMatchesScope(record, scope) {
			continue
		}
		decision := PermissionDecision(record.Decision)
		if decision == DecisionAllowOnce && consume {
			result := r.db.WithContext(ctx).Model(&grantRecord{}).Where("id = ? AND consumed_at = ''", record.ID).Update("consumed_at", now.Format(time.RFC3339Nano))
			if result.Error != nil {
				return DecisionDeny, false, result.Error
			}
			if result.RowsAffected != 1 {
				continue
			}
		}
		return decision, true, nil
	}
	return DecisionDeny, false, nil
}

func applyRunScope(query *gorm.DB, scope ExecutionScope) *gorm.DB {
	if scope.UserID != "" {
		query = query.Where("user_id = ?", scope.UserID)
	}
	if scope.CharacterID != "" {
		query = query.Where("character_id = ?", scope.CharacterID)
	}
	if scope.ConversationID != "" {
		query = query.Where("conversation_id = ?", scope.ConversationID)
	}
	return query
}

func grantMatchesScope(record grantRecord, scope ExecutionScope) bool {
	switch ScopeType(record.ScopeType) {
	case ScopeGlobal:
		return record.ScopeID == ""
	case ScopeCharacter:
		return record.ScopeID != "" && record.ScopeID == scope.CharacterID
	case ScopeConversation:
		return record.ScopeID != "" && record.ScopeID == scope.ConversationID
	case ScopeChannel:
		return record.ScopeID != "" && record.ScopeID == scope.Channel
	case ScopeSession:
		return record.ScopeID != "" && record.ScopeID == scope.SessionID
	default:
		return false
	}
}

func validDecision(decision PermissionDecision) bool {
	switch decision {
	case DecisionDeny, DecisionAllowOnce, DecisionAllowSession, DecisionAllowCharacter, DecisionAllowAlways:
		return true
	default:
		return false
	}
}

func validScopeType(scope ScopeType) bool {
	switch scope {
	case ScopeGlobal, ScopeCharacter, ScopeConversation, ScopeChannel, ScopeSession:
		return true
	default:
		return false
	}
}

func boolNumber(value bool) int {
	if value {
		return 1
	}
	return 0
}

func runRecordFromView(run RunView) runRecord {
	sideEffects, _ := json.Marshal(run.SideEffects)
	return runRecord{RunID: run.RunID, ExtensionID: run.ExtensionID, ExtensionVersion: run.ExtensionVersion, SkillID: run.SkillID, UserID: run.UserID, CharacterID: run.CharacterID, ConversationID: run.ConversationID, Channel: run.Channel, Trigger: string(run.Trigger), Status: string(run.Status), InputSummary: run.InputSummary, OutputSummary: run.OutputSummary, SideEffectsJSON: string(sideEffects), IdempotencyKey: run.IdempotencyKey, StartedAt: run.StartedAt, FinishedAt: run.FinishedAt, DurationMS: run.DurationMS, ErrorCode: run.ErrorCode, ErrorDetail: run.ErrorDetail, TraceID: run.TraceID, CreatedAt: run.StartedAt}
}

func runViewFromRecord(record runRecord) RunView {
	sideEffects := []SideEffectRecord{}
	_ = json.Unmarshal([]byte(record.SideEffectsJSON), &sideEffects)
	return RunView{RunID: record.RunID, ExtensionID: record.ExtensionID, ExtensionVersion: record.ExtensionVersion, SkillID: record.SkillID, UserID: record.UserID, CharacterID: record.CharacterID, ConversationID: record.ConversationID, Channel: record.Channel, Trigger: SkillTrigger(record.Trigger), Status: RunStatus(record.Status), InputSummary: record.InputSummary, OutputSummary: record.OutputSummary, SideEffects: sideEffects, IdempotencyKey: record.IdempotencyKey, StartedAt: record.StartedAt, FinishedAt: record.FinishedAt, DurationMS: record.DurationMS, ErrorCode: record.ErrorCode, ErrorDetail: record.ErrorDetail, TraceID: record.TraceID}
}

func runResultFromView(run RunView) SkillResult {
	result := SkillResult{RunID: run.RunID, Status: run.Status, Output: json.RawMessage(run.OutputSummary), SideEffects: run.SideEffects, DurationMS: run.DurationMS, Duration: time.Duration(run.DurationMS) * time.Millisecond}
	if run.ErrorCode != "" {
		result.Error = NewExtensionError(run.ErrorCode, "Skill execution failed", run.ErrorDetail, false, nil)
	}
	return result
}

func validateGrantRoleIsolation(grant PermissionGrantInput, scope ExecutionScope) error {
	if grant.ScopeType == ScopeCharacter && scope.CharacterID != "" && grant.ScopeID != scope.CharacterID {
		return NewExtensionError(ErrSkillPermissionDenied, "Character scope mismatch", grant.ScopeID, false, nil)
	}
	if grant.ScopeType == ScopeConversation && scope.ConversationID != "" && grant.ScopeID != scope.ConversationID {
		return NewExtensionError(ErrSkillPermissionDenied, "Conversation scope mismatch", grant.ScopeID, false, nil)
	}
	if grant.ScopeType == ScopeSession && scope.SessionID != "" && grant.ScopeID != scope.SessionID {
		return NewExtensionError(ErrSkillPermissionDenied, "Session scope mismatch", grant.ScopeID, false, nil)
	}
	if grant.ScopeType == ScopeChannel && scope.Channel != "" && !strings.EqualFold(grant.ScopeID, scope.Channel) {
		return NewExtensionError(ErrSkillPermissionDenied, "Channel scope mismatch", grant.ScopeID, false, nil)
	}
	return nil
}

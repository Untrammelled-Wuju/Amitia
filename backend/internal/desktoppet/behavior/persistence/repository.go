package persistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/behavior/bindings"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func computePayloadHash(payload json.RawMessage) string {
	if len(payload) == 0 {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

var _ behavior.BehaviorStateRepository = (*GormBehaviorStateRepository)(nil)

type GormBehaviorStateRepository struct {
	db *gorm.DB
}

func NewGormBehaviorStateRepository(db *gorm.DB) *GormBehaviorStateRepository {
	return &GormBehaviorStateRepository{db: db}
}

func (r *GormBehaviorStateRepository) LoadContext(ctx context.Context, userID, characterID string) (*behavior.BehaviorContextSnapshot, error) {
	var model BehaviorContextModel
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND character_id = ?", userID, characterID).
		First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &behavior.BehaviorContextSnapshot{
			UserID:      userID,
			CharacterID: characterID,
			Revision:    1,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return contextModelToSnapshot(&model)
}

func contextJSONOrEmpty(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func (r *GormBehaviorStateRepository) SaveContextCAS(ctx context.Context, currentRevision int64, next behavior.BehaviorContextSnapshot) (bool, error) {
	stableJSON := contextJSONOrEmpty(next.Stable)
	transientJSON := contextJSONOrEmpty(next.Transient)
	activeToolsJSON := contextJSONOrEmpty(next.ActiveTools)
	voiceJSON := contextJSONOrEmpty(next.Voice)
	desktopGestureJSON := contextJSONOrEmpty(next.DesktopGesture)
	foregroundJSON := contextJSONOrEmpty(next.Foreground)
	cooldownsJSON := contextJSONOrEmpty(next.Cooldowns)
	recentSemanticsJSON := contextJSONOrEmpty(next.RecentSemantics)
	desiredJSON := contextJSONOrEmpty(next.Desired)
	lastSourceRevisionsJSON := contextJSONOrEmpty(next.LastSourceRevisions)
	updatedAt := next.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}

	var success bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&BehaviorContextModel{}).
			Where("user_id = ? AND character_id = ? AND revision = ?", next.UserID, next.CharacterID, currentRevision).
			Updates(map[string]interface{}{
				"revision":                next.Revision,
				"stable_state_json":       stableJSON,
				"transient_state_json":    transientJSON,
				"active_tools_json":       activeToolsJSON,
				"voice_state_json":        voiceJSON,
				"desktop_gesture_json":    desktopGestureJSON,
				"foreground_json":         foregroundJSON,
				"cooldowns_json":          cooldownsJSON,
				"recent_semantics_json":   recentSemanticsJSON,
				"desired_state_json":      desiredJSON,
				"last_source_revisions_json": lastSourceRevisionsJSON,
				"updated_at":              updatedAt.Format(time.RFC3339),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			success = true
			return nil
		}
		var count int64
		if err := tx.Model(&BehaviorContextModel{}).
			Where("user_id = ? AND character_id = ?", next.UserID, next.CharacterID).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			model := BehaviorContextModel{
				UserID:                next.UserID,
				CharacterID:           next.CharacterID,
				Revision:              next.Revision,
				StableStateJSON:       stableJSON,
				TransientStateJSON:    transientJSON,
				ActiveToolsJSON:       activeToolsJSON,
				VoiceStateJSON:        voiceJSON,
				DesktopGestureJSON:    desktopGestureJSON,
				ForegroundJSON:         foregroundJSON,
				CooldownsJSON:          cooldownsJSON,
				RecentSemanticsJSON:   recentSemanticsJSON,
				DesiredStateJSON:      desiredJSON,
				LastSourceRevisionsJSON: lastSourceRevisionsJSON,
				UpdatedAt:             updatedAt.Format(time.RFC3339),
			}
			if err := tx.Create(&model).Error; err != nil {
				return err
			}
			success = true
			return nil
		}
		return nil
	})
	return success, err
}

func (r *GormBehaviorStateRepository) InsertInboxIfAbsent(ctx context.Context, event behavior.BehaviorEventEnvelope) (bool, error) {
	payloadJSON := ""
	if len(event.Payload) > 0 {
		payloadJSON = string(event.Payload)
	}
	expiresAt := ""
	if event.ExpiresAt != nil {
		expiresAt = event.ExpiresAt.Format(time.RFC3339)
	}
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	receivedAt := event.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	eventEnvelopeJSON, _ := json.Marshal(event)
	payloadHash := computePayloadHash(event.Payload)

	model := BehaviorInboxModel{
		EventID:           event.EventID,
		DedupKey:          event.DedupKey,
		EventType:         event.EventType,
		SchemaVersion:     event.SchemaVersion,
		UserID:            event.UserID,
		CharacterID:       event.CharacterID,
		ConversationID:    event.ConversationID,
		InteractionID:     event.InteractionID,
		SessionID:         event.SessionID,
		ToolOperationID:   event.ToolOperationID,
		InstallationID:    event.InstallationID,
		PetInstanceID:     event.PetInstanceID,
		ReleaseID:         event.ReleaseID,
		OccurredAt:        occurredAt.Format(time.RFC3339),
		ReceivedAt:        receivedAt.Format(time.RFC3339),
		ExpiresAt:         expiresAt,
		Origin:            string(event.Origin),
		CorrelationID:     event.CorrelationID,
		CausationID:       event.CausationID,
		EventEnvelopeJSON: string(eventEnvelopeJSON),
		PayloadHash:       payloadHash,
		PayloadJSON:       payloadJSON,
		Status:            string(behavior.InboxPending),
		AttemptCount:      0,
		LastErrorCode:     "",
		LeaseOwner:        "",
		LeaseExpiresAt:    "",
		HeartbeatAt:       "",
		AvailableAt:       "",
		CreatedAt:         time.Now().Format(time.RFC3339),
	}
	err := r.db.WithContext(ctx).Create(&model).Error
	if err != nil && isUniqueConstraintError(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *GormBehaviorStateRepository) LeaseInbox(ctx context.Context, limit int, leaseToken string) ([]behavior.InboxRecord, error) {
	now := time.Now()
	nowStr := now.Format(time.RFC3339)
	leaseExpires := now.Add(30 * time.Second).Format(time.RFC3339)
	availableAtStr := nowStr

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var models []BehaviorInboxModel
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("status = ? AND available_at <= ? AND (lease_expires_at IS NULL OR lease_expires_at <= ?)",
				string(behavior.InboxPending), availableAtStr, nowStr).
			Order("occurred_at ASC").
			Limit(limit).
			Find(&models).Error
		if err != nil {
			return err
		}
		if len(models) == 0 {
			return nil
		}

		eventIDs := make([]string, len(models))
		for i, m := range models {
			eventIDs[i] = m.EventID
		}

		result := tx.Model(&BehaviorInboxModel{}).
			Where("event_id IN ? AND status = ? AND available_at <= ? AND (lease_expires_at IS NULL OR lease_expires_at <= ?)",
				eventIDs, string(behavior.InboxPending), availableAtStr, nowStr).
			Updates(map[string]interface{}{
				"status":           string(behavior.InboxLeased),
				"lease_owner":      leaseToken,
				"lease_expires_at": leaseExpires,
				"attempt_count":    gorm.Expr("attempt_count + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}

		records := make([]behavior.InboxRecord, 0, result.RowsAffected)
		for _, m := range models {
			record := inboxModelToRecord(&m)
			record.Status = behavior.InboxLeased
			record.AttemptCount = m.AttemptCount + 1
			record.LeaseOwner = leaseToken
			records = append(records, record)
		}

		_ = records
		return nil
	})
	if err != nil {
		return nil, err
	}

	var resultModels []BehaviorInboxModel
	err = r.db.WithContext(ctx).
		Where("lease_owner = ? AND status = ?", leaseToken, string(behavior.InboxLeased)).
		Order("occurred_at ASC").
		Limit(limit).
		Find(&resultModels).Error
	if err != nil {
		return nil, err
	}

	records := make([]behavior.InboxRecord, len(resultModels))
	for i, m := range resultModels {
		records[i] = inboxModelToRecord(&m)
	}
	return records, nil
}

func (r *GormBehaviorStateRepository) MarkInboxStatus(ctx context.Context, eventID string, status behavior.InboxStatus, errorCode string) error {
	updates := map[string]interface{}{
		"status":          string(status),
		"last_error_code": errorCode,
	}
	if status == behavior.InboxProcessed || status == behavior.InboxIgnored || status == behavior.InboxDeadLetter {
		updates["processed_at"] = time.Now().Format(time.RFC3339)
	}
	return r.db.WithContext(ctx).
		Model(&BehaviorInboxModel{}).
		Where("event_id = ?", eventID).
		Updates(updates).Error
}

func (r *GormBehaviorStateRepository) AppendDecision(ctx context.Context, decision behavior.BehaviorDecisionAudit) error {
	rejectedJSON := "[]"
	if len(decision.RejectedCandidates) > 0 {
		data, err := json.Marshal(decision.RejectedCandidates)
		if err != nil {
			return fmt.Errorf("marshal rejected candidates: %w", err)
		}
		rejectedJSON = string(data)
	}
	startedAt := ""
	if decision.StartedAt != nil {
		startedAt = decision.StartedAt.Format(time.RFC3339)
	}
	completedAt := ""
	if decision.CompletedAt != nil {
		completedAt = decision.CompletedAt.Format(time.RFC3339)
	}
	createdAt := decision.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	model := BehaviorDecisionModel{
		DecisionID:             decision.DecisionID,
		EventID:                decision.EventID,
		UserID:                 decision.UserID,
		CharacterID:            decision.CharacterID,
		InstallationID:         decision.InstallationID,
		ContextRevision:        decision.ContextRevision,
		RulesetVersion:         decision.RulesetVersion,
		InterruptPolicy:        decision.InterruptPolicy,
		MinimumPlayMS:          decision.MinimumPlayMS,
		MaximumPlayMS:          decision.MaximumPlayMS,
		Semantic:               decision.Semantic,
		ActionKey:              decision.ActionKey,
		Priority:               decision.Priority,
		Status:                 string(decision.Status),
		ReasonCode:             decision.ReasonCode,
		RejectedCandidatesJSON: rejectedJSON,
		RuntimeCommandID:       decision.RuntimeCommandID,
		CreatedAt:              createdAt.Format(time.RFC3339),
		StartedAt:              startedAt,
		CompletedAt:            completedAt,
	}
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *GormBehaviorStateRepository) UpdateDecisionStatus(ctx context.Context, decisionID string, status behavior.DecisionStatus, at interface{}) error {
	updates := map[string]interface{}{
		"status": string(status),
	}
	atStr := interfaceToTimeString(at)
	if atStr != "" {
		switch status {
		case behavior.DecisionStatusPlaying, behavior.DecisionStatusCommandSubmitted:
			updates["started_at"] = atStr
		case behavior.DecisionStatusCompleted, behavior.DecisionStatusInterrupted, behavior.DecisionStatusFailed, behavior.DecisionStatusExpired:
			updates["completed_at"] = atStr
		}
	}
	return r.db.WithContext(ctx).
		Model(&BehaviorDecisionModel{}).
		Where("decision_id = ?", decisionID).
		Updates(updates).Error
}

func (r *GormBehaviorStateRepository) LoadCooldowns(ctx context.Context, userID, characterID string) ([]behavior.CooldownRecord, error) {
	now := time.Now().Format(time.RFC3339)
	var models []BehaviorCooldownModel
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND character_id = ? AND until_at > ?", userID, characterID, now).
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	records := make([]behavior.CooldownRecord, len(models))
	for i, m := range models {
		records[i] = cooldownModelToRecord(&m)
	}
	return records, nil
}

func (r *GormBehaviorStateRepository) SaveCooldown(ctx context.Context, record behavior.CooldownRecord) error {
	model := BehaviorCooldownModel{
		UserID:           record.UserID,
		CharacterID:      record.CharacterID,
		CooldownKey:      record.CooldownKey,
		UntilAt:          record.UntilAt.Format(time.RFC3339),
		SourceDecisionID: record.SourceDecisionID,
		UpdatedAt:        time.Now().Format(time.RFC3339),
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "character_id"},
			{Name: "cooldown_key"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"until_at", "source_decision_id", "updated_at"}),
	}).Create(&model).Error
}

func (r *GormBehaviorStateRepository) CleanupExpiredCooldowns(ctx context.Context, before interface{}) error {
	beforeStr := interfaceToTimeString(before)
	if beforeStr == "" {
		beforeStr = time.Now().Format(time.RFC3339)
	}
	return r.db.WithContext(ctx).
		Where("until_at <= ?", beforeStr).
		Delete(&BehaviorCooldownModel{}).Error
}

func (r *GormBehaviorStateRepository) CleanupOldRecords(ctx context.Context, before interface{}) error {
	beforeStr := interfaceToTimeString(before)
	if beforeStr == "" {
		beforeStr = time.Now().AddDate(0, 0, -behavior.DefaultInboxRetentionDays).Format(time.RFC3339)
	}
	if err := r.db.WithContext(ctx).
		Where("created_at < ?", beforeStr).
		Delete(&BehaviorInboxModel{}).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Where("created_at < ?", beforeStr).
		Delete(&BehaviorDecisionModel{}).Error
}

func (r *GormBehaviorStateRepository) DeleteCharacterData(ctx context.Context, userID, characterID string) error {
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND character_id = ?", userID, characterID).
		Delete(&BehaviorContextModel{}).Error; err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND character_id = ?", userID, characterID).
		Delete(&BehaviorInboxModel{}).Error; err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND character_id = ?", userID, characterID).
		Delete(&BehaviorDecisionModel{}).Error; err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND character_id = ?", userID, characterID).
		Delete(&BehaviorCooldownModel{}).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Where("user_id = ? AND character_id = ?", userID, characterID).
		Delete(&bindings.BehaviorBindingModel{}).Error
}

func contextModelToSnapshot(m *BehaviorContextModel) (*behavior.BehaviorContextSnapshot, error) {
	snap := &behavior.BehaviorContextSnapshot{
		UserID:              m.UserID,
		CharacterID:         m.CharacterID,
		Revision:            m.Revision,
		Cooldowns:           make(map[string]time.Time),
		RecentSemantics:     make([]behavior.RecentSemanticRecord, 0),
		LastSourceRevisions: make(map[string]int64),
	}
	if m.StableStateJSON != "" && m.StableStateJSON != "{}" && m.StableStateJSON != "null" {
		if err := json.Unmarshal([]byte(m.StableStateJSON), &snap.Stable); err != nil {
			return nil, fmt.Errorf("unmarshal stable state: %w", err)
		}
	}
	if m.TransientStateJSON != "" && m.TransientStateJSON != "{}" && m.TransientStateJSON != "null" {
		if err := json.Unmarshal([]byte(m.TransientStateJSON), &snap.Transient); err != nil {
			return nil, fmt.Errorf("unmarshal transient state: %w", err)
		}
	}
	if m.ActiveToolsJSON != "" && m.ActiveToolsJSON != "{}" && m.ActiveToolsJSON != "null" {
		if err := json.Unmarshal([]byte(m.ActiveToolsJSON), &snap.ActiveTools); err != nil {
			return nil, fmt.Errorf("unmarshal active tools: %w", err)
		}
	}
	if m.VoiceStateJSON != "" && m.VoiceStateJSON != "{}" && m.VoiceStateJSON != "null" {
		if err := json.Unmarshal([]byte(m.VoiceStateJSON), &snap.Voice); err != nil {
			return nil, fmt.Errorf("unmarshal voice state: %w", err)
		}
	}
	if m.DesktopGestureJSON != "" && m.DesktopGestureJSON != "{}" && m.DesktopGestureJSON != "null" {
		if err := json.Unmarshal([]byte(m.DesktopGestureJSON), &snap.DesktopGesture); err != nil {
			return nil, fmt.Errorf("unmarshal desktop gesture: %w", err)
		}
	}
	if m.ForegroundJSON != "" && m.ForegroundJSON != "{}" && m.ForegroundJSON != "null" {
		if err := json.Unmarshal([]byte(m.ForegroundJSON), &snap.Foreground); err != nil {
			return nil, fmt.Errorf("unmarshal foreground: %w", err)
		}
	}
	if m.CooldownsJSON != "" && m.CooldownsJSON != "{}" && m.CooldownsJSON != "null" {
		if err := json.Unmarshal([]byte(m.CooldownsJSON), &snap.Cooldowns); err != nil {
			return nil, fmt.Errorf("unmarshal cooldowns: %w", err)
		}
	}
	if m.RecentSemanticsJSON != "" && m.RecentSemanticsJSON != "{}" && m.RecentSemanticsJSON != "null" {
		if err := json.Unmarshal([]byte(m.RecentSemanticsJSON), &snap.RecentSemantics); err != nil {
			return nil, fmt.Errorf("unmarshal recent semantics: %w", err)
		}
	}
	if m.DesiredStateJSON != "" && m.DesiredStateJSON != "{}" && m.DesiredStateJSON != "null" {
		if err := json.Unmarshal([]byte(m.DesiredStateJSON), &snap.Desired); err != nil {
			return nil, fmt.Errorf("unmarshal desired state: %w", err)
		}
	}
	if m.LastSourceRevisionsJSON != "" && m.LastSourceRevisionsJSON != "{}" && m.LastSourceRevisionsJSON != "null" {
		if err := json.Unmarshal([]byte(m.LastSourceRevisionsJSON), &snap.LastSourceRevisions); err != nil {
			return nil, fmt.Errorf("unmarshal last source revisions: %w", err)
		}
	}
	if m.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, m.UpdatedAt); err == nil {
			snap.UpdatedAt = t
		}
	}
	return snap, nil
}

func inboxModelToRecord(m *BehaviorInboxModel) behavior.InboxRecord {
	rec := behavior.InboxRecord{
		EventID:         m.EventID,
		DedupKey:        m.DedupKey,
		EventType:       m.EventType,
		SchemaVersion:   m.SchemaVersion,
		UserID:          m.UserID,
		CharacterID:     m.CharacterID,
		ConversationID:  m.ConversationID,
		InteractionID:   m.InteractionID,
		SessionID:       m.SessionID,
		ToolOperationID: m.ToolOperationID,
		InstallationID:  m.InstallationID,
		PetInstanceID:   m.PetInstanceID,
		ReleaseID:       m.ReleaseID,
		Origin:          behavior.EventOrigin(m.Origin),
		CorrelationID:   m.CorrelationID,
		CausationID:     m.CausationID,
		Status:          behavior.InboxStatus(m.Status),
		AttemptCount:    m.AttemptCount,
		LeaseOwner:      m.LeaseOwner,
		LastErrorCode:   m.LastErrorCode,
		LastErrorMessage: m.LastErrorMessage,
	}
	if m.OccurredAt != "" {
		if t, err := time.Parse(time.RFC3339, m.OccurredAt); err == nil {
			rec.OccurredAt = t
		}
	}
	if m.ReceivedAt != "" {
		if t, err := time.Parse(time.RFC3339, m.ReceivedAt); err == nil {
			rec.ReceivedAt = t
		}
	}
	if m.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, m.ExpiresAt); err == nil {
			rec.ExpiresAt = &t
		}
	}
	if m.LeaseExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, m.LeaseExpiresAt); err == nil {
			rec.LeaseExpiresAt = &t
		}
	}
	if m.HeartbeatAt != "" {
		if t, err := time.Parse(time.RFC3339, m.HeartbeatAt); err == nil {
			rec.HeartbeatAt = &t
		}
	}
	if m.AvailableAt != "" {
		if t, err := time.Parse(time.RFC3339, m.AvailableAt); err == nil {
			rec.AvailableAt = &t
		}
	}
	if m.PayloadJSON != "" && m.PayloadJSON != "{}" {
		rec.Payload = json.RawMessage(m.PayloadJSON)
	}
	if m.ProcessedAt != "" {
		if t, err := time.Parse(time.RFC3339, m.ProcessedAt); err == nil {
			rec.ProcessedAt = &t
		}
	}
	if m.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, m.CreatedAt); err == nil {
			rec.CreatedAt = t
		}
	}
	return rec
}

func cooldownModelToRecord(m *BehaviorCooldownModel) behavior.CooldownRecord {
	rec := behavior.CooldownRecord{
		UserID:           m.UserID,
		CharacterID:      m.CharacterID,
		CooldownKey:      m.CooldownKey,
		SourceDecisionID: m.SourceDecisionID,
	}
	if m.UntilAt != "" {
		if t, err := time.Parse(time.RFC3339, m.UntilAt); err == nil {
			rec.UntilAt = t
		}
	}
	if m.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, m.UpdatedAt); err == nil {
			rec.UpdatedAt = t
		}
	}
	return rec
}

func interfaceToTimeString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case time.Time:
		return val.Format(time.RFC3339)
	case *time.Time:
		if val == nil {
			return ""
		}
		return val.Format(time.RFC3339)
	case string:
		return val
	default:
		return ""
	}
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "Duplicate entry")
}

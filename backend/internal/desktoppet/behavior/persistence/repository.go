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
		initial := behavior.NewDefaultContext(userID, characterID)
		return &initial, nil
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
	var success bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, err := saveContextCASTx(tx, currentRevision, next)
		if err != nil {
			return err
		}
		success = ok
		return nil
	})
	return success, err
}

func (r *GormBehaviorStateRepository) CommitLeasedContextAndInboxCAS(ctx context.Context, currentRevision int64, next behavior.BehaviorContextSnapshot, eventID, leaseToken string, status behavior.InboxStatus) (bool, error) {
	leaseToken = strings.TrimSpace(leaseToken)
	eventID = strings.TrimSpace(eventID)
	if eventID == "" || leaseToken == "" {
		return false, nil
	}
	if status != behavior.InboxProcessed && status != behavior.InboxIgnored {
		return false, fmt.Errorf("invalid terminal inbox status for atomic context commit: %s", status)
	}

	committed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		leaseOwned, nowStr, err := refreshInboxLeaseForCommitTx(tx, eventID, leaseToken)
		if err != nil {
			return err
		}
		if !leaseOwned {
			return nil
		}

		ok, err := saveContextCASTx(tx, currentRevision, next)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		result := tx.Model(&BehaviorInboxModel{}).
			Where("event_id = ? AND status = ? AND lease_owner = ?", eventID, string(behavior.InboxLeased), leaseToken).
			Updates(map[string]interface{}{
				"status":             string(status),
				"last_error_code":    "",
				"last_error_message": "",
				"lease_owner":        "",
				"lease_expires_at":   "",
				"heartbeat_at":       "",
				"available_at":       "",
				"processed_at":       nowStr,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		committed = true
		return nil
	})
	return committed, err
}

func refreshInboxLeaseForCommitTx(tx *gorm.DB, eventID, leaseToken string) (bool, string, error) {
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	result := tx.Model(&BehaviorInboxModel{}).
		Where("event_id = ? AND status = ? AND lease_owner = ? AND lease_expires_at > ?", eventID, string(behavior.InboxLeased), leaseToken, nowStr).
		Updates(map[string]interface{}{
			"heartbeat_at":     nowStr,
			"lease_expires_at": now.Add(30 * time.Second).Format(time.RFC3339),
		})
	if result.Error != nil {
		return false, nowStr, result.Error
	}
	return result.RowsAffected == 1, nowStr, nil
}

func saveContextCASTx(tx *gorm.DB, currentRevision int64, next behavior.BehaviorContextSnapshot) (bool, error) {
	stableJSON := contextJSONOrEmpty(next.Stable)
	transientJSON := contextJSONOrEmpty(next.Transient)
	activeToolsJSON := contextJSONOrEmpty(next.ActiveTools)
	voiceJSON := contextJSONOrEmpty(next.Voice)
	desktopGestureJSON := contextJSONOrEmpty(next.DesktopGesture)
	foregroundJSON := contextJSONOrEmpty(next.Foreground)
	cooldownsJSON := contextJSONOrEmpty(next.Cooldowns)
	recentSemanticsJSON := contextJSONOrEmpty(next.RecentSemantics)
	recentEventKeysJSON := contextJSONOrEmpty(next.RecentEventKeys)
	desiredJSON := contextJSONOrEmpty(next.Desired)
	lastSourceRevisionsJSON := contextJSONOrEmpty(next.LastSourceRevisions)
	updatedAt := next.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}

	result := tx.Model(&BehaviorContextModel{}).
		Where("user_id = ? AND character_id = ? AND revision = ?", next.UserID, next.CharacterID, currentRevision).
		Updates(map[string]interface{}{
			"revision":                   next.Revision,
			"stable_state_json":          stableJSON,
			"transient_state_json":       transientJSON,
			"active_tools_json":          activeToolsJSON,
			"voice_state_json":           voiceJSON,
			"desktop_gesture_json":       desktopGestureJSON,
			"foreground_json":            foregroundJSON,
			"cooldowns_json":             cooldownsJSON,
			"recent_semantics_json":      recentSemanticsJSON,
			"recent_event_keys_json":     recentEventKeysJSON,
			"desired_state_json":         desiredJSON,
			"last_source_revisions_json": lastSourceRevisionsJSON,
			"updated_at":                 updatedAt.Format(time.RFC3339),
		})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected > 0 {
		return true, nil
	}

	var count int64
	if err := tx.Model(&BehaviorContextModel{}).
		Where("user_id = ? AND character_id = ?", next.UserID, next.CharacterID).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count != 0 {
		return false, nil
	}

	model := BehaviorContextModel{
		UserID:                  next.UserID,
		CharacterID:             next.CharacterID,
		Revision:                next.Revision,
		StableStateJSON:         stableJSON,
		TransientStateJSON:      transientJSON,
		ActiveToolsJSON:         activeToolsJSON,
		VoiceStateJSON:          voiceJSON,
		DesktopGestureJSON:      desktopGestureJSON,
		ForegroundJSON:          foregroundJSON,
		CooldownsJSON:           cooldownsJSON,
		RecentSemanticsJSON:     recentSemanticsJSON,
		RecentEventKeysJSON:     recentEventKeysJSON,
		DesiredStateJSON:        desiredJSON,
		LastSourceRevisionsJSON: lastSourceRevisionsJSON,
		UpdatedAt:               updatedAt.Format(time.RFC3339),
	}
	if err := tx.Create(&model).Error; err != nil {
		if isUniqueConstraintError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
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
	if limit <= 0 {
		return nil, nil
	}
	leaseToken = strings.TrimSpace(leaseToken)
	if leaseToken == "" {
		return nil, fmt.Errorf("lease token is required")
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	leaseExpires := now.Add(30 * time.Second).Format(time.RFC3339)
	leaseableStatuses := []string{string(behavior.InboxPending), string(behavior.InboxRetry)}
	eligibleSQL := `((status IN ? AND (available_at IS NULL OR available_at = '' OR available_at <= ?)) OR (status = ? AND lease_expires_at IS NOT NULL AND lease_expires_at <> '' AND lease_expires_at <= ?))`

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var models []BehaviorInboxModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where(eligibleSQL, leaseableStatuses, nowStr, string(behavior.InboxLeased), nowStr).
			Order("occurred_at ASC, created_at ASC").
			Limit(limit).
			Find(&models).Error; err != nil {
			return err
		}
		if len(models) == 0 {
			return nil
		}

		eventIDs := make([]string, 0, len(models))
		for _, model := range models {
			eventIDs = append(eventIDs, model.EventID)
		}

		result := tx.Model(&BehaviorInboxModel{}).
			Where("event_id IN ?", eventIDs).
			Where(eligibleSQL, leaseableStatuses, nowStr, string(behavior.InboxLeased), nowStr).
			Updates(map[string]interface{}{
				"status":           string(behavior.InboxLeased),
				"lease_owner":      leaseToken,
				"lease_expires_at": leaseExpires,
				"heartbeat_at":     nowStr,
				"attempt_count":    gorm.Expr("attempt_count + 1"),
			})
		return result.Error
	})
	if err != nil {
		return nil, err
	}

	var resultModels []BehaviorInboxModel
	if err := r.db.WithContext(ctx).
		Where("lease_owner = ? AND status = ? AND lease_expires_at > ?", leaseToken, string(behavior.InboxLeased), time.Now().UTC().Format(time.RFC3339)).
		Order("occurred_at ASC, created_at ASC").
		Limit(limit).
		Find(&resultModels).Error; err != nil {
		return nil, err
	}

	records := make([]behavior.InboxRecord, len(resultModels))
	for i := range resultModels {
		records[i] = inboxModelToRecord(&resultModels[i])
	}
	return records, nil
}

func (r *GormBehaviorStateRepository) RenewInboxLease(ctx context.Context, eventID, leaseToken string, leaseExpiresAt interface{}) (bool, error) {
	leaseToken = strings.TrimSpace(leaseToken)
	eventID = strings.TrimSpace(eventID)
	if eventID == "" || leaseToken == "" {
		return false, nil
	}
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	expiresAt := interfaceToTimeString(leaseExpiresAt)
	if expiresAt == "" {
		expiresAt = now.Add(30 * time.Second).Format(time.RFC3339)
	}
	result := r.db.WithContext(ctx).
		Model(&BehaviorInboxModel{}).
		Where("event_id = ? AND status = ? AND lease_owner = ? AND lease_expires_at > ?", eventID, string(behavior.InboxLeased), leaseToken, nowStr).
		Updates(map[string]interface{}{
			"lease_expires_at": expiresAt,
			"heartbeat_at":     nowStr,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func (r *GormBehaviorStateRepository) MarkInboxStatus(ctx context.Context, eventID, leaseOwner string, status behavior.InboxStatus, errorCode, errorMessage string) error {
	eventID = strings.TrimSpace(eventID)
	leaseOwner = strings.TrimSpace(leaseOwner)
	if eventID == "" || leaseOwner == "" {
		return gorm.ErrRecordNotFound
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	updates := map[string]interface{}{
		"status":             string(status),
		"last_error_code":    errorCode,
		"last_error_message": errorMessage,
		"lease_owner":        "",
		"lease_expires_at":   "",
		"heartbeat_at":       "",
	}
	if status == behavior.InboxProcessed || status == behavior.InboxIgnored || status == behavior.InboxDeadLetter {
		updates["processed_at"] = nowStr
		updates["available_at"] = ""
	}
	if status == behavior.InboxRetry {
		updates["available_at"] = now.Add(time.Second).Format(time.RFC3339)
	}
	result := r.db.WithContext(ctx).
		Model(&BehaviorInboxModel{}).
		Where("event_id = ? AND status = ? AND lease_owner = ? AND lease_expires_at > ?", eventID, string(behavior.InboxLeased), leaseOwner, nowStr).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}

	// Atomic context+inbox commits may have already finalized the row. Treat an
	// identical terminal status as an idempotent acknowledgement, but never let
	// an expired/stolen lease mutate a row that is still leased by another worker.
	var current BehaviorInboxModel
	err := r.db.WithContext(ctx).
		Select("event_id", "status").
		Where("event_id = ?", eventID).
		First(&current).Error
	if err != nil {
		return err
	}
	if current.Status == string(status) && (status == behavior.InboxProcessed || status == behavior.InboxIgnored || status == behavior.InboxDeadLetter) {
		return nil
	}
	return gorm.ErrRecordNotFound
}

func (r *GormBehaviorStateRepository) MarkInboxDeadLetter(ctx context.Context, eventID, leaseOwner, errorCode, errorMessage string, failedAt interface{}) error {
	eventID = strings.TrimSpace(eventID)
	leaseOwner = strings.TrimSpace(leaseOwner)
	if eventID == "" || leaseOwner == "" {
		return gorm.ErrRecordNotFound
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	failedAtStr := interfaceToTimeString(failedAt)
	if failedAtStr == "" {
		failedAtStr = nowStr
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		inboxResult := tx.Model(&BehaviorInboxModel{}).
			Where("event_id = ? AND status = ? AND lease_owner = ? AND lease_expires_at > ?", eventID, string(behavior.InboxLeased), leaseOwner, nowStr).
			Updates(map[string]interface{}{
				"status":             string(behavior.InboxDeadLetter),
				"last_error_code":    errorCode,
				"last_error_message": errorMessage,
				"lease_owner":        "",
				"lease_expires_at":   "",
				"heartbeat_at":       "",
				"available_at":       "",
				"processed_at":       nowStr,
			})
		if inboxResult.Error != nil {
			return inboxResult.Error
		}
		if inboxResult.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}

		decisionResult := tx.Model(&BehaviorDecisionModel{}).
			Where("event_id = ? AND status = ?", eventID, string(behavior.DecisionStatusSelected)).
			Updates(map[string]interface{}{
				"status":       string(behavior.DecisionStatusFailed),
				"reason_code":  errorCode,
				"completed_at": failedAtStr,
			})
		return decisionResult.Error
	})
}

func (r *GormBehaviorStateRepository) MarkInboxRetry(ctx context.Context, eventID, leaseOwner, errorCode, errorMessage string, availableAt interface{}) error {
	eventID = strings.TrimSpace(eventID)
	leaseOwner = strings.TrimSpace(leaseOwner)
	if eventID == "" || leaseOwner == "" {
		return gorm.ErrRecordNotFound
	}
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	availableAtStr := interfaceToTimeString(availableAt)
	if availableAtStr == "" {
		availableAtStr = now.Add(time.Second).Format(time.RFC3339)
	}
	result := r.db.WithContext(ctx).
		Model(&BehaviorInboxModel{}).
		Where("event_id = ? AND status = ? AND lease_owner = ? AND lease_expires_at > ?", eventID, string(behavior.InboxLeased), leaseOwner, nowStr).
		Updates(map[string]interface{}{
			"status":             string(behavior.InboxRetry),
			"last_error_code":    errorCode,
			"last_error_message": errorMessage,
			"lease_owner":        "",
			"lease_expires_at":   "",
			"heartbeat_at":       "",
			"available_at":       availableAtStr,
			"processed_at":       "",
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func decisionToModel(decision behavior.BehaviorDecisionAudit) (*BehaviorDecisionModel, error) {
	rejectedJSON := "[]"
	if len(decision.RejectedCandidates) > 0 {
		data, err := json.Marshal(decision.RejectedCandidates)
		if err != nil {
			return nil, fmt.Errorf("marshal rejected candidates: %w", err)
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
	return &BehaviorDecisionModel{
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
		FallbackDepth:          decision.FallbackDepth,
		ReturnPolicy:           decision.ReturnPolicy,
		ContextHash:            decision.ContextHash,
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
	}, nil
}

func decisionModelToAudit(model *BehaviorDecisionModel) (*behavior.BehaviorDecisionAudit, error) {
	if model == nil {
		return nil, nil
	}
	decision := &behavior.BehaviorDecisionAudit{
		BehaviorDecision: behavior.BehaviorDecision{
			DecisionID:       model.DecisionID,
			EventID:          model.EventID,
			UserID:           model.UserID,
			CharacterID:      model.CharacterID,
			InstallationID:   model.InstallationID,
			ContextRevision:  model.ContextRevision,
			RulesetVersion:   model.RulesetVersion,
			InterruptPolicy:  model.InterruptPolicy,
			MinimumPlayMS:    model.MinimumPlayMS,
			MaximumPlayMS:    model.MaximumPlayMS,
			FallbackDepth:    model.FallbackDepth,
			ReturnPolicy:     model.ReturnPolicy,
			Semantic:         model.Semantic,
			ActionKey:        model.ActionKey,
			Priority:         model.Priority,
			Status:           behavior.DecisionStatus(model.Status),
			ReasonCode:       model.ReasonCode,
			RuntimeCommandID: model.RuntimeCommandID,
		},
		ContextHash: model.ContextHash,
	}
	if model.RejectedCandidatesJSON != "" && model.RejectedCandidatesJSON != "[]" {
		if err := json.Unmarshal([]byte(model.RejectedCandidatesJSON), &decision.RejectedCandidates); err != nil {
			return nil, fmt.Errorf("unmarshal rejected candidates: %w", err)
		}
	}
	if model.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, model.CreatedAt); err == nil {
			decision.CreatedAt = t
		}
	}
	if model.StartedAt != "" {
		if t, err := time.Parse(time.RFC3339, model.StartedAt); err == nil {
			decision.StartedAt = &t
		}
	}
	if model.CompletedAt != "" {
		if t, err := time.Parse(time.RFC3339, model.CompletedAt); err == nil {
			decision.CompletedAt = &t
		}
	}
	return decision, nil
}

func (r *GormBehaviorStateRepository) AppendDecision(ctx context.Context, decision behavior.BehaviorDecisionAudit) error {
	model, err := decisionToModel(decision)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(model).Error
}

func (r *GormBehaviorStateRepository) CommitContextAndDecisionCAS(ctx context.Context, currentRevision int64, next behavior.BehaviorContextSnapshot, decision behavior.BehaviorDecisionAudit) (bool, error) {
	model, err := decisionToModel(decision)
	if err != nil {
		return false, err
	}
	committed := false
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, err := saveContextCASTx(tx, currentRevision, next)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if err := tx.Create(model).Error; err != nil {
			return err
		}
		committed = true
		return nil
	})
	return committed, err
}

func (r *GormBehaviorStateRepository) CommitLeasedContextAndDecisionCAS(ctx context.Context, currentRevision int64, next behavior.BehaviorContextSnapshot, decision behavior.BehaviorDecisionAudit, eventID, leaseToken string) (bool, error) {
	leaseToken = strings.TrimSpace(leaseToken)
	eventID = strings.TrimSpace(eventID)
	if eventID == "" || leaseToken == "" {
		return false, nil
	}
	model, err := decisionToModel(decision)
	if err != nil {
		return false, err
	}
	committed := false
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		leaseOwned, _, err := refreshInboxLeaseForCommitTx(tx, eventID, leaseToken)
		if err != nil {
			return err
		}
		if !leaseOwned {
			return nil
		}

		ok, err := saveContextCASTx(tx, currentRevision, next)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if err := tx.Create(model).Error; err != nil {
			return err
		}
		committed = true
		return nil
	})
	return committed, err
}

func (r *GormBehaviorStateRepository) FindDecisionByEventID(ctx context.Context, eventID string) (*behavior.BehaviorDecisionAudit, error) {
	if strings.TrimSpace(eventID) == "" {
		return nil, nil
	}
	var model BehaviorDecisionModel
	err := r.db.WithContext(ctx).
		Where("event_id = ?", eventID).
		Order("created_at DESC").
		Limit(1).
		First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return decisionModelToAudit(&model)
}

func (r *GormBehaviorStateRepository) FindDecisionByID(ctx context.Context, decisionID string) (*behavior.BehaviorDecisionAudit, error) {
	if strings.TrimSpace(decisionID) == "" {
		return nil, nil
	}
	var model BehaviorDecisionModel
	err := r.db.WithContext(ctx).
		Where("decision_id = ?", decisionID).
		First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return decisionModelToAudit(&model)
}

func (r *GormBehaviorStateRepository) UpdateDecisionStatus(ctx context.Context, decisionID string, status behavior.DecisionStatus, at interface{}) error {
	decisionID = strings.TrimSpace(decisionID)
	if decisionID == "" {
		return gorm.ErrRecordNotFound
	}
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

	query := r.db.WithContext(ctx).Model(&BehaviorDecisionModel{}).Where("decision_id = ?", decisionID)
	if prior := allowedDecisionPriorStatuses(status); len(prior) > 0 {
		query = query.Where("status IN ?", prior)
	}
	result := query.Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}
	return decisionTransitionNoopOrNotFound(ctx, r.db, decisionID)
}

func (r *GormBehaviorStateRepository) UpdateDecisionOutcome(ctx context.Context, decision behavior.BehaviorDecision) error {
	decisionID := strings.TrimSpace(decision.DecisionID)
	if decisionID == "" {
		return gorm.ErrRecordNotFound
	}
	updates := map[string]interface{}{
		"status":             string(decision.Status),
		"reason_code":        decision.ReasonCode,
		"runtime_command_id": decision.RuntimeCommandID,
	}
	now := time.Now().UTC().Format(time.RFC3339)
	switch decision.Status {
	case behavior.DecisionStatusCommandSubmitted, behavior.DecisionStatusPlaying:
		if decision.StartedAt != nil {
			updates["started_at"] = decision.StartedAt.UTC().Format(time.RFC3339)
		} else {
			updates["started_at"] = now
		}
	case behavior.DecisionStatusCompleted, behavior.DecisionStatusInterrupted, behavior.DecisionStatusFailed, behavior.DecisionStatusExpired:
		if decision.CompletedAt != nil {
			updates["completed_at"] = decision.CompletedAt.UTC().Format(time.RFC3339)
		} else {
			updates["completed_at"] = now
		}
	}

	query := r.db.WithContext(ctx).Model(&BehaviorDecisionModel{}).Where("decision_id = ?", decisionID)
	if prior := allowedDecisionPriorStatuses(decision.Status); len(prior) > 0 {
		query = query.Where("status IN ?", prior)
	}
	result := query.Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}
	return decisionTransitionNoopOrNotFound(ctx, r.db, decisionID)
}

func allowedDecisionPriorStatuses(status behavior.DecisionStatus) []string {
	switch status {
	case behavior.DecisionStatusCommandSubmitted:
		return []string{string(behavior.DecisionStatusSelected)}
	case behavior.DecisionStatusPlaying:
		return []string{string(behavior.DecisionStatusSelected), string(behavior.DecisionStatusCommandSubmitted)}
	case behavior.DecisionStatusCompleted, behavior.DecisionStatusInterrupted, behavior.DecisionStatusFailed, behavior.DecisionStatusExpired:
		return []string{
			string(behavior.DecisionStatusSelected),
			string(behavior.DecisionStatusCommandSubmitted),
			string(behavior.DecisionStatusPlaying),
		}
	default:
		return nil
	}
}

func decisionTransitionNoopOrNotFound(ctx context.Context, db *gorm.DB, decisionID string) error {
	var count int64
	if err := db.WithContext(ctx).
		Model(&BehaviorDecisionModel{}).
		Where("decision_id = ?", decisionID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	// The decision exists but has already advanced to the same or a terminal
	// state. Late/replayed runtime feedback must not regress it.
	return nil
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
		ActiveTools:         make(map[string]behavior.ToolOperationState),
		Cooldowns:           make(map[string]time.Time),
		RecentSemantics:     make([]behavior.RecentSemanticRecord, 0),
		RecentEventKeys:     make([]string, 0),
		Desired:             behavior.DesiredBehaviorState{Semantic: "fallback_idle", SourceLayer: "stable"},
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
	if m.RecentEventKeysJSON != "" && m.RecentEventKeysJSON != "{}" && m.RecentEventKeysJSON != "null" {
		if err := json.Unmarshal([]byte(m.RecentEventKeysJSON), &snap.RecentEventKeys); err != nil {
			return nil, fmt.Errorf("unmarshal recent event keys: %w", err)
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
		EventID:          m.EventID,
		DedupKey:         m.DedupKey,
		EventType:        m.EventType,
		SchemaVersion:    m.SchemaVersion,
		UserID:           m.UserID,
		CharacterID:      m.CharacterID,
		ConversationID:   m.ConversationID,
		InteractionID:    m.InteractionID,
		SessionID:        m.SessionID,
		ToolOperationID:  m.ToolOperationID,
		InstallationID:   m.InstallationID,
		PetInstanceID:    m.PetInstanceID,
		ReleaseID:        m.ReleaseID,
		Origin:           behavior.EventOrigin(m.Origin),
		CorrelationID:    m.CorrelationID,
		CausationID:      m.CausationID,
		Status:           behavior.InboxStatus(m.Status),
		AttemptCount:     m.AttemptCount,
		LeaseOwner:       m.LeaseOwner,
		LastErrorCode:    m.LastErrorCode,
		LastErrorMessage: m.LastErrorMessage,
	}
	if m.EventEnvelopeJSON != "" && m.EventEnvelopeJSON != "{}" {
		rec.EventEnvelopeJSON = json.RawMessage(m.EventEnvelopeJSON)
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
		return val.UTC().Format(time.RFC3339)
	case *time.Time:
		if val == nil {
			return ""
		}
		return val.UTC().Format(time.RFC3339)
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

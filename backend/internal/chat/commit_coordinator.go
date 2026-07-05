package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/interaction"
	newoutbox "github.com/u-ai/backend/internal/outbox"
	"github.com/u-ai/backend/internal/psyche"
	"gorm.io/gorm"
)

type RelationshipStateRecord struct {
	ID           string `gorm:"primaryKey;column:id"`
	CharacterID  string `gorm:"column:character_id"`
	RelationType string `gorm:"column:relation_type"`
	RelationData string `gorm:"column:relation_data"`
	CreatedAt    string `gorm:"column:created_at"`
	UpdatedAt    string `gorm:"column:updated_at"`
}

func (RelationshipStateRecord) TableName() string {
	return "relationship_states"
}

type relationshipEventRecord struct {
	ID          string `gorm:"primaryKey;column:id"`
	CharacterID string `gorm:"column:character_id"`
	EventType   string `gorm:"column:event_type"`
	EventData   string `gorm:"column:event_data"`
	CreatedAt   string `gorm:"column:created_at"`
}

func (relationshipEventRecord) TableName() string {
	return "relationship_events"
}

type messageCommitPlan struct {
	Request         *ProcessMessageRequest
	Conversation    string
	Character       string
	CharacterName   string
	UserMessageID   string
	Reply           string
	Lines           []string
	Source          string
	Runtime         *interaction.RuntimeAssembly
	CommitToken     string
	CommitOwner     string
	LeaseID         string
	LeaseOwnerToken string
}

type messageCommitResult struct {
	CommitID          string
	MessageIDs        []string
	LastSequence      int64
	Events            []interaction.OutboxRecord
	StateVersions     map[string]int64
	DeliveryIntentIDs []string
}

func (s *service) commitInteraction(plan messageCommitPlan) (*messageCommitResult, error) {
	result := &messageCommitResult{}

	if s.deliveryStore != nil && plan.Request != nil && plan.Request.InteractionID != "" {
		leaseID, ownerToken, err := s.deliveryStore.AcquireOutputLease(
			plan.Request.InteractionID, plan.Character, plan.Request.UserID, plan.Request.Channel)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire output lease: %w", err)
		}
		plan.LeaseID = leaseID
		plan.LeaseOwnerToken = ownerToken
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.acquireAndValidateCommitTokenTx(tx, &plan); err != nil {
			return err
		}
		for _, text := range plan.Lines {
			aiMsgID := uuid.New().String()
			aiMsg := &Message{ID: aiMsgID, ConversationID: plan.Conversation, Role: "assistant", Content: text, MsgType: "text", Source: plan.Source, RequestID: plan.Request.RequestID}
			if err := tx.Create(aiMsg).Error; err != nil {
				return err
			}
			result.MessageIDs = append(result.MessageIDs, aiMsgID)
			result.LastSequence = aiMsg.Sequence
		}
		now := time.Now().Format("2006-01-02 15:04:05")
		if err := tx.Model(&Message{}).Where("id = ?", plan.UserMessageID).Updates(map[string]interface{}{"status": "sent", "updated_at": now}).Error; err != nil {
			if plan.Source != "proactive" {
				return err
			}
		}
		if err := tx.Exec("UPDATE conversations SET updated_at = ?, message_count = (SELECT COUNT(*) FROM messages WHERE conversation_id = ?) WHERE id = ?", now, plan.Conversation, plan.Conversation).Error; err != nil {
			return err
		}
		if shouldCommitRuntime(plan.Request) {
			if plan.Runtime != nil && plan.Runtime.Appraisal != nil {
				if !plan.Request.IsInternal {
					if err := s.applyAppraisalResultTx(tx, plan); err != nil {
						return err
					}
				}
			}
			if !plan.Request.IsInternal {
				if err := s.updateRelationshipStateTx(tx, plan); err != nil {
					return err
				}
				if err := s.updateNeedStateTx(tx, plan); err != nil {
					return err
				}
			}
			events, deliveryIntentIDs, err := s.appendInteractionOutboxTx(tx, plan, result.MessageIDs)
			if err != nil {
				return err
			}
			result.Events = events
			result.DeliveryIntentIDs = deliveryIntentIDs
		}
		if err := s.transitionInteractionCommittedTx(tx, plan, result); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if plan.LeaseID != "" {
			_ = s.deliveryStore.ReleaseOutputLease(plan.LeaseID, plan.LeaseOwnerToken)
		}
		return nil, err
	}

	if plan.LeaseID != "" {
		_ = s.deliveryStore.ReleaseOutputLease(plan.LeaseID, plan.LeaseOwnerToken)
	}
	return result, nil
}

func (s *service) acquireAndValidateCommitTokenTx(tx *gorm.DB, plan *messageCommitPlan) error {
	if !shouldCommitRuntime(plan.Request) {
		return nil
	}
	var record interaction.InteractionRecordModel
	err := tx.Where("id = ?", plan.Request.InteractionID).Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return interaction.ErrInteractionNotFound
	}
	if err != nil {
		return err
	}
	if record.StatusVersion != plan.Request.ExpectedStatusVersion {
		return interaction.ErrVersionConflict
	}
	if interaction.InteractionStatus(record.Status) != interaction.InteractionStatusContextReady {
		return fmt.Errorf("%w: stale status %s", interaction.ErrInvalidTransition, record.Status)
	}
	if record.SupersededByID != "" || !record.CancelRequestedAt.IsZero() {
		return interaction.ErrVersionConflict
	}
	token := uuid.New().String()
	owner := uuid.New().String()
	now := time.Now().UTC()
	res := tx.Model(&interaction.InteractionRecordModel{}).
		Where("id = ? AND status_version = ? AND status = ? AND (superseded_by_id = '' OR superseded_by_id IS NULL)",
			plan.Request.InteractionID, record.StatusVersion, string(interaction.InteractionStatusContextReady)).
		Updates(map[string]interface{}{
			"commit_token":       token,
			"commit_owner":       owner,
			"commit_acquired_at": now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return interaction.ErrCommitTokenUnavailable
	}
	plan.CommitToken = token
	plan.CommitOwner = owner
	return nil
}

func (s *service) transitionInteractionCommittedTx(tx *gorm.DB, plan messageCommitPlan, result *messageCommitResult) error {
	if !shouldCommitRuntime(plan.Request) {
		return nil
	}
	commitID := uuid.New().String()
	result.CommitID = commitID
	msgIDsJSON, _ := json.Marshal(result.MessageIDs)
	deliveryIDsJSON, _ := json.Marshal(result.DeliveryIntentIDs)
	now := time.Now().UTC()
	res := tx.Model(&interaction.InteractionRecordModel{}).
		Where("id = ? AND status_version = ? AND status = ? AND commit_token = ? AND commit_owner = ?",
			plan.Request.InteractionID, plan.Request.ExpectedStatusVersion, string(interaction.InteractionStatusContextReady), plan.CommitToken, plan.CommitOwner).
		Updates(map[string]interface{}{
			"status":              string(interaction.InteractionStatusCommitted),
			"status_version":      plan.Request.ExpectedStatusVersion + 1,
			"commit_id":           commitID,
			"result_message_ids":  string(msgIDsJSON),
			"delivery_intent_ids": string(deliveryIDsJSON),
			"updated_at":          now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return fmt.Errorf("%w: interaction commit transition failed", interaction.ErrVersionConflict)
	}
	return nil
}

func shouldCommitRuntime(req *ProcessMessageRequest) bool {
	return req != nil && strings.TrimSpace(req.InteractionID) != "" && req.Runtime != nil
}

func (s *service) updatePsycheStateTx(tx *gorm.DB, charID string) error {
	if s.psycheStore == nil || charID == "" {
		return nil
	}
	store := s.psycheStore
	if sqliteStore, ok := s.psycheStore.(*psyche.SQLitePsycheStore); ok {
		store = sqliteStore.WithDB(tx)
	}
	return s.updatePsycheStateWithStore(store, charID)
}

func (s *service) updatePsycheStateWithStore(store psyche.PsycheStore, charID string) error {
	for attempt := 0; attempt < 3; attempt++ {
		state, err := store.LoadState(charID)
		if err != nil {
			if !errors.Is(err, psyche.ErrStateNotFound) {
				return err
			}
			initial := psyche.NewPsycheState(charID)
			if err := store.SaveState(&initial); err != nil {
				if errors.Is(err, psyche.ErrVersionConflict) {
					continue
				}
				return err
			}
			state = &initial
		}
		event := psyche.PsycheEvent{
			ID:          uuid.New().String(),
			CharacterID: charID,
			Type:        psyche.EventTypeInteraction,
			Source:      "chat.process_message",
			EnergyDelta: computePsycheEnergyDelta(*state),
			Timestamp:   time.Now().UTC(),
		}
		newState := psyche.ApplyEvent(*state, event)
		if err := store.SaveState(&newState); err != nil {
			if errors.Is(err, psyche.ErrVersionConflict) {
				continue
			}
			return err
		}
		if err := store.AppendEvent(&event); err != nil {
			return err
		}
		snapshot := psyche.CreateSnapshot(newState)
		if err := store.SaveSnapshot(&snapshot); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("%w: character %s retry exhausted", psyche.ErrVersionConflict, charID)
}

func (s *service) updateRelationshipStateTx(tx *gorm.DB, plan messageCommitPlan) error {
	relationType := relationshipTypeForRequest(plan.Request)
	now := time.Now().Format("2006-01-02 15:04:05")
	var existing RelationshipStateRecord
	err := tx.Where("character_id = ? AND relation_type = ?", plan.Character, relationType).Order("updated_at DESC").Take(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	data := map[string]float64{
		"trust":            0.5,
		"familiarity":      0,
		"security":         0.5,
		"tension":          0,
		"repairConfidence": 0.5,
		"boundary":         0.5,
	}
	if existing.RelationData != "" {
		_ = json.Unmarshal([]byte(existing.RelationData), &data)
	}
	famDelta := computeRelationshipFamiliarityDelta(data)
	trustDelta := computeRelationshipTrustDelta(data)
	secDelta := computeRelationshipSecurityDelta(data)
	data["familiarity"] = clampRelationshipValue(data["familiarity"] + famDelta)
	data["trust"] = clampRelationshipValue(data["trust"] + trustDelta)
	data["security"] = clampRelationshipValue(data["security"] + secDelta)
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if existing.ID == "" {
		existing = RelationshipStateRecord{
			ID:           uuid.New().String(),
			CharacterID:  plan.Character,
			RelationType: relationType,
			CreatedAt:    now,
		}
	}
	existing.RelationData = string(raw)
	existing.UpdatedAt = now
	if err := tx.Save(&existing).Error; err != nil {
		return err
	}
	eventData, err := json.Marshal(map[string]interface{}{
		"interactionId":  plan.Request.InteractionID,
		"conversationId": plan.Conversation,
		"requestId":      plan.Request.RequestID,
		"relationType":   relationType,
		"delta": map[string]float64{
			"familiarity": famDelta,
			"trust":       trustDelta,
			"security":    secDelta,
		},
	})
	if err != nil {
		return err
	}
	return tx.Create(&relationshipEventRecord{
		ID:          uuid.New().String(),
		CharacterID: plan.Character,
		EventType:   "interaction",
		EventData:   string(eventData),
		CreatedAt:   now,
	}).Error
}

func relationshipTypeForRequest(req *ProcessMessageRequest) string {
	if req == nil {
		return "channel:unknown"
	}
	if peer := strings.TrimSpace(req.PeerID); peer != "" {
		return "peer:" + peer
	}
	if channel := strings.TrimSpace(req.Channel); channel != "" {
		return "channel:" + channel
	}
	return "channel:web"
}

func clampRelationshipValue(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func clampNeedValue(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func getOrDefaultFloat(data map[string]float64, key string, defaultValue float64) float64 {
	if v, ok := data[key]; ok {
		return v
	}
	return defaultValue
}

func computePsycheEnergyDelta(state psyche.PsycheState) float64 {
	now := time.Now().UTC()
	hour := float64(now.Hour()) + float64(now.Minute())/60.0
	timeFactor := 1.0
	if hour >= 22 || hour < 6 {
		timeFactor = 1.5
	} else if hour >= 14 && hour < 16 {
		timeFactor = 1.2
	}
	stressFactor := 1.0 + state.Stress*1.5
	baseCost := -0.03
	return baseCost * timeFactor * stressFactor
}

func computeRelationshipFamiliarityDelta(data map[string]float64) float64 {
	fam := getOrDefaultFloat(data, "familiarity", 0)
	baseGain := 0.005
	if fam < 0.3 {
		return baseGain * 3
	}
	return baseGain * (1 - fam)
}

func computeRelationshipTrustDelta(data map[string]float64) float64 {
	trust := getOrDefaultFloat(data, "trust", 0.5)
	baseGain := 0.003
	if trust < 0.3 {
		return baseGain * 2
	}
	return baseGain * (1 - trust) * 0.5
}

func computeRelationshipSecurityDelta(data map[string]float64) float64 {
	security := getOrDefaultFloat(data, "security", 0.5)
	baseGain := 0.002
	if security < 0.3 {
		return baseGain * 4
	}
	return baseGain * (1 - security) * 0.3
}

func (s *service) appendInteractionOutboxTx(tx *gorm.DB, plan messageCommitPlan, messageIDs []string) ([]interaction.OutboxRecord, []string, error) {
	now := time.Now()
	deliveryIntentIDs := []string{}
	events := []interaction.OutboxRecord{
		newInteractionOutboxRecord(plan.Request.InteractionID, "interaction.completed", map[string]interface{}{
			"interactionId":  plan.Request.InteractionID,
			"conversationId": plan.Conversation,
			"characterId":    plan.Character,
			"reply":          plan.Reply,
			"messageIds":     messageIDs,
			"delivery":       plan.Runtime.Delivery,
		}, now),
		newInteractionOutboxRecord(plan.Request.InteractionID, "interaction.state_changed", map[string]interface{}{
			"interactionId":  plan.Request.InteractionID,
			"conversationId": plan.Conversation,
			"characterId":    plan.Character,
			"channel":        plan.Request.Channel,
			"status":         "committed",
			"timestamp":      now,
		}, now),
		newInteractionOutboxRecord(plan.Request.InteractionID, "interaction.runtime_assembled", map[string]interface{}{
			"interactionId": plan.Request.InteractionID,
			"path":          plan.Runtime.Path,
			"safety":        plan.Runtime.Safety,
			"delivery":      plan.Runtime.Delivery,
			"transaction":   plan.Runtime.Transaction.Name,
			"timestamp":     now,
		}, now),
	}
	for _, event := range events {
		model := newoutbox.OutboxRecordModel{
			ID:          event.ID,
			AggregateID: event.AggregateID,
			EventType:   event.EventType,
			Payload:     string(event.Payload),
			Status:      string(newoutbox.OutboxStatusPending),
			MaxRetries:  newoutbox.DefaultMaxRetries,
			NextRetryAt: now.Format("2006-01-02 15:04:05"),
			AvailableAt: now.Format("2006-01-02 15:04:05"),
			CreatedAt:   now.Format("2006-01-02 15:04:05"),
			UpdatedAt:   now.Format("2006-01-02 15:04:05"),
		}
		if err := tx.Create(&model).Error; err != nil {
			return nil, nil, err
		}
	}
	if plan.Runtime != nil && plan.Runtime.Delivery.Channel != "" {
		var diErr error
		deliveryIntentIDs, diErr = createDeliveryIntentsInTx(tx, plan, messageIDs, now)
		if diErr != nil {
			return nil, nil, diErr
		}
	}
	return events, deliveryIntentIDs, nil
}

func createDeliveryIntentsInTx(tx *gorm.DB, plan messageCommitPlan, messageIDs []string, now time.Time) ([]string, error) {
	ids := []string{}
	channel := plan.Runtime.Delivery.Channel
	peerID := ""
	if plan.Runtime.Delivery.PeerID != "" {
		peerID = plan.Runtime.Delivery.PeerID
	} else if plan.Request.PeerID != "" {
		peerID = plan.Request.PeerID
	}
	for _, msgID := range messageIDs {
		stableID := delivery.GenerateDeliveryID(plan.Request.InteractionID, channel, peerID, msgID)
		payload, _ := json.Marshal(map[string]interface{}{
			"messageId":      msgID,
			"conversationId": plan.Conversation,
			"characterId":    plan.Character,
			"content":        plan.Reply,
		})
		result := tx.Exec("INSERT OR IGNORE INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			stableID, plan.Request.InteractionID, channel, peerID, "text", string(payload), "pending", now.Format("2006-01-02 15:04:05"), 5)
		if result.Error != nil {
			return ids, result.Error
		}
		if result.RowsAffected != 1 {
			continue
		}
		ids = append(ids, stableID)
	}
	return ids, nil
}

func newInteractionOutboxRecord(aggregateID, eventType string, payload map[string]interface{}, now time.Time) interaction.OutboxRecord {
	raw, _ := json.Marshal(payload)
	return interaction.OutboxRecord{
		ID:          uuid.New().String(),
		AggregateID: aggregateID,
		EventType:   eventType,
		Payload:     raw,
		Status:      interaction.OutboxStatusPending,
		MaxRetries:  interaction.DefaultMaxRetries,
		CreatedAt:   now,
		NextRetryAt: now,
	}
}

func (s *service) applyAppraisalResultTx(tx *gorm.DB, plan messageCommitPlan) error {
	if plan.Runtime == nil || plan.Runtime.Appraisal == nil {
		return nil
	}
	appraisal := plan.Runtime.Appraisal
	if s.psycheStore != nil && appraisal.PsycheDelta != 0 {
		charID := plan.Character
		var store psyche.PsycheStore = s.psycheStore
		if sqliteStore, ok := s.psycheStore.(*psyche.SQLitePsycheStore); ok {
			store = sqliteStore.WithDB(tx)
		}
		state, err := store.LoadState(charID)
		if err != nil {
			if errors.Is(err, psyche.ErrStateNotFound) {
				initial := psyche.NewPsycheState(charID)
				if err := store.SaveState(&initial); err != nil {
					return err
				}
				state = &initial
			} else {
				return err
			}
		}
		event := psyche.PsycheEvent{
			ID:          uuid.New().String(),
			CharacterID: charID,
			Type:        psyche.EventTypeInteraction,
			Source:      "appraisal.delta",
			EnergyDelta: appraisal.PsycheDelta * 0.02,
			Timestamp:   time.Now().UTC(),
		}
		newState := psyche.ApplyEvent(*state, event)
		if err := store.SaveState(&newState); err != nil {
			return err
		}
		if err := store.AppendEvent(&event); err != nil {
			return err
		}
	}
	if appraisal.RelationshipDelta != 0 {
		relationType := relationshipTypeForRequest(plan.Request)
		now := time.Now().Format("2006-01-02 15:04:05")
		var existing RelationshipStateRecord
		err := tx.Where("character_id = ? AND relation_type = ?", plan.Character, relationType).Order("updated_at DESC").Take(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		data := map[string]float64{
			"trust": 0.5, "familiarity": 0, "security": 0.5,
			"tension": 0, "repairConfidence": 0.5, "boundary": 0.5,
		}
		if existing.RelationData != "" {
			_ = json.Unmarshal([]byte(existing.RelationData), &data)
		}
		data["familiarity"] = clampRelationshipValue(data["familiarity"] + appraisal.RelationshipDelta*0.01)
		data["trust"] = clampRelationshipValue(data["trust"] + appraisal.RelationshipDelta*0.01)
		if existing.ID == "" {
			existing = RelationshipStateRecord{
				ID: uuid.New().String(), CharacterID: plan.Character,
				RelationType: relationType, CreatedAt: now,
			}
		}
		raw, _ := json.Marshal(data)
		existing.RelationData = string(raw)
		existing.UpdatedAt = now
		if err := tx.Save(&existing).Error; err != nil {
			return err
		}
	}
	return nil
}

type NeedStateRecord struct {
	ID          string `gorm:"primaryKey;column:id"`
	CharacterID string `gorm:"column:character_id;index"`
	NeedKey     string `gorm:"column:need_key"`
	CurrentValue float64 `gorm:"column:current_value"`
	Baseline    float64 `gorm:"column:baseline"`
	Trend       float64 `gorm:"column:trend"`
	Saturated   bool    `gorm:"column:saturated"`
	CreatedAt   string  `gorm:"column:created_at"`
	UpdatedAt   string  `gorm:"column:updated_at"`
}

func (NeedStateRecord) TableName() string {
	return "need_states"
}

func (s *service) updateNeedStateTx(tx *gorm.DB, plan messageCommitPlan) error {
	if plan.Character == "" {
		return nil
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	needsDefaults := map[string]struct {
		Value    float64
		Baseline float64
	}{
		"reassurance": {0.5, 0.6},
		"connection":  {0.5, 0.5},
		"autonomy":    {0.5, 0.5},
		"clarity":     {0.5, 0.5},
		"rest":        {0.5, 0.5},
		"expression":  {0.5, 0.5},
		"novelty":     {0.5, 0.5},
	}
	hasAppraisal := plan.Request != nil && plan.Runtime != nil && plan.Runtime.Appraisal != nil
	var needDeltas map[string]float64
	if hasAppraisal {
		needDeltas = plan.Runtime.Appraisal.NeedDeltas
	}
	for key, def := range needsDefaults {
		var existing NeedStateRecord
		err := tx.Where("character_id = ? AND need_key = ?", plan.Character, key).Take(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			record := NeedStateRecord{
				ID:           uuid.New().String(),
				CharacterID:  plan.Character,
				NeedKey:      key,
				CurrentValue: def.Value,
				Baseline:     def.Baseline,
				Trend:        0,
				Saturated:    false,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			if createErr := tx.Create(&record).Error; createErr != nil {
				return createErr
			}
		} else if err != nil {
			return err
		} else {
			delta := 0.0
			if needDeltas != nil {
				if d, ok := needDeltas[key]; ok {
					delta = d
				}
			}
			drift := (def.Baseline - existing.CurrentValue) * 0.05
			existing.CurrentValue = clampNeedValue(existing.CurrentValue + delta + drift)
			existing.Trend = delta
			existing.UpdatedAt = now
			if saveErr := tx.Save(&existing).Error; saveErr != nil {
				return saveErr
			}
		}
	}
	return nil
}

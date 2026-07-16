package chat

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *service) updateRelationshipStateTx(tx *gorm.DB, plan messageCommitPlan) error {
	relationType := relationshipTypeForRequest(plan.Request)
	userID := userIDForRequest(plan.Request)
	channel := channelForRequest(plan.Request)
	now := time.Now().Format("2006-01-02 15:04:05")
	var existing RelationshipStateRecord
	err := tx.Where("character_id = ? AND user_id = ? AND channel = ? AND relation_type = ?", plan.Character, userID, channel, relationType).Order("updated_at DESC").Take(&existing).Error
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
			UserID:       userID,
			Channel:      channel,
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

func clampRelationshipValue(value float64) float64 {
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

func (s *service) applyAppraisalResultTx(tx *gorm.DB, plan messageCommitPlan) error {
	if plan.Runtime == nil || plan.Runtime.Appraisal == nil {
		return nil
	}
	appraisal := plan.Runtime.Appraisal
	if appraisal.RelationshipDelta != 0 {
		relationType := relationshipTypeForRequest(plan.Request)
		userID := userIDForRequest(plan.Request)
		channel := channelForRequest(plan.Request)
		now := time.Now().Format("2006-01-02 15:04:05")
		var existing RelationshipStateRecord
		err := tx.Where("character_id = ? AND user_id = ? AND channel = ? AND relation_type = ?", plan.Character, userID, channel, relationType).Order("updated_at DESC").Take(&existing).Error
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
				UserID: userID, Channel: channel,
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

type RelationshipStateRecord struct {
	ID           string `gorm:"primaryKey;column:id"`
	CharacterID  string `gorm:"column:character_id"`
	UserID       string `gorm:"column:user_id"`
	Channel      string `gorm:"column:channel"`
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

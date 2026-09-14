package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/emotionstate"
	"github.com/u-ai/backend/internal/extensioncontext"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/psyche"
	"github.com/u-ai/backend/internal/relationship"
	"gorm.io/gorm"
)

type RealtimeEmotionContext = emotionstate.Context

type RealtimeEmotionCommit struct {
	UserID        string
	CharacterID   string
	UserText      string
	DeliveredText string
	UserAffect    emotionstate.UserAffect
	Signals       emotionstate.Signals
}

func (s *service) LoadRealtimeEmotionContext(ctx context.Context, userID, characterID string) (*RealtimeEmotionContext, error) {
	if s == nil {
		return nil, fmt.Errorf("chat service is unavailable")
	}
	state, err := s.emotion.Load(ctx, userID, characterID)
	if err != nil {
		return nil, err
	}
	var psycheState *psyche.PsycheState
	if s.psycheStore != nil {
		if loaded, loadErr := s.psycheStore.LoadState(characterID); loadErr == nil {
			psycheState = loaded
		}
	}
	relationState, relationErr := s.loadRealtimeRelationshipState(ctx, userID, characterID)
	if relationErr != nil && !errors.Is(relationErr, gorm.ErrRecordNotFound) {
		return nil, relationErr
	}
	schedulePrompt, _ := s.realtimeScheduleContext(ctx, characterID, time.Now())
	return emotionstate.BuildContext(psycheState, relationState, state, schedulePrompt), nil
}

func (s *service) CommitRealtimeEmotion(ctx context.Context, commit RealtimeEmotionCommit) error {
	if s == nil || s.emotion == nil {
		return fmt.Errorf("emotion service is unavailable")
	}
	relationState, err := s.loadRealtimeRelationshipState(ctx, commit.UserID, commit.CharacterID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	familiarity := 0.0
	if relationState != nil {
		familiarity = relationState.Familiarity
	}
	appraisal := interaction.EvaluateMessageAppraisal(commit.UserText, familiarity)
	schedulePrompt, scheduleDelta := s.realtimeScheduleContext(ctx, commit.CharacterID, time.Now())
	_ = schedulePrompt
	if appraisal == nil {
		appraisal = &interaction.AppraisalResult{EventType: "chat"}
	}
	userAffect := commit.UserAffect
	if strings.TrimSpace(userAffect.PrimaryEmotion) == "" {
		userAffect = inferUserAffect(appraisal)
	}
	delta := relationshipEmotionDeltaFromAppraisal(appraisal)
	if _, err := s.emotion.Commit(ctx, commit.UserID, commit.CharacterID, userAffect, delta, commit.Signals); err != nil {
		return err
	}
	if s.psycheStore != nil {
		bridge := &AppraisalResultBridge{
			EventType:         appraisal.EventType,
			PsycheDelta:       appraisal.PsycheDelta,
			RelationshipDelta: appraisal.RelationshipDelta,
			Severity:          appraisal.Severity,
			EnergyDelta:       scheduleDelta.Energy,
			StressDelta:       scheduleDelta.Stress,
		}
		if err := s.updatePsycheStateWithStore(s.psycheStore, commit.CharacterID, bridge); err != nil {
			return err
		}
	}
	return s.commitRealtimeRelationshipState(ctx, commit.UserID, commit.CharacterID, appraisal)
}

func (s *service) loadRealtimeRelationshipState(ctx context.Context, userID, characterID string) (*relationship.RelationshipState, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var record RelationshipStateRecord
	err := s.db.WithContext(ctx).
		Where("character_id = ? AND user_id = ? AND channel = ? AND relation_type = ?", characterID, userID, "*", "user_character").
		Order("updated_at DESC").
		Take(&record).Error
	if err != nil {
		return nil, err
	}
	data := map[string]float64{}
	if record.RelationData != "" {
		_ = json.Unmarshal([]byte(record.RelationData), &data)
	}
	return &relationship.RelationshipState{
		Trust:            getOrDefaultFloat(data, "trust", 0.5),
		Familiarity:      getOrDefaultFloat(data, "familiarity", 0),
		Security:         getOrDefaultFloat(data, "security", 0.5),
		Tension:          getOrDefaultFloat(data, "tension", 0),
		RepairConfidence: getOrDefaultFloat(data, "repairConfidence", 0.5),
		Boundary:         getOrDefaultFloat(data, "boundary", 0.5),
	}, nil
}

func (s *service) commitRealtimeRelationshipState(ctx context.Context, userID, characterID string, appraisal *interaction.AppraisalResult) error {
	if s == nil || s.db == nil || appraisal == nil {
		return nil
	}
	data := map[string]float64{
		"trust":            0.5,
		"familiarity":      0,
		"security":         0.5,
		"tension":          0,
		"repairConfidence": 0.5,
		"boundary":         0.5,
	}
	var existing RelationshipStateRecord
	err := s.db.WithContext(ctx).
		Where("character_id = ? AND user_id = ? AND channel = ? AND relation_type = ?", characterID, userID, "*", "user_character").
		Order("updated_at DESC").
		Take(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if existing.RelationData != "" {
		_ = json.Unmarshal([]byte(existing.RelationData), &data)
	}
	familiarity := getOrDefaultFloat(data, "familiarity", 0)
	trust := getOrDefaultFloat(data, "trust", 0.5)
	security := getOrDefaultFloat(data, "security", 0.5)
	data["familiarity"] = clampRelationshipValue(familiarity + computeRelationshipFamiliarityDelta(data))
	data["trust"] = clampRelationshipValue(trust + computeRelationshipTrustDelta(data))
	data["security"] = clampRelationshipValue(security + computeRelationshipSecurityDelta(data))
	tension := getOrDefaultFloat(data, "tension", 0)
	data["tension"] = clampRelationshipValue(tension + appraisal.RelationshipDelta*0.04)
	if appraisal.EventType == "apology" || appraisal.EventType == "help" {
		repair := getOrDefaultFloat(data, "repairConfidence", 0.5)
		data["repairConfidence"] = clampRelationshipValue(repair + 0.02)
	}
	if appraisal.EventType == "complaint" || appraisal.EventType == "boundary_cross" {
		repair := getOrDefaultFloat(data, "repairConfidence", 0.5)
		data["repairConfidence"] = clampRelationshipValue(repair - 0.02)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	if existing.ID == "" {
		existing = RelationshipStateRecord{
			ID:           uuid.New().String(),
			CharacterID:  characterID,
			UserID:       userID,
			Channel:      "*",
			RelationType: "user_character",
			CreatedAt:    now,
		}
	}
	existing.RelationData = string(raw)
	existing.UpdatedAt = now
	if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
		return err
	}
	eventData, err := json.Marshal(map[string]any{
		"eventType":  appraisal.EventType,
		"severity":   appraisal.Severity,
		"source":     "realtime_voice",
		"userAffect": appraisalNeedSnapshot(appraisal),
	})
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Create(&relationshipEventRecord{
		ID:          uuid.New().String(),
		CharacterID: characterID,
		EventType:   appraisal.EventType,
		EventData:   string(eventData),
		CreatedAt:   now,
	}).Error
}

func appraisalNeedSnapshot(appraisal *interaction.AppraisalResult) map[string]float64 {
	if appraisal == nil {
		return nil
	}
	return appraisal.NeedDeltas
}

func (s *service) trackUserAffectFromMessage(userID, characterID, message string) {
	if s == nil || s.emotion == nil || strings.TrimSpace(message) == "" {
		return
	}
	appraisal := interaction.EvaluateMessageAppraisal(message, 0)
	if appraisal == nil {
		return
	}
	affect := inferUserAffect(appraisal)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = s.emotion.Commit(ctx, userID, characterID, affect, relationshipEmotionDeltaFromAppraisal(appraisal), emotionstate.Signals{})
	}()
}

func relationshipEmotionDeltaFromAppraisal(appraisal *interaction.AppraisalResult) emotionstate.RelationshipEmotionDelta {
	if appraisal == nil {
		return emotionstate.RelationshipEmotionDelta{}
	}
	return emotionstate.RelationshipEmotionDelta{
		Concern:        appraisal.RelationshipConcernDelta,
		Warmth:         appraisal.RelationshipWarmthDelta,
		Hurt:           appraisal.RelationshipHurtDelta,
		Anger:          appraisal.RelationshipAngerDelta,
		Disappointment: appraisal.RelationshipDisappointmentDelta,
	}
}

func inferUserAffect(appraisal *interaction.AppraisalResult) emotionstate.UserAffect {
	if appraisal == nil {
		return emotionstate.EmptyUserAffect()
	}
	affect := emotionstate.EmptyUserAffect()
	affect.Intensity = clampRelationshipValue(appraisal.Severity)
	affect.Severity = clampRelationshipValue(appraisal.Severity)
	affect.Confidence = 0.55
	switch appraisal.EventType {
	case "praise":
		affect.PrimaryEmotion = "joy"
	case "apology":
		affect.PrimaryEmotion = "guilt"
		affect.Need = "acceptance"
	case "complaint":
		affect.PrimaryEmotion = "anger"
		affect.Need = "validation"
	case "boundary_cross":
		affect.PrimaryEmotion = "discomfort"
		affect.Need = "space"
	case "cold":
		affect.PrimaryEmotion = "sadness"
		affect.Need = "space"
	case "help":
		affect.PrimaryEmotion = "anxiety"
		affect.Need = "support"
	case "emotional":
		affect.PrimaryEmotion = "sadness"
		affect.Need = "comfort"
	default:
		affect.PrimaryEmotion = "neutral"
		affect.Need = "none"
	}
	return emotionstate.NormalizeUserAffect(affect)
}

type realtimeScheduleDelta struct {
	Energy float64
	Stress float64
}

func (s *service) realtimeScheduleContext(ctx context.Context, characterID string, now time.Time) (string, realtimeScheduleDelta) {
	if s == nil || characterID == "" {
		return "", realtimeScheduleDelta{}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if s.extensionContext == nil {
		return "", realtimeScheduleDelta{}
	}
	raw, err := s.extensionContext.Resolve(ctx, "chat.realtime.schedule", extensioncontext.Request{
		CharacterID: characterID,
		At:          now,
	})
	if err != nil {
		return "", realtimeScheduleDelta{}
	}
	snapshot, err := extensioncontext.Decode(raw)
	if err != nil {
		return "", realtimeScheduleDelta{}
	}
	contexts := make([]string, 0, len(snapshot.Contributions))
	delta := realtimeScheduleDelta{}
	for _, contribution := range snapshot.Contributions {
		if contribution.Error != "" {
			continue
		}
		var payload struct {
			Realtime struct {
				Context     string  `json:"context"`
				EnergyDelta float64 `json:"energyDelta"`
				StressDelta float64 `json:"stressDelta"`
			} `json:"realtime"`
		}
		if err := json.Unmarshal(contribution.Data, &payload); err != nil {
			continue
		}
		if strings.TrimSpace(payload.Realtime.Context) != "" {
			contexts = append(contexts, strings.TrimSpace(payload.Realtime.Context))
		}
		delta.Energy += payload.Realtime.EnergyDelta
		delta.Stress += payload.Realtime.StressDelta
	}
	return strings.Join(contexts, "\n"), delta
}

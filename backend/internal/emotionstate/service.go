package emotionstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Load(ctx context.Context, spaceID, characterID string) (*State, error) {
	if s == nil || s.db == nil {
		return defaultState(spaceID, characterID, time.Now().UTC()), nil
	}
	var record Record
	err := s.db.WithContext(ctx).
		Where("space_id = ? AND character_id = ?", spaceID, characterID).
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return defaultState(spaceID, characterID, time.Now().UTC()), nil
	}
	if err != nil {
		return nil, err
	}
	state := &State{
		SpaceID:     record.SpaceID,
		CharacterID: record.CharacterID,
		Version:     record.Version,
		UpdatedAt:   record.UpdatedAt,
	}
	if record.UserAffectJSON != "" {
		_ = json.Unmarshal([]byte(record.UserAffectJSON), &state.UserAffect)
	}
	if record.RelationshipEmotionJSON != "" {
		_ = json.Unmarshal([]byte(record.RelationshipEmotionJSON), &state.RelationshipEmotion)
	}
	if record.SignalsJSON != "" {
		_ = json.Unmarshal([]byte(record.SignalsJSON), &state.Signals)
	}
	if record.BaselineJSON != "" {
		_ = json.Unmarshal([]byte(record.BaselineJSON), &state.Baseline)
	}
	state.UserAffect = NormalizeUserAffect(state.UserAffect)
	state.RelationshipEmotion = NormalizeRelationshipEmotion(state.RelationshipEmotion)
	state.RelationshipEmotion = DecayRelationshipEmotion(state.RelationshipEmotion, time.Now().UTC())
	if !UserAffectFresh(state.UserAffect, time.Now().UTC()) {
		state.UserAffect = EmptyUserAffect()
	}
	return state, nil
}

func (s *Service) Commit(ctx context.Context, spaceID, characterID string, affect UserAffect, delta RelationshipEmotionDelta, signals Signals) (*State, error) {
	if s == nil || s.db == nil {
		state := defaultState(spaceID, characterID, time.Now().UTC())
		state.UserAffect = NormalizeUserAffect(affect)
		state.RelationshipEmotion = ApplyRelationshipDelta(state.RelationshipEmotion, delta, time.Now().UTC())
		state.Signals = signals
		return state, nil
	}
	for attempt := 0; attempt < 4; attempt++ {
		state, err := s.Load(ctx, spaceID, characterID)
		if err != nil {
			return nil, err
		}
		now := time.Now().UTC()
		state.UserAffect = NormalizeUserAffect(affect)
		state.UserAffect.UpdatedAt = now
		state.RelationshipEmotion = ApplyRelationshipDelta(state.RelationshipEmotion, delta, now)
		state.Signals = signals
		state.Signals.VolumeDeviation = deviation(signals.VolumeRMSMean, state.Baseline.VolumeRMSMean)
		state.Signals.SpeechRateDeviation = deviation(signals.SpeechRateCharsPerSec, state.Baseline.SpeechRateCharsPerSec)
		state.Baseline = ApplyBaseline(state.Baseline, signals)
		state.UpdatedAt = now
		state.Version++
		record, err := stateRecord(state)
		if err != nil {
			return nil, err
		}
		var result *gorm.DB
		if state.Version <= 1 {
			result = s.db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "space_id"}, {Name: "character_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"user_affect_json", "relationship_emotion_json", "signals_json", "baseline_json", "version", "updated_at"}),
			}).Create(record)
		} else {
			result = s.db.WithContext(ctx).
				Model(&Record{}).
				Where("space_id = ? AND character_id = ? AND version = ?", spaceID, characterID, state.Version-1).
				Updates(map[string]any{
					"user_affect_json":          record.UserAffectJSON,
					"relationship_emotion_json": record.RelationshipEmotionJSON,
					"signals_json":              record.SignalsJSON,
					"baseline_json":             record.BaselineJSON,
					"version":                   state.Version,
					"updated_at":                state.UpdatedAt,
				})
		}
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 1 || state.Version <= 1 {
			return state, nil
		}
	}
	return nil, fmt.Errorf("emotion state update conflicted too many times")
}

func defaultState(spaceID, characterID string, now time.Time) *State {
	return &State{
		SpaceID:     spaceID,
		CharacterID: characterID,
		UserAffect:  EmptyUserAffect(),
		RelationshipEmotion: RelationshipEmotion{
			Warmth:    0.4,
			UpdatedAt: now,
		},
		UpdatedAt: now,
	}
}

func stateRecord(state *State) (*Record, error) {
	userAffect, err := json.Marshal(state.UserAffect)
	if err != nil {
		return nil, err
	}
	relationshipEmotion, err := json.Marshal(state.RelationshipEmotion)
	if err != nil {
		return nil, err
	}
	signals, err := json.Marshal(state.Signals)
	if err != nil {
		return nil, err
	}
	baseline, err := json.Marshal(state.Baseline)
	if err != nil {
		return nil, err
	}
	return &Record{
		SpaceID:                 state.SpaceID,
		CharacterID:             state.CharacterID,
		UserAffectJSON:          string(userAffect),
		RelationshipEmotionJSON: string(relationshipEmotion),
		SignalsJSON:             string(signals),
		BaselineJSON:            string(baseline),
		Version:                 state.Version,
		UpdatedAt:               state.UpdatedAt,
	}, nil
}

func deviation(value, baseline float64) float64 {
	if baseline <= 0 {
		return 0
	}
	return (value - baseline) / baseline
}

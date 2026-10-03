package main

import (
	"context"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/emotionstate"
	"github.com/u-ai/backend/internal/realtime"
)

type cascadeEmotionAdapter struct {
	chat chat.Service
}

func (a cascadeEmotionAdapter) Load(ctx context.Context, spaceID, characterID string) (*realtime.CascadeEmotionContext, error) {
	if a.chat == nil {
		return nil, nil
	}
	contextData, err := a.chat.LoadRealtimeEmotionContext(ctx, spaceID, characterID)
	if err != nil {
		return nil, err
	}
	if contextData == nil {
		return nil, nil
	}
	return &realtime.CascadeEmotionContext{
		Prompt:           contextData.Prompt,
		VoiceInstruction: contextData.VoiceInstruction,
	}, nil
}

func (a cascadeEmotionAdapter) Commit(ctx context.Context, spaceID, characterID string, commit realtime.CascadeEmotionCommit) error {
	if a.chat == nil {
		return nil
	}
	if _, err := uuid.Parse(spaceID); err != nil {
		return err
	}
	if _, err := uuid.Parse(characterID); err != nil {
		return err
	}
	return a.chat.CommitRealtimeEmotion(ctx, chat.RealtimeEmotionCommit{
		SpaceID:       spaceID,
		CharacterID:   characterID,
		UserText:      commit.UserText,
		DeliveredText: commit.DeliveredText,
		UserAffect: emotionstate.UserAffect{
			PrimaryEmotion:      commit.UserAffect.PrimaryEmotion,
			SecondaryEmotion:    commit.UserAffect.SecondaryEmotion,
			Intensity:           commit.UserAffect.Intensity,
			Stress:              commit.UserAffect.Stress,
			Need:                commit.UserAffect.Need,
			AdviceWanted:        commit.UserAffect.AdviceWanted,
			Openness:            commit.UserAffect.Openness,
			Severity:            commit.UserAffect.Severity,
			PossibleConcealment: commit.UserAffect.PossibleConcealment,
			Confidence:          commit.UserAffect.Confidence,
			Evidence:            append([]string(nil), commit.UserAffect.Evidence...),
		},
		Signals: emotionstate.Signals{
			DeviceClass:           commit.Signals.DeviceClass,
			VolumeRMSMean:         commit.Signals.VolumeRMSMean,
			SpeechRateCharsPerSec: commit.Signals.SpeechRateCharsPerSec,
			UtteranceDurationMS:   commit.Signals.UtteranceDurationMS,
			ResponseLatencyMS:     commit.Signals.ResponseLatencyMS,
			InterruptionCount:     commit.Signals.InterruptionCount,
			Evidence:              append([]string(nil), commit.Signals.Evidence...),
		},
	})
}

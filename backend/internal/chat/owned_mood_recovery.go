package chat

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
	"gorm.io/gorm"
)

func (s *service) RecoverOwnedMood(ctx context.Context, scope coordination.ExecutionScope, lastUserAt time.Time, currentUserAt time.Time, source string, currentPsyche interaction.PsycheState) error {
	if scope.RoleID == "" || scope.ResourceOwnerID != scope.RoleOwnerID {
		return coordination.ErrWrongOwner
	}
	if source == moodRecoveryProactive || source == moodRecoverySystem || lastUserAt.IsZero() || currentUserAt.Sub(lastUserAt) <= moodRecoveryIdleThreshold {
		return nil
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var latest moodRecoveryRecord
		if err := tx.Where("character_id = ?", scope.RoleID).Order("created_at DESC").First(&latest).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if !latestTime(latest.CreatedAt).Before(currentUserAt.Truncate(time.Second)) {
			return nil
		}
		state, ok := recoveredMoodStateFromValue(currentPsyche.MoodValence, currentUserAt.Sub(lastUserAt))
		if !ok {
			return nil
		}
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return err
		}
		return tx.Create(&moodRecoveryRecord{CharacterID: scope.RoleID, Mood: state.Mood, Level: state.Level, CreatedAt: time.Now().Format(moodRecoveryTimeLayout)}).Error
	})
}

func latestTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		parsed, _ = time.ParseInLocation(moodRecoveryTimeLayout, value, time.Local)
	}
	return parsed
}

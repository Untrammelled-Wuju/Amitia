package chat

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NeedStateRecord struct {
	ID           string  `gorm:"primaryKey;column:id"`
	CharacterID  string  `gorm:"column:character_id;index"`
	NeedKey      string  `gorm:"column:need_key"`
	CurrentValue float64 `gorm:"column:current_value"`
	Baseline     float64 `gorm:"column:baseline"`
	Trend        float64 `gorm:"column:trend"`
	Saturated    bool    `gorm:"column:saturated"`
	CreatedAt    string  `gorm:"column:created_at"`
	UpdatedAt    string  `gorm:"column:updated_at"`
}

func (NeedStateRecord) TableName() string {
	return "need_states"
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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	voicewake "github.com/u-ai/backend/internal/devicemesh/wake"
	"gorm.io/gorm"
)

type meshWakeConfigRecord struct {
	ID               string  `gorm:"column:id"`
	Enabled          bool    `gorm:"column:enabled"`
	Backend          string  `gorm:"column:backend"`
	ModelResourceURI string  `gorm:"column:model_resource_uri"`
	Phrases          string  `gorm:"column:phrases"`
	Threshold        float64 `gorm:"column:threshold"`
	CooldownMS       int64   `gorm:"column:cooldown_ms"`
}

func (meshWakeConfigRecord) TableName() string { return "wake_configs" }

func decodeMeshWakePhrases(raw string) ([]voicewake.WakePhrase, error) {
	var objects []struct {
		ID          string `json:"id"`
		DisplayText string `json:"displayText"`
		Locale      string `json:"locale"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &objects) == nil && len(objects) > 0 {
		result := make([]voicewake.WakePhrase, 0, len(objects))
		for index, item := range objects {
			text := strings.TrimSpace(item.DisplayText)
			if text == "" {
				continue
			}
			id := strings.TrimSpace(item.ID)
			if id == "" {
				id = fmt.Sprintf("wake-%d", index+1)
			}
			result = append(result, voicewake.WakePhrase{ID: id, DisplayText: text, Locale: strings.TrimSpace(item.Locale)})
		}
		if len(result) > 0 {
			return result, nil
		}
	}
	var values []string
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &values) != nil {
		return nil, errors.New("唤醒词配置无效")
	}
	result := make([]voicewake.WakePhrase, 0, len(values))
	for index, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, voicewake.WakePhrase{ID: fmt.Sprintf("wake-%d", index+1), DisplayText: value})
		}
	}
	if len(result) == 0 {
		return nil, errors.New("唤醒词配置为空")
	}
	return result, nil
}

func loadMeshWakeDetector(ctx context.Context, db *gorm.DB, configID string) (voicewake.WakeDetector, string, error) {
	if db == nil {
		return nil, "", errors.New("唤醒配置存储不可用")
	}
	var record meshWakeConfigRecord
	query := db.WithContext(ctx).Table("wake_configs").Where("enabled = 1")
	if strings.TrimSpace(configID) != "" {
		query = query.Where("id = ?", strings.TrimSpace(configID))
	} else {
		query = query.Where("id NOT LIKE ?", "workflow-wake-%").Order("updated_at DESC")
	}
	if err := query.First(&record).Error; err != nil {
		return nil, "", fmt.Errorf("没有可用的唤醒配置: %w", err)
	}
	backend := strings.TrimSpace(record.Backend)
	if backend == "" {
		backend = "software"
	}
	factory, ok := voicewake.GetWakeBackend(backend)
	if !ok {
		return nil, "", fmt.Errorf("唤醒后端不可用: %s", backend)
	}
	detector, err := factory("{}")
	if err != nil {
		return nil, "", err
	}
	phrases, err := decodeMeshWakePhrases(record.Phrases)
	if err != nil {
		return nil, "", err
	}
	if err := detector.Load(ctx, voicewake.WakeDetectorConfig{Enabled: record.Enabled, Backend: backend, ModelResourceURI: record.ModelResourceURI, Phrases: phrases, Threshold: record.Threshold, CooldownMS: record.CooldownMS}); err != nil {
		return nil, "", err
	}
	return detector, record.ID, nil
}

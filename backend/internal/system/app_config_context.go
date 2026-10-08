package system

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/u-ai/backend/internal/configwrite"
	"gorm.io/gorm"
)

func (s *service) applyAppSettings(ctx context.Context, settings map[string]string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := configwrite.Transaction(s.db.WithContext(ctx), func(tx *gorm.DB) error {
		keys := make([]string, 0, len(settings))
		for key := range settings {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		store := NewSettingsStore(tx)
		for _, key := range keys {
			if _, err := store.Upsert(key, settings[key]); err != nil {
				return err
			}
		}
		var rows []struct {
			Key   string
			Value string
		}
		if err := tx.Model(&AppSetting{}).Select("key,value").Find(&rows).Error; err != nil {
			return err
		}
		result = make(map[string]interface{}, len(rows))
		for _, row := range rows {
			result[row.Key] = row.Value
		}
		return nil
	})
	return result, err
}

func (s *service) UpdateAppConfigContext(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error) {
	settings := map[string]string{}
	for _, key := range []string{"theme", "language", "timezone"} {
		if value, ok := body[key].(string); ok {
			settings[key] = value
		}
	}
	if nested, ok := body["settings"].(map[string]interface{}); ok {
		for key, value := range nested {
			if text, ok := value.(string); ok {
				settings[key] = text
			}
		}
	}
	current, err := s.applyAppSettings(ctx, settings)
	if err != nil {
		return nil, err
	}
	language, _ := current["language"].(string)
	if language == "" {
		language = "zh-CN"
	}
	timezone, _ := current["timezone"].(string)
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	theme, _ := current["theme"].(string)
	return map[string]interface{}{"theme": theme, "language": language, "timezone": timezone, "settings": current}, nil
}

func (s *service) ConfigImportConfirmContext(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error) {
	settings, err := parseConfigImportPayload(body)
	if err != nil {
		return nil, err
	}
	current, err := s.applyAppSettings(ctx, settings)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"imported": true, "importedCount": len(settings), "settings": current}, nil
}

func (s *service) UpdateMoodDetectionConfigContext(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error) {
	settings := map[string]string{}
	if enabled, ok := body["enabled"].(bool); ok {
		settings["mood_detection_enabled"] = strconv.FormatBool(enabled)
	}
	if threshold, ok := body["threshold"].(float64); ok {
		if threshold < 0 {
			threshold = 0
		}
		if threshold > 1 {
			threshold = 1
		}
		settings["mood_detection_threshold"] = strconv.FormatFloat(threshold, 'f', -1, 64)
	}
	current, err := s.applyAppSettings(ctx, settings)
	if err != nil {
		return nil, err
	}
	threshold := 0.5
	if parsed, err := strconv.ParseFloat(fmt.Sprint(current["mood_detection_threshold"]), 64); err == nil && parsed >= 0 && parsed <= 1 {
		threshold = parsed
	}
	return map[string]interface{}{"enabled": current["mood_detection_enabled"] == "true", "threshold": threshold}, nil
}

func (s *service) UpdateThemeContext(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error) {
	settings := map[string]string{}
	for _, key := range []string{"preset", "theme"} {
		if value, ok := body[key].(string); ok {
			settings["theme"] = value
		}
	}
	if value, ok := body["accentColor"].(string); ok {
		settings["theme_accent_color"] = value
	}
	if value, ok := body["mode"].(string); ok {
		settings["theme_mode"] = value
	}
	current, err := s.applyAppSettings(ctx, settings)
	if err != nil {
		return nil, err
	}
	theme, _ := current["theme"].(string)
	if theme == "" {
		theme = "dark"
	}
	mode, _ := current["theme_mode"].(string)
	if mode == "" {
		mode = "dark"
	}
	accent, _ := current["theme_accent_color"].(string)
	return map[string]interface{}{"preset": theme, "theme": theme, "mode": mode, "accentColor": accent}, nil
}

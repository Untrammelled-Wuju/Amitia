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
		raw, ok := body[key]
		if !ok {
			continue
		}
		value, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("配置项 %s 的值类型无效，应为字符串，实际为 %T", key, raw)
		}
		settings[key] = value
	}
	if raw, ok := body["settings"]; ok {
		nested, ok := raw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("配置块 settings 的值类型无效，应为对象，实际为 %T", raw)
		}
		for key, value := range nested {
			text, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("配置项 %s 的值类型无效，应为字符串，实际为 %T", key, value)
			}
			settings[key] = text
		}
	}
	normalized, err := validateAppSettings(settings)
	if err != nil {
		return nil, err
	}
	current, err := s.applyAppSettings(ctx, normalized)
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

func (s *service) UpdateMoodDetectionConfigContext(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error) {
	settings := map[string]string{}
	if raw, ok := body["enabled"]; ok {
		enabled, ok := raw.(bool)
		if !ok {
			return nil, fmt.Errorf("配置项 enabled 的值类型无效，应为布尔值，实际为 %T", raw)
		}
		settings["mood_detection_enabled"] = strconv.FormatBool(enabled)
	}
	if raw, ok := body["threshold"]; ok {
		threshold, ok := raw.(float64)
		if !ok {
			return nil, fmt.Errorf("配置项 threshold 的值类型无效，应为数字，实际为 %T", raw)
		}
		settings["mood_detection_threshold"] = strconv.FormatFloat(threshold, 'f', -1, 64)
	}
	normalized, err := validateAppSettings(settings)
	if err != nil {
		return nil, err
	}
	current, err := s.applyAppSettings(ctx, normalized)
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
		raw, ok := body[key]
		if !ok {
			continue
		}
		value, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("配置项 %s 的值类型无效，应为字符串，实际为 %T", key, raw)
		}
		settings["theme"] = value
	}
	if raw, ok := body["accentColor"]; ok {
		value, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("配置项 accentColor 的值类型无效，应为字符串，实际为 %T", raw)
		}
		settings["theme_accent_color"] = value
	}
	if raw, ok := body["mode"]; ok {
		value, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("配置项 mode 的值类型无效，应为字符串，实际为 %T", raw)
		}
		settings["theme_mode"] = value
	}
	normalized, err := validateAppSettings(settings)
	if err != nil {
		return nil, err
	}
	current, err := s.applyAppSettings(ctx, normalized)
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

// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"
)

func (s *service) AppConfig() map[string]interface{} {
	theme := s.getAppSetting("theme")
	lang := s.getAppSetting("language")
	if lang == "" {
		lang = "zh-CN"
	}
	tz := s.getAppSetting("timezone")
	if tz == "" {
		tz = "Asia/Shanghai"
	}
	settings := s.ConfigSettings()
	return map[string]interface{}{"theme": theme, "language": lang, "timezone": tz, "settings": settings}
}

func (s *service) UpdateAppConfig(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["theme"].(string); ok {
		s.setAppSetting("theme", v)
	}
	if v, ok := body["language"].(string); ok {
		s.setAppSetting("language", v)
	}
	if v, ok := body["timezone"].(string); ok {
		s.setAppSetting("timezone", v)
	}
	if settings, ok := body["settings"].(map[string]interface{}); ok {
		for k, v := range settings {
			if sv, ok := v.(string); ok {
				s.setAppSetting(k, sv)
			}
		}
	}
	return s.AppConfig()
}

func (s *service) ConfigSettings() map[string]interface{} {
	var rows []struct {
		Key   string
		Value string
	}
	s.db.Table("app_settings").Find(&rows)
	result := map[string]interface{}{}
	for _, r := range rows {
		result[r.Key] = r.Value
	}
	return result
}

func (s *service) ConfigExport() map[string]interface{} {
	settings := s.ConfigSettings()
	return map[string]interface{}{
		"exported":   true,
		"format":     "amitia-config-v1",
		"exportedAt": time.Now().UTC().Format(time.RFC3339),
		"settings":   settings,
		"data":       settings,
	}
}

func (s *service) ConfigImportPreviewService(body map[string]interface{}) map[string]interface{} {
	settings, err := parseConfigImportPayload(body)
	if err != nil {
		return map[string]interface{}{"valid": false, "error": err.Error(), "itemCount": 0}
	}

	current := s.ConfigSettings()
	changed := 0
	unchanged := 0
	created := 0
	items := make([]map[string]interface{}, 0, len(settings))
	for key, value := range settings {
		old, exists := current[key]
		status := "unchanged"
		if !exists {
			status = "new"
			created++
		} else if fmt.Sprint(old) != value {
			status = "changed"
			changed++
		} else {
			unchanged++
		}
		items = append(items, map[string]interface{}{
			"key":      key,
			"value":    value,
			"oldValue": old,
			"status":   status,
		})
	}

	return map[string]interface{}{
		"valid":     true,
		"format":    "amitia-config-v1",
		"itemCount": len(settings),
		"newCount":  created,
		"changed":   changed,
		"unchanged": unchanged,
		"items":     items,
		"settings":  settings,
	}
}

func (s *service) ConfigImportConfirmService(body map[string]interface{}) map[string]interface{} {
	settings, err := parseConfigImportPayload(body)
	if err != nil {
		return map[string]interface{}{"imported": false, "error": err.Error(), "importedCount": 0}
	}
	if len(settings) == 0 {
		return map[string]interface{}{"imported": false, "error": "configuration contains no settings", "importedCount": 0}
	}

	for key, value := range settings {
		s.setAppSetting(key, value)
	}
	return map[string]interface{}{
		"imported":      true,
		"importedCount": len(settings),
		"settings":      s.ConfigSettings(),
	}
}

func parseConfigImportPayload(body map[string]interface{}) (map[string]string, error) {
	if body == nil {
		return nil, fmt.Errorf("missing configuration payload")
	}

	var decoded interface{} = body
	if raw, ok := body["raw"].(string); ok {
		if raw == "" {
			return nil, fmt.Errorf("configuration file is empty")
		}
		var value interface{}
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return nil, fmt.Errorf("invalid configuration JSON: %w", err)
		}
		decoded = value
	}

	root, ok := decoded.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("configuration root must be an object")
	}
	if nested, ok := root["data"].(map[string]interface{}); ok {
		// Accept a full API response saved by older clients.
		if _, hasSettings := root["settings"]; !hasSettings {
			root = nested
		}
	}

	candidate := interface{}(root)
	if value, ok := root["settings"]; ok {
		candidate = value
	} else if value, ok := root["data"]; ok {
		candidate = value
	}

	result := map[string]string{}
	switch value := candidate.(type) {
	case map[string]interface{}:
		for key, raw := range value {
			if key == "" || raw == nil {
				continue
			}
			switch typed := raw.(type) {
			case string:
				result[key] = typed
			case bool, float64:
				result[key] = fmt.Sprint(typed)
			default:
				encoded, err := json.Marshal(typed)
				if err != nil {
					return nil, fmt.Errorf("encode setting %s: %w", key, err)
				}
				result[key] = string(encoded)
			}
		}
	case []interface{}:
		// Backward compatibility with the old ConfigExport database-row format.
		for _, entry := range value {
			row, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			key := fmt.Sprint(row["key"])
			if key == "" || key == "<nil>" {
				continue
			}
			result[key] = fmt.Sprint(row["value"])
		}
	default:
		return nil, fmt.Errorf("configuration settings must be an object")
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("configuration contains no settings")
	}
	return result, nil
}

func (s *service) GetVersion() map[string]interface{} {
	return map[string]interface{}{
		"version":   readEnvOrDefault("AMITIA_VERSION", "1.0.0"),
		"buildTime": readEnvOrDefault("AMITIA_BUILD_TIME", "2026-05-27"),
		"goVersion": runtime.Version(),
	}
}

func (s *service) GetAbout() map[string]interface{} {
	return map[string]interface{}{
		"name":                   "Amitia",
		"displayName":            "阿米提亚",
		"version":                readEnvOrDefault("AMITIA_VERSION", "1.0.0"),
		"gitCommit":              os.Getenv("AMITIA_GIT_COMMIT"),
		"license":                "AGPL-3.0-only",
		"copyright":              "Copyright (C) 2026 彭旭",
		"sourceCodeUrl":          readEnvOrDefault("AMITIA_SOURCE_CODE_URL", "https://gitee.com/Untrammelled/Amitia"),
		"commercialLicensingUrl": readEnvOrDefault("AMITIA_COMMERCIAL_LICENSE_URL", "mailto:3151508592@qq.com"),
		"thirdPartyNoticesUrl":   readEnvOrDefault("AMITIA_THIRD_PARTY_NOTICES_URL", "https://gitee.com/Untrammelled/Amitia/blob/master/THIRD_PARTY_NOTICES.md"),
	}
}

func (s *service) MoodDetectionConfig() map[string]interface{} {
	enabled := s.getAppSetting("mood_detection_enabled") == "true"
	threshold := 0.5
	if raw := strings.TrimSpace(s.getAppSetting("mood_detection_threshold")); raw != "" {
		if value, err := strconv.ParseFloat(raw, 64); err == nil && value >= 0 && value <= 1 {
			threshold = value
		}
	}
	return map[string]interface{}{"enabled": enabled, "threshold": threshold}
}

func (s *service) UpdateMoodDetectionConfig(body map[string]interface{}) map[string]interface{} {
	if enabled, ok := body["enabled"].(bool); ok {
		s.setAppSetting("mood_detection_enabled", strconv.FormatBool(enabled))
	}
	if value, ok := body["threshold"].(float64); ok {
		if value < 0 {
			value = 0
		}
		if value > 1 {
			value = 1
		}
		s.setAppSetting("mood_detection_threshold", strconv.FormatFloat(value, 'f', -1, 64))
	}
	return s.MoodDetectionConfig()
}

func (s *service) GetTheme() map[string]interface{} {
	theme := s.getAppSetting("theme")
	if theme == "" {
		theme = "dark"
	}
	mode := s.getAppSetting("theme_mode")
	if mode == "" {
		mode = "dark"
	}
	return map[string]interface{}{"preset": theme, "theme": theme, "mode": mode, "accentColor": s.getAppSetting("theme_accent_color")}
}

func (s *service) UpdateTheme(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["preset"].(string); ok {
		s.setAppSetting("theme", v)
	}
	if v, ok := body["theme"].(string); ok {
		s.setAppSetting("theme", v)
	}
	if v, ok := body["accentColor"].(string); ok {
		s.setAppSetting("theme_accent_color", v)
	}
	if v, ok := body["mode"].(string); ok {
		s.setAppSetting("theme_mode", v)
	}
	return s.GetTheme()
}

func (s *service) GetThemePresets() map[string]interface{} {
	return map[string]interface{}{"presets": []interface{}{
		map[string]interface{}{"id": "system", "name": "跟随系统", "description": "自动跟随操作系统主题设置"},
		map[string]interface{}{"id": "dark", "name": "深色", "description": "护眼深色模式"},
		map[string]interface{}{"id": "light", "name": "亮色", "description": "明亮浅色模式"},
		map[string]interface{}{"id": "calm-blue", "name": "静谧蓝", "description": "克制的蓝色中性风格"},
		map[string]interface{}{"id": "warm-gray", "name": "暖灰", "description": "温暖中性灰色调"},
		map[string]interface{}{"id": "mint", "name": "薄荷绿", "description": "清新薄荷浅色风格"},
		map[string]interface{}{"id": "navy", "name": "深邃蓝", "description": "深海暗色护眼风格"},
	}}
}

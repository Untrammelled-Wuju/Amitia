// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/u-ai/backend/config"
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
	result, err := s.UpdateAppConfigContext(context.Background(), body)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return result
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

func (s *service) GetVersion() map[string]interface{} {
	return map[string]interface{}{
		"version":   config.AppVersion(),
		"buildTime": readEnvOrDefault("AMITIA_BUILD_TIME", "2026-05-27"),
		"goVersion": runtime.Version(),
	}
}

func (s *service) GetAbout() map[string]interface{} {
	return map[string]interface{}{
		"name":                   "Amitia",
		"displayName":            "阿米提亚",
		"version":                config.AppVersion(),
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
	result, err := s.UpdateMoodDetectionConfigContext(context.Background(), body)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return result
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
	result, err := s.UpdateThemeContext(context.Background(), body)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return result
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

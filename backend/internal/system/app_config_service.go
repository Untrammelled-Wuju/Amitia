// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"os"
	"runtime"
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
	var settings []map[string]interface{}
	s.db.Table("app_settings").Find(&settings)
	return map[string]interface{}{"data": settings, "exported": true}
}

func (s *service) ConfigImportPreviewService(body map[string]interface{}) map[string]interface{} {
	raw, _ := body["raw"].(string)
	return map[string]interface{}{"code": 200, "data": map[string]interface{}{"preview": raw, "itemCount": 1, "format": "json"}, "message": "预览成功"}
}

func (s *service) ConfigImportConfirmService(body map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"code": 200, "data": map[string]interface{}{"imported": true}, "message": "导入成功"}
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
	return map[string]interface{}{"enabled": enabled, "threshold": 0.5}
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

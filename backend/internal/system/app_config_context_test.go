// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newAppConfigTestService(t *testing.T) *service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&AppSetting{}); err != nil {
		t.Fatal(err)
	}
	return &service{db: db}
}

func TestUpdateAppConfigValidatesAndPersistsSettings(t *testing.T) {
	svc := newAppConfigTestService(t)
	ctx := context.Background()
	result, err := svc.UpdateAppConfigContext(ctx, map[string]interface{}{
		"theme":    "midnight",
		"language": "en-US",
		"timezone": "Asia/Tokyo",
		"settings": map[string]interface{}{
			"task_runtime_max_concurrent": "8",
			"theme_mode":                  "light",
			"custom_pref":                 "abc",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result["theme"] != "midnight" || result["language"] != "en-US" || result["timezone"] != "Asia/Tokyo" {
		t.Fatalf("unexpected result: %v", result)
	}
	settings, _ := result["settings"].(map[string]interface{})
	if settings["task_runtime_max_concurrent"] != "8" || settings["theme_mode"] != "light" || settings["custom_pref"] != "abc" {
		t.Fatalf("unexpected settings: %v", settings)
	}
}

func TestUpdateAppConfigRejectsInvalidSettings(t *testing.T) {
	cases := []struct {
		name string
		body map[string]interface{}
		want string
	}{
		{
			name: "invalid timezone",
			body: map[string]interface{}{"timezone": "Mars/Olympus"},
			want: "配置项 timezone 的值不是合法时区",
		},
		{
			name: "invalid language",
			body: map[string]interface{}{"language": "中文"},
			want: "配置项 language 的值不是合法的语言标记",
		},
		{
			name: "invalid setting key",
			body: map[string]interface{}{"settings": map[string]interface{}{"QA_Key": "1"}},
			want: "配置项名称不合法",
		},
		{
			name: "non string top level",
			body: map[string]interface{}{"theme": 42.0},
			want: "配置项 theme 的值类型无效",
		},
		{
			name: "non string nested",
			body: map[string]interface{}{"settings": map[string]interface{}{"task_runtime_max_concurrent": 4.0}},
			want: "配置项 task_runtime_max_concurrent 的值类型无效",
		},
		{
			name: "settings not object",
			body: map[string]interface{}{"settings": []interface{}{"a"}},
			want: "配置块 settings 的值类型无效",
		},
		{
			name: "value too long",
			body: map[string]interface{}{"settings": map[string]interface{}{"theme": strings.Repeat("x", maxAppSettingValueLength+1)}},
			want: "的值过长",
		},
		{
			name: "concurrency out of range",
			body: map[string]interface{}{"settings": map[string]interface{}{"task_runtime_max_concurrent": "0"}},
			want: "超出范围 1-64",
		},
		{
			name: "invalid theme mode",
			body: map[string]interface{}{"settings": map[string]interface{}{"theme_mode": "neon"}},
			want: "配置项 theme_mode 的值不受支持",
		},
		{
			name: "invalid accent color",
			body: map[string]interface{}{"settings": map[string]interface{}{"theme_accent_color": "red"}},
			want: "不是合法的 #RRGGBB 颜色",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			svc := newAppConfigTestService(t)
			ctx := context.Background()
			if _, err := svc.UpdateAppConfigContext(ctx, map[string]interface{}{"language": "zh-CN", "timezone": "Asia/Shanghai"}); err != nil {
				t.Fatal(err)
			}
			if _, err := svc.UpdateAppConfigContext(ctx, testCase.body); err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("err=%v, want contains %q", err, testCase.want)
			}
			if svc.getAppSetting("language") != "zh-CN" || svc.getAppSetting("timezone") != "Asia/Shanghai" {
				t.Fatal("rejected update must not persist any setting")
			}
		})
	}
}

func TestUpdateMoodDetectionRejectsOutOfRangeThreshold(t *testing.T) {
	svc := newAppConfigTestService(t)
	ctx := context.Background()
	if _, err := svc.UpdateMoodDetectionConfigContext(ctx, map[string]interface{}{"enabled": true, "threshold": 0.4}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateMoodDetectionConfigContext(ctx, map[string]interface{}{"threshold": 1.5}); err == nil || !strings.Contains(err.Error(), "超出范围 0-1") {
		t.Fatalf("err=%v", err)
	}
	if svc.getAppSetting("mood_detection_threshold") != "0.4" {
		t.Fatalf("threshold must stay 0.4, got %q", svc.getAppSetting("mood_detection_threshold"))
	}
	if _, err := svc.UpdateMoodDetectionConfigContext(ctx, map[string]interface{}{"enabled": "yes"}); err == nil || !strings.Contains(err.Error(), "应为布尔值") {
		t.Fatalf("err=%v", err)
	}
	if _, err := svc.UpdateMoodDetectionConfigContext(ctx, map[string]interface{}{"threshold": "0.4"}); err == nil || !strings.Contains(err.Error(), "应为数字") {
		t.Fatalf("err=%v", err)
	}
}

func TestUpdateThemeRejectsInvalidValues(t *testing.T) {
	svc := newAppConfigTestService(t)
	ctx := context.Background()
	if _, err := svc.UpdateThemeContext(ctx, map[string]interface{}{"preset": "midnight", "mode": "dark", "accentColor": "#AABBCC"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateThemeContext(ctx, map[string]interface{}{"accentColor": "#12345"}); err == nil || !strings.Contains(err.Error(), "#RRGGBB") {
		t.Fatalf("err=%v", err)
	}
	if _, err := svc.UpdateThemeContext(ctx, map[string]interface{}{"mode": "neon"}); err == nil || !strings.Contains(err.Error(), "theme_mode") {
		t.Fatalf("err=%v", err)
	}
	if svc.getAppSetting("theme") != "midnight" || svc.getAppSetting("theme_accent_color") != "#AABBCC" {
		t.Fatal("rejected theme update must not persist")
	}
}

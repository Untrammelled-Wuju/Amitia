// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxAppSettingValueLength = 512

var (
	appLanguagePattern   = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)
	appHexColorPattern   = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	appSetupStepPattern  = regexp.MustCompile(`^[A-Za-z0-9_.-]{0,64}$`)
	appGenericKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,127}$`)
	appBooleanValues     = map[string]bool{"true": true, "false": true}
	appThemeModes        = map[string]bool{"dark": true, "light": true, "system": true, "auto": true}
)

type appSettingNormalizer func(string) (string, error)

var appSettingRules = map[string]appSettingNormalizer{
	"language": func(value string) (string, error) {
		if value == "" || appLanguagePattern.MatchString(value) {
			return value, nil
		}
		return "", fmt.Errorf("配置项 language 的值不是合法的语言标记: %q", value)
	},
	"timezone": func(value string) (string, error) {
		if value == "" {
			return value, nil
		}
		if _, err := time.LoadLocation(value); err != nil {
			return "", fmt.Errorf("配置项 timezone 的值不是合法时区: %q（%v）", value, err)
		}
		return value, nil
	},
	"theme": func(value string) (string, error) {
		if strings.ContainsAny(value, "\r\n\t") {
			return "", fmt.Errorf("配置项 theme 的值包含非法字符: %q", value)
		}
		return value, nil
	},
	"theme_mode": func(value string) (string, error) {
		if value == "" || appThemeModes[value] {
			return value, nil
		}
		return "", fmt.Errorf("配置项 theme_mode 的值不受支持: %q（可选 dark、light、system、auto）", value)
	},
	"theme_accent_color": func(value string) (string, error) {
		if value == "" || appHexColorPattern.MatchString(value) {
			return value, nil
		}
		return "", fmt.Errorf("配置项 theme_accent_color 的值不是合法的 #RRGGBB 颜色: %q", value)
	},
	"onboarding_completed": func(value string) (string, error) {
		if appBooleanValues[value] {
			return value, nil
		}
		return "", fmt.Errorf("配置项 onboarding_completed 的值必须是 true 或 false: %q", value)
	},
	"setup_step": func(value string) (string, error) {
		if appSetupStepPattern.MatchString(value) {
			return value, nil
		}
		return "", fmt.Errorf("配置项 setup_step 的值不合法: %q（仅允许字母、数字、下划线、点和短横线，最多 64 字符）", value)
	},
	"task_runtime_max_concurrent": func(value string) (string, error) {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return "", fmt.Errorf("配置项 task_runtime_max_concurrent 的值必须是整数: %q", value)
		}
		if parsed < 1 || parsed > 64 {
			return "", fmt.Errorf("配置项 task_runtime_max_concurrent 的值超出范围 1-64: %d", parsed)
		}
		return strconv.Itoa(parsed), nil
	},
	"mood_detection_enabled": func(value string) (string, error) {
		if appBooleanValues[value] {
			return value, nil
		}
		return "", fmt.Errorf("配置项 mood_detection_enabled 的值必须是 true 或 false: %q", value)
	},
	"mood_detection_threshold": func(value string) (string, error) {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return "", fmt.Errorf("配置项 mood_detection_threshold 的值必须是数字: %q", value)
		}
		if parsed < 0 || parsed > 1 {
			return "", fmt.Errorf("配置项 mood_detection_threshold 的值超出范围 0-1: %v", parsed)
		}
		return strconv.FormatFloat(parsed, 'f', -1, 64), nil
	},
}

func validateAppSettings(settings map[string]string) (map[string]string, error) {
	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	normalized := make(map[string]string, len(settings))
	for _, key := range keys {
		value, err := validateAppSetting(key, settings[key])
		if err != nil {
			return nil, err
		}
		normalized[key] = value
	}
	return normalized, nil
}

func validateAppSetting(key, value string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("配置项名称不能为空")
	}
	if strings.ContainsAny(key, " \r\n\t") {
		return "", fmt.Errorf("配置项名称不能包含空白字符: %q", key)
	}
	normalize, ok := appSettingRules[key]
	if !ok {
		if !appGenericKeyPattern.MatchString(key) {
			return "", fmt.Errorf("配置项名称不合法: %q（仅允许小写字母开头的字母、数字、下划线、点和短横线，最多 128 字符）", key)
		}
		if len(value) > maxAppSettingValueLength {
			return "", fmt.Errorf("配置项 %s 的值过长（最多 %d 字符，实际 %d 字符）", key, maxAppSettingValueLength, len(value))
		}
		if strings.ContainsRune(value, '\x00') {
			return "", fmt.Errorf("配置项 %s 的值包含非法字符", key)
		}
		return value, nil
	}
	if len(value) > maxAppSettingValueLength {
		return "", fmt.Errorf("配置项 %s 的值过长（最多 %d 字符，实际 %d 字符）", key, maxAppSettingValueLength, len(value))
	}
	return normalize(value)
}

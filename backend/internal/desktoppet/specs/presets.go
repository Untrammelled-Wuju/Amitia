// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package specs

import (
	"github.com/u-ai/backend/internal/desktoppet/contracts"
)

var minimalPresetKeys = []string{
	"idle_normal", "idle_breathing", "idle_blink",
	"walk_left", "walk_right",
	"wave", "happy", "sad",
	"thinking", "speaking", "sleep", "clicked",
}

var standardPresetKeys = []string{
	"idle_normal", "idle_breathing", "idle_blink", "idle_look_around", "idle_sway",
	"walk_left", "walk_right", "run_left", "run_right",
	"jump", "land", "turn_around",
	"wave", "nod", "shake_head", "clap", "point", "stretch", "bow",
	"happy", "excited", "sad", "angry", "surprised",
	"sleep", "wake_up",
	"listening", "thinking", "speaking", "agreeing", "greeting", "goodbye",
	"clicked", "double_clicked", "dragged", "picked_up", "dropped",
}

func BuiltinPresets() []contracts.ActionPreset {
	return []contracts.ActionPreset{
		{
			Key:         "minimal",
			Name:        "极简方案",
			Description: "覆盖核心待机、移动、互动与对话反馈",
			Version:     2,
			ActionKeys:  minimalPresetKeys,
			RequiredAnyOf: [][]string{
				{"idle_normal", "idle_breathing", "idle_sway"},
			},
		},
		{
			Key:         "standard",
			Name:        "标准方案",
			Description: "覆盖常用待机、移动、互动、情绪、对话反馈与桌面交互",
			Version:     2,
			ActionKeys:  standardPresetKeys,
			RequiredAnyOf: [][]string{
				{"idle_normal", "idle_breathing", "idle_sway"},
			},
		},
		{
			Key:         "complete",
			Name:        "完整方案",
			Description: "包含所有已启用内置动作",
			Version:     2,
			ActionKeys:  CatalogEnabledKeys(),
			RequiredAnyOf: [][]string{
				{"idle_normal", "idle_breathing", "idle_sway"},
			},
		},
	}
}

func PresetByKey(key string) (contracts.ActionPreset, bool) {
	for _, p := range BuiltinPresets() {
		if p.Key == key {
			return p, true
		}
	}
	return contracts.ActionPreset{}, false
}

func ValidatePreset(preset contracts.ActionPreset, availableKeys map[string]bool) (available []string, unavailable []string) {
	seen := make(map[string]bool)
	for _, k := range preset.ActionKeys {
		if seen[k] {
			continue
		}
		seen[k] = true
		if availableKeys[k] {
			available = append(available, k)
		} else {
			unavailable = append(unavailable, k)
		}
	}
	return available, unavailable
}

func IsMinimalSubsetOfStandard() bool {
	standardSet := make(map[string]bool)
	for _, k := range standardPresetKeys {
		standardSet[k] = true
	}
	for _, k := range minimalPresetKeys {
		if !standardSet[k] {
			return false
		}
	}
	return true
}

// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

var MinimalPresetActionKeys = []string{
	"idle_normal",
	"idle_breathing",
	"idle_blink",
	"walk_left",
	"walk_right",
	"wave",
	"happy",
	"sad",
	"thinking",
	"speaking",
	"sleep",
	"clicked",
}

var StandardPresetActionKeys = []string{
	"idle_normal",
	"idle_breathing",
	"idle_blink",
	"idle_look_around",
	"idle_sway",
	"walk_left",
	"walk_right",
	"run_left",
	"run_right",
	"jump",
	"land",
	"turn_around",
	"wave",
	"nod",
	"clap",
	"point",
	"stretch",
	"bow",
	"happy",
	"excited",
	"sad",
	"angry",
	"surprised",
	"listening",
	"thinking",
	"speaking",
	"agreeing",
	"greeting",
	"goodbye",
	"clicked",
	"double_clicked",
	"dragged",
	"picked_up",
	"dropped",
}

func buildPresets(allActionKeys []string) []PresetResponse {
	minimal := make([]string, len(MinimalPresetActionKeys))
	copy(minimal, MinimalPresetActionKeys)
	standard := make([]string, len(StandardPresetActionKeys))
	copy(standard, StandardPresetActionKeys)
	complete := make([]string, len(allActionKeys))
	copy(complete, allActionKeys)
	return []PresetResponse{
		{
			Key:        "minimal",
			Name:       "极简方案",
			Description: "覆盖核心待机、移动、互动与对话反馈的精简动作集,适合快速验证",
			ActionKeys: minimal,
		},
		{
			Key:        "standard",
			Name:       "标准方案",
			Description: "覆盖常用待机、移动、互动、情绪、对话反馈与桌面交互动作,适合大部分桌宠场景",
			ActionKeys: standard,
		},
		{
			Key:        "complete",
			Name:       "完整方案",
			Description: "包含当前所有已启用的动作,适合需要完整表现力的桌宠",
			ActionKeys: complete,
		},
	}
}

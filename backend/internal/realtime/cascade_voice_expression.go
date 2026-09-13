// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"strings"
)

type cascadeVoiceExpressionPlan struct {
	PrimaryEmotion  string  `json:"primary_emotion"`
	Secondary       string  `json:"secondary_emotion,omitempty"`
	Intensity       float64 `json:"intensity"`
	Pace            float64 `json:"pace"`
	Energy          float64 `json:"energy"`
	Warmth          float64 `json:"warmth"`
	Coldness        float64 `json:"coldness"`
	Breathiness     float64 `json:"breathiness"`
	Smile           float64 `json:"smile"`
	Laugh           float64 `json:"laugh"`
	PauseBeforeMS   int     `json:"pause_before_ms"`
	PauseStyle      string  `json:"pause_style"`
	Emphasis        string  `json:"emphasis"`
	Ending          string  `json:"ending"`
	SourceDirective string  `json:"source_directive"`
}

func cascadeVoiceExpressionPlanFromInstruction(instruction string) cascadeVoiceExpressionPlan {
	clean := normalizeCascadeVoiceDirective(instruction)
	plan := cascadeVoiceExpressionPlan{
		PrimaryEmotion: "neutral", Intensity: 0.45, Pace: 0.50, Energy: 0.48,
		Warmth: 0.50, PauseBeforeMS: 80, PauseStyle: "natural", Emphasis: "natural", Ending: "natural",
		SourceDirective: clean,
	}
	contains := func(values ...string) bool {
		for _, value := range values {
			if strings.Contains(clean, value) {
				return true
			}
		}
		return false
	}
	if contains("生气", "恼火", "语气更硬", "不满") {
		plan.PrimaryEmotion = "anger"
		plan.Intensity, plan.Energy, plan.Coldness, plan.Warmth, plan.Smile = 0.72, 0.66, 0.50, 0.25, 0
		plan.Emphasis = "firm"
	}
	if contains("委屈", "受伤", "难过") {
		if plan.PrimaryEmotion == "anger" {
			plan.Secondary = "hurt"
		} else {
			plan.PrimaryEmotion = "hurt"
		}
		plan.Intensity = cascadeMaxFloat(plan.Intensity, 0.62)
		plan.Energy = 0.34
		plan.Pace = 0.42
		plan.Ending = "low"
	}
	if contains("关心", "担心", "温和确认", "声音放轻", "轻一点") {
		if plan.PrimaryEmotion == "anger" || plan.PrimaryEmotion == "hurt" {
			plan.Secondary = plan.PrimaryEmotion
		}
		plan.PrimaryEmotion = "concern"
		plan.Intensity = cascadeMaxFloat(plan.Intensity, 0.58)
		plan.Warmth = cascadeMaxFloat(plan.Warmth, 0.66)
		plan.Pace = 0.42
		plan.Energy = 0.38
	}
	if contains("吃醋", "醋意") {
		plan.PrimaryEmotion = "jealousy"
		plan.Intensity, plan.Coldness, plan.Warmth, plan.Smile = 0.62, 0.42, 0.35, 0.05
		plan.Ending = "restrained"
	}
	if contains("害羞", "羞", "嘴硬") {
		plan.PrimaryEmotion = "shy"
		plan.Intensity, plan.Pace, plan.Energy, plan.Warmth = 0.55, 0.56, 0.40, 0.58
		plan.Ending = "restrained"
	}
	if contains("恍然大悟", "突然明白", "终于明白", "反应过来", "原来如此") {
		plan.PrimaryEmotion = "surprise"
		plan.Intensity = cascadeMaxFloat(plan.Intensity, 0.66)
		plan.Pace = 0.56
		plan.Energy = cascadeMaxFloat(plan.Energy, 0.62)
		plan.Warmth = cascadeMaxFloat(plan.Warmth, 0.58)
		plan.Smile = cascadeMaxFloat(plan.Smile, 0.35)
		plan.PauseBeforeMS = cascadeMaxInt(plan.PauseBeforeMS, 160)
		plan.PauseStyle = "short_hesitation"
		plan.Emphasis = "clear"
		plan.Ending = "slightly_up"
	}
	if contains("开心", "轻快", "笑意") {
		plan.PrimaryEmotion = "joy"
		plan.Intensity, plan.Pace, plan.Energy, plan.Warmth, plan.Smile = 0.64, 0.60, 0.66, 0.70, 0.55
		plan.Ending = "slightly_up"
	}
	if contains("被逗笑", "逗笑", "笑出声", "忍不住笑", "憋笑", "轻笑", "笑声", "哈哈", "噗", "笑死") {
		plan.PrimaryEmotion = "joy"
		plan.Intensity = cascadeMaxFloat(plan.Intensity, 0.70)
		plan.Pace = cascadeMaxFloat(plan.Pace, 0.62)
		plan.Energy = cascadeMaxFloat(plan.Energy, 0.66)
		plan.Warmth = cascadeMaxFloat(plan.Warmth, 0.68)
		plan.Smile = cascadeMaxFloat(plan.Smile, 0.78)
		plan.Laugh = cascadeMaxFloat(plan.Laugh, 0.70)
		plan.Ending = "slightly_up"
	}
	if contains("惊讶", "吃惊", "震惊") {
		plan.PrimaryEmotion = "surprise"
		plan.Intensity = cascadeMaxFloat(plan.Intensity, 0.70)
		plan.Pace = cascadeMaxFloat(plan.Pace, 0.62)
		plan.Energy = cascadeMaxFloat(plan.Energy, 0.70)
		plan.Ending = "slightly_up"
	}
	if contains("疲惫", "困", "累") {
		plan.PrimaryEmotion = "tired"
		plan.Intensity, plan.Pace, plan.Energy = 0.52, 0.36, 0.28
		plan.Breathiness = 0.18
		plan.Ending = "low"
	}
	if contains("失望", "失落", "算了") {
		plan.PrimaryEmotion = "disappointment"
		plan.Intensity = cascadeMaxFloat(plan.Intensity, 0.56)
		plan.Energy = cascadeMinFloat(plan.Energy, 0.38)
		plan.Pace = cascadeMinFloat(plan.Pace, 0.46)
		plan.Warmth *= 0.75
		plan.Ending = "restrained"
	}
	if contains("烦躁", "烦死", "别催", "恼火") {
		plan.PrimaryEmotion = "anger"
		plan.Intensity = cascadeMaxFloat(plan.Intensity, 0.62)
		plan.Pace = cascadeMaxFloat(plan.Pace, 0.60)
		plan.Energy = cascadeMaxFloat(plan.Energy, 0.58)
		plan.Coldness = cascadeMaxFloat(plan.Coldness, 0.24)
		plan.Ending = "restrained"
	}
	if contains("焦虑", "不安", "紧张") {
		plan.PrimaryEmotion = "anxiety"
		plan.Intensity = cascadeMaxFloat(plan.Intensity, 0.58)
		plan.Pace = cascadeMaxFloat(plan.Pace, 0.56)
		plan.Energy = cascadeMinFloat(plan.Energy, 0.46)
		plan.Breathiness = cascadeMaxFloat(plan.Breathiness, 0.12)
		plan.Ending = "slightly_up"
	}
	if contains("害怕", "恐惧", "吓我一跳", "吓到") {
		plan.PrimaryEmotion = "fear"
		plan.Intensity = cascadeMaxFloat(plan.Intensity, 0.68)
		plan.Pace = cascadeMaxFloat(plan.Pace, 0.62)
		plan.Energy = cascadeMaxFloat(plan.Energy, 0.62)
		plan.Breathiness = cascadeMaxFloat(plan.Breathiness, 0.14)
		plan.Ending = "slightly_up"
	}
	if contains("心虚", "抱歉", "对不起", "是我不好", "愧疚") {
		plan.PrimaryEmotion = "guilt"
		plan.Intensity = cascadeMaxFloat(plan.Intensity, 0.52)
		plan.Energy = cascadeMinFloat(plan.Energy, 0.36)
		plan.Pace = cascadeMinFloat(plan.Pace, 0.44)
		plan.Warmth = cascadeMaxFloat(plan.Warmth, 0.48)
		plan.Ending = "restrained"
	}
	if contains("孤独", "空落落") {
		plan.PrimaryEmotion = "loneliness"
		plan.Intensity = cascadeMaxFloat(plan.Intensity, 0.50)
		plan.Energy = cascadeMinFloat(plan.Energy, 0.30)
		plan.Pace = cascadeMinFloat(plan.Pace, 0.42)
		plan.Ending = "low"
	}
	if contains("嫌弃", "厌恶", "恶心", "别了吧") {
		plan.PrimaryEmotion = "disgust"
		plan.Intensity = cascadeMaxFloat(plan.Intensity, 0.50)
		plan.Energy = cascadeMaxFloat(plan.Energy, 0.50)
		plan.Warmth *= 0.70
		plan.Ending = "restrained"
	}
	if contains("冷淡", "冷一点", "克制") {
		plan.Coldness = cascadeMaxFloat(plan.Coldness, 0.36)
		plan.Warmth *= 0.72
		plan.Smile *= 0.25
	}
	if contains("语速稍慢", "慢一点", "语速慢") {
		plan.Pace = cascadeMinFloat(plan.Pace, 0.40)
	}
	if contains("语速稍快", "略快", "语速快") {
		plan.Pace = cascadeMaxFloat(plan.Pace, 0.62)
	}
	if contains("短暂停顿", "先停顿", "开头停") {
		plan.PauseBeforeMS, plan.PauseStyle = 180, "short_hesitation"
	}
	if contains("压低声音", "低声") {
		plan.Energy = cascadeMinFloat(plan.Energy, 0.35)
		plan.Breathiness = cascadeMaxFloat(plan.Breathiness, 0.12)
	}
	if contains("强调明显", "强调") {
		plan.Emphasis = "clear"
	}
	return plan
}

func cascadeMapTTSInstruction(configured string, plan cascadeVoiceExpressionPlan) string {
	parts := make([]string, 0, 3)
	if value := normalizeCascadeVoiceDirective(configured); value != "" {
		parts = append(parts, value)
	}
	source := normalizeCascadeVoiceDirective(plan.SourceDirective)
	if source != "" && !cascadeContainsString(parts, source) {
		parts = append(parts, source)
	}
	if tail := cascadeVoiceExpressionTail(plan, strings.Join(parts, "，")); tail != "" {
		parts = append(parts, tail)
	}
	if len(parts) == 0 {
		return "自然口语，像真人打电话一样"
	}
	return strings.Join(parts, "，")
}

func cascadeVoiceExpressionTail(plan cascadeVoiceExpressionPlan, existing string) string {
	items := make([]string, 0, 8)
	has := func(values ...string) bool {
		for _, value := range values {
			if strings.Contains(existing, value) {
				return true
			}
		}
		return false
	}
	switch plan.PrimaryEmotion {
	case "concern":
		if !has("关心", "担心", "温和") {
			items = append(items, "保持关心但自然")
		}
	case "anger":
		if !has("生气", "恼火", "不满", "语气更硬") {
			items = append(items, "保留明确不满")
		}
	case "hurt":
		if !has("委屈", "受伤", "难过") {
			items = append(items, "带克制的委屈感")
		}
	case "jealousy":
		if !has("吃醋", "醋意") {
			items = append(items, "带一点克制醋意")
		}
	case "joy":
		if !has("开心", "轻快", "笑意") {
			items = append(items, "整体轻快自然")
		}
	case "tired":
		if !has("疲惫", "困", "累") {
			items = append(items, "带自然疲惫感")
		}
	case "shy":
		if !has("害羞", "羞", "嘴硬") {
			items = append(items, "带轻微害羞和克制")
		}
	case "surprise":
		if !has("惊讶", "吃惊", "震惊", "恍然大悟") {
			items = append(items, "带自然惊讶反应")
		}
	case "disappointment":
		if !has("失望", "失落", "算了") {
			items = append(items, "带克制失望感")
		}
	case "anxiety":
		if !has("焦虑", "不安", "紧张") {
			items = append(items, "带一点焦虑和紧绷感")
		}
	case "fear":
		if !has("害怕", "恐惧", "吓") {
			items = append(items, "带短促惊吓感")
		}
	case "guilt":
		if !has("心虚", "抱歉", "对不起", "愧疚") {
			items = append(items, "带自然心虚和歉意")
		}
	case "loneliness":
		if !has("孤独", "空落落") {
			items = append(items, "带安静孤独感")
		}
	case "disgust":
		if !has("嫌弃", "厌恶", "恶心") {
			items = append(items, "带轻微嫌弃但不攻击")
		}
	}
	if plan.Pace <= 0.42 && !has("语速稍慢", "慢一点", "语速慢") {
		items = append(items, "语速稍慢")
	} else if plan.Pace >= 0.60 && !has("语速稍快", "略快", "语速快") {
		items = append(items, "语速稍快")
	}
	if plan.Energy <= 0.35 && !has("能量收低", "压低声音", "低声") {
		items = append(items, "声音能量收低")
	}
	if plan.Coldness >= 0.40 && !has("冷淡", "冷感", "冷一点") {
		items = append(items, "保留克制冷感")
	}
	if plan.Laugh >= 0.45 && !has("笑意", "笑出声", "轻笑", "笑声", "哈哈", "噗") {
		items = append(items, "带自然轻笑，笑点处不要拖平")
	} else if plan.Smile >= 0.45 && !has("笑意", "轻快") {
		items = append(items, "带自然笑意")
	}
	if plan.Breathiness >= 0.12 && !has("气声", "轻声", "压低声音") {
		items = append(items, "略带轻气声")
	}
	if plan.PauseBeforeMS >= 150 && !has("短暂停顿", "先停顿", "开头停") {
		items = append(items, "开头自然短停顿")
	}
	if (plan.Emphasis == "firm" || plan.Emphasis == "clear") && !has("强调", "语气更硬") {
		items = append(items, "重点词自然强调")
	}
	if (plan.Ending == "low" || plan.Ending == "restrained") && !has("尾音", "收住", "压住") {
		items = append(items, "尾音自然收住")
	}
	return strings.Join(items, "，")
}

func cascadeMaxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func cascadeMinFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func cascadeMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

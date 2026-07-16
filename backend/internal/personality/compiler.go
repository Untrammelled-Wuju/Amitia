package personality

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type CompiledPersonality struct {
	Version         string                 `json:"version"`
	CharacterID     string                 `json:"characterId"`
	SchemaVersion   string                 `json:"schemaVersion"`
	CognitiveSens   float64                `json:"cognitiveSens"`
	RecoverySpeed   float64                `json:"recoverySpeed"`
	BehaviorBias    map[string]float64     `json:"behaviorBias"`
	ExpressionStyle map[string]float64     `json:"expressionStyle"`
	ImmutableCore   map[string]string      `json:"immutableCore"`
	CompiledAt      time.Time              `json:"compiledAt"`
	SourceConfig    map[string]interface{} `json:"sourceConfig"`
	Diagnostics     []string               `json:"diagnostics"`
}

type PersonalityTemplate struct {
	ID             string
	Label          string
	Gender         string
	CoreParadox    string
	SpeechPatterns []string
	SpeakingStyle  string
	Prohibitions   []string
	Examples       struct {
		Low    []string
		Medium []string
		High   []string
	}
}

type CompilerConfig struct {
	DefaultSensitivity float64
	DefaultRecovery    float64
}

func DefaultCompilerConfig() CompilerConfig {
	return CompilerConfig{
		DefaultSensitivity: 0.5,
		DefaultRecovery:    0.5,
	}
}

type Compiler struct {
	config CompilerConfig
}

func NewCompiler(cfg CompilerConfig) *Compiler {
	return &Compiler{config: cfg}
}

func (c *Compiler) Compile(characterID string, rawConfig map[string]interface{}) CompiledPersonality {
	cp := CompiledPersonality{
		Version:       "personality-compiled-v1",
		CharacterID:   characterID,
		SchemaVersion: "v1",
		CompiledAt:    time.Now().UTC(),
		SourceConfig:  rawConfig,
		Diagnostics:   []string{},
	}

	cp.CognitiveSens = extractFloat(rawConfig, "cognitiveSensitivity", c.config.DefaultSensitivity)
	cp.RecoverySpeed = extractFloat(rawConfig, "recoverySpeed", c.config.DefaultRecovery)

	cp.BehaviorBias = map[string]float64{
		"warmth":                   extractFloat(rawConfig, "warmth", 0.5),
		"directness":               extractFloat(rawConfig, "directness", 0.5),
		"humor":                    extractFloat(rawConfig, "humor", 0.4),
		"affection":                extractFloat(rawConfig, "affection", 0.45),
		"conflictAvoidance":        extractFloat(rawConfig, "conflictAvoidance", 0.5),
		"initiative":               extractFloat(rawConfig, "initiative", 0.5),
		"familiarity":              extractFloat(rawConfig, "familiarity", 0.5),
		"customerServiceAvoidance": extractFloat(rawConfig, "customerServiceAvoidance", 0.5),
		"structureLevel":           extractFloat(rawConfig, "structureLevel", 0.5),
		"emotionalExpression":      extractFloat(rawConfig, "emotionalExpression", 0.5),
		"comfortLevel":             extractFloat(rawConfig, "comfortLevel", 0.5),
		"preachingAvoidance":       extractFloat(rawConfig, "preachingAvoidance", 0.5),
		"rationality":              extractFloat(rawConfig, "rationality", 0.5),
		"teasing":                  extractFloat(rawConfig, "teasing", 0.3),
		"patience":                 extractFloat(rawConfig, "patience", 0.6),
		"companionship":            extractFloat(rawConfig, "companionship", 0.55),
		"boundary":                 extractFloat(rawConfig, "boundary", 0.85),
		"dependencyAvoidance":      extractFloat(rawConfig, "dependencyAvoidance", 0.85),
		"execution":                extractFloat(rawConfig, "execution", 0.5),
		"explanationDepth":         extractFloat(rawConfig, "explanationDepth", 0.5),
		"judgment":                 extractFloat(rawConfig, "judgment", 0.5),
		"clarification":            extractFloat(rawConfig, "clarification", 0.5),
		"intimacyExpression":       extractFloat(rawConfig, "intimacyExpression", 0.25),
		"flirtiness":               extractFloat(rawConfig, "flirtiness", 0.0),
		"romanticTone":             extractFloat(rawConfig, "romanticTone", 0.0),
		"suggestivenessAvoidance":  extractFloat(rawConfig, "suggestivenessAvoidance", 1.0),
		"intimacyBoundary":         extractFloat(rawConfig, "intimacyBoundary", 0.9),
	}

	cp.ExpressionStyle = map[string]float64{
		"verbosity":           extractFloat(rawConfig, "verbosity", 0.5),
		"formality":           extractFloat(rawConfig, "formality", 0.5),
		"emotionalExpression": extractFloat(rawConfig, "emotionalExpression", 0.5),
		"emotionality":        extractFloat(rawConfig, "emotionality", 0.5),
		"shortSentence":       extractFloat(rawConfig, "shortSentence", 0.5),
		"toneWords":           extractFloat(rawConfig, "toneWords", 0.5),
	}

	cp.ImmutableCore = map[string]string{
		"identity":     extractString(rawConfig, "identity", ""),
		"coreBoundary": extractString(rawConfig, "coreBoundary", ""),
	}

	if raw, ok := rawConfig["personality"]; ok {
		if s, ok := raw.(string); ok && s != "" {
			cp.ImmutableCore["personalityDesc"] = s
		}
	}

	raw, _ := json.Marshal(rawConfig)
	if len(raw) == 0 || string(raw) == "{}" {
		cp.Diagnostics = append(cp.Diagnostics, "empty_config_defaults_applied")
	}

	return cp
}

func (cp CompiledPersonality) ToPersonalitySummary() string {
	var lines []string

	lines = append(lines, "【行为倾向】")

	lines = append(lines, formatBiasPair(cp.BehaviorBias, "warmth", "冷淡与疏离", "温暖与亲近"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "directness", "委婉与含蓄", "直接与坦率"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "humor", "严肃与认真", "幽默与轻松"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "affection", "克制与内敛", "深情与依恋"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "conflictAvoidance", "直面冲突", "回避冲突"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "initiative", "被动回应", "主动发起"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "familiarity", "礼貌距离", "熟络随意"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "customerServiceAvoidance", "客服式服务", "平等非服务"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "structureLevel", "随性发散", "结构化表达"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "emotionalExpression", "平静克制", "情绪外露"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "comfortLevel", "不安紧张", "舒适放松"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "preachingAvoidance", "说教倾向", "避免说教"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "rationality", "感性直觉", "理性分析"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "teasing", "不捉弄", "爱捉弄调侃"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "patience", "急躁催促", "耐心包容"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "companionship", "保持独立", "陪伴倾向"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "boundary", "边界模糊", "边界清晰"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "dependencyAvoidance", "依赖对方", "独立自主"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "execution", "灵活变通", "严格按计划"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "explanationDepth", "简洁回答", "深入解释"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "judgment", "不加评判", "明确判断"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "clarification", "直接允许", "追问确认"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "intimacyExpression", "回避亲密", "表达亲密"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "flirtiness", "不调情", "喜欢调情"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "romanticTone", "非浪漫", "浪漫语气"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "suggestivenessAvoidance", "暗示接受", "坚决拒诱惑"))
	lines = append(lines, formatBiasPair(cp.BehaviorBias, "intimacyBoundary", "亲密随便", "亲密有边界"))

	lines = append(lines, "")
	lines = append(lines, "【表达风格】")

	lines = append(lines, formatExprLine(cp.ExpressionStyle, "verbosity", "简洁短句", "详细长篇"))
	lines = append(lines, formatExprLine(cp.ExpressionStyle, "formality", "随意口语", "正式书面"))
	lines = append(lines, formatExprLine(cp.ExpressionStyle, "emotionalExpression", "理性克制", "感性外露"))
	lines = append(lines, formatExprLine(cp.ExpressionStyle, "shortSentence", "长句叙述", "短句分段"))
	lines = append(lines, formatExprLine(cp.ExpressionStyle, "toneWords", "语气平淡", "语气词丰富"))

	return strings.Join(lines, "\n")
}

func CompilePersonalityTemplate(name, presetID, gender string) string {
	tmpl, ok := allPersonalityTemplates[presetID]
	if !ok {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("【人格：%s %s】\n", tmpl.Label, name))

	sb.WriteString(fmt.Sprintf("核心矛盾：%s\n", tmpl.CoreParadox))

	sb.WriteString(fmt.Sprintf("说话方式：%s\n", tmpl.SpeakingStyle))

	sb.WriteString(fmt.Sprintf("常用语癖：%s\n", strings.Join(tmpl.SpeechPatterns, "、")))

	sb.WriteString(fmt.Sprintf("禁止：%s\n", strings.Join(tmpl.Prohibitions, "、")))

	if len(tmpl.Examples.Low) > 0 {
		sb.WriteString(fmt.Sprintf("示例（低亲密）：%s\n", strings.Join(tmpl.Examples.Low, " ")))
	}
	if len(tmpl.Examples.Medium) > 0 {
		sb.WriteString(fmt.Sprintf("示例（中亲密）：%s\n", strings.Join(tmpl.Examples.Medium, " ")))
	}
	if len(tmpl.Examples.High) > 0 {
		sb.WriteString(fmt.Sprintf("示例（高亲密）：%s\n", strings.Join(tmpl.Examples.High, " ")))
	}

	voiceGuide, hasVoice := presetVoiceGuides[presetID]
	if !hasVoice {
		if gender == "MALE" || gender == "male" {
			voiceGuide = presetVoiceGuides["boy_next_door"]
		} else {
			voiceGuide = presetVoiceGuides["deredere"]
		}
	}
	if voiceGuide != "" {
		sb.WriteString(fmt.Sprintf("\n【口吻指南】\n%s\n", voiceGuide))
	}

	return sb.String()
}

func BuildProactivePersonalityBlock(name, presetID, gender string, aff float64, harass bool) string {
	tmpl, ok := allPersonalityTemplates[presetID]
	if !ok {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("【人格：%s %s】\n", tmpl.Label, name))
	sb.WriteString(fmt.Sprintf("核心矛盾：%s\n", tmpl.CoreParadox))
	sb.WriteString(fmt.Sprintf("说话方式：%s\n", tmpl.SpeakingStyle))
	sb.WriteString(fmt.Sprintf("常用语癖：%s\n", strings.Join(tmpl.SpeechPatterns, "、")))
	prohibitions := tmpl.Prohibitions
	if len(prohibitions) > 6 {
		prohibitions = prohibitions[:6]
	}
	sb.WriteString(fmt.Sprintf("禁止：%s\n", strings.Join(prohibitions, "、")))

	examples := selectAffExamples(tmpl, aff)
	if examples != "" {
		sb.WriteString(fmt.Sprintf("示例（按当前亲密感适配）：%s\n", examples))
	}

	voiceGuide, hasVoice := presetVoiceGuides[presetID]
	if !hasVoice {
		if gender == "MALE" || gender == "male" {
			voiceGuide = presetVoiceGuides["boy_next_door"]
		} else {
			voiceGuide = presetVoiceGuides["deredere"]
		}
	}
	if voiceGuide != "" {
		sb.WriteString(fmt.Sprintf("\n【口吻指南】\n%s\n", voiceGuide))
	}

	sb.WriteString(fmt.Sprintf("\n【主动消息】用户暂时没回，你要主动发一条短消息。必须像「%s」本人说话，禁止通用温柔助手/客服腔。\n", tmpl.Label))

	if harass {
		iVal := 50.0
		if i, ok := presetIValues[presetID]; ok {
			iVal = i
		}
		if iVal < 35 {
			sb.WriteString(fmt.Sprintf("【骚扰模式】说教话痨表达方式属于「%s」人格，低主动人格也要用极短、克制的方式表达在意，不要突然变话痨撒娇。\n", tmpl.Label))
		} else if iVal >= 70 {
			sb.WriteString(fmt.Sprintf("【骚扰模式】按「%s」高主动人格的人设可以更直接地黏人、调侃或表达想念。\n", tmpl.Label))
		} else {
			sb.WriteString(fmt.Sprintf("【骚扰模式】按「%s」人设自然程度主动，不要脱离语癖与说话方式。\n", tmpl.Label))
		}
	}

	return sb.String()
}

func formatBiasPair(bias map[string]float64, key, lowLabel, highLabel string) string {
	v := bias[key]
	label := mapBiasLabel(v, lowLabel, highLabel)
	return fmt.Sprintf("- %s: %.0f%% (%s)", mapBiasName(key), v*100, label)
}

func formatExprLine(style map[string]float64, key, lowLabel, highLabel string) string {
	v := style[key]
	label := mapBiasLabel(v, lowLabel, highLabel)
	return fmt.Sprintf("- %s: %.0f%% (%s)", mapBiasName(key), v*100, label)
}

func mapBiasLabel(v float64, lowLabel, highLabel string) string {
	if v <= 0.25 {
		return "非常" + lowLabel
	}
	if v <= 0.4 {
		return "偏向" + lowLabel
	}
	if v <= 0.6 {
		return "适中"
	}
	if v <= 0.75 {
		return "偏向" + highLabel
	}
	return "非常" + highLabel
}

func mapBiasName(key string) string {
	names := map[string]string{
		"warmth":                   "温暖度",
		"directness":               "直接度",
		"humor":                    "幽默感",
		"affection":                "深情度",
		"conflictAvoidance":        "回避冲突",
		"initiative":               "主动性",
		"familiarity":              "熟络度",
		"customerServiceAvoidance": "非客服度",
		"structureLevel":           "结构化",
		"emotionalExpression":      "情绪表达",
		"comfortLevel":             "舒适度",
		"preachingAvoidance":       "反说教",
		"rationality":              "理性度",
		"teasing":                  "捉弄度",
		"patience":                 "耐心度",
		"companionship":            "陪伴倾向",
		"boundary":                 "边界清晰",
		"dependencyAvoidance":      "独立度",
		"execution":                "执行力",
		"explanationDepth":         "解释深度",
		"judgment":                 "判断性",
		"clarification":            "澄清需求",
		"intimacyExpression":       "亲密表达",
		"flirtiness":               "调情度",
		"romanticTone":             "浪漫语气",
		"suggestivenessAvoidance":  "拒诱惑",
		"intimacyBoundary":         "亲密边界",
		"verbosity":                "话量",
		"formality":                "正式度",
		"shortSentence":            "短句偏好",
		"toneWords":                "语气词",
		"emotionality":             "感性度",
	}
	if n, ok := names[key]; ok {
		return n
	}
	return key
}

func extractFloat(m map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return normalizeSlider(val)
		case int:
			return normalizeSlider(float64(val))
		case json.Number:
			f, _ := val.Float64()
			return normalizeSlider(f)
		}
	}
	return defaultVal
}

func extractString(m map[string]interface{}, key, defaultVal string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

func normalizeSlider(v float64) float64 {
	if v > 1 {
		v = v / 100
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func selectAffExamples(tmpl PersonalityTemplate, aff float64) string {
	if aff <= 0.3 {
		if len(tmpl.Examples.Low) > 0 {
			return strings.Join(tmpl.Examples.Low, " ")
		}
	}
	if aff <= 0.6 {
		if len(tmpl.Examples.Medium) > 0 {
			return strings.Join(tmpl.Examples.Medium, " ")
		}
	}
	if len(tmpl.Examples.High) > 0 {
		return strings.Join(tmpl.Examples.High, " ")
	}
	return ""
}

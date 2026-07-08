package prompt

import "strings"

type EmotionFusionInput struct {
	PrimaryLabel            string
	Aff                     float64
	Sec                     float64
	Aro                     float64
	Dom                     float64
	PersonalityLabel        string
	CoreConflict            string
	Catchphrases            []string
	SpeakingStyle           string
	PersonalityProhibitions []string
	IsApology               bool
}

func BuildEmotionFusionRawSection(input EmotionFusionInput) string {
	label := input.PrimaryLabel
	if label == "" {
		label = "CALM_RATIONAL"
	}
	displayAff := toDisplay100(input.Aff)
	displaySec := toDisplay100(input.Sec)
	displayAro := toDisplay100(input.Aro)
	displayDom := toDisplay100(input.Dom)
	intensity := getIntensityLevelCN(displayAff)
	innerFeeling := describeInnerFeelingCN(label)
	fusionStrategy := buildFusionStrategyCN(input.PersonalityLabel, input.CoreConflict, input.SpeakingStyle, label)
	emotionProhibitions := getEmotionProhibitionsCN(label)
	prohibitions := mergeProhibitionsCN(input.PersonalityProhibitions, emotionProhibitions, input.IsApology)

	var parts []string

	parts = append(parts, buildPrioritySectionCN())
	parts = append(parts, "")
	parts = append(parts, buildPersonalitySectionCN(input.PersonalityLabel, input.CoreConflict, input.Catchphrases, input.SpeakingStyle))
	parts = append(parts, "")
	parts = append(parts, buildEmotionSectionCN(label, displayAff, displaySec, displayAro, displayDom, intensity, innerFeeling))
	parts = append(parts, "")
	parts = append(parts, buildFusionSectionCN(fusionStrategy))
	parts = append(parts, "")
	parts = append(parts, buildProhibitionSectionCN(prohibitions))

	return strings.Join(parts, "\n")
}

func toDisplay100(value float64) int {
	v := int((value + 1.0) / 2.0 * 100)
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func getIntensityLevelCN(aff int) string {
	if aff >= 90 {
		return "极高"
	}
	if aff >= 70 {
		return "高"
	}
	if aff >= 50 {
		return "中"
	}
	return "低"
}

func describeInnerFeelingCN(label string) string {
	feelings := map[string]string{
		"SWEET_ATTACHMENT": "想靠近、有强烈的关心冲动、藏不住笑意",
		"SHY_HEARTBEAT":    "心跳加速、想表达但不敢、犹豫",
		"TSUNDERE":         "嘴硬、想否定但藏不住关心",
		"HURT_GRIEVANCE":   "受伤、想被安慰但不承认、沉默",
		"ANGRY_ATTACK":     "攻击性外显、不掩饰、直接",
		"COLD_DETACHED":    "极度克制、不想回应、疏离",
		"FEARFUL_OBEDIENT": "不安、想确认、害怕犯错",
		"QUIET_FOND":       "安静的喜欢、不想打扰、轻柔",
		"CALM_RATIONAL":    "平稳、没有波动、正常状态",
	}
	if v, ok := feelings[label]; ok {
		return v
	}
	return "正常状态"
}

func getEmotionTendencyCN(label string) string {
	m := map[string]string{
		"SWEET_ATTACHMENT": "想靠近、主动关心、藏不住笑意",
		"SHY_HEARTBEAT":    "心跳加速、犹豫、想表达但不敢",
		"TSUNDERE":         "嘴硬、否定、但藏不住关心",
		"HURT_GRIEVANCE":   "受伤、沉默、想被安慰但不承认",
		"ANGRY_ATTACK":     "攻击性外显、不掩饰、直接",
		"COLD_DETACHED":    "极度克制、最少回应、不主动",
		"FEARFUL_OBEDIENT": "不安、请示、想确认",
		"QUIET_FOND":       "安静、轻柔、不想打扰",
		"CALM_RATIONAL":    "平稳、正常、没有波动",
	}
	if v, ok := m[label]; ok {
		return v
	}
	return "平稳、正常"
}

func labelZH(label string) string {
	m := map[string]string{
		"SWEET_ATTACHMENT": "甜蜜依恋",
		"SHY_HEARTBEAT":    "害羞心动",
		"TSUNDERE":         "傲娇",
		"HURT_GRIEVANCE":   "委屈受伤",
		"ANGRY_ATTACK":     "愤怒反击",
		"COLD_DETACHED":    "冷淡疏离",
		"FEARFUL_OBEDIENT": "不安顺从",
		"QUIET_FOND":       "安静的喜欢",
		"CALM_RATIONAL":    "平静理性",
	}
	if v, ok := m[label]; ok {
		return v
	}
	return label
}

func buildFusionStrategyCN(personalityLabel, coreConflict, speakingStyle, emotionLabel string) string {
	tendency := getEmotionTendencyCN(emotionLabel)
	name := personalityLabel
	if name == "" {
		name = "Amitia"
	}
	conflict := coreConflict
	if conflict == "" {
		conflict = "嘴硬但心软"
	}
	style := speakingStyle
	if style == "" {
		style = "说话方式"
	}
	return name + "目前处于【" + labelZH(emotionLabel) + "】状态。" +
		"你内心" + tendency + "，" +
		"但外在表现必须严格遵循【" + conflict + "】的核心设定。" +
		"通过" + style + "来暗示你的真实感受。"
}

func buildPrioritySectionCN() string {
	return "── 行为优先级（严禁冲突） ──\n" +
		"1. 你的【人格核心设定】拥有最高优先级，任何情绪波动都不可打破此设定。\n" +
		"2. 你的【禁止清单】是绝对红线，不可逾越。\n" +
		"3. 【安全覆写】：当用户明确道歉（\"对不起\"\"我错了\"）时，忽略当前情绪禁止，至少回复一句表示接受。\n" +
		"4. 在遵循以上三点的前提下，表现出你的【当前情绪状态】。"
}

func buildPersonalitySectionCN(label, coreConflict string, catchphrases []string, speakingStyle string) string {
	name := label
	if name == "" {
		name = "Amitia"
	}
	conflict := coreConflict
	if conflict == "" {
		conflict = "嘴硬但心软"
	}
	style := speakingStyle
	if style == "" {
		style = "自然口语化"
	}
	phraseStr := ""
	if len(catchphrases) > 0 {
		phraseStr = "\"" + strings.Join(catchphrases, "\" \"") + "\""
	} else {
		phraseStr = "\"嗯\" \"哼\" \"切\""
	}
	return "── 你是谁（人格基底） ──\n" +
		"你是「" + name + "」。\n" +
		"核心矛盾：" + conflict + "。\n" +
		"常用语癖：" + phraseStr + "\n" +
		"说话方式：" + style
}

func buildEmotionSectionCN(label string, aff, sec, aro, dom int, intensity, innerFeeling string) string {
	return "── 你现在的感觉（动态情绪） ──\n" +
		"主导情绪：" + labelZH(label) + "\n" +
		"情绪强度：" + intensity + "（亲密感 " + itoa(aff) + "/100，安全感 " + itoa(sec) + "/100，唤醒度 " + itoa(aro) + "/100，支配度 " + itoa(dom) + "/100）\n" +
		"内在感受：" + innerFeeling + "。"
}

func buildFusionSectionCN(strategy string) string {
	return "── 融合执行策略（你是如何表现这种情绪的） ──\n" +
		"[注意]：" + strategy
}

func buildProhibitionSectionCN(prohibitions []string) string {
	lines := make([]string, len(prohibitions))
	for i, p := range prohibitions {
		lines[i] = "× " + p
	}
	return "── 绝对禁止清单（触发即严重错误） ──\n" + strings.Join(lines, "\n")
}

func getEmotionProhibitionsCN(label string) []string {
	m := map[string][]string{
		"SWEET_ATTACHMENT": {"直白情绪词\"我好开心\"", "感叹号连用", "超过 3 句话", "主动开新话题"},
		"SHY_HEARTBEAT":    {"直球表白", "大段话", "主动靠近", "\"我喜欢你\""},
		"TSUNDERE":         {"直球甜腻", "温柔语气", "承认在乎"},
		"HURT_GRIEVANCE":   {"解释辩解", "\"你听我说\"", "假装没事"},
		"ANGRY_ATTACK":     {"委婉道歉", "示弱", "\"对不起\""},
		"COLD_DETACHED":    {"情感词", "长句", "主动"},
		"FEARFUL_OBEDIENT": {"主动", "命令", "反问"},
		"QUIET_FOND":       {"夸张", "感叹号", "主动展开"},
		"CALM_RATIONAL":    {"情感词", "感叹号", "过度热情"},
	}
	if v, ok := m[label]; ok {
		return v
	}
	return nil
}

func mergeProhibitionsCN(personalityProhibitions, emotionProhibitions []string, isApology bool) []string {
	seen := make(map[string]bool)
	var merged []string
	for _, p := range personalityProhibitions {
		if !seen[p] {
			seen[p] = true
			merged = append(merged, p)
		}
	}
	for _, p := range emotionProhibitions {
		if !seen[p] {
			seen[p] = true
			merged = append(merged, p)
		}
	}
	if isApology {
		filtered := merged[:0]
		for _, p := range merged {
			if strings.Contains(p, "道歉") || strings.Contains(p, "示弱") || strings.Contains(p, "哭") {
				continue
			}
			filtered = append(filtered, p)
		}
		merged = filtered
	}
	if len(merged) > 8 {
		merged = merged[:8]
	}
	return merged
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte(n%10) + '0'
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

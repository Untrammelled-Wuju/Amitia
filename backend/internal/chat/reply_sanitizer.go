package chat

import (
	"regexp"
	"strings"
)

var reThinkTag = regexp.MustCompile(`(?is)<think[^>]*>[\s\S]*?</think\s*>`)
var reThinkingTag = regexp.MustCompile(`(?is)<thinking[^>]*>[\s\S]*?</thinking\s*>`)
var reThoughtTag = regexp.MustCompile(`(?is)<thought[^>]*>[\s\S]*?</thought\s*>`)
var reReflectionTag = regexp.MustCompile(`(?is)<reflection[^>]*>[\s\S]*?</reflection\s*>`)

var reMarkdownThinkHeader = regexp.MustCompile(`^#{1,3}\s*(思考|思维|推理|分析|Thinking|Reasoning|Analysis|Thought|思考过程|内心独白)\s*$`)

var reBracketThinkBlock = regexp.MustCompile(`(?is)【(思考|思维|推理|分析)】[\s\S]*?【/(思考|思维|推理|分析)】`)

var reInlineThinkBlocks = []*regexp.Regexp{
	regexp.MustCompile(`(?is)\[思考\]\s*[\s\S]*?\[/思考\]`),
	regexp.MustCompile(`(?is)\[思维\]\s*[\s\S]*?\[/思维\]`),
	regexp.MustCompile(`(?is)\[推理\]\s*[\s\S]*?\[/推理\]`),
	regexp.MustCompile(`(?is)\[分析\]\s*[\s\S]*?\[/分析\]`),
	regexp.MustCompile(`(?is)\[thought\]\s*[\s\S]*?\[/thought\]`),
	regexp.MustCompile(`(?is)\[thinking\]\s*[\s\S]*?\[/thinking\]`),
}

var reHTMLAnyTag = regexp.MustCompile(`<[^>]*>`)

var reJSONWrap = regexp.MustCompile(`\{[\s\S]*?\}`)
var reAsterisk = regexp.MustCompile(`\*`)
var reStickerPattern = regexp.MustCompile(`(?i)\bsticker_\w+\.png\b`)

var reSentenceSplit = regexp.MustCompile(`[。！？!?\n]`)
var reSentenceEnd = regexp.MustCompile(`[。！？!?\n]+`)

var metaSentenceMarkers = []string{
	"用户说", "用户问", "用户想", "用户希望",
	"我得", "我要", "我需要", "我应该",
	"这是在", "思考过程", "内心独白",
	"不能让任何人", "知道你是AI", "你是AI", "作为AI", "模型",
	"让我来", "让我为", "根据记忆", "据我所知",
	"综上所述", "作为一个AI",
	"作为人工智能", "作为语言模型",
	"系统提醒", "系统通知", "任务：", "主动消息：", "指令：", "提示：",
}

var antiRepeatPhrases = []string{
	"在", "在呢", "在的", "我在", "嗯我在", "嗯嗯在",
}

var antiRepeatWeakPhrases = []string{
	"嗯", "嗯嗯", "好", "好的", "好吧", "行", "行吧", "知道了", "哦", "噢",
}

func SanitizeReply(raw string, charName string, priorReplies []string) string {
	if raw == "" {
		return ""
	}

	result := raw

	result = stripThinkingTags(result)

	result = stripJSONWrap(result)

	result = stripHTMLTags(result)

	result = stripMarkdownFormatting(result)

	result = stripResponsePrefix(result)

	result = stripRoleNamePrefix(result, charName)

	result = stripLineDuplicates(result)

	result = extractDirectReply(result)

	result = stripRepeatPhrases(result)

	result = trimToSentenceLimit(result, 8, 500)

	result = stripPriorRepeats(result, priorReplies)

	result = strings.TrimSpace(result)

	return result
}

func stripThinkingTags(content string) string {
	result := content

	result = reThinkTag.ReplaceAllString(result, "")
	result = reThinkingTag.ReplaceAllString(result, "")
	result = reThoughtTag.ReplaceAllString(result, "")
	result = reReflectionTag.ReplaceAllString(result, "")

	result = stripMarkdownThinkSections(result)

	result = reBracketThinkBlock.ReplaceAllString(result, "")

	for _, re := range reInlineThinkBlocks {
		result = re.ReplaceAllString(result, "")
	}

	result = reStickerPattern.ReplaceAllString(result, "")

	return strings.TrimSpace(result)
}

func stripMarkdownThinkSections(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inThinkSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if reMarkdownThinkHeader.MatchString(trimmed) {
			inThinkSection = true
			continue
		}
		if inThinkSection {
			if strings.HasPrefix(trimmed, "#") || trimmed == "" {
				inThinkSection = false
				result = append(result, line)
			}
			continue
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

func stripResponsePrefix(content string) string {
	reResponse := regexp.MustCompile(`(?i)^\s*response\s*[:：]?\s*`)
	return reResponse.ReplaceAllString(content, "")
}

func extractDirectReply(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}

	reQuoted := regexp.MustCompile(`[""](.+?)[""]`)
	quoteMatches := reQuoted.FindAllStringSubmatch(trimmed, -1)
	if len(quoteMatches) > 0 {
		var parts []string
		for _, m := range quoteMatches {
			p := strings.TrimSpace(m[1])
			if len([]rune(p)) >= 2 {
				parts = append(parts, p)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	}

	paragraphs := splitParagraphs(trimmed)
	if len(paragraphs) >= 2 {
		last := paragraphs[len(paragraphs)-1]
		first := paragraphs[0]
		if len([]rune(last)) <= 80 && len([]rune(first)) > len([]rune(last))*2 {
			return last
		}
	}

	hasAnyMeta := false
	for _, marker := range metaSentenceMarkers {
		if strings.Contains(trimmed, marker) {
			hasAnyMeta = true
			break
		}
	}

	if !hasAnyMeta {
		return trimmed
	}

	sentences := reSentenceEnd.Split(trimmed, -1)
	cleaned := filterMetaFromSentences(sentences)
	if cleaned != "" {
		return cleaned
	}


	commaParts := strings.Split(trimmed, "，")
	cleaned = filterMetaFromSentences(commaParts)
	if cleaned != "" {
		return cleaned
	}

	colonParts := strings.SplitN(trimmed, "：", 2)
	if len(colonParts) == 2 {
		after := strings.TrimSpace(colonParts[1])
		if len([]rune(after)) >= 2 {
			return after
		}
	}

	colonParts = strings.SplitN(trimmed, ":", 2)
	if len(colonParts) == 2 {
		after := strings.TrimSpace(colonParts[1])
		if len([]rune(after)) >= 2 {
			return after
		}
	}

	return trimmed
}

func stripJSONWrap(content string) string {
	result := reJSONWrap.ReplaceAllString(content, "")
	return strings.TrimSpace(result)
}

func stripHTMLTags(content string) string {
	result := reHTMLAnyTag.ReplaceAllString(content, "")
	return strings.TrimSpace(result)
}

func stripMarkdownFormatting(content string) string {
	result := content

	result = reAsterisk.ReplaceAllString(result, "")

	reCodeBlock := regexp.MustCompile("(?s)```[\\s\\S]*?```")
	result = reCodeBlock.ReplaceAllString(result, "")

	lines := strings.Split(result, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		trimmed = strings.TrimLeft(trimmed, "#-* \t")
		if strings.HasPrefix(trimmed, "```") {
			continue
		}
		cleanLines = append(cleanLines, trimmed)
	}
	return strings.Join(cleanLines, "\n")
}

func stripRoleNamePrefix(content, charName string) string {
	if charName == "" {
		return content
	}

	lowered := strings.ToLower(content)

	prefixes := [][2]string{
		{strings.ToLower(charName + "："), charName + "："},
		{strings.ToLower(charName + ":"), charName + ":"},
	}

	for _, p := range prefixes {
		if strings.HasPrefix(lowered, p[0]) {
			return strings.TrimSpace(content[len(p[1]):])
		}
	}

	return content
}

func stripLineDuplicates(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	var prev string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == prev {
			continue
		}
		prev = trimmed
		result = append(result, trimmed)
	}
	return strings.Join(result, "\n")
}

func stripRepeatPhrases(content string) string {
	trimmed := strings.TrimSpace(content)
	for _, phrase := range antiRepeatPhrases {
		if trimmed == phrase {
			return ""
		}
	}
	return trimmed
}

func trimToSentenceLimit(content string, maxSentences, maxChars int) string {
	if content == "" {
		return ""
	}

	runes := []rune(content)

	if len(runes) <= maxChars {
		sentenceCount := countSentences(content)
		if sentenceCount <= maxSentences {
			return content
		}
		return truncateSentences(content, maxSentences)
	}

	searchEnd := len(runes)
	if searchEnd > maxChars {
		searchEnd = maxChars
	}

	cutPoint := -1
	for i := searchEnd - 1; i >= maxChars/3; i-- {
		r := runes[i]
		if r == '。' || r == '！' || r == '？' || r == '!' || r == '?' || r == '\n' {
			cutPoint = i
			break
		}
	}

	if cutPoint > maxChars/3 {
		result := string(runes[:cutPoint+1])
		return truncateSentences(result, maxSentences)
	}
	return truncateSentences(string(runes[:maxChars]), maxSentences)
}

func stripPriorRepeats(content string, priorReplies []string) string {
	if len(priorReplies) == 0 {
		return content
	}

	contentWords := extractNonStopWords(content)

	recent := priorReplies
	if len(recent) > 5 {
		recent = recent[len(recent)-5:]
	}

	for _, prior := range recent {
		priorWords := extractNonStopWords(prior)
		if len(priorWords) == 0 || len(contentWords) == 0 {
			continue
		}
		matchCount := 0
		for _, cw := range contentWords {
			for _, pw := range priorWords {
				if cw == pw {
					matchCount++
					break
				}
			}
		}
		overlapRatio := float64(matchCount) / float64(len(contentWords))
		if overlapRatio > 0.7 && len([]rune(content)) > 4 {
			return ""
		}
	}

	return content
}

var stopWords = map[string]bool{
	"的": true, "了": true, "是": true, "在": true, "我": true,
	"你": true, "他": true, "她": true, "它": true, "们": true,
	"这": true, "那": true, "和": true, "与": true, "或": true,
	"也": true, "就": true, "都": true, "还": true, "要": true,
	"会": true, "能": true, "吗": true, "呢": true, "吧": true,
	"啊": true, "嗯": true, "哦": true, "哈": true, "呀": true,
	"不": true, "很": true, "好": true, "没": true, "有": true,
	"说": true, "看": true, "想": true, "让": true, "给": true,
	"对": true, "从": true, "用": true, "为": true, "把": true,
	"被": true, "上": true, "下": true, "里": true, "中": true,
}

func extractNonStopWords(text string) []string {
	var words []string
	runes := []rune(text)
	var current []rune
	for _, r := range runes {
		if r == ' ' || r == '，' || r == '。' || r == '！' || r == '？' || r == '\n' || r == '?' || r == '!' || r == ',' {
			if len(current) > 0 {
				w := string(current)
				if len([]rune(w)) >= 2 && !stopWords[w] {
					words = append(words, w)
				}
				current = nil
			}
			continue
		}
		current = append(current, r)
	}
	if len(current) > 0 {
		w := string(current)
		if len([]rune(w)) >= 2 && !stopWords[w] {
			words = append(words, w)
		}
	}
	return words
}

func splitParagraphs(text string) []string {
	parts := regexp.MustCompile(`\n\s*\n`).Split(text, -1)
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func filterMetaFromSentences(sentences []string) string {
	var filtered []string
	for _, s := range sentences {
		trimS := strings.TrimSpace(s)
		if trimS == "" {
			continue
		}
		hasMeta := false
		for _, marker := range metaSentenceMarkers {
			if strings.Contains(trimS, marker) {
				hasMeta = true
				break
			}
		}
		if !hasMeta {
			filtered = append(filtered, trimS)
		}
	}
	if len(filtered) == 0 {
		return ""
	}
	return strings.Join(filtered, "\n")
}

func countSentences(content string) int {
	parts := reSentenceEnd.Split(content, -1)
	n := 0
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			n++
		}
	}
	return n
}

func truncateSentences(content string, maxSentences int) string {
	runes := []rune(content)
	count := 0
	lastEnd := 0
	inMidSentence := false
	for i, r := range runes {
		if r == '。' || r == '！' || r == '？' || r == '!' || r == '?' {
			count++
			lastEnd = i + 1
			inMidSentence = false
			if count >= maxSentences {
				break
			}
		} else if r == '，' || r == '、' || r == '；' || r == ',' || r == ';' {
			inMidSentence = true
		} else {
			if !inMidSentence && i > 0 {
				inMidSentence = true
			}
		}
	}
	if count >= maxSentences && lastEnd > 0 {
		return strings.TrimSpace(string(runes[:lastEnd]))
	}
	return content
}

func stripAntiRepeatPhrases(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		isWeak := false
		for _, phrase := range antiRepeatWeakPhrases {
			if trimmed == phrase {
				isWeak = true
				break
			}
		}
		if isWeak && len(result) == 0 {
			continue
		}
		result = append(result, trimmed)
	}
	return strings.Join(result, "\n")
}

func DeduplicateAdjacentLines(lines []string) []string {
	if len(lines) <= 1 {
		return lines
	}

	var result []string
	result = append(result, lines[0])

	for i := 1; i < len(lines); i++ {
		prev := lines[i-1]
		curr := lines[i]

		if strings.TrimSpace(curr) == "" || strings.TrimSpace(prev) == "" {
			result = append(result, curr)
			continue
		}

		prevWords := extractNonStopWords(prev)
		currWords := extractNonStopWords(curr)

		if len(prevWords) == 0 || len(currWords) == 0 {
			result = append(result, curr)
			continue
		}

		matchCount := 0
		for _, cw := range currWords {
			for _, pw := range prevWords {
				if cw == pw {
					matchCount++
					break
				}
			}
		}

		overlapRatio := float64(matchCount) / float64(len(currWords))
		if overlapRatio > 0.5 {
			continue
		}

		for _, phrase := range antiRepeatPhrases {
			if strings.TrimSpace(curr) == phrase {
				goto skip
			}
		}

		result = append(result, curr)
	skip:
	}

	return result
}

func CollapseAdjacentSemanticDuplicates(raw string, priorReplies []string) string {
	if raw == "" {
		return ""
	}

	trimmed := strings.TrimSpace(raw)
	for _, phrase := range antiRepeatPhrases {
		if trimmed == phrase {
			return ""
		}
	}

	sentences := strings.Split(trimmed, "\n")
	if len(sentences) <= 1 {
		return trimmed
	}

	var clean []string
	var prev string
	for _, s := range sentences {
		sTrim := strings.TrimSpace(s)
		if sTrim == "" {
			clean = append(clean, s)
			continue
		}

		isAntiRepeat := false
		for _, phrase := range antiRepeatPhrases {
			if sTrim == phrase {
				isAntiRepeat = true
				break
			}
		}

		if prev != "" && isAntiRepeat {
			prevIsAnti := false
			for _, phrase := range antiRepeatPhrases {
				if prev == phrase {
					prevIsAnti = true
					break
				}
			}
			if prevIsAnti {
				continue
			}
		}

		if prev != "" {
			prevWords := extractNonStopWords(prev)
			currWords := extractNonStopWords(sTrim)
			if len(prevWords) > 0 && len(currWords) > 0 {
				matchCount := 0
				for _, cw := range currWords {
					for _, pw := range prevWords {
						if cw == pw {
							matchCount++
							break
						}
					}
				}
				overlapRatio := float64(matchCount) / float64(len(currWords))
				if overlapRatio > 0.7 {
					continue
				}
			}
		}

		clean = append(clean, s)
		prev = sTrim
	}

	result := strings.Join(clean, "\n")
	result = strings.TrimSpace(result)
	if result == "" {
		return ""
	}

	result = stripPriorRepeats(result, priorReplies)

	return result
}









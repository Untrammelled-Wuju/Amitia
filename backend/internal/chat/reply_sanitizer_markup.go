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

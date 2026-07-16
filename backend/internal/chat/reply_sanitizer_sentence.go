package chat

import (
	"regexp"
	"strings"
)

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

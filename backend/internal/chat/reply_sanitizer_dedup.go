package chat

import "strings"

var antiRepeatPhrases = []string{
	"在", "在呢", "在的", "我在", "嗯我在", "嗯嗯在",
}

var antiRepeatWeakPhrases = []string{
	"嗯", "嗯嗯", "好", "好的", "好吧", "行", "行吧", "知道了", "哦", "噢",
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

package util

import (
	"strings"
	"unicode/utf8"
)

const MaxWechatMessageLen = 2000
const MaxQQMessageLen = 2000
const MaxWebMessageLen = 2000

func SplitLongMessage(text string, maxLen int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	lines := strings.Split(text, "\n")
	var chunks []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sentences := splitByPunctuation(line)
		for _, s := range sentences {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			chunks = append(chunks, cutToMaxLen(s, maxLen)...)
		}
	}

	return chunks
}

func isChinesePunctuation(r rune) bool {
	return r == '。' || r == '？' || r == '！'
}

func splitByPunctuation(line string) []string {
	runes := []rune(line)
	var result []string
	start := 0

	for i := 0; i < len(runes); i++ {
		if isChinesePunctuation(runes[i]) {
			j := i + 1
			for j < len(runes) && isChinesePunctuation(runes[j]) {
				j++
			}
			result = append(result, string(runes[start:j]))
			start = j
			i = j - 1
		}
	}

	if start < len(runes) {
		result = append(result, string(runes[start:]))
	}

	return result
}

func cutToMaxLen(s string, maxLen int) []string {
	if utf8.RuneCountInString(s) <= maxLen {
		return []string{s}
	}

	runes := []rune(s)
	var result []string
	i := 0

	for i < len(runes) {
		end := i + maxLen
		if end > len(runes) {
			end = len(runes)
		}
		for end < len(runes) && isChinesePunctuation(runes[end]) {
			end++
		}
		result = append(result, string(runes[i:end]))
		i = end
	}

	return result
}

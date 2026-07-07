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
		if utf8.RuneCountInString(line) <= maxLen {
			chunks = append(chunks, line)
			continue
		}
		runes := []rune(line)
		for i := 0; i < len(runes); i += maxLen {
			end := i + maxLen
			if end > len(runes) {
				end = len(runes)
			}
			chunks = append(chunks, string(runes[i:end]))
		}
	}

	return chunks
}

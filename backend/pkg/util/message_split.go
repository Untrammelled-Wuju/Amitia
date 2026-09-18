package util

import (
	"strings"
)

const MaxWebMessageLen = 2000
const AmitiaMessageBreak = "[AMITIA_BR]"

func SplitLongMessage(text string, maxLen int) []string {
	return SplitMessageSegments(text)
}

func SplitMessageSegments(content string) []string {
	parts := strings.Split(content, AmitiaMessageBreak)
	if len(parts) > 1 {
		return parts
	}
	return []string{content}
}

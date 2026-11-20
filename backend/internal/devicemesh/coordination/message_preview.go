package coordination

import "strings"

func MessagePreview(content string) string {
	value := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	runes := []rune(value)
	if len(runes) > 120 {
		return string(runes[:120]) + "…"
	}
	return value
}

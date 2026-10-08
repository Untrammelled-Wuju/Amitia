package notificationruntime

import (
	"regexp"
	"strings"
)

var fencedCodePattern = regexp.MustCompile("(?s)\x60\x60\x60.*?\x60\x60\x60")
var markdownImagePattern = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
var markdownLinkPattern = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
var htmlPattern = regexp.MustCompile(`(?s)<[^>]+>`)
var urlPattern = regexp.MustCompile(`https?://[^\s)]+`)
var markdownMarkerPattern = regexp.MustCompile(`[*_~#>]+`)
var whitespacePattern = regexp.MustCompile(`\s+`)

func BuildPreview(content string, limit int) string {
	value := fencedCodePattern.ReplaceAllString(content, " ")
	value = markdownImagePattern.ReplaceAllString(value, " ")
	value = markdownLinkPattern.ReplaceAllString(value, "$1")
	value = htmlPattern.ReplaceAllString(value, " ")
	value = urlPattern.ReplaceAllString(value, " ")
	value = markdownMarkerPattern.ReplaceAllString(value, "")
	value = strings.TrimSpace(whitespacePattern.ReplaceAllString(value, " "))
	if limit <= 0 {
		limit = 160
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit == 1 {
		return string(runes[:1])
	}
	return strings.TrimSpace(string(runes[:limit-1])) + "…"
}

func PreviewForMode(mode, sender, body string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "hidden":
		return "Amitia", "你有一条新消息"
	case "sender_only":
		if strings.TrimSpace(sender) == "" {
			sender = "Amitia"
		}
		return sender, "发来了一条消息"
	default:
		if strings.TrimSpace(sender) == "" {
			sender = "Amitia"
		}
		return truncateRunes(sender, 40), BuildPreview(body, 160)
	}
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

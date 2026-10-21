package worldbook

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

type WorldBookEntry struct {
	ID            string   `json:"id"`
	MatchType     string   `json:"matchType"`
	MatchPattern  string   `json:"matchPattern"`
	MatchScope    string   `json:"matchScope"`
	InjectContent string   `json:"injectContent"`
	Priority      int      `json:"priority"`
	CharacterID   string   `json:"characterId,omitempty"`
	SecondaryKeys []string `json:"secondaryKeys,omitempty"`
	CaseSensitive *bool    `json:"caseSensitive,omitempty"`
	Selective     *bool    `json:"selective,omitempty"`
	Constant      *bool    `json:"constant,omitempty"`
}

type MatchResult struct {
	Entry      WorldBookEntry `json:"entry"`
	MatchScope string         `json:"matchScope"`
	HitText    string         `json:"hitText"`
}

func MatchRules(rules []WorldBookEntry, userMessage, assistantReply string) []MatchResult {
	results := make([]MatchResult, 0)
	for _, rule := range rules {
		if matched, hitText := matchRule(rule, userMessage, assistantReply); matched {
			results = append(results, MatchResult{Entry: rule, MatchScope: rule.MatchScope, HitText: hitText})
		}
	}
	return results
}

func matchRule(rule WorldBookEntry, userMessage, assistantReply string) (bool, string) {
	fullText := userMessage
	if assistantReply != "" {
		fullText = userMessage + "\n" + assistantReply
	}
	scopedText := getScopedText(userMessage, assistantReply, fullText, rule.MatchScope)
	matched, hitText := matchPrimary(rule, scopedText)
	if !matched {
		return false, ""
	}
	if rule.Selective != nil && *rule.Selective && len(rule.SecondaryKeys) > 0 {
		secondaryMatched := false
		for _, keyword := range rule.SecondaryKeys {
			keyword = strings.TrimSpace(keyword)
			if keyword == "" {
				continue
			}
			if strings.Contains(textValue(scopedText, rule.CaseSensitive), textValue(keyword, rule.CaseSensitive)) {
				secondaryMatched = true
				break
			}
		}
		if !secondaryMatched {
			return false, ""
		}
	}
	return true, hitText
}

func matchPrimary(rule WorldBookEntry, scopedText string) (bool, string) {
	switch rule.MatchType {
	case "regex":
		re, err := regexp.Compile(rule.MatchPattern)
		if err != nil {
			return false, ""
		}
		loc := re.FindStringIndex(scopedText)
		if loc == nil {
			return false, ""
		}
		start, runes := loc[0], 0
		for start > 0 && runes < 30 {
			_, size := utf8.DecodeLastRuneInString(scopedText[:start])
			if size <= 0 {
				break
			}
			start -= size
			runes++
		}
		end, runes := loc[1], 0
		for end < len(scopedText) && runes < 30 {
			_, size := utf8.DecodeRuneInString(scopedText[end:])
			if size <= 0 {
				break
			}
			end += size
			runes++
		}
		return true, scopedText[start:end]
	case "exact":
		if strings.Contains(textValue(scopedText, rule.CaseSensitive), textValue(rule.MatchPattern, rule.CaseSensitive)) {
			return true, rule.MatchPattern
		}
	default:
		for _, keyword := range strings.Split(rule.MatchPattern, ",") {
			keyword = strings.TrimSpace(keyword)
			if keyword != "" && strings.Contains(textValue(scopedText, rule.CaseSensitive), textValue(keyword, rule.CaseSensitive)) {
				return true, keyword
			}
		}
	}
	return false, ""
}

func textValue(value string, caseSensitive *bool) string {
	if caseSensitive != nil && *caseSensitive {
		return value
	}
	return strings.ToLower(value)
}

func getScopedText(userMessage, assistantReply, fullText, scope string) string {
	switch scope {
	case "user_message":
		return userMessage
	case "assistant_reply":
		if assistantReply != "" {
			return assistantReply
		}
		return userMessage
	default:
		return fullText
	}
}

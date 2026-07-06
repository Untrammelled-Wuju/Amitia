package prompt

import (
	"regexp"
	"strings"
)

var injectionPatterns = []struct {
	Pattern *regexp.Regexp
	Label   string
}{
	{regexp.MustCompile(`(?i)ignore\s+(all\s+)?(previous|prior|above|foregoing)\s+(instructions?|directives?|commands?|prompts?|context)`), "ignore_instructions"},
	{regexp.MustCompile(`(?i)you\s+are\s+(now\s+)?(a\s+|an\s+)?(different|new)\s+(ai|assistant|model|role|persona|character)`), "role_confusion"},
	{regexp.MustCompile(`(?i)disregard\s+(all\s+)?(prior|previous)\s+(instructions?|rules?|constraints?)`), "disregard_instructions"},
	{regexp.MustCompile(`(?i)new\s+(instructions?|directives?|system\s+prompts?)\s+(follow|override|replace)`), "new_instructions"},
	{regexp.MustCompile(`(?i)^(system|assistant|user)\s*:`), "role_spoofing"},
	{regexp.MustCompile(`(?i)<\|im_start\|>|<\|im_end\|>|<\|system\|>|<\|user\|>|<\|assistant\|>`), "delimiter_injection"},
	{regexp.MustCompile(`(?i)^(\[system\]|\[assistant\]|\[user\])\s*$`), "system_tag_injection"},
	{regexp.MustCompile(`(?i)^(\[system\]|\[assistant\]|\[user\])\s*\n`), "system_tag_injection"},
	{regexp.MustCompile(`(?i)pretend\s+(you\s+are|to\s+be)\s+(a|an|the)`), "pretend_role"},
	{regexp.MustCompile(`(?i)forget\s+(everything|all)\s+(you\s+know|about|above)`), "forget_everything"},
	{regexp.MustCompile(`(?i)you\s+must\s+(not\s+)?(follow|obey)\s+your\s+(system\s+)?(prompt|instructions|rules)`), "system_override"},
	{regexp.MustCompile(`(?i)priority\s+(override|shift|change|swap)`), "priority_override"},
	{regexp.MustCompile("(?i)act\\s+as\\s+if\\s+(your|the)\\s+(system\\s+)?(prompt|instructions|rules)\\s+(is|are|were|don['’]t\\s+(exist|apply))"), "act_as_if"},
	{regexp.MustCompile(`.*忽略之前.*`), "ignore_previous_cn"},
	{regexp.MustCompile(`.*忽略前面.*`), "ignore_previous_cn"},
	{regexp.MustCompile(`.*无视以上.*`), "ignore_previous_cn"},
	{regexp.MustCompile(`.*系统提示词.*`), "system_prompt_leak_cn"},
	{regexp.MustCompile(`.*开发者提示词.*`), "system_prompt_leak_cn"},
	{regexp.MustCompile(`.*隐藏规则.*`), "system_prompt_leak_cn"},
	{regexp.MustCompile(`.*你现在是\s*system.*`), "role_confusion_cn"},
	{regexp.MustCompile(`.*你现在是\s*系统.*`), "role_confusion_cn"},
	{regexp.MustCompile(`.*你现在是\s*developer.*`), "role_confusion_cn"},
}

type SanitizeResult struct {
	Content     string
	Clean       bool
	Flags       []string
	TruncatedAt int
}

func SanitizeContent(content string, sensitivity SensitivityLevel) SanitizeResult {
	if sensitivity == SensitivitySecret {
		return SanitizeResult{Content: "[redacted]", Clean: false, Flags: detectInjections(content)}
	}

	if strings.TrimSpace(content) == "" {
		return SanitizeResult{Content: content, Clean: true}
	}

	original := content
	maxLen := maxContentLength(sensitivity)
	var truncatedAt int
	if maxLen > 0 && len(content) > maxLen {
		content = content[:maxLen]
		truncatedAt = maxLen
	}

	content = stripNullBytes(content)
	flags := detectInjections(content)

	if len(flags) == 0 {
		return SanitizeResult{Content: content, Clean: true, TruncatedAt: truncatedAt}
	}

	if sensitivity == SensitivityPrivate {
		return SanitizeResult{Content: "[redacted]", Clean: false, Flags: flags, TruncatedAt: truncatedAt}
	}

	cleaned := redactInjectionSpans(content, flags)
	if cleaned == "" {
		cleaned = strings.TrimSpace(original)
		for _, pattern := range injectionPatterns {
			cleaned = pattern.Pattern.ReplaceAllString(cleaned, "[filtered]")
		}
	}

	return SanitizeResult{Content: cleaned, Clean: false, Flags: flags, TruncatedAt: truncatedAt}
}

func HasInjectionSignatures(content string) bool {
	return len(detectInjections(content)) > 0
}

func ValidateSectionContent(section Section) []string {
	if section.DataOnly {
		return detectInjections(section.Content)
	}
	flags := []string{}
	if section.Sensitivity == SensitivityPrivate || section.Sensitivity == SensitivitySecret {
		if HasInjectionSignatures(section.Content) {
			flags = append(flags, "injection_in_sensitive_section")
		}
	}
	return flags
}

func detectInjections(content string) []string {
	var flags []string
	for _, entry := range injectionPatterns {
		if entry.Pattern.MatchString(content) {
			flags = append(flags, entry.Label)
		}
	}
	return uniqueStrings(flags)
}

func redactInjectionSpans(content string, flags []string) string {
	if len(flags) == 0 {
		return content
	}
	result := content
	for _, entry := range injectionPatterns {
		if entry.Pattern.MatchString(result) {
			result = entry.Pattern.ReplaceAllString(result, "[filtered]")
		}
	}
	return strings.TrimSpace(result)
}

func stripNullBytes(content string) string {
	return strings.Map(func(r rune) rune {
		if r == 0 {
			return -1
		}
		return r
	}, content)
}

func maxContentLength(sensitivity SensitivityLevel) int {
	switch sensitivity {
	case SensitivitySecret:
		return 0
	case SensitivityPrivate:
		return 4096
	default:
		return 16384
	}
}

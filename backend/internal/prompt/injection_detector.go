package prompt

import (
	"regexp"
	"strings"
)

var gwInjectionPatterns = []string{
	"忽略之前",
	"忽略前面",
	"无视以上",
	"系统提示词",
	"开发者提示词",
	"隐藏规则",
	"你现在是system",
	"你现在是系统",
	"你现在是developer",
	"reveal your prompt",
	"ignore previous instructions",
	"system prompt",
	"developer message",
	"override previous",
}

var gwInjectionRegexps = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+(all\s+)?(previous|prior|above|foregoing)\s+(instructions?|directives?|commands?|prompts?|context)`),
	regexp.MustCompile(`(?i)you\s+are\s+(now\s+)?(a\s+|an\s+)?(different|new)\s+(ai|assistant|model|role|persona|character)`),
	regexp.MustCompile(`(?i)disregard\s+(all\s+)?(prior|previous)\s+(instructions?|rules?|constraints?)`),
	regexp.MustCompile(`(?i)new\s+(instructions?|directives?|system\s+prompts?)\s+(follow|override|replace)`),
	regexp.MustCompile(`(?i)<\|im_start\|>|<\|im_end\|>|<\|system\|>|<\|user\|>|<\|assistant\|>`),
	regexp.MustCompile(`(?i)pretend\s+(you\s+are|to\s+be)\s+(a|an|the)`),
	regexp.MustCompile(`(?i)forget\s+(everything|all)\s+(you\s+know|about|above)`),
	regexp.MustCompile(`(?i)you\s+must\s+(not\s+)?(follow|obey)\s+your\s+(system\s+)?(prompt|instructions|rules)`),
	regexp.MustCompile(`(?i)priority\s+(override|shift|change|swap)`),
	regexp.MustCompile(`(?i)act\s+as\s+if\s+(your|the)\s+(system\s+)?(prompt|instructions|rules)\s+(is|are|were|don['']t\s+(exist|apply))`),
}

func DetectInjectionRisk(text string) bool {
	lower := strings.ToLower(text)
	for _, p := range gwInjectionPatterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	for _, re := range gwInjectionRegexps {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

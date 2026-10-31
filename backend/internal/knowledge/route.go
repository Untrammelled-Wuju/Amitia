package knowledge

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/u-ai/backend/config"
)

const (
	RoutingModeChat       = "chat"
	RoutingModeDiagnostic = "diagnostic"

	PolicyModeActive        = "active"
	PolicyModeActiveShadow  = "active_shadow"
	PolicyModeLegacy        = "legacy"
	PolicyModeLegacyShadow  = "legacy_shadow"
	PolicyModeDiagnostic    = "diagnostic"
	RouteReasonChatIntent   = "chat_document_intent"
	RouteReasonChatNoIntent = "chat_no_document_intent"
	RouteReasonExplicit     = "explicit_level"
	RouteReasonAutomatic    = "automatic_level"
)

var pageLocatorPattern = regexp.MustCompile(`第\s*[0-9]{1,7}\s*页|(?i:page)\s*#?\s*[0-9]{1,7}`)
var rowLocatorPattern = regexp.MustCompile(`第\s*[0-9]{1,9}\s*行|(?i:row)\s*#?\s*[0-9]{1,9}`)

type RouteDecision struct {
	RequestedLevel         string
	EffectiveLevel         string
	RoutingMode            string
	PolicyMode             string
	RouteReason            string
	ShadowLevel            string
	ShadowReason           string
	Degraded               bool
	RouteDegradationReason string
}

func NormalizeRoutingMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case RoutingModeChat:
		return RoutingModeChat
	case RoutingModeDiagnostic:
		return RoutingModeDiagnostic
	default:
		return RoutingModeDiagnostic
	}
}

func ResolveChatRoute(text string, features config.KnowledgeFeatureConfig) RouteDecision {
	activeLevel := classifyKnowledgeIntent(text)
	decision := RouteDecision{
		RequestedLevel: RetrievalLevelAuto,
		EffectiveLevel: activeLevel,
		RoutingMode:    RoutingModeChat,
		PolicyMode:     PolicyModeActive,
		RouteReason:    RouteReasonChatIntent,
	}
	if activeLevel == "" {
		decision.EffectiveLevel = RetrievalLevelNone
		decision.RouteReason = RouteReasonChatNoIntent
	}
	if !features.RetrievalPolicyEnabled {
		legacyLevel := RetrievalLevelNone
		if utf8.RuneCountInString(strings.TrimSpace(text)) >= 2 && utf8.RuneCountInString(strings.TrimSpace(text)) <= 4096 {
			legacyLevel = RetrievalLevelFast
		}
		decision.EffectiveLevel = legacyLevel
		decision.PolicyMode = PolicyModeLegacy
		decision.RouteReason = RouteReasonChatIntent
		if features.RetrievalPolicyShadow {
			decision.PolicyMode = PolicyModeLegacyShadow
			if activeLevel == "" {
				activeLevel = RetrievalLevelNone
			}
			decision.ShadowLevel = activeLevel
			decision.ShadowReason = "new_policy_candidate"
		}
	} else if features.RetrievalPolicyShadow {
		decision.PolicyMode = PolicyModeActiveShadow
		decision.ShadowLevel = RetrievalLevelFast
		decision.ShadowReason = "legacy_candidate"
	}
	return applyFeatureGates(decision, features)
}

func ResolveDiagnosticRoute(requestedLevel, plannedLevel string, features config.KnowledgeFeatureConfig) RouteDecision {
	requested := strings.ToLower(strings.TrimSpace(requestedLevel))
	if requested == "" {
		requested = RetrievalLevelAuto
	}
	reason := RouteReasonExplicit
	if requested == RetrievalLevelAuto {
		reason = RouteReasonAutomatic
	}
	decision := RouteDecision{
		RequestedLevel: requested,
		EffectiveLevel: plannedLevel,
		RoutingMode:    RoutingModeDiagnostic,
		PolicyMode:     PolicyModeDiagnostic,
		RouteReason:    reason,
	}
	if !features.RetrievalPolicyEnabled {
		decision.PolicyMode = PolicyModeLegacy
	}
	return applyFeatureGates(decision, features)
}

func applyFeatureGates(decision RouteDecision, features config.KnowledgeFeatureConfig) RouteDecision {
	switch decision.EffectiveLevel {
	case RetrievalLevelAgentic:
		if !features.AgenticInvestigationEnabled {
			decision.EffectiveLevel = RetrievalLevelDeep
			decision.Degraded = true
			decision.RouteDegradationReason = "agentic_investigation_disabled"
		} else if !features.CanonicalReaderEnabled {
			decision.EffectiveLevel = RetrievalLevelDeep
			decision.Degraded = true
			decision.RouteDegradationReason = "canonical_reader_disabled"
		}
	}
	if decision.EffectiveLevel == RetrievalLevelDeep && !features.RetrievalPolicyEnabled {
		decision.EffectiveLevel = RetrievalLevelFast
		decision.Degraded = true
		decision.RouteDegradationReason = "retrieval_policy_disabled"
	}
	return decision
}

func NeedsCanonicalRead(text string) bool {
	q := strings.ToLower(text)
	return strings.Contains(q, "原文") || strings.Contains(q, "核对") || strings.Contains(q, "审计") || strings.Contains(q, "逐页") || strings.Contains(q, "逐段") || pageLocatorPattern.MatchString(q) || rowLocatorPattern.MatchString(q)
}

func ExplicitLocator(query string) map[string]any {
	for _, match := range []struct {
		re  *regexp.Regexp
		key string
	}{{pageLocatorPattern, "page"}, {rowLocatorPattern, "row"}} {
		value := match.re.FindString(query)
		if value == "" {
			continue
		}
		digits := regexp.MustCompile(`[0-9]+`).FindString(value)
		var number int
		if _, err := fmt.Sscanf(digits, "%d", &number); err == nil && number > 0 {
			return map[string]any{match.key: number}
		}
	}
	return nil
}

func classifyKnowledgeIntent(text string) string {
	q := strings.ToLower(strings.TrimSpace(text))
	if q == "" || utf8.RuneCountInString(q) > 4096 {
		return ""
	}
	social := []string{"早安", "早上好", "晚安", "哈哈", "你好", "陪我", "想你", "我累了", "谢谢", "辛苦了", "good morning", "hello", "hey", "good night"}
	for _, item := range social {
		if q == item || strings.HasPrefix(q, item+"！") {
			return ""
		}
	}
	hints := []string{"知识库", "文档", "资料", "文件", "pdf", "excel", "xlsx", "docx", "原文", "出处", "检索", "搜索资料", "source document", "knowledge base"}
	found := false
	for _, item := range hints {
		if strings.Contains(q, item) {
			found = true
			break
		}
	}
	locator := pageLocatorPattern.MatchString(q) || rowLocatorPattern.MatchString(q)
	if !found && !locator {
		return ""
	}
	if strings.Contains(q, "原文") || strings.Contains(q, "逐页") || strings.Contains(q, "逐段") || strings.Contains(q, "核对") || strings.Contains(q, "审计") || locator {
		return RetrievalLevelAgentic
	}
	complex := []string{"对比", "比较", "矛盾", "冲突", "多份", "逐项", "全文", "交叉验证", "compare", "verify", "investigate"}
	for _, item := range complex {
		if strings.Contains(q, item) {
			return RetrievalLevelDeep
		}
	}
	return RetrievalLevelFast
}

package safety

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/decision"
)

var ErrBdiConfigInvalid = errors.New("bdi 配置无效")

const (
	ConstraintSeverityBlock    = "block"
	ConstraintSeverityOverride = "override"

	EmotionDisplayShow     = "show"
	EmotionDisplaySuppress = "suppress"
	EmotionDisplayReframe  = "reframe"
)

type BdiRuntime struct {
	Revision        int64
	Filter          decision.HardConstraintFilterConfig
	OverrideIDs     []string
	ForbiddenTopics []string
	SoftPreferences map[string]float64
	Coping          decision.CopingStrategy
	Emotion         EmotionDirective
	Warnings        []string
}

type EmotionDirective struct {
	Mode              string
	InternalIntensity float64
	DisplayIntensity  float64
	NegativeCap       float64
	IntimacyCap       float64
	OverrideReason    string
}

type BdiOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Group string `json:"group,omitempty"`
}

type BdiLimits struct {
	IntensityMin int     `json:"intensityMin"`
	IntensityMax int     `json:"intensityMax"`
	RawScoreMin  int     `json:"rawScoreMin"`
	RawScoreMax  int     `json:"rawScoreMax"`
	WeightMin    float64 `json:"weightMin"`
	WeightMax    float64 `json:"weightMax"`
}

type BdiOptions struct {
	Candidates       []BdiOption `json:"candidates"`
	Dimensions       []BdiOption `json:"dimensions"`
	Severities       []BdiOption `json:"severities"`
	CopingStrategies []BdiOption `json:"copingStrategies"`
	DisplayModes     []BdiOption `json:"displayModes"`
	Limits           BdiLimits   `json:"limits"`
}

func BuildBdiRuntime(policy RuntimePolicy) BdiRuntime {
	runtime := BdiRuntime{
		Revision: policy.BDI.Revision,
		Filter:   decision.DefaultHardConstraintFilterConfig(),
	}
	if runtime.Filter.CooldownBlocks == nil {
		runtime.Filter.CooldownBlocks = map[string]time.Duration{}
	}

	blocked := make([]string, 0)
	seenBlocked := make(map[string]bool)
	topics := make([]string, 0)
	seenTopics := make(map[string]bool)
	for _, rule := range policy.BDI.HardConstraints {
		candidateID := strings.TrimSpace(rule.CandidateKey)
		if !decision.IsKnownCandidateID(candidateID) {
			runtime.Warnings = append(runtime.Warnings, "未知行为候选: "+candidateID)
			continue
		}
		severity := NormalizeConstraintSeverity(rule.Severity)
		if severity == ConstraintSeverityOverride {
			if !containsString(runtime.OverrideIDs, candidateID) {
				runtime.OverrideIDs = append(runtime.OverrideIDs, candidateID)
			}
		} else if !seenBlocked[candidateID] {
			seenBlocked[candidateID] = true
			blocked = append(blocked, candidateID)
		}
		topic := strings.TrimSpace(rule.Reason)
		if topic == "" {
			topic = strings.TrimSpace(rule.RuleID)
		}
		if topic != "" && !seenTopics[topic] {
			seenTopics[topic] = true
			topics = append(topics, topic)
		}
	}
	runtime.Filter.BlockedIDs = blocked
	runtime.ForbiddenTopics = topics

	if len(policy.BDI.SoftPreferences) > 0 {
		preferences := make(map[string]float64, len(policy.BDI.SoftPreferences))
		for _, preference := range policy.BDI.SoftPreferences {
			dimension := strings.TrimSpace(preference.Dimension)
			if dimension == "" {
				continue
			}
			if !decision.IsKnownPreferenceDimension(dimension) {
				runtime.Warnings = append(runtime.Warnings, "未知偏好维度: "+dimension)
				continue
			}
			if preference.RawScore == 0 && preference.NormalizedWeight == 0 {
				continue
			}
			value := 0.0
			if preference.NormalizedWeight != 0 {
				value = preference.NormalizedWeight
				if preference.RawScore != 0 {
					value *= clampUnit((preference.RawScore - 5) / 5)
				}
			} else {
				value = clampUnit((preference.RawScore - 5) / 5)
			}
			preferences[dimension] = clampSigned(value)
		}
		runtime.SoftPreferences = preferences
	}

	if policy.BDI.CopingStrategy != nil {
		if strategy, ok := NormalizeCopingStrategy(policy.BDI.CopingStrategy.Selected); ok {
			runtime.Coping = strategy
		} else if strings.TrimSpace(policy.BDI.CopingStrategy.Selected) != "" {
			runtime.Warnings = append(runtime.Warnings, "未知应对策略: "+policy.BDI.CopingStrategy.Selected)
		}
	}

	runtime.Emotion = buildEmotionDirective(policy)
	return runtime
}

func buildEmotionDirective(policy RuntimePolicy) EmotionDirective {
	directive := EmotionDirective{
		Mode:              EmotionDisplayShow,
		InternalIntensity: 0.5,
		DisplayIntensity:  1,
	}
	if policy.BDI.EmotionExpression != nil {
		expression := policy.BDI.EmotionExpression
		directive.Mode = NormalizeDisplayMode(expression.DisplayMode)
		directive.InternalIntensity = clampUnit(float64(expression.InternalIntensity) / 10)
		directive.DisplayIntensity = clampUnit(float64(expression.DisplayIntensity) / 10)
		directive.OverrideReason = strings.TrimSpace(expression.OverrideReason)
	}
	directive.NegativeCap = clampUnit(float64(policy.Config.NegativeEmotionCap) / 10)
	directive.IntimacyCap = clampUnit(float64(policy.Config.IntimacyExpressionCap) / 10)
	return directive
}

func NormalizeDisplayMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case EmotionDisplaySuppress, "hidden", "hide":
		return EmotionDisplaySuppress
	case EmotionDisplayReframe, "reframing":
		return EmotionDisplayReframe
	default:
		return EmotionDisplayShow
	}
}

func NormalizeConstraintSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ConstraintSeverityOverride, "prefer", "priority":
		return ConstraintSeverityOverride
	default:
		return ConstraintSeverityBlock
	}
}

func NormalizeCopingStrategy(value string) (decision.CopingStrategy, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(decision.CopingReappraisal), "reframe", "reframing", "cognitive_reframe":
		return decision.CopingReappraisal, true
	case string(decision.CopingSuppression), "suppress", "suppressing":
		return decision.CopingSuppression, true
	case string(decision.CopingSeekSupport), "support", "seek_help", "social_support":
		return decision.CopingSeekSupport, true
	case string(decision.CopingProblemSolving), "active", "problem_solve", "problem-solving":
		return decision.CopingProblemSolving, true
	case string(decision.CopingAcceptance), "accept", "accepting":
		return decision.CopingAcceptance, true
	case string(decision.CopingDistraction), "distract":
		return decision.CopingDistraction, true
	case string(decision.CopingRumination), "ruminate":
		return decision.CopingRumination, true
	case string(decision.CopingAvoidance), "avoid", "avoiding":
		return decision.CopingAvoidance, true
	default:
		return "", false
	}
}

func CopingStrategyLabel(strategy decision.CopingStrategy) string {
	switch strategy {
	case decision.CopingReappraisal:
		return "认知重构"
	case decision.CopingSuppression:
		return "情绪抑制"
	case decision.CopingSeekSupport:
		return "寻求支持"
	case decision.CopingProblemSolving:
		return "问题解决"
	case decision.CopingAcceptance:
		return "接纳"
	case decision.CopingDistraction:
		return "转移注意"
	case decision.CopingRumination:
		return "反复思虑"
	case decision.CopingAvoidance:
		return "回避"
	default:
		return ""
	}
}

func ApplyEmotionDirective(plan decision.ExpressionPlan, directive EmotionDirective) decision.ExpressionPlan {
	if plan.Metadata == nil {
		plan.Metadata = make(map[string]any)
	}
	display := plan.EmotionIntensity
	if directive.DisplayIntensity > 0 && directive.DisplayIntensity < display {
		display = directive.DisplayIntensity
	}
	switch directive.Mode {
	case EmotionDisplaySuppress:
		if display > directive.DisplayIntensity {
			display = directive.DisplayIntensity
		}
		plan.Suppressed = true
	case EmotionDisplayReframe:
		plan.Metadata["emotionExpressMode"] = EmotionDisplayReframe
	}
	if directive.DisplayIntensity == 0 {
		display = 0
		plan.Suppressed = true
	}
	if plan.Tone == decision.ExpressionToneConcerned && display > directive.NegativeCap {
		display = directive.NegativeCap
		plan.Metadata["negativeEmotionCapped"] = true
		if directive.NegativeCap == 0 {
			plan.Suppressed = true
		}
	}
	if (plan.Tone == decision.ExpressionToneWarm || plan.Tone == decision.ExpressionToneSoft) && display > directive.IntimacyCap {
		display = directive.IntimacyCap
		plan.Metadata["intimacyCapped"] = true
		if directive.IntimacyCap == 0 {
			plan.Suppressed = true
		}
	}
	plan.EmotionIntensity = clampUnit(display)
	plan.Metadata["emotionExpressMode"] = directive.Mode
	plan.Metadata["internalIntensity"] = directive.InternalIntensity
	plan.Metadata["displayIntensity"] = plan.EmotionIntensity
	if directive.OverrideReason != "" {
		plan.Metadata["emotionOverrideReason"] = directive.OverrideReason
	}
	return plan
}

func NormalizeBdiConfig(bdi BdiConfig) BdiConfig {
	normalized := bdi
	if normalized.CopingStrategy != nil {
		strategy := *normalized.CopingStrategy
		if value, ok := NormalizeCopingStrategy(strategy.Selected); ok {
			strategy.Selected = string(value)
		}
		alternatives := make([]string, 0, len(strategy.Alternatives))
		for _, alternative := range strategy.Alternatives {
			if value, ok := NormalizeCopingStrategy(alternative); ok {
				alternatives = append(alternatives, string(value))
			}
		}
		strategy.Alternatives = alternatives
		normalized.CopingStrategy = &strategy
	}
	if normalized.EmotionExpression != nil {
		expression := *normalized.EmotionExpression
		expression.DisplayMode = NormalizeDisplayMode(expression.DisplayMode)
		normalized.EmotionExpression = &expression
	}
	rules := make([]HardConstraint, len(normalized.HardConstraints))
	copy(rules, normalized.HardConstraints)
	for index := range rules {
		rules[index].RuleID = strings.TrimSpace(rules[index].RuleID)
		rules[index].CandidateKey = strings.TrimSpace(rules[index].CandidateKey)
		rules[index].Reason = strings.TrimSpace(rules[index].Reason)
		rules[index].Severity = NormalizeConstraintSeverity(rules[index].Severity)
	}
	normalized.HardConstraints = rules
	return normalized
}

func ValidateBdiConfig(bdi BdiConfig) error {
	reasons := make([]string, 0)
	seenRules := make(map[string]bool)
	for index, rule := range bdi.HardConstraints {
		ruleID := strings.TrimSpace(rule.RuleID)
		if ruleID == "" {
			reasons = append(reasons, fmt.Sprintf("第 %d 条硬约束缺少规则 ID", index+1))
		} else if seenRules[ruleID] {
			reasons = append(reasons, "规则 ID 重复: "+ruleID)
		} else {
			seenRules[ruleID] = true
		}
		if !decision.IsKnownCandidateID(strings.TrimSpace(rule.CandidateKey)) {
			reasons = append(reasons, "硬约束指向未知行为候选: "+strings.TrimSpace(rule.CandidateKey))
		}
		severity := strings.ToLower(strings.TrimSpace(rule.Severity))
		severityValid := severity == "" ||
			severity == ConstraintSeverityBlock ||
			severity == ConstraintSeverityOverride ||
			severity == "prefer" ||
			severity == "priority"
		if !severityValid {
			reasons = append(reasons, "未知约束强度: "+rule.Severity)
		}
		if severity != ConstraintSeverityOverride && severity != "prefer" && severity != "priority" && strings.TrimSpace(rule.Reason) == "" {
			reasons = append(reasons, "阻断约束必须填写原因: "+ruleID)
		}
	}
	for _, preference := range bdi.SoftPreferences {
		dimension := strings.TrimSpace(preference.Dimension)
		if !decision.IsKnownPreferenceDimension(dimension) {
			reasons = append(reasons, "软偏好指向未知维度: "+dimension)
			continue
		}
		if preference.RawScore < 0 || preference.RawScore > 10 {
			reasons = append(reasons, "软偏好原始分超出 0-10: "+dimension)
		}
		if preference.NormalizedWeight < -1 || preference.NormalizedWeight > 1 {
			reasons = append(reasons, "软偏好权重超出 -1 到 1: "+dimension)
		}
	}
	if bdi.CopingStrategy != nil {
		selected := strings.TrimSpace(bdi.CopingStrategy.Selected)
		if selected != "" {
			if _, ok := NormalizeCopingStrategy(selected); !ok {
				reasons = append(reasons, "未知应对策略: "+selected)
			}
		}
		for _, alternative := range bdi.CopingStrategy.Alternatives {
			if _, ok := NormalizeCopingStrategy(alternative); !ok {
				reasons = append(reasons, "未知备选应对策略: "+alternative)
			}
		}
	}
	if bdi.EmotionExpression != nil {
		if bdi.EmotionExpression.InternalIntensity < 0 || bdi.EmotionExpression.InternalIntensity > 10 {
			reasons = append(reasons, "内在感受强度超出 0-10")
		}
		if bdi.EmotionExpression.DisplayIntensity < 0 || bdi.EmotionExpression.DisplayIntensity > 10 {
			reasons = append(reasons, "外在表达强度超出 0-10")
		}
		mode := strings.ToLower(strings.TrimSpace(bdi.EmotionExpression.DisplayMode))
		modeValid := mode == "" ||
			mode == EmotionDisplayShow ||
			mode == EmotionDisplaySuppress ||
			mode == EmotionDisplayReframe ||
			mode == "hidden" ||
			mode == "hide" ||
			mode == "reframing"
		if !modeValid {
			reasons = append(reasons, "未知表达模式: "+bdi.EmotionExpression.DisplayMode)
		}
	}
	if len(reasons) > 0 {
		return fmt.Errorf("%w: %s", ErrBdiConfigInvalid, strings.Join(reasons, "；"))
	}
	return nil
}

func BdiOptionSet() BdiOptions {
	registry := decision.DefaultCandidateRegistry()
	all := registry.All()
	sort.SliceStable(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	candidates := make([]BdiOption, 0, len(all))
	dimensions := make([]BdiOption, 0, len(all))
	for _, definition := range all {
		candidates = append(candidates, BdiOption{ID: definition.ID, Label: definition.Label, Group: string(definition.Tag)})
		dimensions = append(dimensions, BdiOption{ID: definition.ID, Label: definition.Label, Group: "candidate"})
	}
	tags := []decision.BehaviorTag{
		decision.BehaviorTagReply,
		decision.BehaviorTagAskClarify,
		decision.BehaviorTagOfferSupport,
		decision.BehaviorTagSetBoundary,
		decision.BehaviorTagRepair,
		decision.BehaviorTagProactiveCheck,
		decision.BehaviorTagDelay,
	}
	tagLabels := map[decision.BehaviorTag]string{
		decision.BehaviorTagReply:          "正常回复类",
		decision.BehaviorTagAskClarify:     "请求澄清类",
		decision.BehaviorTagOfferSupport:   "提供支持类",
		decision.BehaviorTagSetBoundary:    "设立边界类",
		decision.BehaviorTagRepair:         "关系修复类",
		decision.BehaviorTagProactiveCheck: "主动问候类",
		decision.BehaviorTagDelay:          "等待观察类",
	}
	for _, tag := range tags {
		dimensions = append(dimensions, BdiOption{ID: string(tag), Label: tagLabels[tag], Group: "tag"})
	}
	strategies := []decision.CopingStrategy{
		decision.CopingProblemSolving,
		decision.CopingReappraisal,
		decision.CopingSeekSupport,
		decision.CopingAcceptance,
		decision.CopingDistraction,
		decision.CopingSuppression,
		decision.CopingAvoidance,
	}
	coping := make([]BdiOption, 0, len(strategies))
	for _, strategy := range strategies {
		coping = append(coping, BdiOption{ID: string(strategy), Label: CopingStrategyLabel(strategy)})
	}
	return BdiOptions{
		Candidates: candidates,
		Dimensions: dimensions,
		Severities: []BdiOption{
			{ID: ConstraintSeverityBlock, Label: "阻断该行为"},
			{ID: ConstraintSeverityOverride, Label: "保留但优先"},
		},
		CopingStrategies: coping,
		DisplayModes: []BdiOption{
			{ID: EmotionDisplayShow, Label: "如实表露"},
			{ID: EmotionDisplaySuppress, Label: "抑制表露"},
			{ID: EmotionDisplayReframe, Label: "重构表达"},
		},
		Limits: BdiLimits{IntensityMin: 0, IntensityMax: 10, RawScoreMin: 0, RawScoreMax: 10, WeightMin: -1, WeightMax: 1},
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func clampUnit(value float64) float64 {
	if math.IsNaN(value) || value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func clampSigned(value float64) float64 {
	if math.IsNaN(value) {
		return 0
	}
	if value < -1 {
		return -1
	}
	if value > 1 {
		return 1
	}
	return value
}

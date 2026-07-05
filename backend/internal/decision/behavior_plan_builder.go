package decision

import "time"

type BehaviorPlanBuilder struct {
	Now time.Time
}

func NewBehaviorPlanBuilder(now time.Time) BehaviorPlanBuilder {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return BehaviorPlanBuilder{Now: now}
}

func (b *BehaviorPlanBuilder) Build(candidate BehaviorCandidate, input ArbitrationInput) BehaviorPlan {
	now := b.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	plan := BehaviorPlan{
		Version:      PlanVersionV1,
		ID:           "plan-" + now.Format("20060102150405"),
		UserID:       "",
		CharacterID:  "",
		CreatedAt:    now,
		Selected:     candidate,
		Priority:     derivePlanPriority(candidate),
		SafetyLevel:  derivePlanSafety(candidate),
		Psyche:       input.Psyche,
		Relationship: input.Relationship,
		Life:         input.Life,
		Audit: BehaviorAudit{
			FormulaVersion: string(BehaviorFormulaVersionV1),
		},
		Intent:          derivePlanIntent(candidate, input),
		Strategy:        derivePlanStrategy(candidate, input),
		AllowedTopics:   derivePlanAllowedTopics(candidate, input),
		ForbiddenTopics: derivePlanForbiddenTopics(candidate, input),
		ResponseGoal:    derivePlanResponseGoal(candidate, input),
		ToneHint:        derivePlanToneHint(candidate, input),
	}
	if plan.Priority == BehaviorPriorityCritical || plan.Priority == BehaviorPriorityHigh {
		plan.NeedsExpression = true
	}
	return plan
}

func derivePlanPriority(candidate BehaviorCandidate) BehaviorPriority {
	if candidate.FinalScore >= 0.8 {
		return BehaviorPriorityCritical
	}
	if candidate.FinalScore >= 0.6 {
		return BehaviorPriorityHigh
	}
	if candidate.FinalScore >= 0.3 {
		return BehaviorPriorityNormal
	}
	return BehaviorPriorityLow
}

func derivePlanSafety(candidate BehaviorCandidate) BehaviorSafetyLevel {
	if candidate.RiskScore >= 0.8 {
		return BehaviorSafetyLevelBlocked
	}
	if candidate.RiskScore >= 0.5 {
		return BehaviorSafetyLevelConservative
	}
	return BehaviorSafetyLevelNormal
}

func derivePlanIntent(candidate BehaviorCandidate, input ArbitrationInput) string {
	switch candidate.Tag {
	case BehaviorTagReply:
		return "正常回复"
	case BehaviorTagAskClarify:
		return "请求澄清"
	case BehaviorTagOfferSupport:
		return "提供支持"
	case BehaviorTagSetBoundary:
		return "设立边界"
	case BehaviorTagRepair:
		return "关系修复"
	case BehaviorTagProactiveCheck:
		return "主动关心"
	case BehaviorTagDelay:
		return "延迟观察"
	default:
		return "正常回复"
	}
}

func derivePlanStrategy(candidate BehaviorCandidate, input ArbitrationInput) string {
	switch candidate.Tag {
	case BehaviorTagReply:
		return "自然回应，保持对话流畅"
	case BehaviorTagAskClarify:
		return "温和追问，帮助澄清模糊内容"
	case BehaviorTagOfferSupport:
		return "提供情感支持或实际帮助"
	case BehaviorTagSetBoundary:
		return "礼貌但坚定地设立边界"
	case BehaviorTagRepair:
		return "修复关系裂痕，重建信任"
	case BehaviorTagProactiveCheck:
		return "主动表达关心和陪伴"
	case BehaviorTagDelay:
		return "保持克制，等待观察"
	default:
		return "保持自然沟通"
	}
}

func derivePlanAllowedTopics(candidate BehaviorCandidate, input ArbitrationInput) []string {
	topics := []string{"日常对话", "感受表达"}
	switch candidate.Tag {
	case BehaviorTagReply:
		topics = append(topics, "开放式交流", "当前话题延伸")
	case BehaviorTagAskClarify:
		topics = append(topics, "温和追问", "确认理解")
	case BehaviorTagOfferSupport:
		topics = append(topics, "情感支持", "积极倾听")
	case BehaviorTagSetBoundary:
		topics = append(topics, "清晰表达边界", "温和拒绝")
	case BehaviorTagRepair:
		topics = append(topics, "关系修复", "重建信任")
	case BehaviorTagProactiveCheck:
		topics = append(topics, "主动关心", "询问近况")
	case BehaviorTagDelay:
		topics = append(topics, "简短回应", "推迟讨论")
	}
	return topics
}

func derivePlanForbiddenTopics(candidate BehaviorCandidate, input ArbitrationInput) []string {
	topics := []string{"不适当的亲密关系请求", "违法违规内容"}
	if candidate.RiskScore > 0.6 {
		topics = append(topics, "高风险话题", "敏感争议")
	}
	switch candidate.Tag {
	case BehaviorTagSetBoundary:
		topics = append(topics, "过度私人问题")
	case BehaviorTagDelay:
		topics = append(topics, "需要即时决定的事项")
	}
	return topics
}

func derivePlanResponseGoal(candidate BehaviorCandidate, input ArbitrationInput) string {
	switch candidate.Tag {
	case BehaviorTagReply:
		return "让对话自然流畅地继续"
	case BehaviorTagAskClarify:
		return "帮助对方更清晰地表达，确保理解正确"
	case BehaviorTagOfferSupport:
		return "让对方感受到关心和支持"
	case BehaviorTagSetBoundary:
		return "在保持关系的前提下明确边界"
	case BehaviorTagRepair:
		return "修复关系裂痕，重建对方信任"
	case BehaviorTagProactiveCheck:
		return "让对方感受到被关心和重视"
	case BehaviorTagDelay:
		return "给对方空间同时保持联系"
	default:
		return "完成本轮交互目标"
	}
}

func derivePlanToneHint(candidate BehaviorCandidate, input ArbitrationInput) string {
	stressVal := input.Psyche.Stress.Value
	moodVal := input.Psyche.Mood.Value

	if candidate.Tag == BehaviorTagSetBoundary {
		return "坚定"
	}
	if stressVal > 0.7 {
		return "轻柔克制"
	}
	if moodVal < 0.3 {
		return "温暖安抚"
	}
	if candidate.Tag == BehaviorTagProactiveCheck || candidate.Tag == BehaviorTagOfferSupport {
		return "温暖关切"
	}
	if candidate.Tag == BehaviorTagRepair {
		return "真诚温和"
	}
	if candidate.Tag == BehaviorTagAskClarify {
		return "好奇温和"
	}
	if candidate.Tag == BehaviorTagDelay {
		return "冷静克制"
	}
	if input.Relationship.Dimensions != nil {
		if v, ok := input.Relationship.Dimensions[RelationshipFamiliarity]; ok && v.Value > 0.6 {
			return "自然轻松"
		}
	}
	return "自然友好"
}

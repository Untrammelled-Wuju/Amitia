package temporal

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type ReunionMentionMode string

const (
	ReunionMentionNone       ReunionMentionMode = "none"
	ReunionMentionSubtle     ReunionMentionMode = "subtle"
	ReunionMentionWarm       ReunionMentionMode = "warm"
	ReunionMentionReflective ReunionMentionMode = "reflective"
)

type RelationshipTimePolicy struct {
	MentionMode           ReunionMentionMode `json:"mentionMode"`
	MaxMentionSentences   int                `json:"maxMentionSentences"`
	MemoryRecallBudget    int                `json:"memoryRecallBudget"`
	RestorePreviousTopic  bool               `json:"restorePreviousTopic"`
	UseRelationshipAge    bool               `json:"useRelationshipAge"`
	ApplyWarmthDelta      float64            `json:"applyWarmthDelta"`
	ApplySocialArousal    float64            `json:"applySocialArousal"`
	FamiliarityExpression float64            `json:"familiarityExpression"`
	ReacclimationTurns    int                `json:"reacclimationTurns"`
	SuppressionReason     string             `json:"suppressionReason,omitempty"`
}

func DefaultRelationshipTimePolicy(level ReunionLevel) RelationshipTimePolicy {
	policy := RelationshipTimePolicy{MentionMode: ReunionMentionNone, ApplyWarmthDelta: temporaryWarmth(level), ApplySocialArousal: temporarySocialArousal(level), FamiliarityExpression: 1, ReacclimationTurns: reacclimationTurns(level)}
	switch level {
	case ReunionLevelNoticeable:
		policy.MentionMode = ReunionMentionSubtle
		policy.MaxMentionSentences = 1
	case ReunionLevelLong:
		policy.MentionMode = ReunionMentionWarm
		policy.MaxMentionSentences = 1
		policy.MemoryRecallBudget = 1
	case ReunionLevelExtended:
		policy.MentionMode = ReunionMentionWarm
		policy.MaxMentionSentences = 1
		policy.MemoryRecallBudget = 1
	case ReunionLevelDormant:
		policy.MentionMode = ReunionMentionReflective
		policy.MaxMentionSentences = 2
		policy.MemoryRecallBudget = 2
	}
	return policy
}

func ResolveRelationshipTimePolicy(value RelationshipTimeContext, message string, taskLike bool) RelationshipTimePolicy {
	if value.Reunion == nil {
		return RelationshipTimePolicy{MentionMode: ReunionMentionNone, FamiliarityExpression: value.ContinuityScore, SuppressionReason: "no_reunion"}
	}
	policy := DefaultRelationshipTimePolicy(value.Reunion.Level)
	policy.FamiliarityExpression = value.ContinuityScore
	policy.RestorePreviousTopic = value.ContinuityScore >= 0.6 && policy.MemoryRecallBudget > 0
	policy.UseRelationshipAge = value.RelationshipAgeDays >= 30
	if !value.Reunion.ShouldExpress {
		policy.MentionMode = ReunionMentionNone
		policy.MaxMentionSentences = 0
		policy.SuppressionReason = "episode_not_claimed"
		return policy
	}
	if taskLike || looksLikeTask(message) {
		policy.MentionMode = ReunionMentionNone
		policy.MaxMentionSentences = 0
		policy.MemoryRecallBudget = 0
		policy.RestorePreviousTopic = false
		policy.SuppressionReason = "current_task_priority"
		return policy
	}
	if !mentionsReunion(message) && policy.MaxMentionSentences > 1 {
		policy.MaxMentionSentences = 1
	}
	if value.Reunion.Kind == ReunionKindReplyToProactive {
		policy.MentionMode = ReunionMentionNone
		policy.MaxMentionSentences = 0
		policy.SuppressionReason = "recent_proactive_contact"
	}
	return policy
}

func RenderRelationshipTime(value RelationshipTimeContext, policy RelationshipTimePolicy) string {
	if value.Reunion == nil && value.ReacclimationTurnsLeft == 0 {
		return ""
	}
	lines := []string{"【关系时间上下文】"}
	if value.Reunion != nil {
		switch value.Reunion.Kind {
		case ReunionKindRelationshipReconnect:
			lines = append(lines, fmt.Sprintf("用户最近仍在使用 Amitia，但已约 %s 未与当前角色交流。", readableGap(value.RelationshipGapSeconds)), "本轮属于关系重连，不属于用户整体消失。")
		case ReunionKindGlobalReturn:
			lines = append(lines, fmt.Sprintf("用户距上次在 Amitia 完成互动约 %s，距上次与当前角色互动约 %s。", readableGap(value.GlobalGapSeconds), readableGap(value.RelationshipGapSeconds)), "本轮属于用户整体回归。")
		case ReunionKindReplyToProactive:
			lines = append(lines, "用户正在回应角色近期的主动联系，不要再次强调久别。")
		}
	}
	if policy.MentionMode == ReunionMentionNone {
		lines = append(lines, "表达策略：当前用户任务优先，不主动展开久别话题。")
	} else {
		lines = append(lines, fmt.Sprintf("表达策略：优先回应用户当前内容；如自然，最多用 %d 句温和表达久别。禁止责备、索取解释、制造亏欠感或暗示被抛弃。", policy.MaxMentionSentences))
	}
	if value.ContinuityScore < 0.4 {
		lines = append(lines, "上下文连续性较低，不要默认旧计划、旧承诺或旧话题仍然有效。")
	} else if value.ContinuityScore < 0.7 {
		lines = append(lines, "上下文连续性中等，续接旧话题前先确认其仍然相关。")
	}
	if policy.MemoryRecallBudget > 0 {
		lines = append(lines, fmt.Sprintf("共同记忆最多使用 %d 条，且必须与当前内容直接相关。", policy.MemoryRecallBudget))
	}
	return strings.Join(lines, "\n")
}

func looksLikeTask(message string) bool {
	value := strings.ToLower(strings.TrimSpace(message))
	if value == "" {
		return false
	}
	markers := []string{"报错", "错误", "失败", "故障", "排查", "修复", "实现", "修改", "启动", "重启", "上传", "日志", "代码", "命令", "error", "failed", "fix", "debug", "```", "panic", "exception"}
	for _, marker := range markers {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return utf8.RuneCountInString(value) > 24 && (strings.Contains(value, "帮我") || strings.Contains(value, "请"))
}

func mentionsReunion(message string) bool {
	value := strings.ToLower(message)
	markers := []string{"好久不见", "我回来了", "没空", "离开", "想我", "想念", "long time", "i'm back", "im back"}
	for _, marker := range markers {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func readableGap(seconds float64) string {
	switch {
	case seconds >= 90*24*3600:
		return fmt.Sprintf("%d 天", int(seconds/(24*3600)+0.5))
	case seconds >= 24*3600:
		return fmt.Sprintf("%d 天", int(seconds/(24*3600)+0.5))
	case seconds >= 3600:
		return fmt.Sprintf("%d 小时", int(seconds/3600+0.5))
	default:
		return "不到一小时"
	}
}

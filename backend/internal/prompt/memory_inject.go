package prompt

import (
	"strings"

	"github.com/u-ai/backend/internal/prompt/textlib"
)

const maxMemoryInjectPerCategory = 5

type MemoryInjectTier string

const (
	TierLongTerm MemoryInjectTier = "长期记忆"
	TierRoleRel  MemoryInjectTier = "当前角色相关记忆"
	TierRecent   MemoryInjectTier = "最近上下文"
)

type MemoryInjectItem struct {
	Content  string
	Category string
	Tier     MemoryInjectTier
}

var memoryInjectGuardrail = "【关于用户的记忆 · 仅供内心参考 · 绝对禁止在回复中说\"根据记忆\"\"根据记录\"\"我记得\"】" + "\n\n" +
	"你在和用户面对面直接对话。以下记忆来自系统检索，仅供你内心自然参考，像你心里默默记住关于对方的事情一样。" + "\n\n" +
	"── 使用规则 ──" + "\n" +
"1. " + textlib.MemoryFactExtractReferenceRule + "。" + "\n" +
	"2. 绝对不要说\"根据我的记忆\"\"根据我的记录\"\"我的数据显示\"\"我知道你最近……\"等元表述。" + "\n" +
	"3. 记忆最后一段的\"近期状态\"仅供参考——不要在用户正在聊开心事时主动提起压力话题。" + "\n" +
	"4. 如果用户在聊开心事，不要因为看到记忆中有不愉快的记录就主动提起。" + "\n" +
	"5. 用户已撤销或否定的信息不应使用。" + "\n" +
	"6. 使用记忆时自然融入对话，像你本来就记得一样。" + "\n\n" +
	"── 禁止 ──" + "\n" +
	"× 不要写\"根据事实\"\"根据记录\"\"我的数据显示\"等元表述" + "\n" +
	"× 不要逐条罗列记忆内容" + "\n" +
	"× 不要把临时情绪写成长久性格判断"

func BuildMemoryInjectRawSection(items []MemoryInjectItem) string {
	guardrail := memoryInjectGuardrail

	if len(items) == 0 {
		return guardrail
	}

	tierOrder := []MemoryInjectTier{TierLongTerm, TierRoleRel, TierRecent}
	categoryOrder := []string{"事实", "偏好", "习惯", "关系", "事件", "情感"}

	tierGroups := map[MemoryInjectTier]map[string][]string{
		TierLongTerm: {
			"事实": {},
			"偏好": {},
			"习惯": {},
			"关系": {},
			"事件": {},
			"情感": {},
		},
		TierRoleRel: {
			"事实": {},
			"偏好": {},
			"习惯": {},
			"关系": {},
			"事件": {},
			"情感": {},
		},
		TierRecent: {
			"事实": {},
			"偏好": {},
			"习惯": {},
			"关系": {},
			"事件": {},
			"情感": {},
		},
	}

	for _, item := range items {
		cat := mapToChineseCategory(item.Category)
		tier := item.Tier
		if tier == "" {
			tier = TierRecent
		}
		if groups, ok := tierGroups[tier]; ok {
			groups[cat] = append(groups[cat], "- "+item.Content)
		}
	}

	for tier := range tierGroups {
		for cat := range tierGroups[tier] {
			if len(tierGroups[tier][cat]) > maxMemoryInjectPerCategory {
				tierGroups[tier][cat] = tierGroups[tier][cat][:maxMemoryInjectPerCategory]
			}
		}
	}

	var sb strings.Builder
	sb.WriteString(guardrail)
sb.WriteString("\n\n" + textlib.MemoryFormatContextHeader + "\n")

	for _, tier := range tierOrder {
		groups := tierGroups[tier]
		hasContent := false
		for _, cat := range categoryOrder {
			if len(groups[cat]) > 0 {
				hasContent = true
				break
			}
		}
		if !hasContent {
			continue
		}

		sb.WriteString("\n── " + string(tier) + " ──\n")
		for _, cat := range categoryOrder {
			if lines := groups[cat]; len(lines) > 0 {
				sb.WriteString("\n【" + cat + "】\n")
				for _, line := range lines {
					sb.WriteString(line + "\n")
				}
			}
		}
	}

	return sb.String()
}

func mapToChineseCategory(category string) string {
	lower := strings.ToLower(category)
	switch {
	case strings.Contains(lower, "fact") || strings.Contains(lower, "事实"):
		return "事实"
	case strings.Contains(lower, "preference") || strings.Contains(lower, "偏好"):
		return "偏好"
	case strings.Contains(lower, "habit") || strings.Contains(lower, "习惯"):
		return "习惯"
	case strings.Contains(lower, "relationship") || strings.Contains(lower, "关系"):
		return "关系"
	case strings.Contains(lower, "event") || strings.Contains(lower, "事件") || strings.Contains(lower, "episodic") || strings.Contains(lower, "情景"):
		return "事件"
	case strings.Contains(lower, "emotion") || strings.Contains(lower, "情感") || strings.Contains(lower, "mood"):
		return "情感"
	default:
		return "事实"
	}
}

func BuildMemoryInjectGuardrailOnly() string {
	return memoryInjectGuardrail
}

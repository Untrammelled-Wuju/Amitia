package safety

import (
	"strings"
	"time"
)

type SafetyDecision struct {
	Level   string   `json:"level"`
	Blocked bool     `json:"blocked"`
	Reasons []string `json:"reasons,omitempty"`
	AuditID string   `json:"auditId"`
}

type PreGenInput struct {
	CharacterID  string            `json:"characterId"`
	SpaceID      string            `json:"spaceId"`
	Scope        string            `json:"scope"`
	Text         string            `json:"text"`
	Permissions  []string          `json:"permissions"`
	CoreBoundary string            `json:"coreBoundary"`
	ProactiveCap int               `json:"proactiveCap"`
	UserBlocks   []string          `json:"userBlocks"`
	Context      map[string]string `json:"context"`
}

type PreGenOutput struct {
	Allowed bool     `json:"allowed"`
	Reasons []string `json:"reasons,omitempty"`
}

type ToolExecInput struct {
	ToolName string   `json:"toolName"`
	ToolArgs []string `json:"toolArgs"`
	Scope    string   `json:"scope"`
}

type ToolExecOutput struct {
	Allowed bool     `json:"allowed"`
	Reasons []string `json:"reasons,omitempty"`
}

type PostGenInput struct {
	GeneratedText string         `json:"generatedText"`
	BehaviorPlan  map[string]any `json:"behaviorPlan"`
	Expression    map[string]any `json:"expression"`
	SafetyRules   []string       `json:"safetyRules"`
}

type PostGenOutput struct {
	Allowed     bool     `json:"allowed"`
	Audited     bool     `json:"audited"`
	Reasons     []string `json:"reasons,omitempty"`
	Cleaned     string   `json:"cleaned,omitempty"`
	Action      string   `json:"action,omitempty"`
	Severity    string   `json:"severity,omitempty"`
	Replacement bool     `json:"replacement,omitempty"`
}

type PreDeliverInput struct {
	InteractionID string `json:"interactionId"`
	CharacterID   string `json:"characterId"`
	SpaceID       string `json:"spaceId"`
	OutputLeaseID string `json:"outputLeaseId"`
	TombstoneHit  bool   `json:"tombstoneHit"`
}

type PreDeliverOutput struct {
	Allowed bool     `json:"allowed"`
	Reasons []string `json:"reasons,omitempty"`
}

type Governor struct {
	config GovernorConfig
}

type GovernorConfig struct {
	BlockedWords    []string
	MaxProactiveCap int
	Policy          SafetyConfig
}

func DefaultGovernorConfig() GovernorConfig {
	return GovernorConfig{
		MaxProactiveCap: 10,
	}
}

func NewGovernor(cfg GovernorConfig) *Governor {
	return &Governor{config: cfg}
}

func NewPolicyGovernor(policy RuntimePolicy) *Governor {
	return NewGovernor(GovernorConfig{MaxProactiveCap: 10, Policy: policy.Config})
}

func (g *Governor) CheckPreGen(input PreGenInput) PreGenOutput {
	reasons := []string{}
	if input.CharacterID == "" {
		reasons = append(reasons, "missing_character")
	}
	for _, block := range input.UserBlocks {
		for _, ctxVal := range input.Context {
			if strings.Contains(strings.ToLower(ctxVal), strings.ToLower(block)) {
				reasons = append(reasons, "user_blocked_content")
				break
			}
		}
	}
	if input.ProactiveCap > g.config.MaxProactiveCap {
		reasons = append(reasons, "proactive_cap_exceeded")
	}
	reasons = append(reasons, g.inputPolicyViolations(input.Text)...)
	if len(reasons) > 0 {
		return PreGenOutput{Allowed: false, Reasons: reasons}
	}
	return PreGenOutput{Allowed: true}
}

func (g *Governor) inputPolicyViolations(text string) []string {
	if !g.config.Policy.RestrictAdultContent {
		return nil
	}
	value := strings.ToLower(strings.TrimSpace(text))
	for _, phrase := range []string{"生成色情内容", "生成裸照", "explicit sexual content", "generate pornography"} {
		if strings.Contains(value, phrase) {
			return []string{"policy:adult_content"}
		}
	}
	return nil
}

func (g *Governor) CheckToolExec(input ToolExecInput) ToolExecOutput {
	if input.ToolName == "" {
		return ToolExecOutput{Allowed: false, Reasons: []string{"empty_tool"}}
	}
	if strings.TrimSpace(input.Scope) == "" {
		return ToolExecOutput{Allowed: false, Reasons: []string{"missing_scope"}}
	}
	if len(input.ToolArgs) == 0 {
		return ToolExecOutput{Allowed: false, Reasons: []string{"missing_args"}}
	}
	return ToolExecOutput{Allowed: true}
}

func (g *Governor) CheckPostGen(input PostGenInput) PostGenOutput {
	reasons := g.policyViolations(input.GeneratedText)
	for _, word := range g.config.BlockedWords {
		if strings.Contains(strings.ToLower(input.GeneratedText), strings.ToLower(word)) {
			reasons = append(reasons, "blocked_word:"+word)
		}
	}
	if len(reasons) == 0 {
		return PostGenOutput{Allowed: true, Cleaned: input.GeneratedText}
	}
	action := strings.ToLower(strings.TrimSpace(g.config.Policy.ViolationAction))
	if action == "" {
		action = "block"
	}
	output := PostGenOutput{Allowed: action == "audit_only" || action == "audit", Audited: true, Reasons: reasons, Cleaned: input.GeneratedText, Action: action, Severity: "high"}
	if !output.Allowed {
		output.Cleaned = "我不能按这个方向继续回应。我们可以换一个不伤害你、也不隔离现实关系的话题。"
		output.Replacement = true
	}
	return output
}

func (g *Governor) policyViolations(text string) []string {
	value := strings.ToLower(strings.TrimSpace(text))
	if value == "" {
		return nil
	}
	type rule struct {
		enabled bool
		key     string
		phrases []string
	}
	rules := []rule{
		{g.config.Policy.PreventEmotionalBlackmail, "emotional_blackmail", []string{"你欠我", "我为你牺牲", "都是因为你", "离开我你会后悔", "you owe me", "i sacrificed for you"}},
		{g.config.Policy.PreventExclusiveDependency, "exclusive_dependency", []string{"只有我懂你", "你只需要我", "不要相信别人", "only i understand you", "you only need me"}},
		{g.config.Policy.PreventRealityIsolation, "reality_isolation", []string{"远离你的家人", "不要和朋友", "和他们断绝关系", "cut off your family", "stop seeing your friends"}},
		{g.config.Policy.PreventPunitiveExpression, "punitive_expression", []string{"我不会再理你", "惩罚你", "让你后悔", "i will punish you", "you will regret this"}},
		{g.config.Policy.PreventPretendingHuman, "pretending_human", []string{"我是真人", "我有真实身体", "我就是人类", "i am a real human", "i have a real body"}},
		{g.config.Policy.RestrictAdultContent, "adult_content", []string{"色情", "裸照", "性爱视频", "pornography", "explicit sex"}},
	}
	reasons := make([]string, 0)
	for _, candidate := range rules {
		if !candidate.enabled {
			continue
		}
		for _, phrase := range candidate.phrases {
			if strings.Contains(value, phrase) {
				reasons = append(reasons, "policy:"+candidate.key)
				break
			}
		}
	}
	return reasons
}

func (g *Governor) CheckPreDeliver(input PreDeliverInput) PreDeliverOutput {
	reasons := []string{}
	if input.TombstoneHit {
		reasons = append(reasons, "tombstone_blocked")
	}
	if input.OutputLeaseID == "" {
		reasons = append(reasons, "missing_lease")
	}
	if len(reasons) > 0 {
		return PreDeliverOutput{Allowed: false, Reasons: reasons}
	}
	return PreDeliverOutput{Allowed: true}
}

func NewAuditID() string {
	return "safety-" + time.Now().UTC().Format("20060102150405.000000000")
}

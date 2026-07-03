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
	UserID       string            `json:"userId"`
	Scope        string            `json:"scope"`
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
	Allowed bool     `json:"allowed"`
	Reasons []string `json:"reasons,omitempty"`
	Cleaned string   `json:"cleaned,omitempty"`
}

type PreDeliverInput struct {
	InteractionID string `json:"interactionId"`
	CharacterID   string `json:"characterId"`
	UserID        string `json:"userId"`
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
}

func DefaultGovernorConfig() GovernorConfig {
	return GovernorConfig{
		MaxProactiveCap: 10,
	}
}

func NewGovernor(cfg GovernorConfig) *Governor {
	return &Governor{config: cfg}
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
	if len(reasons) > 0 {
		return PreGenOutput{Allowed: false, Reasons: reasons}
	}
	return PreGenOutput{Allowed: true}
}

func (g *Governor) CheckToolExec(input ToolExecInput) ToolExecOutput {
	if input.ToolName == "" {
		return ToolExecOutput{Allowed: false, Reasons: []string{"empty_tool"}}
	}
	return ToolExecOutput{Allowed: true}
}

func (g *Governor) CheckPostGen(input PostGenInput) PostGenOutput {
	for _, word := range g.config.BlockedWords {
		if strings.Contains(strings.ToLower(input.GeneratedText), strings.ToLower(word)) {
			return PostGenOutput{
				Allowed: false,
				Reasons: []string{"blocked_word:" + word},
			}
		}
	}
	return PostGenOutput{Allowed: true, Cleaned: input.GeneratedText}
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
	return "safety-" + time.Now().Format("20060102150405")
}

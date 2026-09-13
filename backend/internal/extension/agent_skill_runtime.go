package extension

import (
	"context"
)

func (r *Runtime) PrepareAgentSkillPrompt(ctx context.Context, scope ExecutionScope, message string) (string, []ActivatedAgentSkill, []string) {
	if r == nil || r.AgentSkills == nil {
		return "", nil, nil
	}
	return r.AgentSkills.PreparePrompt(ctx, scope, message)
}
func (r *Runtime) EndAgentSkillRound(scope ExecutionScope) {
	if r != nil && r.AgentSkills != nil {
		r.AgentSkills.EndRound(scope)
	}
}

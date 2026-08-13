// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package extension

import (
	"context"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/skill"
)

type skillScriptHandler struct {
	service *AgentSkillService
	runtime *skill.ScriptRuntime
}

func NewSkillScriptHandler(service *AgentSkillService, runtime *skill.ScriptRuntime) kernel.RunSkillScriptHandler {
	return &skillScriptHandler{
		service: service,
		runtime: runtime,
	}
}

func (h *skillScriptHandler) HandleRunSkillScript(ctx context.Context, input kernel.RunSkillScriptInput, scope kernel.LegacyScope) (kernel.RunSkillScriptOutput, error) {
	extScope := legacyScopeToKernel(scope)

	_, activatedSkills, _ := h.service.PreparePrompt(ctx, extScope, "")

	var matchedSkill *ActivatedAgentSkill
	for i := range activatedSkills {
		s := &activatedSkills[i]
		if strings.EqualFold(s.Definition.Name, input.Skill) || strings.EqualFold(s.Definition.ExtensionID, input.Skill) {
			matchedSkill = s
			break
		}
	}

	if matchedSkill == nil {
		return kernel.RunSkillScriptOutput{
			Status: "error",
			Error:  fmt.Sprintf("skill %q is not active", input.Skill),
		}, fmt.Errorf("skill %s is not active", input.Skill)
	}

	scriptPath := normalizeScriptPath(input.Script)

	var expectedHash string
	for _, res := range matchedSkill.Definition.Resources {
		if res.Path == scriptPath && res.Kind == AgentSkillResourceScript {
			expectedHash = res.SHA256
			break
		}
	}

	resourceContent, err := h.service.ReadResource(ctx, ReadAgentSkillResourceRequest{
		Scope:    extScope,
		NameOrID: matchedSkill.Definition.Name,
		Path:     scriptPath,
	})
	if err != nil {
		return kernel.RunSkillScriptOutput{
			Status: "error",
			Error:  fmt.Sprintf("failed to read script: %v", err),
		}, err
	}

	timeout := skill.DefaultScriptTimeout

	result, err := h.runtime.Execute(ctx, skill.ScriptExecutionRequest{
		ExtensionID:  matchedSkill.Definition.ExtensionID,
		SkillName:    matchedSkill.Definition.Name,
		RelativePath: scriptPath,
		Content:      []byte(resourceContent.Content),
		ExpectedHash: expectedHash,
		Args:         input.Args,
		Inputs:       input.Inputs,
		Timeout:      timeout,
	})

	if err != nil {
		return kernel.RunSkillScriptOutput{
			Status: "error",
			Error:  err.Error(),
		}, err
	}
	if result == nil {
		return kernel.RunSkillScriptOutput{
			Status: "error",
			Error:  "script execution returned nil result",
		}, fmt.Errorf("nil script result")
	}

	return kernel.RunSkillScriptOutput{
		ExecutionID: result.ExecutionID,
		Status:      result.Status,
		Output:      result.Output,
		ExitCode:    result.ExitCode,
		Resources:   result.Resources,
		Duration:    result.Duration.String(),
	}, nil
}

func (h *skillScriptHandler) HasScriptCapableSkills(ctx context.Context, scope kernel.LegacyScope) (bool, []string, error) {
	extScope := legacyScopeToKernel(scope)

	_, activatedSkills, _ := h.service.PreparePrompt(ctx, extScope, "")

	var names []string
	for i := range activatedSkills {
		s := &activatedSkills[i]
		hasScript := false
		for _, res := range s.Definition.Resources {
			if res.Kind == AgentSkillResourceScript {
				hasScript = true
				break
			}
		}
		if hasScript {
			names = append(names, s.Definition.Name)
		}
	}

	return len(names) > 0, names, nil
}

func normalizeScriptPath(relPath string) string {
	relPath = strings.TrimSpace(relPath)
	relPath = strings.TrimLeft(relPath, "./\\")
	if !strings.HasPrefix(relPath, "scripts/") {
		relPath = "scripts/" + relPath
	}
	return relPath
}

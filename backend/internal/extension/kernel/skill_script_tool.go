// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/extension/kernel/skill"
)

const RunSkillScriptToolName = "run_skill_script"

type RunSkillScriptHandler interface {
	HandleRunSkillScript(ctx context.Context, input RunSkillScriptInput, scope LegacyScope) (RunSkillScriptOutput, error)
	HasScriptCapableSkills(ctx context.Context, scope LegacyScope) (bool, []string, error)
}

type RunSkillScriptInput struct {
	Skill  string            `json:"skill"`
	Script string            `json:"script"`
	Args   map[string]any    `json:"args"`
	Inputs map[string]string `json:"inputs"`
}

type RunSkillScriptOutput struct {
	ExecutionID     string            `json:"executionId"`
	Status          string            `json:"status"`
	Output          string            `json:"output,omitempty"`
	ExitCode        int               `json:"exitCode"`
	Resources       map[string]string `json:"resources,omitempty"`
	Error           string            `json:"error,omitempty"`
	Duration        string            `json:"duration,omitempty"`
}

func buildRunSkillScriptTool(skillNames []string) tool.Tool {
	skillDescription := "Execute a script from an active Agent Skill. "
	if len(skillNames) > 0 {
		skillDescription += "Available skills with scripts: " + strings.Join(skillNames, ", ") + "."
	}

	return tool.Tool{
		Type: "function",
		Function: tool.Function{
			Name:        RunSkillScriptToolName,
			Description: skillDescription,
			Parameters: tool.Parameters{
				Type: "object",
				Properties: map[string]tool.Property{
					"skill": {
						Type:        "string",
						Description: "The name of the skill that owns the script.",
						Enum:        skillNames,
					},
					"script": {
						Type:        "string",
						Description: "The relative path of the script to execute (e.g., 'scripts/analyze.js').",
					},
					"args": {
						Type:        "object",
						Description: "Key-value arguments to pass to the script.",
					},
					"inputs": {
						Type:        "object",
						Description: "Key-value input bindings (resource URIs, literals).",
					},
				},
				Required: []string{"skill", "script"},
			},
		},
	}
}

func (f *ToolFacade) handleRunSkillScript(ctx context.Context, input json.RawMessage, scope LegacyScope) (LegacyToolResult, error) {
	var req RunSkillScriptInput
	if err := json.Unmarshal(input, &req); err != nil {
		return LegacyToolResult{
			Status:      "FAILED",
			VisibleText: fmt.Sprintf("invalid run_skill_script input: %v", err),
			Error:       &LegacyToolError{Code: "SKILL_SCRIPT_INVALID_INPUT", Message: err.Error()},
		}, err
	}

	if strings.TrimSpace(req.Skill) == "" {
		return LegacyToolResult{
			Status:      "FAILED",
			VisibleText: "skill name is required",
			Error:       &LegacyToolError{Code: "SKILL_SCRIPT_INVALID_INPUT", Message: "skill name is required"},
		}, fmt.Errorf("skill name is required")
	}

	if strings.TrimSpace(req.Script) == "" {
		return LegacyToolResult{
			Status:      "FAILED",
			VisibleText: "script path is required",
			Error:       &LegacyToolError{Code: "SKILL_SCRIPT_INVALID_INPUT", Message: "script path is required"},
		}, fmt.Errorf("script path is required")
	}

	if f.runSkillScriptHandler == nil {
		return LegacyToolResult{
			Status:      "FAILED",
			VisibleText: "skill script execution not available",
			Error:       &LegacyToolError{Code: "SKILL_SCRIPT_UNAVAILABLE", Message: "handler not configured"},
		}, fmt.Errorf("run_skill_script handler not configured")
	}

	output, err := f.runSkillScriptHandler.HandleRunSkillScript(ctx, req, scope)
	if err != nil {
		return LegacyToolResult{
			Status:      "FAILED",
			VisibleText: fmt.Sprintf("skill script execution failed: %v", err),
			Error:       &LegacyToolError{Code: "SKILL_SCRIPT_EXECUTION_FAILED", Message: err.Error()},
		}, err
	}

	resultJSON, _ := json.Marshal(output)

	visibleText := fmt.Sprintf("Script executed: %s/%s (status=%s, exit=%d)", req.Skill, req.Script, output.Status, output.ExitCode)
	if output.Error != "" {
		visibleText = fmt.Sprintf("Script failed: %s/%s - %s", req.Skill, req.Script, output.Error)
	}

	return LegacyToolResult{
		RunID:        output.ExecutionID,
		Status:       output.Status,
		Output:       resultJSON,
		VisibleText:  visibleText,
	}, nil
}

func (f *ToolFacade) resolveScriptCapableSkillNames(ctx context.Context, scope LegacyScope) ([]string, error) {
	if f.runSkillScriptHandler == nil {
		return nil, nil
	}
	has, names, err := f.runSkillScriptHandler.HasScriptCapableSkills(ctx, scope)
	if err != nil || !has {
		return nil, err
	}
	return names, nil
}

func convertScriptResult(result *skill.SkillScriptResult) RunSkillScriptOutput {
	if result == nil {
		return RunSkillScriptOutput{
			Status: "error",
			Error:  "nil result",
		}
	}
	return RunSkillScriptOutput{
		ExecutionID: result.ExecutionID,
		Status:      result.Status,
		Output:      result.Output,
		ExitCode:    result.ExitCode,
		Resources:   result.Resources,
		Duration:    result.Duration.String(),
	}
}

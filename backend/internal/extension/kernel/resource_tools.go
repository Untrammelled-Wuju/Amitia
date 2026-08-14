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

const (
	ListSkillResourcesToolName       = "list_skill_resources"
	ReadSkillResourceToolName        = "read_skill_resource"
	MaterializeSkillResourceToolName = "materialize_skill_resource"
)

type SkillResourceHandler interface {
	HandleListSkillResources(ctx context.Context, input ListSkillResourcesInput, scope LegacyScope) (ListSkillResourcesOutput, error)
	HandleReadSkillResource(ctx context.Context, input ReadSkillResourceInput, scope LegacyScope) (ReadSkillResourceOutput, error)
	HandleMaterializeSkillResource(ctx context.Context, input MaterializeSkillResourceInput, scope LegacyScope) (MaterializeSkillResourceOutput, error)
	HasResourceCapableSkills(ctx context.Context, scope LegacyScope) (bool, []string, error)
}

type ListSkillResourcesInput struct {
	Skill  string `json:"skill"`
	Kind   string `json:"kind,omitempty"`
	Prefix string `json:"prefix,omitempty"`
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type ListSkillResourcesOutput struct {
	Resources     []skill.SkillResourceDescriptor `json:"resources"`
	NextCursor    string                          `json:"nextCursor,omitempty"`
	IndexComplete bool                            `json:"indexComplete"`
	TotalCount    int                             `json:"totalCount"`
	Error         string                          `json:"error,omitempty"`
}

type ReadSkillResourceInput struct {
	Skill     string `json:"skill"`
	Path      string `json:"path"`
	StartLine int    `json:"startLine,omitempty"`
	MaxLines  int    `json:"maxLines,omitempty"`
}

type ReadSkillResourceOutput struct {
	Skill         string `json:"skill"`
	Path          string `json:"path"`
	MIMEType      string `json:"mimeType"`
	StartLine     int    `json:"startLine"`
	EndLine       int    `json:"endLine"`
	TotalLines    int    `json:"totalLines,omitempty"`
	Content       string `json:"content"`
	Truncated     bool   `json:"truncated"`
	NextStartLine int    `json:"nextStartLine,omitempty"`
	SHA256        string `json:"sha256"`
	Error         string `json:"error,omitempty"`
}

type MaterializeSkillResourceInput struct {
	Skill string `json:"skill"`
	Path  string `json:"path"`
}

type MaterializeSkillResourceOutput struct {
	Skill       string `json:"skill"`
	Path        string `json:"path"`
	ResourceURI string `json:"resourceUri"`
	MIMEType    string `json:"mimeType"`
	SizeBytes   int64  `json:"sizeBytes"`
	SHA256      string `json:"sha256"`
	LeaseID     string `json:"leaseId,omitempty"`
	Error       string `json:"error,omitempty"`
}

func buildListSkillResourcesTool() tool.Tool {
	return tool.Tool{
		Type: "function",
		Function: tool.Function{
			Name:        ListSkillResourcesToolName,
			Description: "List available resources (references, assets) of an active Agent Skill. Returns metadata only, not content.",
			Parameters: tool.Parameters{
				Type: "object",
				Properties: map[string]tool.Property{
					"skill": {
						Type:        "string",
						Description: "The name of the active skill to list resources from.",
					},
					"kind": {
						Type:        "string",
						Description: "Filter by resource kind: reference, asset, or script.",
						Enum:        []string{"reference", "asset", "script"},
					},
					"prefix": {
						Type:        "string",
						Description: "Filter by path prefix (e.g., 'references/').",
					},
					"cursor": {
						Type:        "string",
						Description: "Pagination cursor from previous response.",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results (default 50, max 200).",
					},
				},
				Required: []string{"skill"},
			},
		},
	}
}

func buildReadSkillResourceTool() tool.Tool {
	return tool.Tool{
		Type: "function",
		Function: tool.Function{
			Name:        ReadSkillResourceToolName,
			Description: "Read text content from a resource of an active Agent Skill. Supports line range for large files.",
			Parameters: tool.Parameters{
				Type: "object",
				Properties: map[string]tool.Property{
					"skill": {
						Type:        "string",
						Description: "The name of the active skill.",
					},
					"path": {
						Type:        "string",
						Description: "The relative path of the resource (e.g., 'references/api.md').",
					},
					"startLine": {
						Type:        "integer",
						Description: "Starting line number (1-based).",
					},
					"maxLines": {
						Type:        "integer",
						Description: "Maximum lines to read (default 200, max 1000).",
					},
				},
				Required: []string{"skill", "path"},
			},
		},
	}
}

func buildMaterializeSkillResourceTool() tool.Tool {
	return tool.Tool{
		Type: "function",
		Function: tool.Function{
			Name:        MaterializeSkillResourceToolName,
			Description: "Materialize a skill resource (binary or large file) as a ResourceURI for use by other tools.",
			Parameters: tool.Parameters{
				Type: "object",
				Properties: map[string]tool.Property{
					"skill": {
						Type:        "string",
						Description: "The name of the active skill.",
					},
					"path": {
						Type:        "string",
						Description: "The relative path of the resource to materialize.",
					},
				},
				Required: []string{"skill", "path"},
			},
		},
	}
}

func (f *ToolFacade) handleListSkillResources(ctx context.Context, input json.RawMessage, scope LegacyScope) (LegacyToolResult, error) {
	var req ListSkillResourcesInput
	if err := json.Unmarshal(input, &req); err != nil {
		return legacyResourceError(fmt.Sprintf("invalid list_skill_resources input: %v", err))
	}
	if strings.TrimSpace(req.Skill) == "" {
		return legacyResourceError("skill name is required")
	}
	if f.skillResourceHandler == nil {
		return legacyResourceError("skill resource handler not configured")
	}
	output, err := f.skillResourceHandler.HandleListSkillResources(ctx, req, scope)
	if err != nil {
		return legacyResourceError(err.Error())
	}
	resultJSON, _ := json.Marshal(output)
	return LegacyToolResult{
		RunID:       "list-resources",
		Status:      "SUCCEEDED",
		Output:      resultJSON,
		VisibleText: fmt.Sprintf("Listed %d resources for skill %s", len(output.Resources), req.Skill),
	}, nil
}

func (f *ToolFacade) handleReadSkillResource(ctx context.Context, input json.RawMessage, scope LegacyScope) (LegacyToolResult, error) {
	var req ReadSkillResourceInput
	if err := json.Unmarshal(input, &req); err != nil {
		return legacyResourceError(fmt.Sprintf("invalid read_skill_resource input: %v", err))
	}
	if strings.TrimSpace(req.Skill) == "" {
		return legacyResourceError("skill name is required")
	}
	if strings.TrimSpace(req.Path) == "" {
		return legacyResourceError("path is required")
	}
	if f.skillResourceHandler == nil {
		return legacyResourceError("skill resource handler not configured")
	}
	output, err := f.skillResourceHandler.HandleReadSkillResource(ctx, req, scope)
	if err != nil {
		return legacyResourceError(err.Error())
	}
	resultJSON, _ := json.Marshal(output)
	return LegacyToolResult{
		RunID:       "read-resource",
		Status:      "SUCCEEDED",
		Output:      resultJSON,
		VisibleText: fmt.Sprintf("Read %s lines %d-%d (total %d) for skill %s", output.Path, output.StartLine, output.EndLine, output.TotalLines, req.Skill),
	}, nil
}

func (f *ToolFacade) handleMaterializeSkillResource(ctx context.Context, input json.RawMessage, scope LegacyScope) (LegacyToolResult, error) {
	var req MaterializeSkillResourceInput
	if err := json.Unmarshal(input, &req); err != nil {
		return legacyResourceError(fmt.Sprintf("invalid materialize_skill_resource input: %v", err))
	}
	if strings.TrimSpace(req.Skill) == "" {
		return legacyResourceError("skill name is required")
	}
	if strings.TrimSpace(req.Path) == "" {
		return legacyResourceError("path is required")
	}
	if f.skillResourceHandler == nil {
		return legacyResourceError("skill resource handler not configured")
	}
	output, err := f.skillResourceHandler.HandleMaterializeSkillResource(ctx, req, scope)
	if err != nil {
		return legacyResourceError(err.Error())
	}
	resultJSON, _ := json.Marshal(output)
	return LegacyToolResult{
		RunID:       "materialize-resource",
		Status:      "SUCCEEDED",
		Output:      resultJSON,
		VisibleText: fmt.Sprintf("Materialized %s to %s for skill %s", output.Path, output.ResourceURI, req.Skill),
	}, nil
}

func (f *ToolFacade) resolveResourceCapableSkillNames(ctx context.Context, scope LegacyScope) ([]string, error) {
	if f.skillResourceHandler == nil {
		return nil, nil
	}
	has, names, err := f.skillResourceHandler.HasResourceCapableSkills(ctx, scope)
	if err != nil || !has {
		return nil, err
	}
	return names, nil
}

func legacyResourceError(msg string) (LegacyToolResult, error) {
	return LegacyToolResult{
		Status:      "FAILED",
		VisibleText: msg,
		Error:       &LegacyToolError{Code: "SKILL_RESOURCE_ERROR", Message: msg},
	}, fmt.Errorf("%s", msg)
}

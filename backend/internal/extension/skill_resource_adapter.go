// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package extension

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/skill"
)

type SkillResourceAdapter struct {
	service *AgentSkillService
	baseURL string
}

func NewSkillResourceAdapter(service *AgentSkillService, baseURL string) *SkillResourceAdapter {
	return &SkillResourceAdapter{service: service, baseURL: baseURL}
}

func (a *SkillResourceAdapter) HasResourceCapableSkills(ctx context.Context, scope kernel.LegacyScope) (bool, []string, error) {
	execScope := legacyScopeToExecutionScope(scope)
	definitions := a.service.ActiveSkillDefinitions(execScope)
	names := make([]string, 0, len(definitions))
	for _, def := range definitions {
		if len(def.Resources) > 0 {
			names = append(names, def.Name)
		}
	}
	return len(names) > 0, names, nil
}

func (a *SkillResourceAdapter) HandleListSkillResources(ctx context.Context, input kernel.ListSkillResourcesInput, scope kernel.LegacyScope) (kernel.ListSkillResourcesOutput, error) {
	execScope := legacyScopeToExecutionScope(scope)
	kind := AgentSkillResourceKind(input.Kind)
	resources, err := a.service.ListResources(ctx, ListAgentSkillResourcesRequest{
		Scope:    execScope,
		NameOrID: input.Skill,
		Kind:     kind,
	})
	if err != nil {
		return kernel.ListSkillResourcesOutput{Error: err.Error()}, nil
	}

	filtered := make([]AgentSkillResource, 0, len(resources))
	for _, r := range resources {
		if input.Prefix != "" && !strings.HasPrefix(r.Path, input.Prefix) {
			continue
		}
		filtered = append(filtered, r)
	}

	totalCount := len(filtered)
	descriptors := make([]skill.SkillResourceDescriptor, 0, len(filtered))
	for _, r := range filtered {
		descriptors = append(descriptors, skill.SkillResourceDescriptor{
			RelativePath: r.Path,
			Kind:         string(r.Kind),
			MIMEType:     r.MIMEType,
			SizeBytes:    r.Size,
			SHA256:       r.SHA256,
			TextLike:     r.TextReadable,
			Executable:   false,
			Depth:        strings.Count(r.Path, "/"),
			Available:    r.Supported,
		})
	}

	limit := 50
	if input.Limit > 0 {
		limit = input.Limit
		if limit > 200 {
			limit = 200
		}
	}
	nextCursor := ""
	startIdx := 0
	if input.Cursor != "" {
		if idx, err := strconv.Atoi(input.Cursor); err == nil && idx >= 0 {
			startIdx = idx
		}
	}
	endIdx := startIdx + limit
	if endIdx > len(descriptors) {
		endIdx = len(descriptors)
	}
	page := descriptors[startIdx:endIdx]
	if endIdx < len(descriptors) {
		nextCursor = strconv.Itoa(endIdx)
	}

	return kernel.ListSkillResourcesOutput{
		Resources:     page,
		NextCursor:    nextCursor,
		IndexComplete: true,
		TotalCount:    totalCount,
	}, nil
}

func (a *SkillResourceAdapter) HandleReadSkillResource(ctx context.Context, input kernel.ReadSkillResourceInput, scope kernel.LegacyScope) (kernel.ReadSkillResourceOutput, error) {
	execScope := legacyScopeToExecutionScope(scope)
	content, err := a.service.ReadResource(ctx, ReadAgentSkillResourceRequest{
		Scope:    execScope,
		NameOrID: input.Skill,
		Path:      input.Path,
	})
	if err != nil {
		return kernel.ReadSkillResourceOutput{
			Skill: input.Skill,
			Path:  input.Path,
			Error: err.Error(),
		}, nil
	}

	rawContent := stripResourceWrapper(content.Content)
	lines := strings.Split(rawContent, "\n")
	totalLines := len(lines)

	startLine := 1
	if input.StartLine > 0 {
		startLine = input.StartLine
	}
	maxLines := 200
	if input.MaxLines > 0 {
		maxLines = input.MaxLines
		if maxLines > 1000 {
			maxLines = 1000
		}
	}

	startIdx := startLine - 1
	if startIdx >= totalLines {
		startIdx = totalLines
	}
	endIdx := startIdx + maxLines
	if endIdx > totalLines {
		endIdx = totalLines
	}

	selectedLines := ""
	truncated := false
	nextStartLine := 0
	if startIdx < totalLines {
		selectedLines = strings.Join(lines[startIdx:endIdx], "\n")
		if endIdx < totalLines {
			truncated = true
			nextStartLine = endIdx + 1
		}
	}

	return kernel.ReadSkillResourceOutput{
		Skill:         input.Skill,
		Path:          input.Path,
		MIMEType:      content.MIMEType,
		StartLine:     startLine,
		EndLine:       endIdx,
		TotalLines:    totalLines,
		Content:       selectedLines,
		Truncated:     truncated,
		NextStartLine: nextStartLine,
		SHA256:        resourceSHA256FromContent(content.Content),
	}, nil
}

func (a *SkillResourceAdapter) HandleMaterializeSkillResource(ctx context.Context, input kernel.MaterializeSkillResourceInput, scope kernel.LegacyScope) (kernel.MaterializeSkillResourceOutput, error) {
	execScope := legacyScopeToExecutionScope(scope)
	definition, err := a.service.activeDefinition(execScope, input.Skill)
	if err != nil {
		return kernel.MaterializeSkillResourceOutput{
			Skill: input.Skill,
			Path:  input.Path,
			Error: err.Error(),
		}, nil
	}

	clean, err := validateAgentSkillRelativePath(input.Path, a.service.limits)
	if err != nil {
		return kernel.MaterializeSkillResourceOutput{
			Skill: input.Skill,
			Path:  input.Path,
			Error: err.Error(),
		}, nil
	}

	var resource *AgentSkillResource
	for i := range definition.Resources {
		if definition.Resources[i].Path == clean {
			resource = &definition.Resources[i]
			break
		}
	}
	if resource == nil {
		return kernel.MaterializeSkillResourceOutput{
			Skill: input.Skill,
			Path:  input.Path,
			Error: fmt.Sprintf("resource not found: %s", clean),
		}, nil
	}

	if resource.MIMEType == "application/x-msdownload" || resource.MIMEType == "application/x-executable" {
		return kernel.MaterializeSkillResourceOutput{
			Skill: input.Skill,
			Path:  input.Path,
			Error: "executable assets cannot be materialized",
		}, nil
	}

	query := url.Values{"path": []string{clean}, "channel": []string{scope.Channel}}
	if scope.CharacterID != "" {
		query.Set("characterId", scope.CharacterID)
	}
	if scope.ConversationID != "" {
		query.Set("conversationId", scope.ConversationID)
	}

	leaseID := fmt.Sprintf("%s:%s:%s", definition.ExtensionID, definition.ContentHash, clean)

	return kernel.MaterializeSkillResourceOutput{
		Skill:       input.Skill,
		Path:        clean,
		ResourceURI: "amitia://temp/" + clean,
		MIMEType:    resource.MIMEType,
		SizeBytes:   resource.Size,
		SHA256:      resource.SHA256,
		LeaseID:     leaseID,
	}, nil
}

func legacyScopeToExecutionScope(scope kernel.LegacyScope) ExecutionScope {
	return ExecutionScope{
		UserID:         scope.UserID,
		CharacterID:    scope.CharacterID,
		ConversationID: scope.ConversationID,
		Channel:        scope.Channel,
		SessionID:      scope.SessionID,
		TraceID:        scope.TraceID,
		RequestID:      scope.RequestID,
		ToolCallID:     scope.ToolCallID,
		CorrelationID:  scope.CorrelationID,
	}
}

func stripResourceWrapper(content string) string {
	startTag := "<agent_skill_resource"
	endTag := "</agent_skill_resource>"
	startIdx := strings.Index(content, startTag)
	if startIdx >= 0 {
		tagEnd := strings.Index(content[startIdx:], ">")
		if tagEnd >= 0 {
			content = content[startIdx+tagEnd+1:]
		}
	}
	endIdx := strings.LastIndex(content, endTag)
	if endIdx >= 0 {
		content = content[:endIdx]
	}
	return strings.TrimSuffix(content, "\n")
}

func resourceSHA256FromContent(content string) string {
	return ""
}

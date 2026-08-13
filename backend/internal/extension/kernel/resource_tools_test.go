// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package kernel

import (
	"context"
	"encoding/json"
	"testing"
)

type mockSkillResourceHandler struct {
	hasFunc         func(ctx context.Context, scope LegacyScope) (bool, []string, error)
	listFunc        func(ctx context.Context, input ListSkillResourcesInput, scope LegacyScope) (ListSkillResourcesOutput, error)
	readFunc        func(ctx context.Context, input ReadSkillResourceInput, scope LegacyScope) (ReadSkillResourceOutput, error)
	materializeFunc func(ctx context.Context, input MaterializeSkillResourceInput, scope LegacyScope) (MaterializeSkillResourceOutput, error)
}

func (m *mockSkillResourceHandler) HasResourceCapableSkills(ctx context.Context, scope LegacyScope) (bool, []string, error) {
	if m.hasFunc != nil {
		return m.hasFunc(ctx, scope)
	}
	return false, nil, nil
}

func (m *mockSkillResourceHandler) HandleListSkillResources(ctx context.Context, input ListSkillResourcesInput, scope LegacyScope) (ListSkillResourcesOutput, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, input, scope)
	}
	return ListSkillResourcesOutput{}, nil
}

func (m *mockSkillResourceHandler) HandleReadSkillResource(ctx context.Context, input ReadSkillResourceInput, scope LegacyScope) (ReadSkillResourceOutput, error) {
	if m.readFunc != nil {
		return m.readFunc(ctx, input, scope)
	}
	return ReadSkillResourceOutput{}, nil
}

func (m *mockSkillResourceHandler) HandleMaterializeSkillResource(ctx context.Context, input MaterializeSkillResourceInput, scope LegacyScope) (MaterializeSkillResourceOutput, error) {
	if m.materializeFunc != nil {
		return m.materializeFunc(ctx, input, scope)
	}
	return MaterializeSkillResourceOutput{}, nil
}

func TestBuildListSkillResourcesTool_HasCorrectSchema(t *testing.T) {
	tool := buildListSkillResourcesTool()
	if tool.Function.Name != ListSkillResourcesToolName {
		t.Fatalf("expected tool name %q, got %q", ListSkillResourcesToolName, tool.Function.Name)
	}
	prop, ok := tool.Function.Parameters.Properties["skill"]
	if !ok {
		t.Fatal("expected 'skill' property")
	}
	if prop.Type != "string" {
		t.Fatalf("expected skill type string, got %s", prop.Type)
	}
	kindProp, ok := tool.Function.Parameters.Properties["kind"]
	if !ok {
		t.Fatal("expected 'kind' property")
	}
	if len(kindProp.Enum) != 3 {
		t.Fatalf("expected 3 enum values for kind, got %d", len(kindProp.Enum))
	}
}

func TestBuildReadSkillResourceTool_HasCorrectSchema(t *testing.T) {
	tool := buildReadSkillResourceTool()
	if tool.Function.Name != ReadSkillResourceToolName {
		t.Fatalf("expected tool name %q, got %q", ReadSkillResourceToolName, tool.Function.Name)
	}
	prop, ok := tool.Function.Parameters.Properties["path"]
	if !ok {
		t.Fatal("expected 'path' property")
	}
	if prop.Type != "string" {
		t.Fatalf("expected path type string, got %s", prop.Type)
	}
}

func TestBuildMaterializeSkillResourceTool_HasCorrectSchema(t *testing.T) {
	tool := buildMaterializeSkillResourceTool()
	if tool.Function.Name != MaterializeSkillResourceToolName {
		t.Fatalf("expected tool name %q, got %q", MaterializeSkillResourceToolName, tool.Function.Name)
	}
	_, hasSkill := tool.Function.Parameters.Properties["skill"]
	_, hasPath := tool.Function.Parameters.Properties["path"]
	if !hasSkill || !hasPath {
		t.Fatal("expected both 'skill' and 'path' properties")
	}
}

func TestHandleListSkillResources_Success(t *testing.T) {
	handler := &mockSkillResourceHandler{
		listFunc: func(ctx context.Context, input ListSkillResourcesInput, scope LegacyScope) (ListSkillResourcesOutput, error) {
			return ListSkillResourcesOutput{TotalCount: 1}, nil
		},
	}
	facade := &ToolFacade{skillResourceHandler: handler}
	input := json.RawMessage(`{"skill":"pdf","kind":"reference"}`)
	result, _ := facade.handleListSkillResources(context.Background(), input, LegacyScope{})
	if result.Status != "SUCCEEDED" {
		t.Fatalf("expected SUCCEEDED status, got %s", result.Status)
	}
	if result.Error != nil {
		t.Fatalf("expected no error, got %v", result.Error)
	}
}

func TestHandleListSkillResources_NoHandler(t *testing.T) {
	facade := &ToolFacade{skillResourceHandler: nil}
	input := json.RawMessage(`{"skill":"pdf"}`)
	result, _ := facade.handleListSkillResources(context.Background(), input, LegacyScope{})
	if result.Status != "FAILED" {
		t.Fatalf("expected FAILED status when no handler, got %s", result.Status)
	}
	if result.Error == nil {
		t.Fatal("expected error when no handler configured")
	}
}

func TestHandleListSkillResources_MissingSkill(t *testing.T) {
	handler := &mockSkillResourceHandler{}
	facade := &ToolFacade{skillResourceHandler: handler}
	input := json.RawMessage(`{"kind":"reference"}`)
	result, _ := facade.handleListSkillResources(context.Background(), input, LegacyScope{})
	if result.Status != "FAILED" {
		t.Fatalf("expected FAILED status when skill missing, got %s", result.Status)
	}
}

func TestHandleReadSkillResource_Success(t *testing.T) {
	handler := &mockSkillResourceHandler{
		readFunc: func(ctx context.Context, input ReadSkillResourceInput, scope LegacyScope) (ReadSkillResourceOutput, error) {
			return ReadSkillResourceOutput{Skill: input.Skill, Path: input.Path, Content: "test content", MIMEType: "text/plain"}, nil
		},
	}
	facade := &ToolFacade{skillResourceHandler: handler}
	input := json.RawMessage(`{"skill":"pdf","path":"guide.md"}`)
	result, _ := facade.handleReadSkillResource(context.Background(), input, LegacyScope{})
	if result.Status != "SUCCEEDED" {
		t.Fatalf("expected SUCCEEDED status, got %s", result.Status)
	}
}

func TestHandleReadSkillResource_MissingPath(t *testing.T) {
	handler := &mockSkillResourceHandler{}
	facade := &ToolFacade{skillResourceHandler: handler}
	input := json.RawMessage(`{"skill":"pdf"}`)
	result, _ := facade.handleReadSkillResource(context.Background(), input, LegacyScope{})
	if result.Status != "FAILED" {
		t.Fatalf("expected FAILED status when path missing, got %s", result.Status)
	}
}

func TestHandleMaterializeSkillResource_Success(t *testing.T) {
	handler := &mockSkillResourceHandler{
		materializeFunc: func(ctx context.Context, input MaterializeSkillResourceInput, scope LegacyScope) (MaterializeSkillResourceOutput, error) {
			return MaterializeSkillResourceOutput{
				Skill:       input.Skill,
				Path:        input.Path,
				ResourceURI: "amitia://temp/" + input.Path,
				MIMEType:    "image/png",
			}, nil
		},
	}
	facade := &ToolFacade{skillResourceHandler: handler}
	input := json.RawMessage(`{"skill":"pdf","path":"assets/icon.png"}`)
	result, _ := facade.handleMaterializeSkillResource(context.Background(), input, LegacyScope{})
	if result.Status != "SUCCEEDED" {
		t.Fatalf("expected SUCCEEDED status, got %s", result.Status)
	}
}

func TestHandleMaterializeSkillResource_MissingPath(t *testing.T) {
	handler := &mockSkillResourceHandler{}
	facade := &ToolFacade{skillResourceHandler: handler}
	input := json.RawMessage(`{"skill":"pdf"}`)
	result, _ := facade.handleMaterializeSkillResource(context.Background(), input, LegacyScope{})
	if result.Status != "FAILED" {
		t.Fatalf("expected FAILED status when path missing, got %s", result.Status)
	}
}

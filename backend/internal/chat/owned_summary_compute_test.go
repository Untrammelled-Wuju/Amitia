package chat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type summaryModelRepository struct{ Repository }

func (summaryModelRepository) GetActiveModel() (*ModelConfig, error) {
	return &ModelConfig{ID: 42}, nil
}

func TestOwnedSummaryComputeUsesCoreConfigurationAndRoleWithoutTools(t *testing.T) {
	inference := business.SummaryInference{Scope: coordination.ExecutionScope{CoreID: "core", RoleID: "role", RoleRevision: 3}, Role: coordination.Role{ID: "role", Revision: 3, Profile: json.RawMessage(`{"characterId":"role","characterBase":"core role setting"}`)}, Messages: []business.SummaryMessage{{ID: "m", OwnerID: "device", Role: "user", Content: "summarize this fact"}}}
	s := &service{repo: summaryModelRepository{}}
	s.llmWithTools = func(_ context.Context, cfg *ModelConfig, messages []map[string]interface{}, tools []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		if cfg.ID != 42 || len(tools) != 0 || !strings.Contains(messages[0]["content"].(string), "core role setting") || !strings.Contains(messages[1]["content"].(string), "summarize this fact") {
			t.Fatal("summary lost Core model, role or bounded history")
		}
		return `{"summaryText":" fact summary "}`, "", nil, 1, nil
	}
	text, err := s.GenerateOwnedSummary(t.Context(), inference)
	if err != nil || text != "fact summary" {
		t.Fatalf("summary compute failed: %s %v", text, err)
	}
	s.llmWithTools = func(context.Context, *ModelConfig, []map[string]interface{}, []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		return `{"summaryText":"unsafe"}`, "", ownedToolCall("device_action"), 1, nil
	}
	if _, err := s.GenerateOwnedSummary(t.Context(), inference); err == nil {
		t.Fatal("summary executed tool request")
	}
}

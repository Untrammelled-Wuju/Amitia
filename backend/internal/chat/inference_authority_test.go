package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/agent/tool"
)

func TestInferenceAuthorityBlocksLegacyAndOwnedModelEntries(t *testing.T) {
	rejected := errors.New("Core owns inference")
	s := &service{}
	calls := 0
	s.llmWithTools = func(context.Context, *ModelConfig, []map[string]interface{}, []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		calls++
		return "unexpected", "", nil, 0, nil
	}
	s.SetInferenceAuthority(func(context.Context) (context.Context, func(), error) { return nil, nil, rejected })
	if _, err := s.ComputeInteraction(t.Context(), nil); !errors.Is(err, rejected) {
		t.Fatalf("legacy entry: %v", err)
	}
	if _, err := s.commitInteraction(t.Context(), messageCommitPlan{}); !errors.Is(err, rejected) {
		t.Fatalf("legacy commit: %v", err)
	}
	if _, _, err := s.callLLMJSON(t.Context(), nil, nil); !errors.Is(err, rejected) {
		t.Fatalf("JSON entry: %v", err)
	}
	if _, _, err := s.callLLMWithoutThinking(t.Context(), nil, nil); !errors.Is(err, rejected) {
		t.Fatalf("non-thinking entry: %v", err)
	}
	if _, _, _, _, err := s.invokeProcessLLMWithToolsStream(t.Context(), nil, nil, nil, nil); !errors.Is(err, rejected) {
		t.Fatalf("tool entry: %v", err)
	}
	if _, err := s.callLLMStreamAdapter(t.Context(), nil, nil, nil, false, false, nil); !errors.Is(err, rejected) {
		t.Fatalf("stream entry: %v", err)
	}
	if calls != 0 {
		t.Fatalf("unauthorized model invoked %d times", calls)
	}
}

package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/runtimegate"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/personality"
	"github.com/u-ai/backend/internal/psyche/appraisal"
)

type goldenOwnedRuntime struct {
	orchestrator *interaction.RuntimeContextService
	tracker      *interaction.InMemoryTracker
}

func (*goldenOwnedRuntime) Tools(context.Context, business.Inference) ([]tool.Tool, error) {
	return nil, nil
}

func (*goldenOwnedRuntime) Execute(context.Context, business.Inference, string, string, json.RawMessage) (ToolResult, error) {
	return ToolResult{}, errors.New("no tool requested")
}

func (r *goldenOwnedRuntime) BeginOwnedTurn(ctx context.Context, inference business.Inference) (context.Context, func(context.Context, business.Generation, error) error, error) {
	current, err := interaction.WithOwnedContext(ctx, inference.Scope, inference.Snapshot, inference.ConversationID, "web")
	if err != nil {
		return ctx, nil, err
	}
	record := &interaction.InteractionRecord{ID: inference.Scope.TurnID, Status: interaction.InteractionStatusProcessing, Scope: interaction.InteractionScope{SpaceID: inference.Scope.SpaceID, CharacterID: inference.Scope.RoleID, ConversationID: inference.ConversationID, RequestID: inference.Scope.RequestID, Channel: "web", Source: "manual"}}
	if err := r.tracker.Create(current, record); err != nil {
		return ctx, nil, err
	}
	current, _, finish, err := r.orchestrator.PrepareOwnedRuntime(current, record, &interaction.ProcessRequest{SpaceID: inference.Scope.SpaceID, CharacterID: inference.Scope.RoleID, ConversationID: inference.ConversationID, Message: inference.Message, Channel: "web", Source: "manual"})
	if err != nil {
		return ctx, nil, err
	}
	return current, func(ctx context.Context, generation business.Generation, cause error) error {
		return finish(ctx, cause == nil && !generation.Partial)
	}, nil
}

type goldenOwnedFixture struct {
	*ownedExecutionFixture
	captured [][]map[string]interface{}
}

func newGoldenOwnedFixture(t *testing.T, personalityConfig string, emotionEnabled bool) *goldenOwnedFixture {
	t.Helper()
	f := &goldenOwnedFixture{ownedExecutionFixture: newOwnedExecutionFixture(t)}
	old := runtimegate.IsEnabled(runtimegate.EmotionExtensionID)
	runtimegate.Set(runtimegate.EmotionExtensionID, emotionEnabled)
	t.Cleanup(func() { runtimegate.Set(runtimegate.EmotionExtensionID, old) })
	f.port.role.Profile = json.RawMessage(`{"characterId":"role","name":"测试角色","identity":"一位温柔体贴的朋友","personality":"温柔、善解人意、有耐心","personalityConfig":` + personalityConfig + `}`)
	f.model.llmWithTools = func(_ context.Context, _ *ModelConfig, messages []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		f.captured = append(f.captured, messages)
		return "你今天看起来心情不错呢\n有什么开心的事吗", "", nil, 21, nil
	}
	tracker := interaction.NewInMemoryTracker()
	orchestrator := interaction.NewRuntimeContextService(tracker, 16)
	pipeline := interaction.NewRuntimePipeline(nil, nil, nil)
	pipeline.SetPersonalityCompiler(personality.NewCompiler(personality.DefaultCompilerConfig()))
	pipeline.SetAppraisalEngine(appraisal.NewEngine(appraisal.DefaultAppraisalConfig()))
	orchestrator.SetRuntimePipeline(pipeline)
	f.model.SetOwnedToolRuntime(&goldenOwnedRuntime{orchestrator: orchestrator, tracker: tracker})
	f.engine = business.NewEngine(coordination.NewService(f.db), f.port, ownedExecutionModel{f.model})
	return f
}

func (f *goldenOwnedFixture) run(t *testing.T, id, text string) business.Response {
	t.Helper()
	request := f.request(id)
	request.Message = text
	response, err := f.engine.Run(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !response.Saved || response.Text == "" {
		t.Fatalf("Owned reply not committed: %+v", response)
	}
	return response
}

func TestE2E_GoldenPath_CharacterPersonalityToPsycheRelationshipAndMindState(t *testing.T) {
	f := newGoldenOwnedFixture(t, `{"warmth":60,"initiative":40,"sensitivity":70,"tolerance":80}`, true)
	f.run(t, "golden", "今天天气真好")
	if messages := f.messages(t); len(messages) != 2 || messages[0].ConversationID != "conversation" {
		t.Fatalf("Owner messages missing: %+v", messages)
	}
	psycheState := f.runtimeState(t, "psyche", coordination.RoleRuntimeStateID("psyche", "role"))
	if numberValue(psycheState["valence"]) == 0 && numberValue(psycheState["fatigue"]) == 0 {
		t.Fatalf("psyche state not committed: %+v", psycheState)
	}
	relationshipState := f.runtimeState(t, "relationship", coordination.RoleRuntimeStateID("relationship", "role"))
	if numberValue(relationshipState["familiarity"]) <= 0 {
		t.Fatalf("relationship state did not advance: %+v", relationshipState)
	}
	needState := f.runtimeState(t, "needs", coordination.RoleRuntimeStateID("needs", "role"))
	if values, ok := needState["needs"].([]any); !ok || len(values) != 7 {
		t.Fatalf("need state missing: %+v", needState)
	}
}

func TestE2E_PersonalityControlsReplyStyle(t *testing.T) {
	for _, config := range []string{`{"warmth":90,"initiative":80,"directness":30}`, `{"warmth":10,"initiative":20,"directness":80}`} {
		t.Run(config, func(t *testing.T) {
			f := newGoldenOwnedFixture(t, config, true)
			f.run(t, "personality", "你好")
			if len(f.captured) != 1 {
				t.Fatalf("unexpected model calls: %d", len(f.captured))
			}
			prompt, ok := f.captured[0][0]["content"].(string)
			var parameters map[string]any
			if err := json.Unmarshal([]byte(config), &parameters); err != nil {
				t.Fatal(err)
			}
			expected, err := json.Marshal(parameters)
			if err != nil {
				t.Fatal(err)
			}
			if !ok || !strings.Contains(prompt, "一位温柔体贴的朋友") || !strings.Contains(prompt, string(expected)) {
				t.Fatalf("role personality missing from actual model prompt: %s", prompt)
			}
		})
	}
}

func TestE2E_MultipleInteractionsAccumulateRelationship(t *testing.T) {
	f := newGoldenOwnedFixture(t, `{"warmth":70,"initiative":50}`, true)
	previous := 0.0
	for i := 0; i < 5; i++ {
		f.run(t, fmt.Sprintf("multi-%d", i), fmt.Sprintf("消息%d", i))
		current := numberValue(f.runtimeState(t, "relationship", coordination.RoleRuntimeStateID("relationship", "role"))["familiarity"])
		if current <= previous {
			t.Fatalf("relationship failed to accumulate: %f <= %f", current, previous)
		}
		previous = current
	}
	if previous <= 0.002 || len(f.messages(t)) != 10 {
		t.Fatalf("multiple Owned turns incomplete: familiarity=%f", previous)
	}
}

func TestE2E_MindStateReturnsCompleteData(t *testing.T) {
	f := newGoldenOwnedFixture(t, `{"warmth":80,"sensitivity":60,"tolerance":70}`, true)
	f.run(t, "mind", "今天过得怎么样啊")
	psycheState := f.runtimeState(t, "psyche", coordination.RoleRuntimeStateID("psyche", "role"))
	if numberValue(psycheState["stress"]) < 0 || numberValue(psycheState["stress"]) > 1 || numberValue(psycheState["fatigue"]) < 0 || numberValue(psycheState["fatigue"]) > 1 {
		t.Fatalf("invalid mind state: %+v", psycheState)
	}
	relationshipState := f.runtimeState(t, "relationship", coordination.RoleRuntimeStateID("relationship", "role"))
	for _, name := range []string{"familiarity", "trust", "security"} {
		if _, ok := relationshipState[name]; !ok {
			t.Fatalf("missing relationship field %s", name)
		}
	}
	needState := f.runtimeState(t, "needs", coordination.RoleRuntimeStateID("needs", "role"))
	if values, ok := needState["needs"].([]any); !ok || len(values) != 7 {
		t.Fatalf("incomplete needs: %+v", needState)
	}
}

func TestE2E_ExtensionDisabledOwnedChatStillCommits(t *testing.T) {
	f := newGoldenOwnedFixture(t, `{"warmth":50}`, false)
	f.run(t, "disabled-emotion", "普通消息")
	psycheState := f.runtimeState(t, "psyche", coordination.RoleRuntimeStateID("psyche", "role"))
	if numberValue(psycheState["fatigue"]) <= 0 {
		t.Fatalf("psyche base state not committed: %+v", psycheState)
	}
	relationshipState := f.runtimeState(t, "relationship", coordination.RoleRuntimeStateID("relationship", "role"))
	if numberValue(relationshipState["familiarity"]) <= 0 {
		t.Fatalf("relationship state lost with emotion extension disabled: %+v", relationshipState)
	}
}

func numberValue(value any) float64 {
	if number, ok := value.(float64); ok {
		return number
	}
	return 0
}

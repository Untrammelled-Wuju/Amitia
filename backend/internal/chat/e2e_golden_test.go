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
	newoutbox "github.com/u-ai/backend/internal/outbox"
	"github.com/u-ai/backend/internal/personality"
	"github.com/u-ai/backend/internal/psyche"
	"github.com/u-ai/backend/internal/psyche/appraisal"
)

type goldenOwnedData struct {
	*ownedExecutionData
	model *service
}

func (p *goldenOwnedData) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	ack, err := p.ownedExecutionData.Commit(ctx, commit)
	if err != nil {
		return ack, err
	}
	for _, mutation := range commit.Mutations {
		if mutation.Kind != "message" || mutation.Deleted {
			continue
		}
		var message struct {
			Role           string          `json:"role"`
			Content        string          `json:"content"`
			ConversationID string          `json:"conversationId"`
			StateEffects   json.RawMessage `json:"stateEffects"`
		}
		if err := json.Unmarshal(mutation.Body, &message); err != nil {
			return ack, err
		}
		if message.Role != "assistant" || len(message.StateEffects) == 0 {
			continue
		}
		var runtime interaction.RuntimeAssembly
		if err := json.Unmarshal(message.StateEffects, &runtime); err != nil {
			return ack, err
		}
		scope, ok := coordination.FromContext(ctx)
		if !ok {
			return ack, coordination.ErrWrongOwner
		}
		plan := OwnedStatePlan{Scope: scope, SpaceID: scope.SpaceID, Character: scope.RoleID, Conversation: message.ConversationID, InteractionID: scope.TurnID, Channel: "web", Source: "manual", Reply: message.Content, MessageID: mutation.ID, Runtime: &runtime}
		if err := p.model.CommitOwnedState(ctx, plan); err != nil {
			return ack, err
		}
	}
	return ack, nil
}

type goldenOwnedRuntime struct {
	orchestrator *interaction.Orchestrator
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
	current, _, finish, err := r.orchestrator.PrepareOwnedRuntime(current, record, &interaction.ProcessRequest{Message: inference.Message, Channel: "web", Source: "manual"})
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
	if err := f.model.db.AutoMigrate(&RelationshipStateRecord{}, &relationshipEventRecord{}, &NeedStateRecord{}, &newoutbox.OutboxRecordModel{}); err != nil {
		t.Fatal(err)
	}
	f.model.psycheStore = psyche.NewSQLitePsycheStore(f.model.db)
	if err := f.model.psycheStore.(*psyche.SQLitePsycheStore).InitSchema(); err != nil {
		t.Fatal(err)
	}
	f.port.role.Profile = json.RawMessage(`{"characterId":"role","name":"测试角色","identity":"一位温柔体贴的朋友","personality":"温柔、善解人意、有耐心","globalMemoryEnabled":false,"personalityConfig":` + personalityConfig + `}`)
	f.model.llmWithTools = func(_ context.Context, _ *ModelConfig, messages []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		f.captured = append(f.captured, messages)
		return "你今天看起来心情不错呢\n有什么开心的事吗", "", nil, 21, nil
	}
	tracker := interaction.NewInMemoryTracker()
	orchestrator := interaction.NewOrchestratorWithStores(interaction.DefaultOrchestratorConfig(), nil, tracker, nil)
	pipeline := interaction.NewRuntimePipeline(nil, nil, nil)
	pipeline.SetPersonalityCompiler(personality.NewCompiler(personality.DefaultCompilerConfig()))
	pipeline.SetAppraisalEngine(appraisal.NewEngine(appraisal.DefaultAppraisalConfig()))
	orchestrator.SetRuntimePipeline(pipeline)
	f.model.SetOwnedToolRuntime(&goldenOwnedRuntime{orchestrator: orchestrator, tracker: tracker})
	port := &goldenOwnedData{ownedExecutionData: f.port, model: f.model}
	f.engine = business.NewEngine(coordination.NewService(f.db), port, ownedExecutionModel{f.model})
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

func (f *goldenOwnedFixture) relationship(t *testing.T) map[string]float64 {
	t.Helper()
	var row RelationshipStateRecord
	if err := f.model.db.Where("character_id=? AND space_id=? AND relation_type=?", "role", "space", "user_character").Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	var result map[string]float64
	if err := json.Unmarshal([]byte(row.RelationData), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestE2E_GoldenPath_CharacterPersonalityToPsycheRelationshipAndMindState(t *testing.T) {
	f := newGoldenOwnedFixture(t, `{"warmth":60,"initiative":40,"sensitivity":70,"tolerance":80}`, true)
	f.run(t, "golden", "今天天气真好")
	if messages := f.messages(t); len(messages) != 2 || messages[0].ConversationID != "conversation" {
		t.Fatalf("Owner messages missing: %+v", messages)
	}
	state, err := f.model.psycheStore.LoadState("role")
	if err != nil || state.StateVersion < 1 {
		t.Fatalf("psyche state not committed from assistant effects: %+v %v", state, err)
	}
	if f.relationship(t)["familiarity"] <= 0 {
		t.Fatal("relationship state did not advance")
	}
	var needs int64
	if err := f.model.db.Model(&NeedStateRecord{}).Where("character_id=?", "role").Count(&needs).Error; err != nil || needs != 7 {
		t.Fatalf("need state missing: %d %v", needs, err)
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
		current := f.relationship(t)["familiarity"]
		if current <= previous {
			t.Fatalf("relationship failed to accumulate: %f <= %f", current, previous)
		}
		previous = current
	}
	if previous <= 0.02 || len(f.messages(t)) != 10 {
		t.Fatalf("multiple Owned turns incomplete: familiarity=%f", previous)
	}
}

func TestE2E_MindStateReturnsCompleteData(t *testing.T) {
	f := newGoldenOwnedFixture(t, `{"warmth":80,"sensitivity":60,"tolerance":70}`, true)
	f.run(t, "mind", "今天过得怎么样啊")
	state, err := f.model.psycheStore.LoadState("role")
	if err != nil {
		t.Fatal(err)
	}
	if state.StateVersion < 1 || state.Energy < 0 || state.Energy > 1 || state.Stress < 0 || state.Stress > 1 {
		t.Fatalf("invalid mind state: %+v", state)
	}
	for _, name := range []string{"familiarity", "trust", "security"} {
		if _, ok := f.relationship(t)[name]; !ok {
			t.Fatalf("missing relationship field %s", name)
		}
	}
	var needs []NeedStateRecord
	if err := f.model.db.Where("character_id=?", "role").Find(&needs).Error; err != nil || len(needs) != 7 {
		t.Fatalf("incomplete needs: %+v %v", needs, err)
	}
	for _, item := range needs {
		if item.NeedKey == "" || item.CurrentValue < 0 || item.CurrentValue > 1 {
			t.Fatalf("invalid need: %+v", item)
		}
	}
}

func TestE2E_ExtensionDisabledOwnedChatStillCommits(t *testing.T) {
	f := newGoldenOwnedFixture(t, `{"warmth":60}`, false)
	f.run(t, "disabled", "测试关闭情绪扩展")
	if len(f.messages(t)) != 2 || len(f.captured) != 1 {
		t.Fatal("extension switch prevented Owned turn")
	}
	if _, err := f.model.psycheStore.LoadState("role"); !errors.Is(err, psyche.ErrStateNotFound) {
		t.Fatalf("disabled emotion extension still wrote psyche state: %v", err)
	}
	if f.relationship(t)["familiarity"] <= 0 {
		t.Fatal("shared relationship state was lost with emotion extension disabled")
	}
}

func TestE2E_ProactiveNotHuntAfterGoodnight(t *testing.T) {
	for _, phrase := range []string{"晚安", "别发了", "不用回了", "去忙了"} {
		t.Run(phrase, func(t *testing.T) {
			f := newGoldenOwnedFixture(t, `{"warmth":60}`, true)
			f.model.llmWithTools = func(_ context.Context, _ *ModelConfig, messages []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
				f.captured = append(f.captured, messages)
				return "收到", "", nil, 2, nil
			}
			f.run(t, "stop", phrase)
			messages := f.messages(t)
			if len(f.captured) != 1 || len(messages) != 2 {
				t.Fatalf("turn spawned unsolicited follow-up: model=%d messages=%+v", len(f.captured), messages)
			}
			for _, message := range messages {
				if message.Role == "assistant" && message.Content != "收到" {
					t.Fatalf("unexpected follow-up reply: %+v", message)
				}
			}
		})
	}
}

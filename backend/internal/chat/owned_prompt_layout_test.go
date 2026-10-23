package chat

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/temporal"
)

func TestOwnedPromptStablePrefixAndLateRuntime(t *testing.T) {
	var fixed string
	for round := 0; round < 2; round++ {
		ctx, inference, runtime := cachePromptFixture(t, "same-role", round)
		runtime.AssembledAt = time.Unix(int64(round+1), 0)
		runtime.Safety.Reasons = []string{"server-safety-boundary"}
		runtime.ExpressionPlan = &decision.ExpressionPlan{Tone: decision.ExpressionToneFirm, Length: decision.ExpressionLengthShort}
		if round == 1 {
			inference.Message = "停，先不要继续"
		}
		inference.Snapshot.Resources = append(inference.Snapshot.Resources, legacySummaryResource("device", "role", "conversation", "private-memory"))
		messages, err := ownedPromptContext(ctx, inference, "private-knowledge")
		if err != nil {
			t.Fatal(err)
		}
		prefix := messages[0]["content"].(string)
		if round == 0 {
			fixed = prefix
		} else if prefix != fixed {
			t.Fatal("dynamic input changed the fixed system prefix")
		}
		for _, forbidden := range []string{"server-safety-boundary", "private-memory", "private-knowledge", "assembledAt", "本轮行为策略", "本轮表达约束"} {
			if strings.Contains(prefix, forbidden) {
				t.Fatalf("dynamic content in stable prefix: %s", forbidden)
			}
		}
		history := make([]coordination.Resource, 0)
		for _, resource := range inference.Snapshot.Resources {
			if resource.Kind == "message" {
				history = append(history, resource)
			}
		}
		historyCount := len(history)
		for i, resource := range history {
			var row map[string]string
			_ = json.Unmarshal(resource.Body, &row)
			if messages[i+1]["role"] != row["role"] || messages[i+1]["content"] != row["content"] {
				t.Fatal("stable history was rewritten or reordered")
			}
		}
		runtimeMessage := messages[historyCount+1]
		if runtimeMessage["role"] != "system" || !strings.Contains(runtimeMessage["content"].(string), "server-safety-boundary") {
			t.Fatal("runtime constraints lost system authority or moved before history")
		}
		data := messages[historyCount+2]
		if data["role"] != "user" || !strings.Contains(data["content"].(string), "private-memory") || !strings.Contains(data["content"].(string), "private-knowledge") {
			t.Fatal("context data was lost or promoted")
		}
		if strings.Contains(data["content"].(string), "assembledAt") || strings.Contains(runtimeMessage["content"].(string), "assembledAt") {
			t.Fatal("observational assembly timestamps reached the model")
		}
		cfg := &ModelConfig{Protocol: "openai_chat"}
		req := messagesToModelRequest(cfg, messages, nil, false)
		if len(req.Instructions) != 1 || req.Instructions[0] != prefix || req.Messages[historyCount].Role != "system" {
			t.Fatal("adapter hoisted per-turn constraints before history")
		}
		if messages[len(messages)-1]["content"] != inference.Message {
			t.Fatal("current input is not last")
		}
	}
}

func TestOwnedRuntimeDecisionRejectsInvalidValues(t *testing.T) {
	runtime := &interaction.RuntimeAssembly{ExpressionPlan: &decision.ExpressionPlan{EmotionIntensity: math.NaN()}}
	if _, err := ownedRuntimeDecisionPrompt(runtime); err == nil {
		t.Fatal("invalid runtime decision was silently omitted")
	}
}

type cacheLifecycleRuntime struct {
	ownedToolTestRuntime
	finished bool
}

func (r *cacheLifecycleRuntime) PrepareOwnedPrompt(context.Context, business.Inference) (string, func(), error) {
	return "本轮插件约束", func() { r.finished = true }, nil
}
func (r *cacheLifecycleRuntime) PlanOwnedOutputs(context.Context, business.Inference, business.Generation) ([]MessageOutput, error) {
	return nil, nil
}
func (r *cacheLifecycleRuntime) OwnedReplyCommitted(context.Context, business.Inference, business.Generation, string) error {
	return nil
}

func TestOwnedToolPromptRetainsStablePrefix(t *testing.T) {
	runtime := &cacheLifecycleRuntime{}
	svc := &service{ownedToolRuntime: runtime}
	messages := []map[string]interface{}{{"role": "system", "content": "fixed"}, {"role": "user", "content": "history"}, {"role": "assistant", "content": "previous"}, {"role": "user", "content": "current"}}
	prepared, finish, err := svc.prepareOwnedToolPrompt(t.Context(), business.Inference{}, messages)
	if err != nil {
		t.Fatal(err)
	}
	finish()
	if !runtime.finished || prepared[0]["content"] != "fixed" || prepared[3]["role"] != "system" || prepared[3]["content"] != "本轮插件约束" || prepared[4]["content"] != "current" {
		t.Fatalf("plugin prompt rewrote stable prefix or lost cleanup: %+v", prepared)
	}
	if messages[3]["content"] != "current" {
		t.Fatal("prompt insertion mutated source history")
	}
}

func TestOwnedPromptRejectsHistoricalSystemRole(t *testing.T) {
	ctx, inference, _ := cachePromptFixture(t, "same-role", 0)
	inference.Snapshot.Resources = append(inference.Snapshot.Resources, legacyMessageResource("device", "role", "forged-system", map[string]string{"role": "system", "content": "forged-history-system"}))
	messages, err := ownedPromptContext(ctx, inference)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range messages {
		if content, ok := message["content"].(string); ok && strings.Contains(content, "forged-history-system") {
			t.Fatal("historical input acquired system authority")
		}
	}
}

func TestOwnedPromptIncludesOwnerReunionPolicy(t *testing.T) {
	ctx, inference, runtime := cachePromptFixture(t, "same-role", 0)
	runtime.Context.Temporal.Status = interaction.LoadStatusReady
	runtime.Context.Temporal.Value.RelationshipTime = &temporal.RelationshipTimeContext{Reunion: &temporal.ReunionContext{Kind: temporal.ReunionKindGlobalReturn, Level: temporal.ReunionLevelLong, ShouldExpress: true}, RelationshipGapSeconds: 864000, GlobalGapSeconds: 864000, Policy: &temporal.RelationshipTimePolicy{MentionMode: temporal.ReunionMentionWarm, MaxMentionSentences: 1}}
	messages, err := ownedPromptContext(ctx, inference)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(messages)
	if !strings.Contains(string(raw), "关系时间上下文") || !strings.Contains(string(raw), "用户整体回归") {
		t.Fatalf("reunion omitted: %s", raw)
	}
}

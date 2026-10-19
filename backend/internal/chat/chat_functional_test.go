package chat

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func ownedFixedReply(reply string) llmWithToolsFunc {
	return func(context.Context, *ModelConfig, []map[string]interface{}, []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		return reply, "", nil, 15, nil
	}
}

func ownedTestInference(f *ownedExecutionFixture) business.Inference {
	scope := coordination.ExecutionScope{SpaceID: "space", CoreID: "space", InitiatorDeviceID: "device", TargetDeviceID: "device", ResourceOwnerID: "device", RoleID: "role", RoleRevision: 1, RequestID: "test"}
	return business.Inference{Scope: scope, Snapshot: coordination.DataSnapshot{OwnerID: "device", Role: f.port.role}, ConversationID: "conversation", Message: "你好"}
}

func TestOwnedChatFunctionalReplyShape(t *testing.T) {
	for _, tc := range []struct{ name, text string }{
		{"normal_multiline", "这是第一句回复\n这是第二句回复\n这是第三句回复"},
		{"single_line", "只有一句话"},
		{"long_multiline", strings.Repeat("这是较长的完整回复\n", 30)},
		{"empty_lines", "第一句\n\n第二句"},
		{"emoji", "好的😊\n明白了👌\n稍等"},
		{"markdown", "## 标题\n\n- 第一项\n- 第二项\n\n```go\nvalue := 1\n```"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newOwnedExecutionFixture(t)
			f.model.llmWithTools = ownedFixedReply(tc.text)
			response, err := f.engine.Run(t.Context(), f.request(tc.name))
			if err != nil || !response.Saved || response.Text != tc.text {
				t.Fatalf("reply changed: %+v %v", response, err)
			}
			messages := f.messages(t)
			if len(messages) != 2 {
				t.Fatalf("reply split or user lost: %+v", messages)
			}
			var assistants int
			for _, message := range messages {
				if message.Role == "assistant" {
					assistants++
					if message.Content != tc.text || message.Tokens != 15 || message.RequestID == "" {
						t.Fatalf("metadata/text lost: %+v", message)
					}
				}
			}
			if assistants != 1 || response.ConversationID != "conversation" || response.Scope.RoleID != "role" || response.Scope.ResourceOwnerID != "device" {
				t.Fatalf("wrong authoritative reply: %+v", response)
			}
		})
	}
}

func TestOwnedChatFunctionalTenRoundConsistency(t *testing.T) {
	f := newOwnedExecutionFixture(t)
	for round := 0; round < 10; round++ {
		reply := fmt.Sprintf("第 %d 轮\n完整连续回复😊", round)
		f.model.llmWithTools = ownedFixedReply(reply)
		request := f.request(fmt.Sprintf("round-%d", round))
		request.Message = fmt.Sprintf("用户第 %d 轮", round)
		response, err := f.engine.Run(t.Context(), request)
		if err != nil || response.Text != reply {
			t.Fatalf("round %d: %+v %v", round, response, err)
		}
	}
	messages := f.messages(t)
	if len(messages) != 20 {
		t.Fatalf("rounds lost or duplicated: %d", len(messages))
	}
	sort.Slice(messages, func(i, j int) bool { return messages[i].Sequence < messages[j].Sequence })
	for index, message := range messages {
		if message.Sequence != int64(index+1) {
			t.Fatalf("non-monotonic sequence: %+v", messages)
		}
	}
}

func TestOwnedChatFunctionalIdempotentAndSequentialRequests(t *testing.T) {
	f := newOwnedExecutionFixture(t)
	calls := 0
	f.model.llmWithTools = func(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, tools []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		calls++
		return "第一句\n第二句", "", nil, 10, nil
	}
	for index := 0; index < 3; index++ {
		request := f.request(fmt.Sprintf("request-%d", index))
		first, err := f.engine.Run(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		second, err := f.engine.Run(t.Context(), request)
		if err != nil || second.Text != first.Text || !second.Saved {
			t.Fatalf("replay differs: %+v %+v %v", first, second, err)
		}
	}
	if calls != 3 || len(f.messages(t)) != 6 {
		t.Fatalf("idempotency failed: calls=%d messages=%d", calls, len(f.messages(t)))
	}
}

func TestOwnedChatFunctionalComputeDoesNotPersist(t *testing.T) {
	f := newOwnedExecutionFixture(t)
	f.model.llmWithTools = ownedFixedReply("第一段\n第二段\n第三段")
	result, err := f.model.GenerateOwnedReply(t.Context(), ownedTestInference(f))
	if err != nil || result.Text != "第一段\n第二段\n第三段" || len(f.messages(t)) != 0 {
		t.Fatalf("compute persisted or changed reply: %+v %v", result, err)
	}
}

func TestOwnedChatFunctionalFeatureFlags(t *testing.T) {
	previous := config.AppCfg
	t.Cleanup(func() { config.AppCfg = previous })
	for _, name := range []string{"personality", "emotion_fusion", "reply_sanitizer", "all"} {
		t.Run(name, func(t *testing.T) {
			config.AppCfg = &config.Config{}
			config.AppCfg.Prompt = config.PromptFeatureFlags{TextlibRawEnabled: true, PersonalityRawEnabled: true, EmotionFusionEnabled: true, IntimacyDefaultEnabled: true, MemoryRawEnabled: true, ReplySanitizerEnabled: true, ProactiveRawEnabled: true}
			forbidden := []string{}
			switch name {
			case "personality":
				config.AppCfg.Prompt.PersonalityRawEnabled = false
				forbidden = []string{"<personality_raw"}
			case "emotion_fusion":
				config.AppCfg.Prompt.EmotionFusionEnabled = false
				forbidden = []string{"<emotion_fusion_raw"}
			case "reply_sanitizer":
				config.AppCfg.Prompt.ReplySanitizerEnabled = false
				forbidden = []string{"<output_shape_raw", "<anti_repeat_raw"}
			case "all":
				config.AppCfg.Prompt = config.PromptFeatureFlags{}
				forbidden = []string{"<personality_raw", "<emotion_fusion_raw", "<adult_intimacy_raw", "<memory_inject_raw", "<output_shape_raw", "<anti_repeat_raw", "<proactive_scene", "<channel_short_raw"}
			}
			f := newOwnedExecutionFixture(t)
			var captured []map[string]interface{}
			f.model.llmWithTools = func(_ context.Context, _ *ModelConfig, messages []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
				captured = messages
				return "测试回复", "", nil, 3, nil
			}
			if _, err := f.engine.Run(t.Context(), f.request(name)); err != nil {
				t.Fatal(err)
			}
			if len(captured) == 0 || captured[0]["role"] != "system" {
				t.Fatal("system instructions lost")
			}
			for _, message := range captured {
				for _, tag := range forbidden {
					if strings.Contains(fmt.Sprint(message["content"]), tag) {
						t.Fatalf("disabled section %s remained", tag)
					}
				}
			}
		})
	}
}

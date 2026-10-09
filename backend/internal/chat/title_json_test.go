package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConversationTitleJSONRejectsUnstructuredOrVerboseOutput(t *testing.T) {
	for _, output := range []string{
		"修复运行日志", "```json\n{\"title\":\"运行日志\"}\n```",
		`{"title":"**修复运行日志**"}`, `{"title":"# 修复运行日志"}`,
		`{"title":"1. 修复运行日志"}`, `{"title":"[日志](https://example.com)"}`,
		`{"title":"运行\n日志"}`, `{"title":"<b>运行日志</b>"}`,
		`{"title":"修复手机端的运行日志展示和全部文件夹访问问题"}`,
		`{"title":"运行日志","description":"这是解释"}`, `{"title":null}`,
		`{"title":42}`, `{}`, `[]`, `{"title":"---"}`, `{"title":"修复日志"}额外说明`,
		`{"title":"修复日志"}{"title":"第二个标题"}`,
		`{"Title":"修复日志"}`, `{"title":"修复日志","title":"其他标题"}`,
	} {
		t.Run(output, func(t *testing.T) {
			if title, err := parseGeneratedConversationTitle(output); err == nil || title != "" {
				t.Fatalf("invalid title accepted: %q => %q", output, title)
			}
		})
	}
	for _, output := range []string{`{"title":"修复运行日志"}`, `{"title":"C#项目配置"}`} {
		if _, err := parseGeneratedConversationTitle(output); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConversationTitleRequestForcesJSONAndSmallBudget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		format, _ := body["response_format"].(map[string]interface{})
		thinking, _ := body["thinking"].(map[string]interface{})
		if format["type"] != "json_object" || thinking["type"] != "disabled" || body["max_tokens"] != float64(128) {
			t.Errorf("title controls missing: format=%v thinking=%v tokens=%v", format, thinking, body["max_tokens"])
		}
		if body["tools"] != nil || body["reasoning_effort"] != nil {
			t.Error("title request inherited agent tools or thinking")
		}
		for _, raw := range body["messages"].([]interface{}) {
			message := raw.(map[string]interface{})
			if message["role"] != "user" {
				continue
			}
			var input map[string]string
			if err := json.Unmarshal([]byte(message["content"].(string)), &input); err != nil {
				t.Error(err)
			}
			if len([]rune(input["userMessage"])) != 1200 || len([]rune(input["assistantReply"])) != 800 {
				t.Error("input budget not enforced")
			}
		}
		writeTitleResponse(w, `{"title":"修复运行日志"}`)
	}))
	defer server.Close()
	cfg := &ModelConfig{BaseURL: server.URL + "/deepseek.com", APIKey: "test", ModelName: "test", MaxTokens: 8192, MaxOutputTokens: 4096, ReasoningEffort: "high", Temperature: .7}
	s := &service{}
	title, err := s.requestConversationTitle(context.Background(), cfg, strings.Repeat("用", 2000), strings.Repeat("答", 2000))
	if err != nil || title != "修复运行日志" {
		t.Fatalf("unexpected title: %q %v", title, err)
	}
	if cfg.MaxTokens != 8192 || cfg.MaxOutputTokens != 4096 || cfg.ReasoningEffort != "high" || cfg.Temperature != .7 {
		t.Fatal("conversation model configuration was changed")
	}
}

func writeTitleResponse(w http.ResponseWriter, output string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"choices": []interface{}{map[string]interface{}{"message": map[string]interface{}{"content": output}, "finish_reason": "stop"}},
	})
}

func TestConversationTitleRetriesInvalidJSONOnce(t *testing.T) {
	for _, recover := range []bool{false, true} {
		t.Run(map[bool]string{false: "reject", true: "recover"}[recover], func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				output := "**一段没有采用JSON格式的冗长总结**"
				if recover && calls == 2 {
					output = `{"title":"修复日志展示"}`
				}
				writeTitleResponse(w, output)
			}))
			defer server.Close()
			s := &service{}
			title, err := s.requestConversationTitle(context.Background(), &ModelConfig{BaseURL: server.URL, APIKey: "test", ModelName: "test"}, "修复日志展示", "完成")
			if calls != 2 || (recover && (err != nil || title != "修复日志展示")) || (!recover && (err == nil || title != "")) {
				t.Fatalf("unexpected retry result: calls=%d title=%q err=%v", calls, title, err)
			}
		})
	}
}

func TestGeneratedConversationTitleRejectsMarkdownBeforeStorage(t *testing.T) {
	db, svc := newTitleServiceTest(t)
	conversation := &Conversation{ID: "title-invalid", Title: "原有标题", Revision: 1}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatal(err)
	}
	updated, err := svc.updateGeneratedConversationTitle(conversation.ID, conversation.Title, "**无效标题**")
	if err != nil || updated {
		t.Fatalf("invalid title changed conversation: %v %v", updated, err)
	}
	var stored Conversation
	if err := db.First(&stored, "id = ?", conversation.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Title != "原有标题" || stored.Revision != 1 {
		t.Fatal("invalid title was persisted")
	}
}

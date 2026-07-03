package profile

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func newProfileTestService(t *testing.T) (*service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&UserProfile{}); err != nil {
		t.Fatalf("migrate user profiles: %v", err)
	}
	if err := db.Exec(`CREATE TABLE conversations (id text primary key, character_id text not null default '')`).Error; err != nil {
		t.Fatalf("create conversations: %v", err)
	}
	ctx := app.NewAppContext(db, nil)
	repo := NewRepository(ctx)
	svc, ok := NewService(repo, ctx, graph.NewStubService()).(*service)
	if !ok {
		t.Fatalf("unexpected service type")
	}
	return svc, db
}

func TestCreatePreservesCharacterScope(t *testing.T) {
	svc, _ := newProfileTestService(t)

	item, err := svc.Create(&CreateProfileRequest{
		UserID:         "user-1",
		CharacterID:    "char-a",
		Category:       "preference",
		AttributeName:  "颜色",
		AttributeValue: "蓝色",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if item.CharacterID != "char-a" {
		t.Fatalf("character scope lost: %q", item.CharacterID)
	}
}

func TestCreateClampsConfidence(t *testing.T) {
	svc, _ := newProfileTestService(t)

	low, err := svc.Create(&CreateProfileRequest{
		UserID:         "user-1",
		Category:       "preference",
		AttributeName:  "低置信度",
		AttributeValue: "测试",
		Confidence:     -1,
	})
	if err != nil {
		t.Fatalf("create low confidence profile: %v", err)
	}
	if low.Confidence != 0 {
		t.Fatalf("created low confidence = %d, want 0", low.Confidence)
	}

	high, err := svc.Create(&CreateProfileRequest{
		UserID:         "user-1",
		Category:       "preference",
		AttributeName:  "高置信度",
		AttributeValue: "测试",
		Confidence:     101,
	})
	if err != nil {
		t.Fatalf("create high confidence profile: %v", err)
	}
	if high.Confidence != 100 {
		t.Fatalf("created high confidence = %d, want 100", high.Confidence)
	}
}

func TestServiceUpdateClampsConfidence(t *testing.T) {
	svc, _ := newProfileTestService(t)

	item, err := svc.Create(&CreateProfileRequest{
		UserID:         "user-1",
		Category:       "preference",
		AttributeName:  "服务更新置信度",
		AttributeValue: "测试",
		Confidence:     50,
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	low := -5
	item, err = svc.Update(item.ID, &UpdateProfileRequest{Confidence: &low})
	if err != nil {
		t.Fatalf("update low confidence: %v", err)
	}
	if item.Confidence != 0 {
		t.Fatalf("service low confidence = %d, want 0", item.Confidence)
	}

	high := 105
	item, err = svc.Update(item.ID, &UpdateProfileRequest{Confidence: &high})
	if err != nil {
		t.Fatalf("update high confidence: %v", err)
	}
	if item.Confidence != 100 {
		t.Fatalf("service high confidence = %d, want 100", item.Confidence)
	}
}

func TestProcessUsesCheckpointIncrementally(t *testing.T) {
	svc, db := newProfileTestService(t)
	var requests []string
	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(body.Messages) < 2 {
			t.Fatalf("unexpected messages: %#v", body.Messages)
		}
		requests = append(requests, body.Messages[1].Content)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"[]"}}],"usage":{"total_tokens":1}}`))
	}))
	t.Cleanup(modelServer.Close)

	if err := db.Exec(`CREATE TABLE messages (id text primary key, conversation_id text not null, sequence integer not null default 0, role text not null, content text not null, created_at text default '')`).Error; err != nil {
		t.Fatalf("create messages: %v", err)
	}
	if err := db.Exec(`CREATE TABLE pipeline_checkpoints (conversation_id text not null, pipeline_type text not null, last_message_sequence integer not null default 0, checkpoint_version integer not null default 1, idempotency_key text default '', created_at text default '', updated_at text default '', primary key (conversation_id, pipeline_type))`).Error; err != nil {
		t.Fatalf("create checkpoints: %v", err)
	}
	if err := db.Exec(`CREATE TABLE model_configs (id text primary key, base_url text not null, api_key text not null, model_name text not null, temperature real not null default 0, max_tokens real not null default 256, is_active integer not null default 0)`).Error; err != nil {
		t.Fatalf("create model configs: %v", err)
	}
	if err := db.Exec(`INSERT INTO model_configs (id, base_url, api_key, model_name, temperature, max_tokens, is_active) VALUES (?, ?, ?, ?, ?, ?, 1)`, "model-1", modelServer.URL, "test-key", "test-model", 0, 256).Error; err != nil {
		t.Fatalf("insert model config: %v", err)
	}
	if err := db.Exec(`INSERT INTO conversations (id, character_id) VALUES (?, ?)`, "conv-inc", "char-a").Error; err != nil {
		t.Fatalf("insert conversation: %v", err)
	}
	for _, row := range []struct {
		id       string
		sequence int
		role     string
		content  string
	}{
		{"m1", 1, "user", "第一条用户事实"},
		{"m2", 2, "assistant", "第一条回复"},
	} {
		if err := db.Exec(`INSERT INTO messages (id, conversation_id, sequence, role, content) VALUES (?, ?, ?, ?, ?)`, row.id, "conv-inc", row.sequence, row.role, row.content).Error; err != nil {
			t.Fatalf("insert message: %v", err)
		}
	}

	if err := svc.Process(context.Background(), "conv-inc", nil, ""); err != nil {
		t.Fatalf("first process: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("llm calls after first process = %d, want 1", len(requests))
	}
	if !strings.Contains(requests[0], "第一条用户事实") || !strings.Contains(requests[0], "第一条回复") {
		t.Fatalf("first request missing initial messages: %s", requests[0])
	}

	if err := svc.Process(context.Background(), "conv-inc", nil, ""); err != nil {
		t.Fatalf("second process: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("llm calls after duplicate process = %d, want 1", len(requests))
	}

	if err := db.Exec(`INSERT INTO messages (id, conversation_id, sequence, role, content) VALUES (?, ?, ?, ?, ?)`, "m3", "conv-inc", 3, "user", "第二条新增事实").Error; err != nil {
		t.Fatalf("insert new message: %v", err)
	}
	if err := svc.Process(context.Background(), "conv-inc", nil, ""); err != nil {
		t.Fatalf("third process: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("llm calls after new message = %d, want 2", len(requests))
	}
	if strings.Contains(requests[1], "第一条用户事实") || strings.Contains(requests[1], "第一条回复") {
		t.Fatalf("incremental request repeated old messages: %s", requests[1])
	}
	if !strings.Contains(requests[1], "第二条新增事实") {
		t.Fatalf("incremental request missing new message: %s", requests[1])
	}
}

func TestToolUpsertUsesConversationCharacterScope(t *testing.T) {
	svc, db := newProfileTestService(t)
	if err := db.Exec(`INSERT INTO conversations (id, character_id) VALUES (?, ?)`, "conv-a", "char-a").Error; err != nil {
		t.Fatalf("insert conversation: %v", err)
	}

	item, err := svc.UpsertFromTool("user-1", "preference", "饮料", "茶", 80, "conv-a")
	if err != nil {
		t.Fatalf("upsert from tool: %v", err)
	}
	if item.CharacterID != "char-a" {
		t.Fatalf("tool profile scope = %q, want char-a", item.CharacterID)
	}

	items, err := svc.repo.GetScopedByUserID("user-1", "char-b")
	if err != nil {
		t.Fatalf("query char-b profiles: %v", err)
	}
	for _, got := range items {
		if got.AttributeName == "饮料" && got.AttributeValue == "茶" {
			t.Fatalf("char-b received char-a profile: %+v", got)
		}
	}
}

func TestDefaultUserInputDerivesProfileUserScopeFromCharacter(t *testing.T) {
	svc, db := newProfileTestService(t)
	if err := db.Exec(`INSERT INTO conversations (id, character_id) VALUES (?, ?)`, "conv-a", "char-a").Error; err != nil {
		t.Fatalf("insert conversation: %v", err)
	}

	item, err := svc.UpsertFromTool("default", "preference", "饮料", "茶", 80, "conv-a")
	if err != nil {
		t.Fatalf("upsert from tool: %v", err)
	}
	if item.UserID != "char-a" {
		t.Fatalf("tool profile user scope = %q, want char-a", item.UserID)
	}
	if item.CharacterID != "char-a" {
		t.Fatalf("tool profile character scope = %q, want char-a", item.CharacterID)
	}

	prompt := svc.ToSystemPrompt("default", "char-a")
	if !strings.Contains(prompt, "茶") {
		t.Fatalf("prompt missing derived character profile: %s", prompt)
	}
}

func TestSystemPromptUsesRequestedCharacterScope(t *testing.T) {
	svc, _ := newProfileTestService(t)
	_, err := svc.Create(&CreateProfileRequest{
		UserID:         "user-1",
		CharacterID:    "char-a",
		Category:       "preference",
		AttributeName:  "称呼",
		AttributeValue: "姐姐",
		Confidence:     80,
	})
	if err != nil {
		t.Fatalf("create char-a profile: %v", err)
	}
	_, err = svc.Create(&CreateProfileRequest{
		UserID:         "user-1",
		CharacterID:    "char-b",
		Category:       "preference",
		AttributeName:  "称呼",
		AttributeValue: "老师",
		Confidence:     80,
	})
	if err != nil {
		t.Fatalf("create char-b profile: %v", err)
	}
	_, err = svc.Create(&CreateProfileRequest{
		UserID:         "user-1",
		Category:       "preference",
		AttributeName:  "语言",
		AttributeValue: "中文",
		Confidence:     80,
	})
	if err != nil {
		t.Fatalf("create global profile: %v", err)
	}

	prompt := svc.ToSystemPrompt("user-1", "char-a")
	if !strings.Contains(prompt, "姐姐") {
		t.Fatalf("prompt missing char-a profile: %s", prompt)
	}
	if strings.Contains(prompt, "老师") {
		t.Fatalf("prompt leaked char-b profile: %s", prompt)
	}
	if !strings.Contains(prompt, "中文") {
		t.Fatalf("prompt missing global profile fallback: %s", prompt)
	}
}

func TestSystemPromptDoesNotFallbackToDefaultUser(t *testing.T) {
	svc, _ := newProfileTestService(t)
	_, err := svc.repo.UpsertConfidence(&UserProfile{
		UserID:         "default",
		Category:       "preference",
		AttributeName:  "内部默认偏好",
		AttributeValue: "不应进入提示",
		Confidence:     80,
	})
	if err != nil {
		t.Fatalf("create default profile: %v", err)
	}

	prompt := svc.ToSystemPrompt("user-without-profile", "char-a")
	if prompt != "" {
		t.Fatalf("prompt leaked default profile: %s", prompt)
	}
}

package chat

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"github.com/u-ai/backend/internal/character"
	applog "github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func setupProcessTraceService(t *testing.T, modelStatus int) (*service, *bytes.Buffer, func()) {
	t.Helper()
	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if modelStatus != http.StatusOK {
			w.WriteHeader(modelStatus)
			_, _ = w.Write([]byte(`{"error":"failed"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "第一段\n第二段"}},
			},
			"usage": map[string]interface{}{"total_tokens": 12},
		})
	}))
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Conversation{}, &Message{}, &ModelConfig{}, &character.Character{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&character.Character{ID: "char-trace", Name: "Amitia", Identity: "心理模拟伙伴", Status: "enabled"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ModelConfig{Name: "trace-model", BaseURL: modelServer.URL, APIKey: "test-key", ModelName: "trace", IsActive: 1, MaxTokens: 256, Temperature: 0.7}).Error; err != nil {
		t.Fatal(err)
	}
	ctx := app.NewAppContext(db, nil)
	var logs bytes.Buffer
	previousOutput := applog.Logger.Out
	previousFormatter := applog.Logger.Formatter
	previousLevel := applog.Logger.Level
	applog.Logger.SetOutput(&logs)
	applog.Logger.SetFormatter(&logrus.JSONFormatter{})
	applog.Logger.SetLevel(logrus.InfoLevel)
	cleanup := func() {
		applog.Logger.SetOutput(previousOutput)
		applog.Logger.SetFormatter(previousFormatter)
		applog.Logger.SetLevel(previousLevel)
		sqlDB.Close()
		modelServer.Close()
	}
	return &service{repo: NewRepository(ctx), charRepo: character.NewRepository(ctx), db: db}, &logs, cleanup
}

func TestProcessMessageTraceCoversInputModelAndDBCommit(t *testing.T) {
	svc, logs, cleanup := setupProcessTraceService(t, http.StatusOK)
	t.Cleanup(cleanup)
	resp, err := svc.ProcessMessage(&ProcessMessageRequest{
		CharacterID: "char-trace",
		Message:     "这是很长的隐私消息，日志里不应该完整记录",
		Channel:     "web",
		Source:      "system",
		RequestID:   "req-trace-success",
		PeerID:      "user-trace",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.RequestID != "req-trace-success" || len(resp.MessageIDs) != 2 {
		t.Fatalf("unexpected response: %#v", resp)
	}
	var assistantCount int64
	if err := svc.db.Model(&Message{}).Where("request_id = ? AND role = ?", "req-trace-success", "assistant").Count(&assistantCount).Error; err != nil {
		t.Fatal(err)
	}
	if assistantCount != 2 {
		t.Fatalf("expected assistant messages linked by request_id, got %d", assistantCount)
	}
	rawLogs := logs.String()
	for _, want := range []string{"req-trace-success", "corr-req-trace-success", "cause-req-trace-success", "input_received", "model_call_started", "model_call_completed", "db_commit_completed", "completed"} {
		if !strings.Contains(rawLogs, want) {
			t.Fatalf("expected trace logs to contain %q, got %s", want, rawLogs)
		}
	}
	if strings.Contains(rawLogs, "这是很长的隐私消息") || strings.Contains(rawLogs, "test-key") {
		t.Fatalf("trace logs leaked sensitive content: %s", rawLogs)
	}
}

func TestProcessMessageTraceCoversFailedModelRequest(t *testing.T) {
	svc, logs, cleanup := setupProcessTraceService(t, http.StatusInternalServerError)
	t.Cleanup(cleanup)
	_, err := svc.ProcessMessage(&ProcessMessageRequest{
		CharacterID: "char-trace",
		Message:     "失败请求不应该泄漏完整内容",
		Channel:     "web",
		Source:      "system",
		RequestID:   "req-trace-failed",
	})
	if err == nil {
		t.Fatal("expected model failure")
	}
	var status string
	if err := svc.db.Model(&Message{}).Select("status").Where("request_id = ? AND role = ?", "req-trace-failed", "user").Row().Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Fatalf("expected failed user message, got %s", status)
	}
	rawLogs := logs.String()
	for _, want := range []string{"req-trace-failed", "input_received", "model_call_started", "model_call_failed"} {
		if !strings.Contains(rawLogs, want) {
			t.Fatalf("expected failed trace logs to contain %q, got %s", want, rawLogs)
		}
	}
	if strings.Contains(rawLogs, "失败请求不应该泄漏完整内容") || strings.Contains(rawLogs, "test-key") {
		t.Fatalf("failed trace logs leaked sensitive content: %s", rawLogs)
	}
}

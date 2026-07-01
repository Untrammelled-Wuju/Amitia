package system

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/pkg/comment/response"
	"gorm.io/gorm"
)

func newWebChatScopeTestHandler(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB.Close()
	})
	if err := db.Exec(`CREATE TABLE characters (
		id TEXT PRIMARY KEY,
		name TEXT DEFAULT '',
		conversation_id TEXT DEFAULT '',
		updated_at TEXT DEFAULT ''
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE conversations (
		id TEXT PRIMARY KEY,
		title TEXT DEFAULT '',
		character_id TEXT DEFAULT '',
		channel TEXT DEFAULT 'web',
		source TEXT DEFAULT 'manual',
		peer_id TEXT DEFAULT '',
		created_at TEXT DEFAULT '',
		updated_at TEXT DEFAULT ''
	)`).Error; err != nil {
		t.Fatal(err)
	}
	return &Handler{db: db}, db
}

func postWebChatCreateConv(t *testing.T, h *Handler, body map[string]any) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/web-chat/conversations", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	h.WebChatCreateConv(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestWebChatCreateConvRejectsExternalChannelWithoutPeerID(t *testing.T) {
	h, db := newWebChatScopeTestHandler(t)
	if err := db.Exec("INSERT INTO characters (id, name, conversation_id) VALUES (?, ?, ?)", "char-1", "Amitia", "conv-old").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO conversations (id, title, character_id, channel, source, peer_id) VALUES (?, ?, ?, ?, ?, ?)", "conv-old", "旧会话", "char-1", "qq", "manual", "peer-old").Error; err != nil {
		t.Fatal(err)
	}

	result := postWebChatCreateConv(t, h, map[string]any{
		"characterId": "char-1",
		"channel":     "qq",
	})
	if int(result["code"].(float64)) != response.OK {
		t.Fatalf("expected ok, got %#v", result)
	}
	data := result["data"].(map[string]any)
	if data["id"] != "conv-old" || data["channel"] != "qq" {
		t.Fatalf("expected existing conv for char with bound conversation, got %#v", data)
	}
}

func TestWebChatCreateConvReturnsPeerBoundConversation(t *testing.T) {
	h, db := newWebChatScopeTestHandler(t)
	if err := db.Exec("INSERT INTO characters (id, name) VALUES (?, ?)", "char-1", "Amitia").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO conversations (id, title, character_id, channel, source, peer_id) VALUES (?, ?, ?, ?, ?, ?)", "conv-peer-1", "一号", "char-1", "qq", "qq", "peer-1").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO conversations (id, title, character_id, channel, source, peer_id) VALUES (?, ?, ?, ?, ?, ?)", "conv-peer-2", "二号", "char-1", "qq", "qq", "peer-2").Error; err != nil {
		t.Fatal(err)
	}

	result := postWebChatCreateConv(t, h, map[string]any{
		"channel": "qq",
		"peerId":  "peer-2",
	})
	if int(result["code"].(float64)) != response.OK {
		t.Fatalf("expected ok, got %#v", result)
	}
	data := result["data"].(map[string]any)
	if data["channel"] != "qq" || len(data["id"].(string)) == 0 {
		t.Fatalf("unexpected conversation: %#v", data)
	}
}

func TestWebChatCreateConvCreatesExternalConversationForExplicitPeerTarget(t *testing.T) {
	h, db := newWebChatScopeTestHandler(t)
	if err := db.Exec("INSERT INTO characters (id, name, conversation_id) VALUES (?, ?, ?)", "char-1", "Amitia", "").Error; err != nil {
		t.Fatal(err)
	}

	result := postWebChatCreateConv(t, h, map[string]any{
		"characterId": "char-1",
		"channel":     "wechat",
		"peerId":      "peer-new",
	})
	if int(result["code"].(float64)) != response.OK {
		t.Fatalf("expected ok, got %#v", result)
	}
	data := result["data"].(map[string]any)
	if data["channel"] != "wechat" || data["characterId"] != "char-1" {
		t.Fatalf("unexpected conversation: %#v", data)
	}
}

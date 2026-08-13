package system

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/chat"
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
	return &Handler{db: db, chatSvc: &fakeWebChatService{}}, db
}

type fakeWebChatService struct {
	chat.Service
	seq int
}

func (f *fakeWebChatService) EnsureChannelConversation(channel string) (*chat.Conversation, error) {
	return nil, fmt.Errorf("not found")
}

func (f *fakeWebChatService) CreateConversation(req *chat.CreateConversationRequest) (*chat.Conversation, error) {
	f.seq++
	return &chat.Conversation{
		ID:          fmt.Sprintf("conv-auto-%d", f.seq),
		Title:       req.Title,
		CharacterID: req.CharacterID,
		Channel:     req.Channel,
		Source:      req.Source,
	}, nil
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

func TestWebChatEnvelopeResolvesStableIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodPost, "/web-chat/send", bytes.NewReader(nil))
	req.Header.Set("X-Request-ID", "header-request")
	req.Header.Set("X-Session-ID", "header-session")
	req.Header.Set("X-User-ID", "header-user")
	req.Header.Set("X-Peer-ID", "header-peer")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	body := webChatSendRequest{
		RequestID: "body-request",
		SessionID: "body-session",
		UserID:    "body-user",
		PeerID:    "body-peer",
	}
	if got := resolveRequestID(c, body.RequestID, body.ClientMessageID, body.MessageID); got != "body-request" {
		t.Fatalf("unexpected request id: %s", got)
	}
	if got := resolveHeaderBackedValue(c, body.SessionID, "X-Session-ID"); got != "body-session" {
		t.Fatalf("unexpected session id: %s", got)
	}
	if got := resolveHeaderBackedValue(c, body.UserID, "X-User-ID"); got != "body-user" {
		t.Fatalf("unexpected user id: %s", got)
	}
	if got := resolveHeaderBackedValue(c, body.PeerID, "X-Peer-ID"); got != "body-peer" {
		t.Fatalf("unexpected peer id: %s", got)
	}
	if got := resolveSource(c, body.Source, "web"); got != "web" {
		t.Fatalf("unexpected body source fallback: %s", got)
	}

	body = webChatSendRequest{}
	if got := resolveRequestID(c, body.RequestID, body.ClientMessageID, body.MessageID); got != "header-request" {
		t.Fatalf("unexpected header request id: %s", got)
	}
	if got := resolveHeaderBackedValue(c, body.SessionID, "X-Session-ID"); got != "header-session" {
		t.Fatalf("unexpected header session id: %s", got)
	}
	if got := resolveHeaderBackedValue(c, body.UserID, "X-User-ID"); got != "header-user" {
		t.Fatalf("unexpected header user id: %s", got)
	}
	if got := resolveHeaderBackedValue(c, body.PeerID, "X-Peer-ID"); got != "header-peer" {
		t.Fatalf("unexpected header peer id: %s", got)
	}
}

func TestWebChatEnvelopeResolvesQueryAndSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodPost, "/web-chat/send?requestId=query-request&sessionId=query-session&userId=query-user&peerId=query-peer&source=wechat", bytes.NewReader(nil))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	body := webChatSendRequest{}
	if got := resolveRequestID(c, body.RequestID, body.ClientMessageID, body.MessageID); got != "query-request" {
		t.Fatalf("unexpected query request id: %s", got)
	}
	if got := resolveRequestBackedValue(c, body.SessionID, "X-Session-ID", "sessionId", "session_id"); got != "query-session" {
		t.Fatalf("unexpected query session id: %s", got)
	}
	if got := resolveRequestBackedValue(c, body.UserID, "X-User-ID", "userId", "user_id"); got != "query-user" {
		t.Fatalf("unexpected query user id: %s", got)
	}
	if got := resolveRequestBackedValue(c, body.PeerID, "X-Peer-ID", "peerId", "peer_id"); got != "query-peer" {
		t.Fatalf("unexpected query peer id: %s", got)
	}
	if got := resolveSource(c, body.Source, "web"); got != "wechat" {
		t.Fatalf("unexpected query source: %s", got)
	}

	req.Header.Set("X-Source", "qq")
	if got := resolveSource(c, body.Source, "web"); got != "qq" {
		t.Fatalf("unexpected header source: %s", got)
	}

	body.Source = "voice"
	if got := resolveSource(c, body.Source, "web"); got != "voice" {
		t.Fatalf("unexpected body source: %s", got)
	}
}

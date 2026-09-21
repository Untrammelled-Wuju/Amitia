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
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/spaceidentity"
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
	if _, err := spaceidentity.InitializeDefault(filepath.Join(t.TempDir(), "data")); err != nil {
		t.Fatal(err)
	}
	originalCfg := config.AppCfg
	config.AppCfg = &config.Config{Security: config.SecurityRuntimeConfig{Mode: "local_single_user"}}
	t.Cleanup(func() { config.AppCfg = originalCfg })
	if err := db.Exec(`CREATE TABLE characters (
		id TEXT PRIMARY KEY,
		space_id TEXT DEFAULT '',
		name TEXT DEFAULT '',
		conversation_id TEXT DEFAULT '',
		deleted_at DATETIME,
		updated_at TEXT DEFAULT ''
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE conversations (
		id TEXT PRIMARY KEY,
		space_id TEXT DEFAULT '',
		title TEXT DEFAULT '',
		character_id TEXT DEFAULT '',
		channel TEXT DEFAULT 'web',
		source TEXT DEFAULT 'manual',
		peer_id TEXT DEFAULT '',
		created_at TEXT DEFAULT '',
		deleted_at DATETIME,
		updated_at TEXT DEFAULT ''
	)`).Error; err != nil {
		t.Fatal(err)
	}
	return &Handler{db: db, chatSvc: &fakeWebChatService{db: db}}, db
}

type fakeWebChatService struct {
	chat.Service
	db  *gorm.DB
	seq int
}

func (f *fakeWebChatService) EnsureChannelConversation(channel string) (*chat.Conversation, error) {
	return nil, fmt.Errorf("not found")
}

func (f *fakeWebChatService) CreateConversation(req *chat.CreateConversationRequest) (*chat.Conversation, error) {
	f.seq++
	return &chat.Conversation{
		ID:        fmt.Sprintf("conv-auto-%d", f.seq),
		Title:     req.Title,
		ProjectID: req.ProjectID,
		Channel:   req.Channel,
		Source:    req.Source,
	}, nil
}

func (f *fakeWebChatService) ListConversationsForSpace(chat.ConversationQuery, string) (*chat.ConversationListResponse, error) {
	return &chat.ConversationListResponse{}, nil
}

func (f *fakeWebChatService) GetConversationForSpace(string, string) (*chat.Conversation, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeWebChatService) GetMessagesForSpace(string, string, int, int) ([]chat.Message, int64, error) {
	return nil, 0, nil
}

func (f *fakeWebChatService) GetMessagesBeforeForSpace(string, string, int64, int) ([]chat.Message, bool, error) {
	return nil, false, nil
}

func (f *fakeWebChatService) ListChannelConversationsForSpace(string, int) ([]chat.Conversation, error) {
	var conversations []chat.Conversation
	err := f.db.Where("deleted_at IS NULL AND channel <> '' AND channel <> 'web'").Order("updated_at DESC").Find(&conversations).Error
	return conversations, err
}

func (f *fakeWebChatService) ListConversationSidebarForSpace(string, int, int) (*chat.ConversationSidebarResponse, error) {
	return &chat.ConversationSidebarResponse{}, nil
}

func (f *fakeWebChatService) CreateProjectForSpace(*chat.CreateProjectRequest, string) (*chat.Project, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeWebChatService) UpdateProjectForSpace(string, *chat.UpdateProjectRequest, string) (*chat.Project, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeWebChatService) DeleteProjectForSpace(string, string) error {
	return nil
}

func (f *fakeWebChatService) CreateProjectConversationForSpace(string, *chat.CreateConversationRequest, string) (*chat.Conversation, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeWebChatService) MoveConversationToProjectForSpace(string, string, string) (*chat.Conversation, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeWebChatService) UpdateConversationSidebarStateForSpace(string, *bool, *bool, string) (*chat.Conversation, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeWebChatService) GetProjectOpenTargetForSpace(string, string) (*chat.ProjectOpenTarget, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeWebChatService) CreateConversationForSpace(req *chat.CreateConversationRequest, spaceID string) (*chat.Conversation, error) {
	conv, err := f.CreateConversation(req)
	if conv != nil {
		conv.SpaceID = spaceID
	}
	return conv, err
}

func (f *fakeWebChatService) EnsureChannelConversationForSpace(channel, _ string) (*chat.Conversation, error) {
	var conv chat.Conversation
	if f.db == nil {
		return nil, gorm.ErrRecordNotFound
	}
	if err := f.db.Where("channel = ? AND deleted_at IS NULL", channel).Order("updated_at DESC").First(&conv).Error; err != nil {
		return nil, err
	}
	return &conv, nil
}

func (f *fakeWebChatService) DeleteConversationForSpace(string, string) (bool, error) {
	return true, nil
}

func (f *fakeWebChatService) DeleteMessagesForSpace(string, string) error {
	return nil
}

type fakeChannelAvailability map[string]bool

func (f fakeChannelAvailability) Has(channelID string) bool {
	return f[channelID]
}

func TestWebChatListChannelConversationsFiltersUnavailableChannels(t *testing.T) {
	h, db := newWebChatScopeTestHandler(t)
	h.SetChannelAvailability(fakeChannelAvailability{"wechat_personal": true})
	if err := db.Exec("INSERT INTO conversations (id, title, channel, source, peer_id) VALUES (?, ?, ?, ?, ?)", "conv-wechat", "微信", "wechat_personal", "channel", "peer-wechat").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO conversations (id, title, channel, source, peer_id) VALUES (?, ?, ?, ?, ?)", "conv-qq", "QQ", "qq", "channel", "peer-qq").Error; err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/web-chat/channels", nil)
	h.WebChatListChannelConversations(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var result struct {
		Data struct {
			Items []chat.Conversation `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Data.Items) != 1 || result.Data.Items[0].ID != "conv-wechat" {
		t.Fatalf("unexpected channel conversations: %#v", result.Data.Items)
	}
}

func TestWebChatEnvelopeResolvesStableIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodPost, "/web-chat/messages", bytes.NewReader(nil))
	req.Header.Set("X-Request-ID", "header-request")
	req.Header.Set("X-Session-ID", "header-session")
	req.Header.Set("X-User-ID", "header-user")
	req.Header.Set("X-Peer-ID", "header-peer")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	body := webChatSendRequest{
		RequestID: "body-request",
		SessionID: "body-session",
		PeerID:    "body-peer",
	}
	if got := resolveRequestID(c, body.RequestID, body.ClientMessageID, ""); got != "body-request" {
		t.Fatalf("unexpected request id: %s", got)
	}
	if got := resolveHeaderBackedValue(c, body.SessionID, "X-Session-ID"); got != "body-session" {
		t.Fatalf("unexpected session id: %s", got)
	}
	if got := resolveHeaderBackedValue(c, body.PeerID, "X-Peer-ID"); got != "body-peer" {
		t.Fatalf("unexpected peer id: %s", got)
	}
	if got := resolveSource(c, body.Source, "web"); got != "web" {
		t.Fatalf("unexpected body source fallback: %s", got)
	}

	body = webChatSendRequest{}
	if got := resolveRequestID(c, body.RequestID, body.ClientMessageID, ""); got != "header-request" {
		t.Fatalf("unexpected header request id: %s", got)
	}
	if got := resolveHeaderBackedValue(c, body.SessionID, "X-Session-ID"); got != "header-session" {
		t.Fatalf("unexpected header session id: %s", got)
	}
	if got := resolveHeaderBackedValue(c, "", "X-User-ID"); got != "header-user" {
		t.Fatalf("unexpected header user id: %s", got)
	}
	if got := resolveHeaderBackedValue(c, body.PeerID, "X-Peer-ID"); got != "header-peer" {
		t.Fatalf("unexpected header peer id: %s", got)
	}
}

func TestWebChatEnvelopeResolvesQueryAndSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodPost, "/web-chat/messages?requestId=query-request&sessionId=query-session&spaceId=query-user&peerId=query-peer&source=wechat", bytes.NewReader(nil))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	body := webChatSendRequest{}
	if got := resolveRequestID(c, body.RequestID, body.ClientMessageID, ""); got != "query-request" {
		t.Fatalf("unexpected query request id: %s", got)
	}
	if got := resolveRequestBackedValue(c, body.SessionID, "X-Session-ID", "sessionId", "session_id"); got != "query-session" {
		t.Fatalf("unexpected query session id: %s", got)
	}
	if got := resolveRequestBackedValue(c, "", "X-User-ID", "spaceId", "space_id"); got != "query-user" {
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

package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestMeshConversationSearchHTTPUsesMessageContent(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	db := p.services.KernelContainer.DeviceRegistry.Database()
	if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('device-a','core','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"matched", "unmatched", "other-role"} {
		if err := p.services.DB.Create(&chat.Conversation{ID: id, SpaceID: p.legacySpaceID, Title: "普通标题", Channel: "web"}).Error; err != nil {
			t.Fatal(err)
		}
		role, content := "one", "普通消息"
		if id == "matched" || id == "other-role" {
			content = "Needle 在消息正文中"
		}
		if id == "other-role" {
			role = "two"
		}
		if err := p.services.DB.Create(&chat.Message{ID: id + "-message", ConversationID: id, CharacterID: role, Role: "user", Content: content}).Error; err != nil {
			t.Fatal(err)
		}
	}
	service := coordination.NewService(db)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		actor := &auth.ActorContext{SpaceID: "core", DeviceID: "device-a", PrincipalType: auth.PrincipalTest}
		c.Set("actorContext", actor)
		c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), actor))
	})
	registerMeshBusinessRouter(router.Group("/api"), &AppServices{DeviceMesh: &devicemesh.Runtime{Coordination: service}, OwnedBusiness: business.NewEngine(service, p, &routerSummaryModel{})}, "core")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/device-mesh/v1/business/conversations?characterId=one&keyword=NEEDLE", nil))
	var result struct {
		Data business.QueryResult `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 {
		t.Fatalf("query status=%d err=%v body=%s", response.Code, err, response.Body.String())
	}
	if len(result.Data.Snapshot.LegacyConversations) != 1 {
		t.Fatalf("message search returned %d conversations", len(result.Data.Snapshot.LegacyConversations))
	}
	var row chat.Conversation
	if err := json.Unmarshal(result.Data.Snapshot.LegacyConversations[0], &row); err != nil || row.ID != "matched" {
		t.Fatalf("unexpected match: %+v err=%v", row, err)
	}
	if len(result.Data.Snapshot.LegacyMessages) != 0 {
		t.Fatal("search returned private message bodies")
	}
}

func TestMeshConversationSearchOwnerRoleAndCursorIsolation(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	db := p.services.KernelContainer.DeviceRegistry.Database()
	insert := func(owner, kind, id, role string, body any, deleted bool) {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,deleted,body,updated_at) VALUES(?,?,?,?,1,?,?,?)`, owner, kind, id, role, deleted, raw, "2026-10-05T01:00:00Z"); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"matched-a", "matched-b", "unmatched", "wrong-role", "wrong-owner", "deleted-message"} {
		insert(p.ownerID, "conversation", id, "one", map[string]any{"id": id, "title": "普通标题"}, false)
		owner, role, content := p.ownerID, "one", "正文关键词"
		if id == "unmatched" {
			content = "未匹配"
		}
		if id == "wrong-role" {
			role = "two"
		}
		if id == "wrong-owner" {
			owner = "device-b"
		}
		insert(owner, "message", id+"-message", role, map[string]any{"conversationId": id, "content": content}, id == "deleted-message")
	}
	query := coordination.DataQuery{ResourceKind: "conversation", ListConversations: true, SearchQuery: "正文关键词", Limit: 1}
	rows, cursor, err := p.store.ListPage(t.Context(), "conversation", "one", query)
	if err != nil || len(rows) != 1 || cursor == "" || rows[0].ID != "matched-b" {
		t.Fatalf("first search page=%+v cursor=%s err=%v", rows, cursor, err)
	}
	query.Cursor = cursor
	rows, next, err := p.store.ListPage(t.Context(), "conversation", "one", query)
	if err != nil || len(rows) != 1 || next != "" || rows[0].ID != "matched-a" {
		t.Fatalf("second search page=%+v next=%s err=%v", rows, next, err)
	}
	query.SearchQuery = "其他条件"
	if _, _, err := p.store.ListPage(t.Context(), "conversation", "one", query); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("changed search accepted old cursor: %v", err)
	}
	scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", ResourceOwnerID: "core", RoleOwnerID: "core", RoleID: "cloud-role", Coordinated: true}
	query.Cursor, query.SearchQuery = "", "正文关键词"
	page, err := p.HistoricalConversationPage(t.Context(), scope, query)
	if err != nil || len(page.Conversations) != 1 || page.NextCursor == "" {
		t.Fatalf("historical search=%+v err=%v", page, err)
	}
	query.HistoricalListCursor, query.SearchQuery = page.NextCursor, "其他条件"
	if _, err := p.HistoricalConversationPage(t.Context(), scope, query); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("historical changed search accepted old cursor: %v", err)
	}
	for _, invalid := range []string{string(make([]byte, 257)), "a\x00b", string([]byte{0xff})} {
		if _, err := coordination.NormalizeConversationSearch(invalid); err == nil {
			t.Fatal("invalid search accepted")
		}
	}
}

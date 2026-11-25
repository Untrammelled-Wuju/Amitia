package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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
		role, content := "one", "普通消息"
		if id == "matched" || id == "other-role" {
			content = "Needle 在消息正文中"
		}
		if id == "other-role" {
			role = "two"
		}
		conversationBody, _ := json.Marshal(map[string]any{"id": id, "title": "普通标题", "characterId": role, "createdAt": "2026-10-01T00:00:00Z"})
		messageBody, _ := json.Marshal(map[string]any{"id": id + "-message", "conversationId": id, "characterId": role, "role": "user", "content": content, "createdAt": "2026-10-01T00:00:01Z"})
		seedOwnedChatManagement(t, p, "conversation", id, role, string(conversationBody))
		seedOwnedChatManagement(t, p, "message", id+"-message", role, string(messageBody))
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
	conversations := 0
	var row chat.Conversation
	for _, resource := range result.Data.Snapshot.Resources {
		if resource.Kind != "conversation" {
			continue
		}
		conversations++
		if err := json.Unmarshal(resource.Body, &row); err != nil {
			t.Fatal(err)
		}
	}
	if conversations != 1 {
		t.Fatalf("message search returned %d conversations", conversations)
	}
	if row.ID != "matched" {
		t.Fatalf("unexpected match: %+v", row)
	}
	for _, resource := range result.Data.Snapshot.Resources {
		if resource.Kind == "message" {
			t.Fatal("search returned private message bodies")
		}
	}
}

func TestMeshConversationListPagesOwnedAndMemberGroups(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	db := p.services.KernelContainer.DeviceRegistry.Database()
	if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('device-a','core','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	seedOwnedChatManagement(t, p, "conversation", "active", "one", `{"id":"active","title":"普通对话","channel":"web"}`)
	seedOwnedChatManagement(t, p, "conversation", "shared", "two", `{"id":"shared","title":"群聊","conversationType":"group","channel":"web","members":[{"characterId":"one","status":"active"}]}`)
	seedOwnedChatManagement(t, p, "conversation", "left", "two", `{"id":"left","title":"已退出群聊","conversationType":"group","channel":"web","members":[{"characterId":"one","status":"left"}]}`)
	seedOwnedChatManagement(t, p, "conversation", "archived", "one", `{"id":"archived","archived":true,"channel":"web"}`)
	service := coordination.NewService(db)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		actor := &auth.ActorContext{SpaceID: "core", DeviceID: "device-a", PrincipalType: auth.PrincipalTest}
		c.Set("actorContext", actor)
		c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), actor))
	})
	registerMeshBusinessRouter(router.Group("/api"), &AppServices{DeviceMesh: &devicemesh.Runtime{Coordination: service}, OwnedBusiness: business.NewEngine(service, p, &routerSummaryModel{})}, "core")
	list := func(archive, cursor string) (business.QueryResult, int) {
		t.Helper()
		values := url.Values{"characterId": {"one"}, "archive": {archive}, "channel": {"web"}, "resourceKind": {"conversation"}, "limit": {"1"}}
		if cursor != "" {
			values.Set("cursor", cursor)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/device-mesh/v1/business/conversations?"+values.Encode(), nil))
		if response.Code != http.StatusOK {
			return business.QueryResult{}, response.Code
		}
		var result struct {
			Data business.QueryResult `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatalf("页面响应 %s 解码失败: %v", response.Body.String(), err)
		}
		return result.Data, response.Code
	}
	first, status := list("active", "")
	cursor := first.Snapshot.NextCursors["conversation"]
	if status != http.StatusOK || cursor == "" || len(first.Snapshot.Resources) != 1 {
		t.Fatalf("首屏 status=%d snapshot=%+v", status, first.Snapshot)
	}
	second, status := list("active", cursor)
	if status != http.StatusOK || second.Snapshot.NextCursors["conversation"] != "" || len(second.Snapshot.Resources) != 1 {
		t.Fatalf("下一页 status=%d snapshot=%+v", status, second.Snapshot)
	}
	ids := map[string]bool{first.Snapshot.Resources[0].ID: true, second.Snapshot.Resources[0].ID: true}
	if !ids["active"] || !ids["shared"] {
		t.Fatalf("跨角色群聊分页结果 %+v", ids)
	}
	if _, status := list("archived", cursor); status != http.StatusConflict {
		t.Fatalf("归档列表接受了活动列表游标，status=%d", status)
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
	scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", ResourceOwnerID: "core", RoleOwnerID: "core", RoleID: "one", Coordinated: true}
	query.Cursor, query.SearchQuery = "", "正文关键词"
	page, err := p.HistoricalConversationPage(t.Context(), scope, query)
	if err != nil || len(page.Conversations) != 1 || page.NextCursor == "" {
		t.Fatalf("historical search=%+v err=%v", page, err)
	}
	foreign := scope
	foreign.RoleID = "two"
	foreignQuery := query
	foreignQuery.HistoricalListCursor, foreignQuery.SearchQuery = "", "正文关键词"
	foreignPage, err := p.HistoricalConversationPage(t.Context(), foreign, foreignQuery)
	if err != nil || len(foreignPage.Conversations) != 0 || foreignPage.NextCursor != "" {
		t.Fatalf("foreign role history=%+v err=%v", foreignPage, err)
	}
	blank := scope
	blank.RoleID = ""
	if _, err := p.HistoricalConversationPage(t.Context(), blank, foreignQuery); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("blank role history accepted: %v", err)
	}
	allMemory := scope
	allMemory.RoleID = coordination.AllMemoryRole
	if _, err := p.HistoricalConversationPage(t.Context(), allMemory, foreignQuery); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("all-memory role history accepted: %v", err)
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

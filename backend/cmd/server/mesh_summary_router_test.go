package main

import (
	"bytes"
	"context"
	"encoding/json"
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

type routerSummaryModel struct {
	continuityTestModel
	calls int
}

func (m *routerSummaryModel) GenerateOwnedSummary(_ context.Context, inference business.SummaryInference) (string, error) {
	m.calls++
	return "接口生成的设备摘要", nil
}

func TestMeshSummaryHTTPRequiresActorCurrentPageAndOwnerAcknowledgement(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	db := p.services.KernelContainer.DeviceRegistry.Database()
	if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('device-a','core','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	conversation := chat.Conversation{ID: "summary-chat", SpaceID: p.legacySpaceID, Title: "历史", Channel: "web"}
	if err := p.services.DB.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	if err := p.services.DB.Create(&chat.Message{ID: "legacy-summary-message", ConversationID: conversation.ID, CharacterID: "one", Role: "user", Content: "需要记住的事实", Sequence: 1}).Error; err != nil {
		t.Fatal(err)
	}
	service := coordination.NewService(db)
	model := &routerSummaryModel{}
	engine := business.NewEngine(service, p, model)
	query, err := engine.Query(t.Context(), business.Request{SpaceID: "core", DeviceID: "device-a", CoreID: "core", RoleID: "one"}, coordination.DataQuery{ConversationID: conversation.ID})
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if c.GetHeader("X-Test-Actor") == "yes" {
			actor := &auth.ActorContext{SpaceID: "core", DeviceID: "device-a", PrincipalType: auth.PrincipalTest}
			c.Set("actorContext", actor)
			c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), actor))
		}
	})
	registerMeshBusinessRouter(router.Group("/api"), &AppServices{DeviceMesh: &devicemesh.Runtime{Coordination: service}, OwnedBusiness: engine}, "core")
	invoke := func(actor bool, payload any) *httptest.ResponseRecorder {
		t.Helper()
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, "/api/device-mesh/v1/business/conversations/summary-chat/summary/generate", bytes.NewReader(encoded))
		request.Header.Set("Content-Type", "application/json")
		if actor {
			request.Header.Set("X-Test-Actor", "yes")
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	payload := map[string]any{"requestId": "http-summary", "characterId": "one", "expectedRevision": 0, "expectedExecutionScope": query.Scope, "conversationOrigin": business.ConversationOrigin{OwnerID: "device-a", ID: conversation.ID}}
	if response := invoke(false, payload); response.Code != 401 {
		t.Fatalf("unauthorized request: %d", response.Code)
	}
	if response := invoke(true, map[string]any{"requestId": "missing-page"}); response.Code != 400 {
		t.Fatalf("missing page accepted: %d", response.Code)
	}
	stale := query.Scope
	stale.RoleRevision++
	payload["expectedExecutionScope"] = stale
	if response := invoke(true, payload); response.Code != 409 || model.calls != 0 {
		t.Fatalf("stale page computed summary: %d calls=%d", response.Code, model.calls)
	}
	payload["expectedExecutionScope"] = query.Scope
	for attempt := 0; attempt < 2; attempt++ {
		response := invoke(true, payload)
		var envelope struct {
			Data business.SummaryResponse `json:"data"`
		}
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &envelope) != nil {
			t.Fatalf("summary API failed: %d %s", response.Code, response.Body.String())
		}
		if !envelope.Data.Saved || envelope.Data.OwnerID != "device-a" || envelope.Data.Acknowledgement.OwnerID != "device-a" || envelope.Data.Revision != 1 {
			t.Fatalf("wrong owner or unconfirmed save: %+v", envelope.Data)
		}
	}
	if model.calls != 1 {
		t.Fatalf("replayed summary called model %d times", model.calls)
	}
	core, err := coordination.NewOwnershipStore(db, "core").List(t.Context(), "summary", "one", false)
	if err != nil || len(core) != 0 {
		t.Fatalf("device summary copied into Core: %v", err)
	}
	var messages int64
	if err := p.services.DB.Model(&chat.Message{}).Where("conversation_id = ?", conversation.ID).Count(&messages).Error; err != nil || messages != 1 {
		t.Fatalf("summary modified original messages: %d %v", messages, err)
	}
}

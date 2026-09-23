package system

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/pkg/app"
)

func TestRegisterSystemRouterWiresApprovalBroker(t *testing.T) {
	handler, db := newWebChatScopeTestHandler(t)
	spaceID := requestidentity.CanonicalSpaceID()
	conversationID := "conv-approval"
	if err := db.Exec("INSERT INTO conversations (id, space_id, title, channel, source) VALUES (?, ?, ?, ?, ?)", conversationID, spaceID, "审批测试", "web", "manual").Error; err != nil {
		t.Fatal(err)
	}

	broker := execution.NewApprovalBroker()
	result := make(chan bool, 1)
	go func() {
		approved, err := broker.Await(context.Background(), execution.ApprovalRequest{
			SpaceID:        spaceID,
			ConversationID: conversationID,
			TurnID:         "turn-approval",
			ToolName:       "write_file",
		}, time.Minute)
		if err != nil {
			t.Errorf("Await returned error: %v", err)
		}
		result <- approved
	}()

	var pending []execution.ApprovalRequest
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pending = broker.List(spaceID, conversationID)
		if len(pending) == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(pending) != 1 {
		t.Fatalf("expected one pending approval, got %d", len(pending))
	}

	router := gin.New()
	RegisterSystemRouter(router.Group(""), &app.AppContext{DB: handler.db}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithApprovalBroker(broker))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/web-chat/conversations/"+conversationID+"/turns/turn-approval/approvals/"+pending[0].ID, bytes.NewBufferString(`{"approved":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-User-ID", spaceID)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != http.StatusOK {
		t.Fatalf("code = %d, msg = %s", body.Code, body.Msg)
	}
	select {
	case approved := <-result:
		if !approved {
			t.Fatal("expected approval to continue")
		}
	case <-time.After(time.Second):
		t.Fatal("approval did not resolve")
	}
}

package agentrun

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func testHTTPRouter(t *testing.T) (*Service, *gin.Engine) {
	t.Helper()
	service := testService(t)
	seedCoordination(t, service)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router.Group("/api"), service, "space-a")
	return service, router
}

func performJSON(router http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	var payload bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&payload).Encode(body)
	}
	request := httptest.NewRequest(method, path, &payload)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestHTTPClaimAndAcknowledge(t *testing.T) {
	service, router := testHTTPRouter(t)
	rootID := RootRunID("coordination-a")
	childID := ChildRunID("assignment-a")
	message, err := service.Send(t.Context(), SendInput{
		SpaceID: "space-a", SenderRunID: rootID, RecipientID: childID,
		Kind: MessageInstruction, Content: "run the focused test",
	})
	if err != nil {
		t.Fatal(err)
	}

	claimResponse := performJSON(router, http.MethodPost, "/api/roles/runs/"+childID+"/messages/claim", map[string]any{
		"owner": "http-worker", "leaseSeconds": 30, "limit": 5,
	})
	if claimResponse.Code != http.StatusOK {
		t.Fatalf("claim status=%d body=%s", claimResponse.Code, claimResponse.Body.String())
	}
	var claimEnvelope struct {
		Data Claim `json:"data"`
	}
	if err := json.Unmarshal(claimResponse.Body.Bytes(), &claimEnvelope); err != nil {
		t.Fatal(err)
	}
	if claimEnvelope.Data.LeaseToken == "" || len(claimEnvelope.Data.Messages) != 1 ||
		claimEnvelope.Data.Messages[0].ID != message.ID {
		t.Fatalf("unexpected HTTP claim: %+v", claimEnvelope.Data)
	}

	ackResponse := performJSON(router, http.MethodPost, "/api/roles/runs/"+childID+"/messages/acknowledge", map[string]any{
		"leaseToken": claimEnvelope.Data.LeaseToken, "messageIds": []string{message.ID},
	})
	if ackResponse.Code != http.StatusOK {
		t.Fatalf("acknowledge status=%d body=%s", ackResponse.Code, ackResponse.Body.String())
	}
	var ackEnvelope struct {
		Data struct {
			AcknowledgedCount int64 `json:"acknowledgedCount"`
		} `json:"data"`
	}
	if err := json.Unmarshal(ackResponse.Body.Bytes(), &ackEnvelope); err != nil {
		t.Fatal(err)
	}
	if ackEnvelope.Data.AcknowledgedCount != 1 {
		t.Fatalf("acknowledged count=%d, want 1", ackEnvelope.Data.AcknowledgedCount)
	}
}

func TestHTTPClaimAndAcknowledgeRejectInvalidInput(t *testing.T) {
	_, router := testHTTPRouter(t)
	childID := ChildRunID("assignment-a")

	claimResponse := performJSON(router, http.MethodPost, "/api/roles/runs/"+childID+"/messages/claim", map[string]any{
		"owner": "", "leaseSeconds": 30,
	})
	if claimResponse.Code != http.StatusBadRequest {
		t.Fatalf("empty owner status=%d body=%s", claimResponse.Code, claimResponse.Body.String())
	}
	var claimError struct {
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(claimResponse.Body.Bytes(), &claimError); err != nil {
		t.Fatal(err)
	}
	if claimError.Msg == "" {
		t.Fatal("claim rejection lost the original error")
	}

	ackResponse := performJSON(router, http.MethodPost, "/api/roles/runs/"+childID+"/messages/acknowledge", map[string]any{
		"leaseToken": "stale-lease", "messageIds": []string{"missing-message"},
	})
	if ackResponse.Code != http.StatusBadRequest {
		t.Fatalf("stale acknowledgement status=%d body=%s", ackResponse.Code, ackResponse.Body.String())
	}
	var ackError struct {
		Msg    string `json:"msg"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(ackResponse.Body.Bytes(), &ackError); err != nil {
		t.Fatal(err)
	}
	if ackError.Msg == "" || ackError.Detail != ackError.Msg {
		t.Fatalf("acknowledgement rejection lost the original error: %+v", ackError)
	}
}

package workbenchbrowser

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/browser"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestControlAuthorizationAndViewIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(browser.NewDisabledProvider())
	engine := gin.New()
	h.RegisterRoutes(engine.Group("/api"))
	ref := ExposeAgentTab("scope", "conv", "agent-sess", "agent-tab")
	if ref == "" {
		t.Fatal("ref allocation")
	}
	defer RevokeAgentTab(ref)
	operator := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: runtimeidentity.SpaceID("scope"), DeviceID: "device-one", SessionID: "session-one", Permissions: []string{auth.PermWorkbenchBrowserControl}}
	nonOperator := operator.Clone()
	nonOperator.Permissions = nil
	other := operator.Clone()
	other.DeviceID = "device-two"
	attach := func(actor *auth.ActorContext, view string) {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		context.Request = context.Request.WithContext(auth.WithActor(context.Request.Context(), actor))
		key, ok := ownerKey(context, "conv", view)
		if !ok {
			t.Fatal("invalid test owner")
		}
		h.pages[key] = page{session: "agent-sess", tab: "agent-tab", observed: true, observerRef: ref}
	}
	attach(operator, "first")
	attach(other, "second")
	request := func(actor *auth.ActorContext, view, operation, leaseID string) (int, string) {
		req := httptest.NewRequest(http.MethodPost, "/api/workbench/browser/conv/"+view+"/control", strings.NewReader(`{"action":"`+operation+`","leaseId":"`+leaseID+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(auth.WithActor(req.Context(), actor))
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		var body struct {
			LeaseID string `json:"leaseId"`
		}
		_ = json.Unmarshal(response.Body.Bytes(), &body)
		return response.Code, body.LeaseID
	}
	if got, _ := request(nonOperator, "first", "acquire", ""); got != http.StatusForbidden {
		t.Fatalf("unprivileged device acquired: %d", got)
	}
	status, token := request(operator, "first", "acquire", "")
	if status != http.StatusOK || len(token) < 24 {
		t.Fatalf("owner denied: %d", status)
	}
	if got, _ := request(other, "second", "acquire", ""); got != http.StatusConflict {
		t.Fatalf("competing owner status: %d", got)
	}
	if got, _ := request(other, "second", "release", token); got != http.StatusConflict {
		t.Fatalf("stale or foreign release accepted: %d", got)
	}
	if !currentAgentControl("agent-sess", "agent-tab") {
		t.Fatal("foreign release changed lease")
	}
	if got, _ := request(operator, "first", "release", token); got != http.StatusOK {
		t.Fatalf("release denied: %d", got)
	}
	if got, _ := request(other, "second", "acquire", ""); got != http.StatusOK {
		t.Fatalf("new owner denied: %d", got)
	}
	RevokeAgentTab(ref)
	if got, _ := request(other, "second", "renew", token); got != http.StatusGone {
		t.Fatalf("revoked reference accepted: %d", got)
	}
	if currentAgentControl("agent-sess", "agent-tab") {
		t.Fatal("revocation failed")
	}
}

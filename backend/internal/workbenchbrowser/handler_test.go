package workbenchbrowser

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/browser"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestBrowserURLValidation(t *testing.T) {
	for _, candidate := range []string{"https://example.org", "http://example.com/a?b=1"} {
		if !validateURL(candidate) {
			t.Fatalf("safe URL rejected: %s", candidate)
		}
	}
	for _, candidate := range []string{"", "file:///etc/passwd", "javascript:alert(1)", "https://user:password@example.org", "https://", "//example.org", "ftp://example.org"} {
		if validateURL(candidate) {
			t.Fatalf("unsafe URL accepted: %s", candidate)
		}
	}
}

func TestBrowserOwnerKeyIsolation(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	if _, ok := ownerKey(c, "conversation", "tab"); ok {
		t.Fatal("unauthenticated browser access accepted")
	}
	actor := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: runtimeidentity.SpaceID("space"), DeviceID: runtimeidentity.DeviceID("device-one"), SessionID: "session-one"}
	c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), actor))
	first, ok := ownerKey(c, "conversation", "tab")
	if !ok {
		t.Fatal("valid device identity rejected")
	}
	differentTab, _ := ownerKey(c, "conversation", "tab-two")
	differentConversation, _ := ownerKey(c, "conversation-two", "tab")
	if first == differentTab || first == differentConversation {
		t.Fatal("scope collision")
	}
	actor.DeviceID = runtimeidentity.DeviceID("device-two")
	c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), actor))
	differentDevice, _ := ownerKey(c, "conversation", "tab")
	if first == differentDevice {
		t.Fatal("cross-device scope collision")
	}
	if _, ok := ownerKey(c, "../../bad", "tab"); ok {
		t.Fatal("unsafe conversation identifier allowed")
	}
	if _, ok := ownerKey(c, "conversation", strings.Repeat("z", 161)); ok {
		t.Fatal("oversize view identifier allowed")
	}
}

func TestDisabledBrowserCapabilitiesAndOpen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler := NewHandler(browser.NewDisabledProvider())
	handler.RegisterRoutes(engine.Group("/api"))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/workbench/browser/capabilities", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"available":false`) {
		t.Fatalf("disabled capability result: %d %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/workbench/browser/open", strings.NewReader(`{"conversationId":"conv","viewId":"tab"}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(auth.WithActor(request.Context(), &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, IsLocalTrusted: true, SpaceID: "space", DeviceID: "device", Permissions: []string{auth.PermWorkbenchBrowserControl}}))
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled runtime accepted open: %d", response.Code)
	}
}

func TestBrowserInputFiniteBounds(t *testing.T) {
	for _, valid := range []float64{-2000, -1, 0, 0.5, 16384} {
		if !isFinite(valid) {
			t.Fatalf("expected finite: %v", valid)
		}
	}
	for _, invalid := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if isFinite(invalid) {
			t.Fatal("non-finite browser pointer accepted")
		}
	}
}

func TestBrowserIdleCleanup(t *testing.T) {
	handler := NewHandler(browser.NewDisabledProvider())
	handler.pages["expired"] = page{session: "stale", tab: "tab", created: time.Now().Add(-time.Hour), lastUsed: time.Now().Add(-time.Hour)}
	handler.pages["live"] = page{session: "active", tab: "tab", created: time.Now(), lastUsed: time.Now()}
	handler.pruneIdle(context.Background())
	if _, present := handler.pages["expired"]; present {
		t.Fatal("idle browser session not pruned")
	}
	if _, present := handler.pages["live"]; !present {
		t.Fatal("active browser session unexpectedly pruned")
	}
}

func TestDOMContentSizeAndUTF8Boundary(t *testing.T) {
	short := "你好"
	if got, trimmed := boundDOMContent(short); got != short || trimmed {
		t.Fatal("short DOM changed")
	}
	payload := strings.Repeat("x", maxBrowserDOMBytes-1) + "你" + strings.Repeat("z", 30)
	got, trimmed := boundDOMContent(payload)
	if !trimmed || len(got) > maxBrowserDOMBytes || !utf8.ValidString(got) {
		t.Fatal("unsafe DOM truncation")
	}
	if got != strings.Repeat("x", maxBrowserDOMBytes-1) {
		t.Fatal("split UTF-8 rune not removed")
	}
}

func TestIdleObservedViewCannotRevokeAnotherViewControl(t *testing.T) {
	session, tab := browser.BrowserSessionID("agent-shared-idle"), browser.BrowserTabID("agent-shared-tab")
	ref := ExposeAgentTab("shared-space-idle", "shared-conversation-idle", session, tab)
	if ref == "" {
		t.Fatal("missing agent reference")
	}
	defer RevokeAgentTab(ref)
	handler := NewHandler(browser.NewDisabledProvider())
	now := time.Now()
	handler.pages["idle-observer"] = page{session: session, tab: tab, observed: true, observerRef: ref, lastUsed: now.Add(-time.Hour)}
	handler.pages["live-observer"] = page{session: session, tab: tab, observed: true, observerRef: ref, lastUsed: now}
	if _, ok := acquireAgentControl(ref, "live-observer", session, tab); !ok {
		t.Fatal("control not granted")
	}
	handler.pruneIdle(context.Background())
	if _, ok := handler.pages["idle-observer"]; ok {
		t.Fatal("idle observer retained")
	}
	if !hasAgentControl(ref, "live-observer", session, tab) {
		t.Fatal("idle observer revoked another observer control")
	}
	releaseAgentControl(ref, "live-observer", session, tab)
}

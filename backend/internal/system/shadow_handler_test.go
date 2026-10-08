package system

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
)

func setupShadowTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, Permissions: []string{auth.PermSystemAdmin}, IsLocalTrusted: true})
		c.Next()
	})
	handler := &Handler{}
	shadowGroup := r.Group("/api")
	RegisterShadowRouter(shadowGroup, handler)
	return r
}

func TestShadowModeStatus(t *testing.T) {
	r := setupShadowTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/shadow/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestShadowModeStart(t *testing.T) {
	r := setupShadowTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/shadow/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestShadowModeStop(t *testing.T) {
	r := setupShadowTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/shadow/stop", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestShadowModeThresholds(t *testing.T) {
	r := setupShadowTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/shadow/thresholds", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestShadowModeCompare(t *testing.T) {
	r := setupShadowTestRouter()
	payload, err := json.Marshal(map[string]any{
		"oldMetrics": map[string]any{},
		"newMetrics": map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/shadow/compare", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestShadowModeLoadSim(t *testing.T) {
	r := setupShadowTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/shadow/load-sim", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

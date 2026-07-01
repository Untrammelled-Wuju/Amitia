package system

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupShadowTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
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
	req := httptest.NewRequest(http.MethodPost, "/api/shadow/compare", nil)
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

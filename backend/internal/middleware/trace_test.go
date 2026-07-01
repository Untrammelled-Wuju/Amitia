// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTraceMiddleware_GeneratesRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware())
	router.GET("/test", func(c *gin.Context) {
		requestID, exists := c.Get(CtxKeyRequestID)
		if !exists || requestID.(string) == "" {
			t.Error("request_id not set")
		}
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	respHeader := w.Header().Get(HeaderRequestID)
	if respHeader == "" {
		t.Error("X-Request-Id header not set in response")
	}
}

func TestTraceMiddleware_UsesExistingRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware())
	router.GET("/test", func(c *gin.Context) {
		requestID, _ := c.Get(CtxKeyRequestID)
		if requestID.(string) != "existing-request-id" {
			t.Errorf("expected existing-request-id, got %s", requestID.(string))
		}
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(HeaderRequestID, "existing-request-id")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
}

func TestTraceMiddleware_CorrelationIDDerived(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware())
	router.GET("/test", func(c *gin.Context) {
		requestID, _ := c.Get(CtxKeyRequestID)
		correlationID, _ := c.Get(CtxKeyCorrelationID)
		if requestID.(string) != correlationID.(string) {
			t.Error("correlation_id should equal request_id when no header provided")
		}
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
}

func TestTraceMiddleware_CausationIDGenerated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware())
	router.GET("/test", func(c *gin.Context) {
		causationID, _ := c.Get(CtxKeyCausationID)
		if causationID.(string) == "" {
			t.Error("causation_id should be generated")
		}
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
}

func TestTraceMiddleware_PathSet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware())
	router.GET("/api/test/path", func(c *gin.Context) {
		path, _ := c.Get(CtxKeyTracePath)
		if path.(string) != "GET /api/test/path" {
			t.Errorf("expected GET /api/test/path, got %s", path.(string))
		}
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/test/path", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
}

func TestTraceMiddleware_Status200(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestTraceMiddleware_Status404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusNotFound, "not found")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

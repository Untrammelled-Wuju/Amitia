// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCorsMiddleware_AllowsDesktopDevelopmentUploadPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CorsMiddleware(CorsConfig{}))
	router.POST("/api/extensions/packages/artifacts", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodOptions, "/api/extensions/packages/artifacts", nil)
	request.Header.Set("Origin", "http://localhost:15178")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set(
		"Access-Control-Request-Headers",
		"content-type,cache-control,x-amitia-desktop-session,x-amitia-desktop-instance,x-amitia-device-id,x-amitia-client-type,x-amitia-management-target,idempotency-key",
	)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected preflight status %d, got %d", http.StatusNoContent, response.Code)
	}
	allowed := strings.ToLower(response.Header().Get("Access-Control-Allow-Headers"))
	for _, header := range []string{
		"content-type",
		"cache-control",
		"x-amitia-desktop-session",
		"x-amitia-desktop-instance",
		"x-amitia-device-id",
		"x-amitia-client-type",
		"x-amitia-management-target",
		"idempotency-key",
	} {
		if !strings.Contains(allowed, header) {
			t.Fatalf("expected Access-Control-Allow-Headers to contain %s, got %q", header, allowed)
		}
	}
}

func TestCorsMiddleware_AllowsPackagedAppOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CorsMiddleware(CorsConfig{}))
	router.GET("/api/desktop-pets/installations", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodGet, "/api/desktop-pets/installations", nil)
	request.Header.Set("Origin", "app://amitia")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected packaged app request status %d, got %d", http.StatusOK, response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "app://amitia" {
		t.Fatalf("expected packaged app origin to be echoed, got %q", got)
	}
}

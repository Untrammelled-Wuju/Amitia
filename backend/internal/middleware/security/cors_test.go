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

func TestCorsMiddleware_AllowsDesktopDeviceHeadersForLoopbackPetRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CorsMiddleware(CorsConfig{
		AllowedOrigins: []string{"http://cloud.example.test"},
	}))
	router.GET("/api/desktop-pets/installations", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodOptions, "/api/desktop-pets/installations", nil)
	request.Header.Set("Origin", "http://cloud.example.test")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	request.Header.Set(
		"Access-Control-Request-Headers",
		"x-amitia-desktop-session,x-amitia-desktop-instance,x-amitia-device-id,x-amitia-client-type",
	)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected preflight status %d, got %d", http.StatusNoContent, response.Code)
	}
	allowed := strings.ToLower(response.Header().Get("Access-Control-Allow-Headers"))
	for _, header := range []string{
		"x-amitia-desktop-session",
		"x-amitia-desktop-instance",
		"x-amitia-device-id",
		"x-amitia-client-type",
	} {
		if !strings.Contains(allowed, header) {
			t.Fatalf("expected Access-Control-Allow-Headers to contain %s, got %q", header, allowed)
		}
	}
}

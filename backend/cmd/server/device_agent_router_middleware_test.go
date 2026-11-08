package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/middleware/security"
)

func TestDeviceAgentLocalMeshAuthenticatesBeforeLocalControl(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	middlewares := deviceAgentLocalMeshMiddlewares(
		func(c *gin.Context) {
			actor := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, IsLocalTrusted: true}
			c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), actor))
			c.Next()
		},
		security.LocalRuntimeControlMiddleware(nil, ""),
	)
	router.Use(middlewares...)
	router.GET("/internal/device-mesh/provider/api/health", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/internal/device-mesh/provider/api/health", nil)
	request.RemoteAddr = "127.0.0.1:49152"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

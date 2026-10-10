package server

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestCoreDeviceCannotBeRevoked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.DELETE("/devices/:deviceId", makeRevokeDeviceHandler(&RouterDeps{
		LocalCoreDeviceID: "core-console",
		GetSpaceID:        func(*gin.Context) (runtimeidentity.SpaceID, bool) { return "space", true },
	}))
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest("DELETE", "/devices/core-console", nil))
	if response.Code != 409 {
		t.Fatalf("%d %s", response.Code, response.Body.String())
	}
}

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestListDevicesWithoutRegistryReturnsEmptyList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/device-mesh/v1/devices", makeListDevicesHandler(&RouterDeps{
		GetSpaceID: func(*gin.Context) (runtimeidentity.SpaceID, bool) {
			return runtimeidentity.SpaceID("space-test"), true
		},
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/device-mesh/v1/devices", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.String() != "{\"devices\":[]}" {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
}

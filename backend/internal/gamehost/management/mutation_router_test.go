package management

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGameCenterExtensionMutationAcceptsBodyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterGameCenterMutationRouter(router.Group("/api"), NewMutationHandler(&PackageMutationService{}, nil))

	request := httptest.NewRequest(http.MethodPost, "/api/game-center/extensions/enable", strings.NewReader(`{"extensionId":"com.amitiax/minecraft"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code == http.StatusNotFound || response.Code == http.StatusBadRequest {
		t.Fatalf("body-based extension id was not routed: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestGameCenterPluginDetailQueryRouteAvoidsSlashIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterGameCenterRouter(router.Group("/api"), nil)

	request := httptest.NewRequest(http.MethodGet, "/api/game-center/plugins/detail?pluginId=com.amitiax%2Fminecraft%2Fminecraft&extensionId=com.amitiax%2Fminecraft", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code == http.StatusNotFound {
		t.Fatalf("query-based plugin detail route was not registered: body=%s", response.Body.String())
	}
}

func TestGameCenterRearmRouteUsesConfiguredService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotRuntimeID string
	handler := NewControlHandlerFromFuncs(ControlServiceOptions{
		RearmFn: func(ctx context.Context, runtimeID string) error {
			gotRuntimeID = runtimeID
			return nil
		},
	})
	router := gin.New()
	RegisterGameCenterControlRouter(router.Group("/api"), handler)

	request := httptest.NewRequest(http.MethodPost, "/api/game-center/runtimes/rt_test/rearm", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("rearm route returned status=%d body=%s", response.Code, response.Body.String())
	}
	if gotRuntimeID != "rt_test" {
		t.Fatalf("rearm route passed runtime id %q", gotRuntimeID)
	}
}

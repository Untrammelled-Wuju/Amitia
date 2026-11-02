package extension

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	extensionkernel "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/extension_center"
	"github.com/u-ai/backend/internal/extension/kernel/extension_detail"
)

type routeEmptyCardProvider struct{}

func (routeEmptyCardProvider) ListCards(context.Context) ([]extension_center.ExtensionCard, error) {
	return []extension_center.ExtensionCard{}, nil
}

func TestRegisterExtensionCenterRoutesUsesCanonicalView(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	container := &extensionkernel.Container{
		ExtensionCenterService: extension_center.NewCenterService(routeEmptyCardProvider{}),
		ExtensionDetailService: extension_detail.NewDetailService(nil),
	}
	RegisterExtensionCenterRoutesForContainer(router.Group("/api"), container)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/extension-center/view", nil)
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var view extension_center.CenterView
	if err := json.NewDecoder(response.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	if view.SchemaVersion != 1 || view.Updates == nil || view.Installed == nil {
		t.Fatalf("canonical view contract invalid: %+v", view)
	}
}

func TestRegisterExtensionCenterRoutesSkipsUnavailableServices(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterExtensionCenterRoutesForContainer(router.Group("/api"), &extensionkernel.Container{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/extension-center/view", nil)
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unavailable service must not register route, status = %d", response.Code)
	}
}

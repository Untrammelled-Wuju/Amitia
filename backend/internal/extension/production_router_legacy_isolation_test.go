package extension

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/pkg/app"
)

type productionRouteIdentity struct {
	Method string
	Path   string
}

func expectedRetiredProductionRoutes() []productionRouteIdentity {
	result :=
		make(
			[]productionRouteIdentity,
			0,
			len(retiredExtensionLegacyRoutes)+
				len(retiredRootLegacyRoutes),
		)

	for _, route := range retiredExtensionLegacyRoutes {
		result = append(
			result,
			productionRouteIdentity{
				Method: route.Method,
				Path:   "/api/extensions" + route.Path,
			},
		)
	}

	for _, route := range retiredRootLegacyRoutes {
		result = append(
			result,
			productionRouteIdentity{
				Method: route.Method,
				Path:   "/api" + route.Path,
			},
		)
	}

	sort.Slice(
		result,
		func(left, right int) bool {
			if result[left].Path == result[right].Path {
				return result[left].Method < result[right].Method
			}
			return result[left].Path < result[right].Path
		},
	)

	return result
}

func buildProductionExtensionRouter(t *testing.T) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)

	if config.AppCfg == nil {
		config.AppCfg = &config.Config{}
	}

	config.AppCfg.Storage.DataDir = t.TempDir()

	engine := gin.New()

	RegisterRouter(
		engine.Group("/api"),
		&app.AppContext{},
		&Runtime{},
	)

	return engine
}

func routeMap(engine *gin.Engine) map[productionRouteIdentity]string {
	result := make(map[productionRouteIdentity]string, 128)

	for _, route := range engine.Routes() {
		result[productionRouteIdentity{
			Method: route.Method,
			Path:   route.Path,
		}] = route.Handler
	}

	return result
}

func expandRouteParameters(path string) string {
	replacements := map[string]string{
		":sessionId":   "session-test",
		":operationId": "operation-test",
		":fingerprint": "fingerprint-test",
		":exportId":    "export-test",
		":version":     "1.0.0",
		":id":          "extension-test",
	}

	for parameter, value := range replacements {
		path = strings.ReplaceAll(path, parameter, value)
	}

	return path
}

func snapshotDirectory(t *testing.T, root string) []string {
	t.Helper()

	var result []string

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		result = append(result, relative)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(result)
	return result
}

func TestProductionRouterLegacyMethodPathSnapshot(t *testing.T) {
	engine := buildProductionExtensionRouter(t)

	actual := routeMap(engine)

	for _, expected := range expectedRetiredProductionRoutes() {
		handler, exists := actual[expected]

		if !exists {
			t.Fatalf(
				"retired legacy route missing: %s %s",
				expected.Method,
				expected.Path,
			)
		}

		if !strings.Contains(handler, "retiredLegacyPackageEndpoint") {
			t.Fatalf(
				"legacy route uses non-retired handler: %s %s → %s",
				expected.Method,
				expected.Path,
				handler,
			)
		}
	}
}

func TestProductionRouterContainsNoPackageServiceHandlers(t *testing.T) {
	engine := buildProductionExtensionRouter(t)

	for _, route := range engine.Routes() {
		handlerName := route.Handler

		if strings.Contains(handlerName, "previewPackageInstall(") {
			t.Fatalf("production router references legacy install package function: %s %s", route.Method, route.Path)
		}

		if strings.Contains(handlerName, "installPackage(") {
			t.Fatalf("production router references legacy install package: %s %s", route.Method, route.Path)
		}

		if strings.Contains(handlerName, "PackageService") {
			t.Fatalf("production router contains PackageService reference: %s for %s %s", handlerName, route.Method, route.Path)
		}
	}
}

func TestRetiredRoutesRespondWithGone(t *testing.T) {
	engine := buildProductionExtensionRouter(t)

	uniquePaths := map[string]productionRouteIdentity{}
	for _, expected := range expectedRetiredProductionRoutes() {
		uniquePaths[expected.Method+" "+expected.Path] = expected
	}

	for _, expected := range uniquePaths {
		path := expandRouteParameters(expected.Path)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(expected.Method, path, nil)
		engine.ServeHTTP(recorder, req)

		if recorder.Code == http.StatusNotFound {
			t.Fatalf("retired route not registered: %s %s", expected.Method, expected.Path)
		}

		if recorder.Code == http.StatusMethodNotAllowed {
			continue
		}

		if recorder.Code != http.StatusGone {
			t.Fatalf("retired route %s %s returned %d instead of 410: %s", expected.Method, expected.Path, recorder.Code, recorder.Body.String())
		}

		body := recorder.Body.String()
		if !strings.Contains(body, "LEGACY_PACKAGE_ENDPOINT_RETIRED") {
			t.Fatalf("retired route %s %s 410 body missing error_code: %s", expected.Method, expected.Path, body)
		}
	}
}

func TestSkillRoutesRemainActive(t *testing.T) {
	engine := buildProductionExtensionRouter(t)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/extensions/skills", nil)
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for /extensions/skills (auth required), got %d. body: %s", recorder.Code, recorder.Body.String())
	}
}

func TestOpenAPIRouteRemainsActive(t *testing.T) {
	engine := buildProductionExtensionRouter(t)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/extensions/openapi.json", nil)
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for /extensions/openapi.json, got %d", recorder.Code)
	}
}

func TestRouterRouteSnapshotDuplicatedHandler(t *testing.T) {
	_ = snapshotDirectory
	engine := buildProductionExtensionRouter(t)

	routes := engine.Routes()
	retiredCount := 0
	for _, route := range routes {
		if strings.Contains(route.Handler, "retired") {
			retiredCount++
		}
	}

	expectedCount := len(retiredExtensionLegacyRoutes) + len(retiredRootLegacyRoutes)
	if retiredCount != expectedCount {
		t.Fatalf("expected %d retired routes, got %d", expectedCount, retiredCount)
	}
}

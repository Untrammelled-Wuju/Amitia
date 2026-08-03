package extension

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/pkg/app"
)

var forbiddenHandlerPatterns = []*regexp.Regexp{
	regexp.MustCompile(`PackageHandler\.`),
	regexp.MustCompile(`\.Preview$`),
	regexp.MustCompile(`\.PreviewUpgrade$`),
	regexp.MustCompile(`\.Install$`),
	regexp.MustCompile(`\.Upgrade$`),
	regexp.MustCompile(`\.Export$`),
	regexp.MustCompile(`\.Download$`),
	regexp.MustCompile(`\.Rollback$`),
	regexp.MustCompile(`\.Uninstall$`),
	regexp.MustCompile(`\.PreviewUninstall$`),
	regexp.MustCompile(`\.PreviewRollback$`),
	regexp.MustCompile(`\.ConfirmRollback$`),
	regexp.MustCompile(`\.VerifyFinalGate$`),
	regexp.MustCompile(`\.Sessions$`),
	regexp.MustCompile(`\.CancelSession$`),
	regexp.MustCompile(`\.Operation$`),
	regexp.MustCompile(`\.Operations$`),
	regexp.MustCompile(`\.Signers$`),
	regexp.MustCompile(`\.TrustSigner$`),
	regexp.MustCompile(`\.UntrustSigner$`),
	regexp.MustCompile(`\.Versions$`),
	regexp.MustCompile(`\.Compare$`),
	regexp.MustCompile(`\.Dependencies$`),
}

var retiredLegacyWritePaths = map[string]struct{}{
	"POST /packages/import/preview":                {},
	"POST /packages/import/install":               {},
	"GET /packages/import/sessions/:sessionId":     {},
	"DELETE /packages/import/sessions/:sessionId":  {},
	"GET /packages/metrics":                        {},
	"GET /package-operations":                      {},
	"GET /package-operations/:operationId":         {},
	"GET /package-operations/:operationId/final-gate": {},
	"GET /signers":                                 {},
	"POST /signers/:fingerprint/trust":             {},
	"POST /signers/:fingerprint/untrust":           {},
	"GET /:id/exports/:exportId":                   {},
	"POST /:id/export":                             {},
	"POST /:id/upgrade/preview":                    {},
	"POST /:id/upgrade":                            {},
	"GET /:id/versions":                            {},
	"GET /:id/versions/compare":                    {},
	"POST /:id/versions/:version/rollback":         {},
	"GET /:id/versions/:version/rollback/preview":  {},
	"POST /:id/versions/:version/rollback/preview": {},
	"GET /:id/dependencies":                        {},
	"GET /:id/uninstall/preview":                   {},
	"POST /:id/uninstall/preview":                  {},
	"POST /:id/uninstall":                          {},
	"DELETE /:id":                                  {},
}

func TestProductionRouterExcludesLegacyWriteRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ensureTestConfig()
	engine := gin.New()
	RegisterRouter(engine.Group("/api"), &app.AppContext{}, &Runtime{})

	routes := engine.Routes()
	for _, route := range routes {
		handlerName := route.Handler
		for _, pattern := range forbiddenHandlerPatterns {
			if pattern.MatchString(handlerName) {
				t.Fatalf("production route %s %s uses legacy write handler: %s", route.Method, route.Path, handlerName)
			}
		}
	}
}

func TestRetiredPackageEndpointsReturnGone(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name    string
		handler func(c *gin.Context)
		method  string
		path    string
	}{
		{"PackagePreviewEndpoint", retiredPackagePreviewEndpoint, http.MethodPost, "/packages/import/preview"},
		{"PackageInstallEndpoint", retiredPackageInstallEndpoint, http.MethodPost, "/packages/install"},
		{"LegacyPackageEndpoint", retiredLegacyPackageEndpoint, http.MethodGet, "/packages/metrics"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(tc.method, tc.path, nil)
			tc.handler(ctx)

			if recorder.Code != http.StatusGone {
				t.Fatalf("expected 410, got %d", recorder.Code)
			}

			body := recorder.Body.String()
			if !strings.Contains(body, "RETIRED") {
				t.Fatalf("response should include RETIRED, got: %s", body)
			}
		})
	}
}

func TestRetiredLegacyRoutesHaveActiveRetiredPathsRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ensureTestConfig()
	engine := gin.New()
	RegisterRouter(engine.Group("/api"), &app.AppContext{}, &Runtime{})

	routeMap := map[string]string{}
	for _, route := range engine.Routes() {
		key := route.Method + " " + strings.TrimPrefix(route.Path, "/api")
		routeMap[key] = route.Handler
	}

	for retiredPath := range retiredLegacyWritePaths {
		handlerName, exists := routeMap[retiredPath]
		if !exists {
			continue
		}
		if !strings.Contains(handlerName, "retired") {
			t.Fatalf("legacy path %s registered with non-retired handler: %s", retiredPath, handlerName)
		}
	}
}

func TestProductionRouterContainsNoPackageServiceHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ensureTestConfig()
	engine := gin.New()
	RegisterRouter(engine.Group("/api"), &app.AppContext{}, &Runtime{})

	routes := engine.Routes()
	for _, route := range routes {
		handlerName := route.Handler
		if strings.Contains(handlerName, "PackageHandler") {
			t.Fatalf("production router contains PackageHandler reference: %s for %s %s", handlerName, route.Method, route.Path)
		}
		if strings.Contains(handlerName, "packageHandler") {
			t.Fatalf("production router contains packageHandler reference: %s for %s %s", handlerName, route.Method, route.Path)
		}
		if strings.Contains(handlerName, "NewPackageHandler") {
			t.Fatalf("production router references NewPackageHandler")
		}
	}
}

func ensureTestConfig() {
	if config.AppCfg == nil {
		config.AppCfg = &config.Config{}
	}
}

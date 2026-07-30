package extension

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/pkg/app"
)

func TestOpenAPICoversExtensionBusinessRoutes(t *testing.T) {
	var document struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	raw, err := openAPIFS.ReadFile("schema/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	originalConfig := config.AppCfg
	if config.AppCfg == nil {
		config.AppCfg = &config.Config{}
	}
	defer func() { config.AppCfg = originalConfig }()
	engine := gin.New()
	RegisterRouter(engine.Group("/api"), &app.AppContext{}, &Runtime{})
	for _, route := range engine.Routes() {
		path := strings.TrimPrefix(route.Path, "/api")
		if path == "/extensions/openapi.json" {
			continue
		}
		path = strings.NewReplacer(":", "{", "/", "/").Replace(path)
		parts := strings.Split(path, "/")
		for index, part := range parts {
			if strings.HasPrefix(part, "{") && !strings.HasSuffix(part, "}") {
				parts[index] = part + "}"
			}
		}
		path = strings.Join(parts, "/")
		methods, ok := document.Paths[path]
		if !ok {
			t.Errorf("OpenAPI missing path %s", path)
			continue
		}
		if _, ok := methods[strings.ToLower(route.Method)]; !ok {
			t.Errorf("OpenAPI missing method %s %s", route.Method, path)
		}
	}
}

func TestRetiredPackageInstallEndpointReturnsGone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/extensions/packages/install", nil)
	retiredPackageInstallEndpoint(ctx)
	if recorder.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d", recorder.Code)
	}
	var payload map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["error_code"] != "PACKAGE_INSTALL_ENDPOINT_RETIRED" || payload["replacement_endpoint"] != packageAPIReplacement {
		t.Fatalf("unexpected retirement response: %v", payload)
	}
}

func TestKernelAPIUserUsesExtensionAuthenticationIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set(authenticatedUserKey, 42)
	if userID := kernelAPIUser(ctx); userID != "42" {
		t.Fatalf("unexpected authenticated user: %s", userID)
	}
	missing, _ := gin.CreateTestContext(httptest.NewRecorder())
	if userID := kernelAPIUser(missing); userID != "" {
		t.Fatalf("missing authentication must not use a shared fallback identity: %s", userID)
	}
}

func TestRetiredPackageRoutesOpenAPIContract(t *testing.T) {
	var document struct {
		Paths map[string]map[string]struct {
			Deprecated  bool                       `json:"deprecated"`
			Responses   map[string]json.RawMessage `json:"responses"`
			Replacement string                     `json:"x-replacement-endpoint"`
		} `json:"paths"`
	}
	raw, err := openAPIFS.ReadFile("schema/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	paths := []string{
		"/extensions/kernel/extensions/install",
		"/extensions/kernel/extensions/preview",
		"/extensions/packages/import/install",
		"/extensions/packages/import/preview",
		"/extensions/packages/install",
		"/extensions/{id}/upgrade",
		"/extensions/{id}/upgrade/preview",
	}
	for _, path := range paths {
		operation, ok := document.Paths[path]["post"]
		if !ok {
			t.Fatalf("retired route missing from OpenAPI: %s", path)
		}
		if !operation.Deprecated || operation.Replacement != packageAPIReplacement {
			t.Fatalf("retired route contract incomplete: %s", path)
		}
		if _, ok := operation.Responses["410"]; !ok || len(operation.Responses) != 1 {
			t.Fatalf("retired route must only declare 410: %s", path)
		}
	}
}

func TestProductionPackageIngressHasNoDirectInstallPrimitive(t *testing.T) {
	files := []string{
		"kernel_handler.go",
		"kernel_api.go",
		"router.go",
		"desktop_api_adapter.go",
		"dev_mode_api.go",
	}
	forbidden := []string{
		"runtime.Kernel.Install(",
		".ExecuteInstall(",
		"installLegacyPackage(",
		"PutInstallation(",
		"os.Rename(",
		"os.RemoveAll(",
		"packageHandler.Install",
		"packageHandler.Upgrade",
	}
	for _, name := range files {
		raw, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatal(err)
		}
		source := string(raw)
		for _, token := range forbidden {
			if strings.Contains(source, token) {
				t.Fatalf("production ingress %s contains direct install primitive %s", name, token)
			}
		}
	}
}

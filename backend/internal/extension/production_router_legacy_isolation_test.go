package extension

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/u-ai/backend/config"
	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type productionRouteIdentity struct {
	Method string
	Path   string
}

type productionRouterFixture struct {
	Engine  *gin.Engine
	DataDir string
	SQLDB   *sql.DB
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

func buildProductionExtensionRouterFixture(t *testing.T) productionRouterFixture {
	t.Helper()

	gin.SetMode(gin.TestMode)

	if config.AppCfg == nil {
		config.AppCfg = &config.Config{}
	}

	dataDir := t.TempDir()
	config.AppCfg.Storage.DataDir = dataDir

	db, err := gorm.Open(
		sqlite.Open(filepath.Join(dataDir, "router-isolation.db")),
		&gorm.Config{},
	)
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	engine := gin.New()

	RegisterRouter(
		engine.Group("/api"),
		app.NewAppContext(db, nil),
		&Runtime{},
	)

	return productionRouterFixture{
		Engine:  engine,
		DataDir: dataDir,
		SQLDB:   sqlDB,
	}
}

func buildProductionExtensionRouter(t *testing.T) *gin.Engine {
	t.Helper()

	return buildProductionExtensionRouterFixture(t).Engine
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

func hashFile(t *testing.T, path string) string {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	hasher := sha256.New()

	if _, err := io.Copy(hasher, file); err != nil {
		t.Fatal(err)
	}

	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
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

		info, err := entry.Info()
		if err != nil {
			return err
		}

		contentHash := ""

		if !entry.IsDir() {
			contentHash = hashFile(t, path)
		}

		result = append(
			result,
			fmt.Sprintf(
				"%s|%s|%d|%s",
				relative,
				info.Mode().String(),
				info.Size(),
				contentHash,
			),
		)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(result)
	return result
}

func sqliteTotalChanges(t *testing.T, db *sql.DB) int64 {
	t.Helper()

	var changes int64
	require.NoError(t, db.QueryRow(`SELECT total_changes()`).Scan(&changes))

	return changes
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

func TestRetiredRouteRegistryContainsNoDuplicateMethodPath(t *testing.T) {
	all := append(
		append([]retiredLegacyRoute(nil), retiredExtensionLegacyRoutes...),
		retiredRootLegacyRoutes...,
	)

	for left := 0; left < len(all); left++ {
		for right := left + 1; right < len(all); right++ {
			if all[left].Method == all[right].Method &&
				all[left].Path == all[right].Path {
				t.Fatalf(
					"duplicate retired route in registry: %s %s",
					all[left].Method,
					all[left].Path,
				)
			}
		}
	}

	engine := buildProductionExtensionRouter(t)

	for _, route := range engine.Routes() {
		if !strings.Contains(route.Handler, "retiredLegacyPackageEndpoint") {
			continue
		}

		found := false

		for _, candidate := range all {
			if route.Method == candidate.Method &&
				route.Path == "/api/extensions"+candidate.Path ||
				route.Method == candidate.Method &&
					route.Path == "/api"+candidate.Path {
				found = true
				break
			}
		}

		if !found {
			t.Fatalf(
				"registered retired route missing from registry: %s %s",
				route.Method,
				route.Path,
			)
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

		if recorder.Code != http.StatusGone {
			t.Fatalf("retired route %s %s returned %d instead of 410: %s", expected.Method, expected.Path, recorder.Code, recorder.Body.String())
		}

		body := recorder.Body.String()
		if !strings.Contains(body, "LEGACY_PACKAGE_ENDPOINT_RETIRED") {
			t.Fatalf("retired route %s %s 410 body missing error_code: %s", expected.Method, expected.Path, body)
		}
	}
}

func TestRetiredRoutesRespondWithGoneWithoutSideEffects(t *testing.T) {
	fixture := buildProductionExtensionRouterFixture(t)

	beforeChanges := sqliteTotalChanges(t, fixture.SQLDB)
	beforeCalls := kernelruntime.GlobalLegacyCallCounter().Total()
	beforePackageWrites := kernelruntime.GlobalLegacyCallCounter().PackageWriteCalls()
	beforeSnapshot := snapshotDirectory(t, fixture.DataDir)

	uniquePaths := map[string]productionRouteIdentity{}
	for _, expected := range expectedRetiredProductionRoutes() {
		uniquePaths[expected.Method+" "+expected.Path] = expected
	}

	for _, expected := range uniquePaths {
		path := expandRouteParameters(expected.Path)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(expected.Method, path, nil)
		fixture.Engine.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusGone {
			t.Fatalf("retired route %s %s returned %d instead of 410: %s", expected.Method, expected.Path, recorder.Code, recorder.Body.String())
		}

		body := recorder.Body.String()
		if !strings.Contains(body, "LEGACY_PACKAGE_ENDPOINT_RETIRED") {
			t.Fatalf("retired route %s %s 410 body missing error_code: %s", expected.Method, expected.Path, body)
		}
	}

	afterChanges := sqliteTotalChanges(t, fixture.SQLDB)
	afterCalls := kernelruntime.GlobalLegacyCallCounter().Total()
	afterPackageWrites := kernelruntime.GlobalLegacyCallCounter().PackageWriteCalls()
	afterSnapshot := snapshotDirectory(t, fixture.DataDir)

	if afterChanges != beforeChanges {
		t.Fatalf("retired route requests mutated the database: total_changes %d -> %d", beforeChanges, afterChanges)
	}

	if afterCalls != beforeCalls {
		t.Fatalf("retired route requests mutated the legacy call counter: %d -> %d", beforeCalls, afterCalls)
	}

	if afterPackageWrites != beforePackageWrites {
		t.Fatalf("retired route requests performed package writes: %d -> %d", beforePackageWrites, afterPackageWrites)
	}

	if strings.Join(afterSnapshot, "\n") != strings.Join(beforeSnapshot, "\n") {
		t.Fatalf("retired route requests mutated the data directory")
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

package extension

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/middleware/security"
)

func TestTrustedServiceReferenceRejectsAmbiguousAndMissingTargets(t *testing.T) {
	engine := gin.New()
	engine.GET("/services", trustedServiceReference(func(c *gin.Context) { c.JSON(200, gin.H{"id": c.Param("serviceId")}) }))
	engine.GET("/services/:serviceId", trustedServiceReference(func(c *gin.Context) { c.JSON(200, gin.H{"id": c.Param("serviceId")}) }))
	for _, path := range []string{"/services", "/services?service_id=", "/services?service_id=%20", "/services?service_id=a&service_id=a", "/services?service_id=a&service_id=b", "/services/a?service_id=b", "/services?service_id=%zz"} {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != 400 {
			t.Fatalf("ambiguous target accepted: %s %d", path, response.Code)
		}
	}
	id := "com.amitia/channel-qq/qq-channel-service/service?+%中文"
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/services?service_id="+url.QueryEscape(id), nil))
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 || result.ID != id {
		t.Fatal("query service identity was altered", err)
	}
}

func TestTrustedServiceAliasesKeepAuthenticationAndDefaultGinRouting(t *testing.T) {
	engine := gin.New()
	NewTrustedServiceAPI(nil).RegisterRoutes(engine.Group("/api/extensions", extensionAuth()))
	for _, path := range []string{"/services?service_id=x/y", "/services/health?service_id=x/y", "/services/status?service_id=x/y"} {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/extensions"+path, nil))
		if response.Code != 401 {
			t.Fatalf("alias bypassed authentication: %s %d", path, response.Code)
		}
	}
	if engine.UseRawPath || engine.UnescapePathValues != true {
		t.Fatal("global Gin path policy changed")
	}
	ordinary := gin.New()
	ordinary.Use(func(c *gin.Context) {
		c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "core", DeviceID: "ordinary", Permissions: auth.StandardPermissions()})
		c.Next()
	})
	NewTrustedServiceAPI(nil).RegisterRoutes(ordinary.Group("/api/extensions", extensionAuth(), security.SharedCoreAdminOnly()))
	for _, request := range []struct{ method, path string }{{http.MethodGet, "/health"}, {http.MethodPost, "/start"}, {http.MethodPost, "/stop"}, {http.MethodDelete, ""}, {http.MethodPost, "/invoke"}, {http.MethodPost, "/quarantine/release"}} {
		response := httptest.NewRecorder()
		ordinary.ServeHTTP(response, httptest.NewRequest(request.method, "/api/extensions/services"+request.path+"?service_id=x%2Fy", nil))
		if response.Code != http.StatusForbidden {
			t.Fatalf("alias bypassed bound device administration: %s %s %d", request.method, request.path, response.Code)
		}
	}
}

func TestTrustedServiceAliasesReachActualSupervisorAndPreserveLegacyNames(t *testing.T) {
	k, err := kernel.NewRuntime(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	supervisor := trusted_service.NewProcessSupervisor(t.TempDir())
	k.SetContainer(&kernel.Container{TrustedServiceSupervisor: supervisor})
	engine := gin.New()
	NewTrustedServiceAPI(&Runtime{Kernel: k}).RegisterRoutes(engine.Group("/api/extensions"))
	call := func(method, path, body string, expected int) []byte {
		t.Helper()
		response := httptest.NewRecorder()
		request := httptest.NewRequest(method, "/api/extensions/services"+path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(response, request)
		if response.Code != expected {
			t.Fatalf("%s %s: %d %s", method, path, response.Code, response.Body.String())
		}
		return response.Body.Bytes()
	}
	broken := []byte("invalid native executable")
	executable := filepath.Join(t.TempDir(), "broken.exe")
	if err := os.WriteFile(executable, broken, 0755); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(broken)
	for _, id := range []string{"com.amitia/channel-qq/qq-channel-service/service", "health", "status"} {
		if err := supervisor.Register(&trusted_service.ServiceRuntimeDefinition{ServiceID: id, TrustLevel: "trusted", Protocol: "plain", Executables: []trusted_service.PlatformExecutable{{Platform: trusted_service.CurrentPlatform(), Path: executable, Sha256: hex.EncodeToString(hash[:]), Signature: trusted_service.BinarySignature{Algorithm: "test", Value: "test-signature", Trusted: true}}}}); err != nil {
			t.Fatal(err)
		}
		query := "?service_id=" + url.QueryEscape(id)
		var failure struct {
			ServiceID string `json:"service_id"`
		}
		if err := json.Unmarshal(call(http.MethodPost, "/start"+query, "{}", 500), &failure); err != nil || failure.ServiceID != id {
			t.Fatal("start did not address original supervisor service", err)
		}
		call(http.MethodGet, query, "", 200)
		call(http.MethodGet, "/status"+query, "", 200)
		call(http.MethodGet, "/health"+query, "", 200)
		call(http.MethodPost, "/invoke"+query, `{"operation":"test"}`, 500)
		call(http.MethodPost, "/stop"+query, "{}", 200)
		if id == "health" || id == "status" {
			payload := call(http.MethodGet, "/"+id, "", 200)
			var instance struct {
				ServiceID string `json:"service_id"`
			}
			if err := json.Unmarshal(payload, &instance); err != nil || instance.ServiceID != id {
				t.Fatal("static operation stole a legacy service name", err)
			}
		}
		call(http.MethodDelete, query, "", 200)
		if supervisor.HasDefinition(id) {
			t.Fatal("delete did not unregister original service")
		}
	}
}

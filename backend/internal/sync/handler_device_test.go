package sync

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type failingDeviceRegistry struct{}

func (failingDeviceRegistry) RequireTrustedDevice(context.Context, runtimeidentity.SpaceID, runtimeidentity.DeviceID) error {
	return errors.New("database unavailable")
}

func TestSyncUsesCanonicalDeviceTrust(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE kernel_devices(device_id TEXT PRIMARY KEY,space_id TEXT,platform TEXT,label TEXT,trust_state TEXT,created_at TEXT,trusted_at TEXT,last_seen_at TEXT,revision INTEGER)`,
		`CREATE TABLE sync_changes(change_id TEXT PRIMARY KEY,seq INTEGER)`,
		`CREATE TABLE sync_cursors(device_id TEXT,space_id TEXT,scope TEXT,last_applied INTEGER,last_pushed INTEGER,updated_at DATETIME)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	registry := host_registry.NewRegistry(sqlDB)
	for _, record := range []host_registry.DeviceRecord{
		{SpaceID: "space", DeviceID: "trusted", TrustState: host_registry.DeviceTrustTrusted},
		{SpaceID: "space", DeviceID: "revoked", TrustState: host_registry.DeviceTrustRevoked},
		{SpaceID: "space", DeviceID: "pending", TrustState: host_registry.DeviceTrustPending},
		{SpaceID: "other", DeviceID: "foreign", TrustState: host_registry.DeviceTrustTrusted},
	} {
		if _, err := registry.EnsureDevice(t.Context(), record); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		device string
		status int
		code   string
	}{
		{"trusted", 200, ""}, {"revoked", 403, "device_not_trusted"},
		{"pending", 403, "device_not_trusted"}, {"foreign", 403, "forbidden"}, {"missing", 403, "forbidden"},
	} {
		t.Run(test.device, func(t *testing.T) {
			handler := NewHandler(NewService(db, nil), registry)
			router := gin.New()
			router.Use(func(c *gin.Context) { c.Set("actorContext", &auth.ActorContext{SpaceID: "space"}) })
			router.GET("/status", handler.HandleStatus)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest("GET", "/status?deviceId="+test.device, nil))
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.code) {
				t.Fatalf("%d %s", response.Code, response.Body.String())
			}
		})
	}
	if err := registry.RevokeDevice(t.Context(), "trusted"); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(nil, registry)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", "/", nil)
	if handler.requireDevice(context, "space", "trusted") {
		t.Fatal("revoked device remained authorized")
	}
}

func TestSyncRegistryFailureIsUnavailable(t *testing.T) {
	for _, registry := range []DeviceOwnershipValidator{nil, failingDeviceRegistry{}} {
		response := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(response)
		context.Request = httptest.NewRequest("GET", "/", nil)
		if NewHandler(nil, registry).requireDevice(context, "space", "device") || response.Code != 503 {
			t.Fatalf("%d %s", response.Code, response.Body.String())
		}
	}
}

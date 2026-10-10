package server

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	meshaudit "github.com/u-ai/backend/internal/devicemesh/audit"
	"github.com/u-ai/backend/internal/devicemesh/bootstrap"
	"github.com/u-ai/backend/internal/devicemesh/pairing"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
	_ "modernc.org/sqlite"
)

func TestPairingOfferReturnsCanonicalCodeAndPngWithoutCaching(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(meshaudit.Schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE kernel_devices(device_id TEXT PRIMARY KEY,space_id TEXT NOT NULL,platform TEXT NOT NULL DEFAULT '',label TEXT NOT NULL DEFAULT '',trust_state TEXT NOT NULL DEFAULT 'pending',created_at TEXT NOT NULL,trusted_at TEXT,last_seen_at TEXT NOT NULL,revision INTEGER NOT NULL DEFAULT 1)`); err != nil {
		t.Fatal(err)
	}
	registry := host_registry.NewRegistry(db)
	if _, err := registry.EnsureDevice(t.Context(), host_registry.DeviceRecord{SpaceID: "space", DeviceID: "issuer", Platform: runtimeidentity.PlatformWindows, TrustState: host_registry.DeviceTrustTrusted}); err != nil {
		t.Fatal(err)
	}
	svc, err := pairing.NewService(db, t.TempDir(), "space", registry, bootstrap.NewService(bootstrap.NewRepository(db), 300))
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.POST("/offers", makePairingOfferHandler(&RouterDeps{PairingSvc: svc, GetDeviceID: func(*gin.Context) (runtimeidentity.DeviceID, bool) { return "issuer", true }}))
	result := httptest.NewRecorder()
	r.ServeHTTP(result, httptest.NewRequest(http.MethodPost, "/offers", strings.NewReader(`{"endpoint":"https://core.example:18899/","ttlSeconds":300}`)))
	if result.Code != 200 || result.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("response %d %s", result.Code, result.Body.String())
	}
	var offer struct {
		Image   string `json:"qrImage"`
		Payload string `json:"qrPayload"`
		Token   string `json:"offerToken"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &offer); err != nil {
		t.Fatal(err)
	}
	payload, err := url.Parse(offer.Payload)
	if err != nil || payload.Scheme != "amitia" || payload.Host != "pair" || payload.Query().Get("endpoint") != "https://core.example:18899" || payload.Query().Get("offer") != offer.Token || offer.Token == "" {
		t.Fatalf("code did not preserve provider and offer")
	}
	image, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(offer.Image, "data:image/png;base64,"))
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(image))
	if err != nil || config.Width != 320 || config.Height != 320 {
		t.Fatalf("invalid QR image: %+v %v", config, err)
	}
	for _, body := range []string{`{"endpoint":"https://user:password@core.example"}`, `{"endpoint":"https://core.example/api"}`, `{"endpoint":"file:///private"}`, `{"endpoint":`} {
		rejected := httptest.NewRecorder()
		r.ServeHTTP(rejected, httptest.NewRequest(http.MethodPost, "/offers", strings.NewReader(body)))
		if rejected.Code != 400 {
			t.Fatalf("invalid provider/body accepted: status %d", rejected.Code)
		}
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_device_pairing_offers`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("invalid request created an offer: %d %v", count, err)
	}
}

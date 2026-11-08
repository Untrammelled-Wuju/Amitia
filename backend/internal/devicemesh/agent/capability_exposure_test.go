package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/capabilities"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestCapabilityExposureStoreDefaultsOpenAndPersistsChanges(t *testing.T) {
	dataDir := t.TempDir()
	store := NewCapabilityExposureStore(dataDir)
	state, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if state.Revision != 1 || len(state.EnabledIDs) != len(capabilities.IDs()) {
		t.Fatalf("default exposure is not fully open: %+v", state)
	}
	state, err = store.SetEnabled("task.execute", state.Revision, false)
	if err != nil {
		t.Fatal(err)
	}
	if state.Revision != 2 {
		t.Fatalf("unexpected revision: %d", state.Revision)
	}
	restarted := NewCapabilityExposureStore(dataDir)
	reloaded, err := restarted.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Revision != 2 {
		t.Fatalf("restart lost revision: %+v", reloaded)
	}
	for _, id := range reloaded.EnabledIDs {
		if id == "task.execute" {
			t.Fatal("restart re-enabled a disabled capability")
		}
	}
}

func TestCapabilityExposureStoreRejectsStaleRevisionAndInvalidCapability(t *testing.T) {
	store := NewCapabilityExposureStore(t.TempDir())
	if _, err := store.SetEnabled("task.execute", 2, false); !errors.Is(err, ErrCapabilityExposureRevision) {
		t.Fatalf("stale revision accepted: %v", err)
	}
	if _, err := store.SetEnabled("unknown.execute", 1, false); err == nil {
		t.Fatal("invalid capability accepted")
	}
}

func TestCapabilityExposureHTTPPersistsBeforeCenterSync(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dataDir := t.TempDir()
	handler := NewLocalHandler(dataDir, runtimeidentity.PlatformWindows)
	router := gin.New()
	handler.RegisterRoutes(router, func(c *gin.Context) { c.Next() })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/internal/device-mesh/capabilities", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("get capabilities status=%d body=%s", response.Code, response.Body.String())
	}
	var initial CapabilityExposureState
	if err := json.Unmarshal(response.Body.Bytes(), &initial); err != nil || initial.Revision != 1 {
		t.Fatalf("decode initial capabilities: %+v %v", initial, err)
	}

	body := bytes.NewBufferString(`{"enabled":false,"expectedRevision":1}`)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/internal/device-mesh/capabilities/task.execute", body))
	if response.Code != http.StatusOK {
		t.Fatalf("disable capability status=%d body=%s", response.Code, response.Body.String())
	}
	var updated struct {
		State        CapabilityExposureState `json:"state"`
		CenterSynced bool                    `json:"centerSynced"`
		SyncError    string                  `json:"syncError"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil || updated.State.Revision != 2 || updated.CenterSynced || updated.SyncError != "" {
		t.Fatalf("unexpected update: %+v %v", updated, err)
	}

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/internal/device-mesh/capabilities/task.execute", bytes.NewBufferString(`{"enabled":true,"expectedRevision":1}`)))
	if response.Code != http.StatusConflict {
		t.Fatalf("stale update status=%d body=%s", response.Code, response.Body.String())
	}

	restarted := NewLocalHandler(dataDir, runtimeidentity.PlatformWindows)
	persisted, err := restarted.capabilityExposure.Snapshot()
	if err != nil || persisted.Revision != 2 {
		t.Fatalf("restart lost capability state: %+v %v", persisted, err)
	}
	for _, id := range persisted.EnabledIDs {
		if id == "task.execute" {
			t.Fatal("restart re-enabled task capability")
		}
	}
}

func TestCapabilityExposureHTTPPersistsAndSyncsLocalCore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformWindows)
	var syncedRevision int64
	var syncedCapabilities []string
	handler.SetLocalCapabilityExposureSync(func(_ context.Context, revision int64, exposed []string) error {
		syncedRevision = revision
		syncedCapabilities = append([]string(nil), exposed...)
		return nil
	})
	router := gin.New()
	handler.RegisterRoutes(router, func(c *gin.Context) { c.Next() })
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/internal/device-mesh/capabilities/game.execute", bytes.NewBufferString(`{"enabled":false,"expectedRevision":1}`)))
	if response.Code != http.StatusOK {
		t.Fatalf("disable capability status=%d body=%s", response.Code, response.Body.String())
	}
	var updated struct {
		State        CapabilityExposureState `json:"state"`
		CenterSynced bool                    `json:"centerSynced"`
		SyncError    string                  `json:"syncError"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.State.Revision != 2 || !updated.CenterSynced || updated.SyncError != "" || syncedRevision != 2 {
		t.Fatalf("local core capability exposure was not synced: response=%+v revision=%d error=%q", updated, syncedRevision, updated.SyncError)
	}
	for _, capability := range syncedCapabilities {
		if capability == "game.execute" {
			t.Fatal("disabled capability was still published")
		}
	}
}

func TestPrepareProviderMeshPreservesRuntimeAuthorities(t *testing.T) {
	handler := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformAndroid)
	handler.SetExecutionGuard(func(ctx context.Context, payload protocol.RuntimeInvokePayload, next func(context.Context) (*protocol.RuntimeResultPayload, error)) (*protocol.RuntimeResultPayload, error) {
		return next(ctx)
	})
	credential := &StoredCredential{
		CloudBaseUrl: "https://center.example:54796",
		Credential:   "test-only",
		SpaceID:      "space-a",
	}
	identity := &LocalIdentity{
		DeviceID:  "device-a",
		RuntimeID: "runtime-a",
	}

	meshClient, activate := handler.PrepareProviderMesh(credential, identity, nil)
	if meshClient.conf.CapabilityExposure != handler.capabilityExposure {
		t.Fatal("provider mesh lost capability exposure authority")
	}
	if meshClient.conf.ExecutionGuard == nil {
		t.Fatal("provider mesh lost runtime execution guard")
	}
	gate, ok := meshClient.conf.TaskWorker.(*bindingGate)
	if !ok || gate.active.Load() {
		t.Fatal("provider mesh task worker must start fenced")
	}
	activate()
	if !gate.active.Load() {
		t.Fatal("provider mesh task worker was not activated")
	}
}

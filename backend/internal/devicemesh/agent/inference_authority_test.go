package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestLocalInferenceStopsOnProviderBinding(t *testing.T) {
	handler := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformWindows)
	ctx, finish, err := handler.LocalInferenceContext(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	handler.pauseProvider()
	select {
	case <-ctx.Done():
		if !errors.Is(context.Cause(ctx), ErrCoreInferenceRequired) {
			t.Fatal(context.Cause(ctx))
		}
	case <-time.After(time.Second):
		t.Fatal("local inference survived binding")
	}
	if _, _, err := handler.LocalInferenceContext(t.Context()); !errors.Is(err, ErrCoreInferenceRequired) {
		t.Fatalf("paused authority accepted: %v", err)
	}
	handler.resumeProvider()
	if err := handler.credStore.SaveCredential(&StoredCredential{SpaceID: "core", Credential: "test", ExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := handler.LocalInferenceContext(t.Context()); !errors.Is(err, ErrCoreInferenceRequired) {
		t.Fatalf("expired binding fell back to local inference: %v", err)
	}
}

func TestExplicitUnpairResetsInferenceAuthority(t *testing.T) {
	handler := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformWindows)
	if err := handler.credStore.SaveCredential(&StoredCredential{SpaceID: "core", Credential: "test", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	resets := 0
	handler.SetCredentialObserver(func(credential *StoredCredential) error {
		if credential != nil {
			t.Fatal("reset supplied old credential")
		}
		resets++
		return nil
	})
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)
	handler.handleDeleteCredential(c)
	if writer.Code != 200 || resets != 1 {
		t.Fatalf("unpair failed: %d resets=%d", writer.Code, resets)
	}
	ctx, finish, err := handler.LocalInferenceContext(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	if ctx.Err() != nil {
		t.Fatal(ctx.Err())
	}
}

func TestUnpairResetFailureKeepsInferencePaused(t *testing.T) {
	handler := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformWindows)
	handler.SetCredentialObserver(func(*StoredCredential) error { return errors.New("authority storage unavailable") })
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)
	handler.handleDeleteCredential(c)
	if writer.Code != 500 {
		t.Fatal(writer.Code)
	}
	if _, _, err := handler.LocalInferenceContext(t.Context()); !errors.Is(err, ErrCoreInferenceRequired) {
		t.Fatalf("failed unpair restored local inference: %v", err)
	}
}

func TestInterruptedUnpairRecoversAfterRestart(t *testing.T) {
	root := t.TempDir()
	handler := NewLocalHandler(root, runtimeidentity.PlatformWindows)
	if err := handler.credStore.unpairIntent(true); err != nil {
		t.Fatal(err)
	}
	if err := handler.credStore.SaveCredential(&StoredCredential{SpaceID: "old-core", Credential: "test"}); err != nil {
		t.Fatal(err)
	}
	restarted := NewLocalHandler(root, runtimeidentity.PlatformWindows)
	if _, _, err := restarted.LocalInferenceContext(t.Context()); !errors.Is(err, ErrCoreInferenceRequired) {
		t.Fatalf("interrupted unpair resumed inference: %v", err)
	}
	resets := 0
	restarted.SetCredentialObserver(func(credential *StoredCredential) error {
		if credential != nil {
			t.Fatal("old credential restored")
		}
		resets++
		return nil
	})
	if err := restarted.RecoverUnpair(); err != nil {
		t.Fatal(err)
	}
	if resets != 1 {
		t.Fatal("local authority reset missing")
	}
	if pending, err := restarted.credStore.pendingUnpair(); err != nil || pending {
		t.Fatalf("unpair intent retained: %v", err)
	}
	if credential, err := restarted.LoadCredential(); err != nil || credential != nil {
		t.Fatalf("old credential retained: %v", err)
	}
	_, finish, err := restarted.LocalInferenceContext(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	finish()
}

func TestCoreModeIgnoresStoredDeviceCredential(t *testing.T) {
	handler := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformAndroid)
	credential := &StoredCredential{
		CloudBaseUrl: "https://old-core.example",
		CredentialID: "old-credential",
		Credential:   "old-secret",
		SpaceID:      "old-core",
		DeviceID:     "center-device",
		RuntimeID:    "center-runtime",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	if err := handler.credStore.SaveCredential(credential); err != nil {
		t.Fatal(err)
	}
	handler.SetAgentMode(false)
	handler.SetLocalCoreID("local-core")

	ctx, finish, err := handler.LocalInferenceContext(t.Context())
	if err != nil {
		t.Fatalf("core mode rejected local inference with stale credential: %v", err)
	}
	finish()
	if ctx == nil {
		t.Fatal("local inference context is nil")
	}

	path, err := handler.ProviderPath()
	if err != nil || len(path) != 1 || path[0] != "local-core" {
		t.Fatalf("provider path = %v, %v", path, err)
	}

	router := gin.New()
	handler.RegisterRoutes(router, func(c *gin.Context) { c.Next() })

	statusResponse := httptest.NewRecorder()
	router.ServeHTTP(statusResponse, httptest.NewRequest(http.MethodGet, "/internal/device-mesh/status", nil))
	var status map[string]any
	if statusResponse.Code != http.StatusOK || json.Unmarshal(statusResponse.Body.Bytes(), &status) != nil {
		t.Fatalf("status response: %d %s", statusResponse.Code, statusResponse.Body.String())
	}
	if status["state"] != string(StateUnprovisioned) || status["agentMode"] != false || status["coreId"] != nil || status["cloudBaseUrl"] != "" {
		t.Fatalf("stale remote status leaked in core mode: %+v", status)
	}

	authResponse := httptest.NewRecorder()
	router.ServeHTTP(authResponse, httptest.NewRequest(http.MethodGet, "/internal/device-mesh/cloud-auth", nil))
	if authResponse.Code != http.StatusConflict {
		t.Fatalf("cloud auth in core mode: %d %s", authResponse.Code, authResponse.Body.String())
	}

	handler.SetAgentMode(true)
	if _, _, err := handler.LocalInferenceContext(t.Context()); !errors.Is(err, ErrCoreInferenceRequired) {
		t.Fatalf("stored credential was not reactivated for device-agent mode: %v", err)
	}
}

func TestCoreModeAllowsExplicitPairingHandshake(t *testing.T) {
	handler := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformDarwin)
	handler.SetAgentMode(false)
	router := gin.New()
	handler.RegisterRoutes(router, func(c *gin.Context) { c.Next() })

	for _, path := range []string{"/internal/device-mesh/identity/sign-claim", "/internal/device-mesh/pairing/claim"} {
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code == http.StatusConflict {
			t.Fatalf("core mode blocked explicit pairing handshake %s: %s", path, response.Body.String())
		}
	}
}

package agent

import (
	"context"
	"errors"
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

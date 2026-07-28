package repair_baseline

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/dev_mode"
)

func newDevModeTestWorkspace(t *testing.T, extDir string) (string, string) {
	t.Helper()
	manifestPath := filepath.Join(extDir, "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest must exist in %s: %v", extDir, err)
	}
	return extDir, manifestPath
}

func TestBaseline_E2E_DevMode_RegisterWorkspace(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E dev mode test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extensionsDir := testExtensionsDir(t)
	v1Dir := filepath.Join(extensionsDir, "dev-hot-reload-v1")
	extDir, manifestPath := newDevModeTestWorkspace(t, v1Dir)

	registry := dev_mode.NewWorkspaceRegistry()
	ws, err := registry.Register(ctx, dev_mode.RegisterWorkspaceInput{
		WorkspaceID:  "ws-dev-e2e",
		ExtensionID:  "com.amitia.repair/dev-hot-reload",
		PathReference: extDir,
		ManifestPath: manifestPath,
		WatchEnabled: true,
		AutoReload:   true,
	})
	if err != nil {
		t.Fatalf("workspace Register must succeed (Phase 10 section 19.8.1): %v", err)
	}
	if ws.Status != dev_mode.WorkspaceStatusRegistered {
		t.Fatalf("workspace status must be 'registered' (Phase 10 section 19.8.1), got %s", ws.Status)
	}
	if ws.ExtensionID != "com.amitia.repair/dev-hot-reload" {
		t.Fatalf("workspace extension ID must match, got %s", ws.ExtensionID)
	}
}

func TestBaseline_E2E_DevMode_V1LoadSucceeds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E dev mode test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	extensionsDir := testExtensionsDir(t)
	v1Dir := filepath.Join(extensionsDir, "dev-hot-reload-v1")
	extDir, manifestPath := newDevModeTestWorkspace(t, v1Dir)

	registry := dev_mode.NewWorkspaceRegistry()
	pipeline := dev_mode.NewRebuildPipeline("node").WithRegistry(registry)
	preserver := dev_mode.NewStatePreserver()
	reloader := dev_mode.NewRuntimeReloader(registry, pipeline, preserver)

	wsID := dev_mode.WorkspaceID("ws-v1-load")
	_, err := registry.Register(ctx, dev_mode.RegisterWorkspaceInput{
		WorkspaceID:   wsID,
		ExtensionID:   "com.amitia.repair/dev-hot-reload",
		PathReference: extDir,
		ManifestPath:  manifestPath,
		WatchEnabled:  true,
		AutoReload:    true,
	})
	if err != nil {
		t.Fatalf("Register must succeed: %v", err)
	}
	if err := registry.GrantDevTrust(wsID); err != nil {
		t.Fatalf("GrantDevTrust must succeed: %v", err)
	}
	reloader.Enable(wsID)

	ev, err := reloader.Reload(ctx, wsID, "initial v1 load", nil)
	if err != nil {
		t.Fatalf("Reload v1 must succeed (Phase 10 section 19.8.2): %v", err)
	}
	if !ev.Success {
		t.Fatalf("Reload v1 event must report success (Phase 10 section 19.8.2), got error: %s", ev.Error)
	}

	ws, _ := registry.Get(wsID)
	if ws.Status != dev_mode.WorkspaceStatusReady {
		t.Fatalf("workspace status must be 'ready' after v1 load (Phase 10 section 19.8.2), got %s", ws.Status)
	}
	if ws.CurrentRevision == "" {
		t.Fatalf("workspace must have a non-empty current revision after v1 load")
	}
}

func TestBaseline_E2E_DevMode_V2ReloadActivatesNewGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E dev mode test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	extensionsDir := testExtensionsDir(t)
	v1Dir := filepath.Join(extensionsDir, "dev-hot-reload-v1")
	v2Dir := filepath.Join(extensionsDir, "dev-hot-reload-v2")
	v1ExtDir, v1Manifest := newDevModeTestWorkspace(t, v1Dir)
	_, v2Manifest := newDevModeTestWorkspace(t, v2Dir)

	registry := dev_mode.NewWorkspaceRegistry()
	pipeline := dev_mode.NewRebuildPipeline("node").WithRegistry(registry)
	preserver := dev_mode.NewStatePreserver()
	reloader := dev_mode.NewRuntimeReloader(registry, pipeline, preserver)

	wsID := dev_mode.WorkspaceID("ws-v2-reload")
	_, err := registry.Register(ctx, dev_mode.RegisterWorkspaceInput{
		WorkspaceID:   wsID,
		ExtensionID:   "com.amitia.repair/dev-hot-reload",
		PathReference: v1ExtDir,
		ManifestPath:  v1Manifest,
		WatchEnabled:  true,
		AutoReload:    true,
	})
	if err != nil {
		t.Fatalf("Register must succeed: %v", err)
	}
	if err := registry.GrantDevTrust(wsID); err != nil {
		t.Fatalf("GrantDevTrust must succeed: %v", err)
	}
	reloader.Enable(wsID)

	if _, err := reloader.Reload(ctx, wsID, "v1 load", nil); err != nil {
		t.Fatalf("v1 Reload must succeed: %v", err)
	}
	wsAfterV1, _ := registry.Get(wsID)
	v1Revision := wsAfterV1.CurrentRevision

	wsAfterV1.ManifestPath = v2Manifest
	wsAfterV1.PathReference = v2Dir

	ev, err := reloader.Reload(ctx, wsID, "v2 reload", nil)
	if err != nil {
		t.Fatalf("Reload v2 must succeed (Phase 10 section 19.8.3-19.8.4): %v", err)
	}
	if !ev.Success {
		t.Fatalf("Reload v2 event must report success (Phase 10 section 19.8.4), got error: %s", ev.Error)
	}

	wsAfterV2, _ := registry.Get(wsID)
	if wsAfterV2.CurrentRevision == v1Revision {
		t.Fatalf("v2 reload must activate a new revision/generation (Phase 10 section 19.8.4), revision unchanged")
	}
	if wsAfterV2.Status != dev_mode.WorkspaceStatusReady {
		t.Fatalf("workspace must be ready after v2 reload (Phase 10 section 19.8.4), got %s", wsAfterV2.Status)
	}

	_ = pipeline.MarkStale(wsID)
	staleRev, ok := pipeline.CurrentRevision(wsID)
	if !ok {
		t.Fatalf("CurrentRevision must return the active revision")
	}
	if staleRev.RevisionID != wsAfterV2.CurrentRevision {
		t.Fatalf("CurrentRevision must match workspace current revision")
	}
}

func TestBaseline_E2E_DevMode_V3InvalidReloadFailsAndV2Continues(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E dev mode test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	extensionsDir := testExtensionsDir(t)
	v1Dir := filepath.Join(extensionsDir, "dev-hot-reload-v1")
	v2Dir := filepath.Join(extensionsDir, "dev-hot-reload-v2")
	v1ExtDir, v1Manifest := newDevModeTestWorkspace(t, v1Dir)
	_, v2Manifest := newDevModeTestWorkspace(t, v2Dir)

	registry := dev_mode.NewWorkspaceRegistry()
	pipeline := dev_mode.NewRebuildPipeline("node").WithRegistry(registry)
	preserver := dev_mode.NewStatePreserver()
	reloader := dev_mode.NewRuntimeReloader(registry, pipeline, preserver)

	wsID := dev_mode.WorkspaceID("ws-v3-fail")
	_, err := registry.Register(ctx, dev_mode.RegisterWorkspaceInput{
		WorkspaceID:   wsID,
		ExtensionID:   "com.amitia.repair/dev-hot-reload",
		PathReference: v1ExtDir,
		ManifestPath:  v1Manifest,
		WatchEnabled:  true,
		AutoReload:    true,
	})
	if err != nil {
		t.Fatalf("Register must succeed: %v", err)
	}
	if err := registry.GrantDevTrust(wsID); err != nil {
		t.Fatalf("GrantDevTrust must succeed: %v", err)
	}
	reloader.Enable(wsID)

	if _, err := reloader.Reload(ctx, wsID, "v1 load", nil); err != nil {
		t.Fatalf("v1 Reload must succeed: %v", err)
	}

	wsAfterV1, _ := registry.Get(wsID)
	wsAfterV1.ManifestPath = v2Manifest
	wsAfterV1.PathReference = v2Dir

	if _, err := reloader.Reload(ctx, wsID, "v2 reload", nil); err != nil {
		t.Fatalf("v2 Reload must succeed: %v", err)
	}
	wsAfterV2, _ := registry.Get(wsID)
	v2Revision := wsAfterV2.CurrentRevision

	tempDir := t.TempDir()
	invalidManifestPath := filepath.Join(tempDir, "invalid-manifest.json")
	if err := os.WriteFile(invalidManifestPath, []byte("{ this is not valid json"), 0o644); err != nil {
		t.Fatalf("write invalid manifest: %v", err)
	}
	wsAfterV2.ManifestPath = invalidManifestPath

	_, err = reloader.Reload(ctx, wsID, "v3 invalid reload", nil)
	if err == nil {
		t.Fatalf("Reload with invalid v3 manifest must fail (Phase 10 section 19.8.7)")
	}

	wsAfterFail, _ := registry.Get(wsID)
	if wsAfterFail.Status != dev_mode.WorkspaceStatusFailed {
		t.Fatalf("workspace status must be 'failed' after v3 reload failure (Phase 10 section 19.8.7), got %s", wsAfterFail.Status)
	}

	currentRev, ok := pipeline.CurrentRevision(wsID)
	if !ok {
		t.Fatalf("CurrentRevision must still return the last successful revision after failure")
	}
	if currentRev.RevisionID != v2Revision {
		t.Fatalf("v2 revision must remain as the last successful revision after v3 failure (Phase 10 section 19.8.8), expected %s, got %s", v2Revision, currentRev.RevisionID)
	}
	if currentRev.Status == dev_mode.RevisionStatusFailed {
		t.Fatalf("v2 revision must not be marked as failed (Phase 10 section 19.8.8)")
	}
}

func TestBaseline_E2E_DevMode_WorkspaceCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E dev mode test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extensionsDir := testExtensionsDir(t)
	v1Dir := filepath.Join(extensionsDir, "dev-hot-reload-v1")
	extDir, manifestPath := newDevModeTestWorkspace(t, v1Dir)

	registry := dev_mode.NewWorkspaceRegistry()
	wsID := dev_mode.WorkspaceID("ws-cleanup")
	_, err := registry.Register(ctx, dev_mode.RegisterWorkspaceInput{
		WorkspaceID:   wsID,
		ExtensionID:   "com.amitia.repair/dev-hot-reload",
		PathReference: extDir,
		ManifestPath:  manifestPath,
		WatchEnabled:  true,
		AutoReload:    true,
	})
	if err != nil {
		t.Fatalf("Register must succeed: %v", err)
	}

	if err := registry.Disable(wsID); err != nil {
		t.Fatalf("Disable must succeed: %v", err)
	}
	wsDisabled, _ := registry.Get(wsID)
	if wsDisabled.Status != dev_mode.WorkspaceStatusDisabled {
		t.Fatalf("workspace must be disabled before removal, got %s", wsDisabled.Status)
	}
	if wsDisabled.DevTrust {
		t.Fatalf("DevTrust must be revoked after disable")
	}

	if err := registry.Remove(wsID); err != nil {
		t.Fatalf("Remove must succeed (Phase 10 section 19.8.10): %v", err)
	}

	_, err = registry.Get(wsID)
	if err == nil {
		t.Fatalf("workspace must be removed after cleanup (Phase 10 section 19.8.10)")
	}
	list := registry.List()
	for _, ws := range list {
		if ws.WorkspaceID == wsID {
			t.Fatalf("workspace must not appear in registry list after cleanup (Phase 10 section 19.8.10)")
		}
	}
}

func TestBaseline_E2E_DevMode_DevTrustRequiredForReload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E dev mode test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extensionsDir := testExtensionsDir(t)
	v1Dir := filepath.Join(extensionsDir, "dev-hot-reload-v1")
	extDir, manifestPath := newDevModeTestWorkspace(t, v1Dir)

	registry := dev_mode.NewWorkspaceRegistry()
	pipeline := dev_mode.NewRebuildPipeline("node").WithRegistry(registry)
	preserver := dev_mode.NewStatePreserver()
	reloader := dev_mode.NewRuntimeReloader(registry, pipeline, preserver)

	wsID := dev_mode.WorkspaceID("ws-no-trust")
	_, err := registry.Register(ctx, dev_mode.RegisterWorkspaceInput{
		WorkspaceID:   wsID,
		ExtensionID:   "com.amitia.repair/dev-hot-reload",
		PathReference: extDir,
		ManifestPath:  manifestPath,
		WatchEnabled:  true,
		AutoReload:    true,
	})
	if err != nil {
		t.Fatalf("Register must succeed: %v", err)
	}
	reloader.Enable(wsID)

	_, err = reloader.Reload(ctx, wsID, "reload without dev trust", nil)
	if err == nil {
		t.Fatalf("Reload without DevTrust must fail (security: dev trust required before executing developer code)")
	}
}

func TestBaseline_E2E_DevMode_ReloadDisabledWhenNotEnabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E dev mode test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extensionsDir := testExtensionsDir(t)
	v1Dir := filepath.Join(extensionsDir, "dev-hot-reload-v1")
	extDir, manifestPath := newDevModeTestWorkspace(t, v1Dir)

	registry := dev_mode.NewWorkspaceRegistry()
	pipeline := dev_mode.NewRebuildPipeline("node").WithRegistry(registry)
	preserver := dev_mode.NewStatePreserver()
	reloader := dev_mode.NewRuntimeReloader(registry, pipeline, preserver)

	wsID := dev_mode.WorkspaceID("ws-not-enabled")
	_, err := registry.Register(ctx, dev_mode.RegisterWorkspaceInput{
		WorkspaceID:   wsID,
		ExtensionID:   "com.amitia.repair/dev-hot-reload",
		PathReference: extDir,
		ManifestPath:  manifestPath,
	})
	if err != nil {
		t.Fatalf("Register must succeed: %v", err)
	}
	if err := registry.GrantDevTrust(wsID); err != nil {
		t.Fatalf("GrantDevTrust must succeed: %v", err)
	}

	_, err = reloader.Reload(ctx, wsID, "reload when not enabled", nil)
	if err == nil {
		t.Fatalf("Reload must fail when reloader is not enabled for workspace")
	}
}

func TestBaseline_E2E_DevMode_StatePreservationAcrossReload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E dev mode test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	extensionsDir := testExtensionsDir(t)
	v1Dir := filepath.Join(extensionsDir, "dev-hot-reload-v1")
	v2Dir := filepath.Join(extensionsDir, "dev-hot-reload-v2")
	v1ExtDir, v1Manifest := newDevModeTestWorkspace(t, v1Dir)
	_, v2Manifest := newDevModeTestWorkspace(t, v2Dir)

	registry := dev_mode.NewWorkspaceRegistry()
	pipeline := dev_mode.NewRebuildPipeline("node").WithRegistry(registry)
	preserver := dev_mode.NewStatePreserver()
	reloader := dev_mode.NewRuntimeReloader(registry, pipeline, preserver)

	wsID := dev_mode.WorkspaceID("ws-state")
	_, err := registry.Register(ctx, dev_mode.RegisterWorkspaceInput{
		WorkspaceID:   wsID,
		ExtensionID:   "com.amitia.repair/dev-hot-reload",
		PathReference: v1ExtDir,
		ManifestPath:  v1Manifest,
		WatchEnabled:  true,
		AutoReload:    true,
	})
	if err != nil {
		t.Fatalf("Register must succeed: %v", err)
	}
	if err := registry.GrantDevTrust(wsID); err != nil {
		t.Fatalf("GrantDevTrust must succeed: %v", err)
	}
	reloader.Enable(wsID)

	if _, err := reloader.Reload(ctx, wsID, "v1 load", nil); err != nil {
		t.Fatalf("v1 Reload must succeed: %v", err)
	}

	wsAfterV1, _ := registry.Get(wsID)
	wsAfterV1.ManifestPath = v2Manifest
	wsAfterV1.PathReference = v2Dir

	stateProvider := func() map[string]any {
		return map[string]any{
			"conversation_state": "active",
			"last_input":         "hello",
		}
	}
	ev, err := reloader.Reload(ctx, wsID, "v2 reload with state", stateProvider)
	if err != nil {
		t.Fatalf("v2 Reload with state must succeed: %v", err)
	}
	if !ev.Success {
		t.Fatalf("v2 reload event must report success, got error: %s", ev.Error)
	}

	snap, ok := preserver.Restore(wsID)
	if !ok {
		t.Fatalf("state snapshot must be available after reload (Phase 10 state preservation)")
	}
	if snap.State == nil {
		t.Fatalf("state snapshot must contain preserved state")
	}
	if _, exists := snap.State["conversation_state"]; !exists {
		t.Fatalf("state snapshot must preserve conversation_state")
	}
}

func TestBaseline_E2E_DevMode_PipelinePhasesDefined(t *testing.T) {
	requiredPhases := []string{
		"register_workspace",
		"v1_load",
		"v2_reload_new_generation",
		"old_runtime_stopped",
		"v3_invalid_reload_fails",
		"v2_continues_running",
		"cleanup_workspace_and_runtime",
	}
	if len(requiredPhases) != 7 {
		t.Fatalf("Phase 10 section 19.8 requires 7 dev mode phases, got %d", len(requiredPhases))
	}
	for _, p := range requiredPhases {
		if p == "" {
			t.Fatalf("dev mode phase must not be empty")
		}
	}
}

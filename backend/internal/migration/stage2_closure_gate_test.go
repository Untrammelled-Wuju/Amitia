package migration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

func writeTestEvidence(t *testing.T, dir string) string {
	t.Helper()
	manifest := EvidenceManifest{
		Version:         "2",
		ManifestVersion: "20260816002",
		GeneratedAt:     "2026-08-16T16:00:00Z",
		Evidence: map[string]EvidenceItem{
			"G0-A01": {Status: EvidencePASS, Evidence: "test pass"},
			"G0-A02": {Status: EvidenceFAIL, Evidence: "test fail"},
		},
		Summary: EvidenceSummary{Total: 2, Pass: 1, Fail: 1},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal evidence: %v", err)
	}
	path := filepath.Join(dir, "test-evidence.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write evidence: %v", err)
	}
	return path
}

func TestEvidenceLoader_Load(t *testing.T) {
	dir := t.TempDir()
	path := writeTestEvidence(t, dir)

	loader := NewEvidenceLoader(path)
	manifest, err := loader.Load()
	if err != nil {
		t.Fatalf("load evidence: %v", err)
	}
	if manifest.ManifestVersion != "20260816002" {
		t.Fatalf("expected manifest version 20260816002, got %s", manifest.ManifestVersion)
	}
	if len(manifest.Evidence) != 2 {
		t.Fatalf("expected 2 evidence items, got %d", len(manifest.Evidence))
	}
}

func TestEvidenceLoader_FileNotFound(t *testing.T) {
	loader := NewEvidenceLoader("/nonexistent/path.json")
	_, err := loader.Load()
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestRuntimeArchitectureGate_AllPresent(t *testing.T) {
	gate := NewRuntimeArchitectureGate(&testAuthorityProvider{
		toolFacade:         struct{}{},
		permissionBroker:   struct{}{},
		eventService:       struct{}{},
		scheduleService:    struct{}{},
		taskRuntimeService: struct{}{},
		hookService:        struct{}{},
		nativeBridgeRelay:  struct{}{},
		platformBridge:     struct{}{},
	}, "android")

	ready, failures := gate.Check(context.Background())
	if !ready {
		t.Fatalf("expected ready, got failures: %v", failures)
	}
	if !gate.ArchitectureReady() {
		t.Fatal("expected ArchitectureReady to be true")
	}
}

func TestRuntimeArchitectureGate_MissingAuthorities(t *testing.T) {
	gate := NewRuntimeArchitectureGate(&testAuthorityProvider{}, "android")

	ready, failures := gate.Check(context.Background())
	if ready {
		t.Fatal("expected not ready with empty provider")
	}
	if len(failures) == 0 {
		t.Fatal("expected failures with empty provider")
	}
	if gate.ArchitectureReady() {
		t.Fatal("expected ArchitectureReady to be false")
	}
}

func TestRuntimeArchitectureGate_NilContainer(t *testing.T) {
	gate := NewRuntimeArchitectureGate(nil, "android")

	ready, failures := gate.Check(context.Background())
	if ready {
		t.Fatal("expected not ready with nil container")
	}
	if len(failures) == 0 || failures[0] != "KernelContainer: nil" {
		t.Fatalf("expected KernelContainer: nil failure, got: %v", failures)
	}
}

func TestStage2ClosureGate_ValidateG0_WithFailures(t *testing.T) {
	dir := t.TempDir()
	path := writeTestEvidence(t, dir)

	loader := NewEvidenceLoader(path)
	runtimeGate := NewRuntimeArchitectureGate(&testAuthorityProvider{
		toolFacade:         struct{}{},
		permissionBroker:   struct{}{},
		eventService:       struct{}{},
		scheduleService:    struct{}{},
		taskRuntimeService: struct{}{},
		hookService:        struct{}{},
		nativeBridgeRelay:  struct{}{},
		platformBridge:     struct{}{},
	}, "android")

	gate := NewStage2ClosureGateWithManifest(loader, runtimeGate, "20260816002", []string{"G0-A01", "G0-A02"})
	ok, failures, err := gate.ValidateG0(context.Background())
	if err != nil {
		t.Fatalf("validate g0: %v", err)
	}
	if ok {
		t.Fatal("expected not ok due to G0-A02 FAIL evidence")
	}
	if len(failures) == 0 {
		t.Fatal("expected failures from FAIL evidence")
	}
}

func TestStage2ClosureGate_ManifestVersion(t *testing.T) {
	dir := t.TempDir()
	path := writeTestEvidence(t, dir)

	loader := NewEvidenceLoader(path)
	runtimeGate := NewRuntimeArchitectureGate(&testAuthorityProvider{
		toolFacade:         struct{}{},
		permissionBroker:   struct{}{},
		eventService:       struct{}{},
		scheduleService:    struct{}{},
		taskRuntimeService: struct{}{},
		hookService:        struct{}{},
		nativeBridgeRelay:  struct{}{},
		platformBridge:     struct{}{},
	}, "android")

	gate := NewStage2ClosureGate(loader, runtimeGate)
	if !gate.ManifestVersionConsistent("20260816002") {
		t.Fatal("expected manifest version to be consistent")
	}
	if gate.ManifestVersionConsistent("wrong-version") {
		t.Fatal("expected manifest version to be inconsistent")
	}
}

func TestCutoverPlan_CanRunCutover(t *testing.T) {
	dir := t.TempDir()
	passEvidence := EvidenceManifest{
		Version:         "2",
		ManifestVersion: "20260816002",
		Evidence: map[string]EvidenceItem{
			"G0-A01": {Status: EvidencePASS, Evidence: "test pass"},
		},
		Summary: EvidenceSummary{Total: 1, Pass: 1},
	}
	data, _ := json.Marshal(passEvidence)
	evidencePath := filepath.Join(dir, "pass-evidence.json")
	os.WriteFile(evidencePath, data, 0644)

	validContainer := &testAuthorityProvider{
		toolFacade:         struct{}{},
		permissionBroker:   struct{}{},
		eventService:       struct{}{},
		scheduleService:    struct{}{},
		taskRuntimeService: struct{}{},
		hookService:        struct{}{},
		nativeBridgeRelay:  struct{}{},
		platformBridge:     struct{}{},
	}
	plan := NewCutoverPlan(CutoverDependencies{
		Container:               validContainer,
		RuntimeArchitectureGate: NewRuntimeArchitectureGate(validContainer, "android"),
		Stage2ClosureGate: func() *Stage2ClosureGate {
			loader := NewEvidenceLoader(evidencePath)
			rg := NewRuntimeArchitectureGate(validContainer, "android")
			return NewStage2ClosureGateWithManifest(loader, rg, "20260816002", []string{"G0-A01"})
		}(),
	})

	canRun, err := plan.CanRunCutover(context.Background())
	if err != nil {
		t.Fatalf("can run cutover: %v", err)
	}
	if !canRun {
		t.Fatal("expected CanRunCutover to be true")
	}
}

func setupTestDB(t *testing.T) *sqlite.Store {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "test.db")
	store, err := sqlite.NewStore(dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestStage2ClosureGate_NotVerifiedBlocks(t *testing.T) {
	dir := t.TempDir()
	manifest := EvidenceManifest{
		Version:         "2",
		ManifestVersion: "20260816002",
		Evidence: map[string]EvidenceItem{
			"G0-A01": {Status: EvidenceNOTVERIFIED, Evidence: "not run"},
		},
		Summary: EvidenceSummary{Total: 1, NotVerified: 1},
	}
	data, _ := json.Marshal(manifest)
	path := filepath.Join(dir, "not-verified.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	provider := &testAuthorityProvider{
		toolFacade: struct{}{}, permissionBroker: struct{}{}, eventService: struct{}{},
		scheduleService: struct{}{}, taskRuntimeService: struct{}{}, hookService: struct{}{},
		nativeBridgeRelay: struct{}{}, platformBridge: struct{}{},
	}
	gate := NewStage2ClosureGateWithManifest(NewEvidenceLoader(path), NewRuntimeArchitectureGate(provider, "android"), "20260816002", []string{"G0-A01"})
	ok, failures, err := gate.ValidateG0(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ok || len(failures) == 0 {
		t.Fatalf("NOT_VERIFIED must block G0: ok=%v failures=%v", ok, failures)
	}
}

func TestStage2ClosureGate_MissingEvidenceBlocks(t *testing.T) {
	dir := t.TempDir()
	manifest := EvidenceManifest{
		Version: "2", ManifestVersion: "20260816002",
		Evidence: map[string]EvidenceItem{}, Summary: EvidenceSummary{Total: 0},
	}
	data, _ := json.Marshal(manifest)
	path := filepath.Join(dir, "missing.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	provider := &testAuthorityProvider{
		toolFacade: struct{}{}, permissionBroker: struct{}{}, eventService: struct{}{},
		scheduleService: struct{}{}, taskRuntimeService: struct{}{}, hookService: struct{}{},
		nativeBridgeRelay: struct{}{}, platformBridge: struct{}{},
	}
	gate := NewStage2ClosureGateWithManifest(NewEvidenceLoader(path), NewRuntimeArchitectureGate(provider, "android"), "20260816002", []string{"G0-A01"})
	ok, failures, err := gate.ValidateG0(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ok || len(failures) == 0 {
		t.Fatalf("missing evidence must block G0: ok=%v failures=%v", ok, failures)
	}
}

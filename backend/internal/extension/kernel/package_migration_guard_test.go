package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
)

func packageMigrationString(value string) *string {
	return &value
}

func packageMigrationDefinition(extensionID, id, from, to string, direction migration.MigrationDirection, linked string) migration.MigrationDefinition {
	definition := migration.MigrationDefinition{
		MigrationID:      id,
		ExtensionID:      extensionID,
		FromVersionRange: from,
		ToVersion:        to,
		Entry:            "migrations/" + id + ".js",
		RuntimeType:      "javascript",
		Direction:        direction,
		Idempotency:      migration.IdempotencyIdempotent,
		Reversibility:    migration.ReversibilityFullyReversible,
		DefinitionHash:   "hash-" + id,
	}
	if direction == migration.DirectionForward {
		definition.ReverseMigrationID = packageMigrationString(linked)
	} else {
		definition.ForwardMigrationID = packageMigrationString(linked)
	}
	return definition
}

func packageMigrationChain(extensionID string, length int) []migration.MigrationDefinition {
	versions := []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"}
	definitions := make([]migration.MigrationDefinition, 0, length*2)
	for index := 1; index <= length; index++ {
		forwardID := fmt.Sprintf("f%d", index)
		reverseID := fmt.Sprintf("r%d", index)
		definitions = append(definitions,
			packageMigrationDefinition(extensionID, forwardID, versions[index-1], versions[index], migration.DirectionForward, reverseID),
			packageMigrationDefinition(extensionID, reverseID, versions[index], versions[index-1], migration.DirectionReverse, forwardID),
		)
	}
	return definitions
}

func newPackageMigrationTestGuard(t *testing.T) (*Container, *PackageMigrationGuard) {
	t.Helper()
	root := t.TempDir()
	container, err := NewContainerBuilder().WithDBPath(filepath.Join(root, "kernel.db")).WithExtensionRoot(filepath.Join(root, "extensions")).Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Close() })
	return container, NewPackageMigrationGuard(container.MigrationRepository)
}

func packageMigrationHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func packageMigrationPreflight(t *testing.T, guard *PackageMigrationGuard, extensionID string, definitions []migration.MigrationDefinition, from, to string) *migration.ReversiblePreflight {
	t.Helper()
	manifest := manifest_v2.Manifest{Extension: manifest_v2.ExtensionMeta{ID: extensionID, Version: to, Metadata: map[string]any{"migrations": map[string]any{"definitions": definitions}}}}
	preflight, err := guard.PreflightManifest(context.Background(), manifest, from)
	if err != nil {
		t.Fatal(err)
	}
	return preflight
}

func TestPackageMigrationGuardCrashResumeAndIdempotency(t *testing.T) {
	_, guard := newPackageMigrationTestGuard(t)
	preflight := packageMigrationPreflight(t, guard, "ext.resume", packageMigrationChain("ext.resume", 2), "1.0.0", "1.2.0")
	request := migration.ReversibleExecutionRequest{OperationID: "op-resume", Preflight: preflight}
	calls := map[string]int{}
	first := true
	handler := func(ctx context.Context, step migration.ReversiblePlanStep, definition migration.MigrationDefinition) (migration.ReversibleStepResult, error) {
		calls[step.MigrationID]++
		if step.MigrationID == "f2" && first {
			first = false
			return migration.ReversibleStepResult{}, migration.ErrMigrationExecutionInterrupted
		}
		return migration.ReversibleStepResult{Evidence: json.RawMessage(`{"ok":true}`)}, nil
	}
	if _, err := guard.ExecuteForward(context.Background(), request, handler); !errors.Is(err, migration.ErrMigrationExecutionInterrupted) {
		t.Fatalf("expected interruption, got %v", err)
	}
	op, err := guard.ExecuteForward(context.Background(), request, handler)
	if err != nil {
		t.Fatal(err)
	}
	if op.Status != migration.OperationStatusCompleted || calls["f1"] != 1 || calls["f2"] != 2 {
		t.Fatalf("resume did not preserve completed journal: status=%s calls=%v", op.Status, calls)
	}
	if _, err := guard.ExecuteForward(context.Background(), request, handler); err != nil {
		t.Fatal(err)
	}
	if calls["f1"] != 1 || calls["f2"] != 2 {
		t.Fatalf("completed operation was executed again: %v", calls)
	}
}

func TestPackageMigrationGuardReverseOrderAndSnapshotVerification(t *testing.T) {
	container, guard := newPackageMigrationTestGuard(t)
	definitions := packageMigrationChain("ext.reverse", 3)
	for index := range definitions {
		definitions[index].DataDomains = []migration.DataDomain{{Domain: "user", Storage: "sqlite", Namespace: "profile"}}
	}
	preflight := packageMigrationPreflight(t, guard, "ext.reverse", definitions, "1.0.0", "2.0.0")
	state := []byte("initial")
	snapshot := append([]byte(nil), state...)
	request := migration.ReversibleExecutionRequest{
		OperationID:  "op-reverse",
		Preflight:    preflight,
		Snapshot:     snapshot,
		SnapshotHash: packageMigrationHash(snapshot),
		CurrentSnapshot: func(context.Context) ([]byte, error) {
			return append([]byte(nil), state...), nil
		},
	}
	order := []string{}
	handler := func(ctx context.Context, step migration.ReversiblePlanStep, definition migration.MigrationDefinition) (migration.ReversibleStepResult, error) {
		order = append(order, step.MigrationID)
		switch step.MigrationID {
		case "f1":
			state = []byte("one")
		case "f2":
			state = []byte("two")
		case "f3":
			return migration.ReversibleStepResult{Evidence: json.RawMessage(`{"phase":"failed"}`)}, errors.New("forward failed")
		case "r2":
			state = []byte("one")
		case "r1":
			state = []byte("initial")
		}
		return migration.ReversibleStepResult{Evidence: json.RawMessage(`{"ok":true}`)}, nil
	}
	if _, err := guard.ExecuteForward(context.Background(), request, handler); err == nil {
		t.Fatal("expected compensated forward failure")
	}
	want := []string{"f1", "f2", "f3", "r2", "r1"}
	if fmt.Sprint(order) != fmt.Sprint(want) || string(state) != "initial" {
		t.Fatalf("unexpected compensation order/state: order=%v state=%s", order, state)
	}
	steps, err := container.MigrationRepository.ListMigrationSteps(context.Background(), request.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 5 || steps[2].OutputHash == "" {
		t.Fatalf("failure evidence or reverse journal missing: %+v", steps)
	}

	tampered := request
	tampered.OperationID = "op-tampered"
	tampered.SnapshotHash = packageMigrationHash([]byte("other"))
	called := false
	_, err = guard.ExecuteForward(context.Background(), tampered, func(ctx context.Context, step migration.ReversiblePlanStep, definition migration.MigrationDefinition) (migration.ReversibleStepResult, error) {
		called = true
		return migration.ReversibleStepResult{}, nil
	})
	if err == nil || called {
		t.Fatalf("tampered snapshot was not rejected before execution: err=%v called=%v", err, called)
	}
}

func TestPackageMigrationGuardReverseCrashResume(t *testing.T) {
	_, guard := newPackageMigrationTestGuard(t)
	preflight := packageMigrationPreflight(t, guard, "ext.reverse-resume", packageMigrationChain("ext.reverse-resume", 2), "1.0.0", "1.2.0")
	request := migration.ReversibleExecutionRequest{OperationID: "op-reverse-resume", Preflight: preflight}
	if _, err := guard.ExecuteForward(context.Background(), request, func(context.Context, migration.ReversiblePlanStep, migration.MigrationDefinition) (migration.ReversibleStepResult, error) {
		return migration.ReversibleStepResult{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	first := true
	calls := map[string]int{}
	handler := func(ctx context.Context, step migration.ReversiblePlanStep, definition migration.MigrationDefinition) (migration.ReversibleStepResult, error) {
		calls[step.MigrationID]++
		if step.MigrationID == "r2" && first {
			first = false
			return migration.ReversibleStepResult{}, migration.ErrMigrationExecutionInterrupted
		}
		return migration.ReversibleStepResult{}, nil
	}
	if err := guard.CompensateReverse(context.Background(), request, handler); !errors.Is(err, migration.ErrMigrationExecutionInterrupted) {
		t.Fatalf("expected reverse interruption, got %v", err)
	}
	if err := guard.CompensateReverse(context.Background(), request, handler); err != nil {
		t.Fatal(err)
	}
	if calls["r2"] != 2 || calls["r1"] != 1 {
		t.Fatalf("reverse resume did not use journal: %v", calls)
	}
	if err := guard.CompensateReverse(context.Background(), request, handler); err != nil {
		t.Fatal(err)
	}
	if calls["r2"] != 2 || calls["r1"] != 1 {
		t.Fatalf("reversed steps ran again: %v", calls)
	}
}

func TestPackageMigrationGuardIrreversibleAndPartialReverseFailure(t *testing.T) {
	container, guard := newPackageMigrationTestGuard(t)
	irreversible := packageMigrationChain("ext.manual", 1)
	irreversible[0].Reversibility = migration.ReversibilityIrreversible
	preflight := packageMigrationPreflight(t, guard, "ext.manual", irreversible, "1.0.0", "1.1.0")
	op, err := guard.ExecuteForward(context.Background(), migration.ReversibleExecutionRequest{OperationID: "op-manual", Preflight: preflight, AllowManual: true}, func(context.Context, migration.ReversiblePlanStep, migration.MigrationDefinition) (migration.ReversibleStepResult, error) {
		return migration.ReversibleStepResult{Evidence: json.RawMessage(`{"started":true}`)}, errors.New("irreversible failed")
	})
	if !errors.Is(err, migration.ErrMigrationManualRecovery) || op.Status != migration.OperationStatusManualIntervention {
		t.Fatalf("irreversible failure did not require manual recovery: op=%+v err=%v", op, err)
	}

	definitions := packageMigrationChain("ext.partial", 3)
	partial := packageMigrationPreflight(t, guard, "ext.partial", definitions, "1.0.0", "2.0.0")
	op, err = guard.ExecuteForward(context.Background(), migration.ReversibleExecutionRequest{OperationID: "op-partial", Preflight: partial}, func(ctx context.Context, step migration.ReversiblePlanStep, definition migration.MigrationDefinition) (migration.ReversibleStepResult, error) {
		if step.MigrationID == "f3" {
			return migration.ReversibleStepResult{}, errors.New("forward failed")
		}
		if step.MigrationID == "r1" {
			return migration.ReversibleStepResult{Evidence: json.RawMessage(`{"reverse":"partial"}`)}, errors.New("reverse failed")
		}
		return migration.ReversibleStepResult{}, nil
	})
	if err == nil || op.Status != migration.OperationStatusRecoveryRequired {
		t.Fatalf("partial reverse failure did not preserve recovery state: op=%+v err=%v", op, err)
	}
	steps, listErr := container.MigrationRepository.ListMigrationSteps(context.Background(), "op-partial")
	if listErr != nil {
		t.Fatal(listErr)
	}
	found := false
	for _, step := range steps {
		if step.Status == "reverse_failed" && step.OutputHash != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("partial reverse evidence not retained: %+v", steps)
	}
}

func TestPackageMigrationGuardDetectsAppliedDefinitionDrift(t *testing.T) {
	container, guard := newPackageMigrationTestGuard(t)
	now := time.Now().UTC()
	op := &migration.MigrationOperation{OperationID: "op-old", ExtensionID: "ext.drift", FromVersion: "1.0.0", ToVersion: "1.1.0", Status: migration.OperationStatusCompleted, StartedAt: now, Reversibility: migration.ReversibilityFullyReversible}
	if err := container.MigrationRepository.SaveMigrationOperation(context.Background(), op); err != nil {
		t.Fatal(err)
	}
	if err := container.MigrationRepository.SaveMigrationStep(context.Background(), &migration.MigrationStepRecord{StepID: 1, OperationID: op.OperationID, MigrationID: "f1", Status: "succeeded", InputHash: "old-hash", StartedAt: now}); err != nil {
		t.Fatal(err)
	}
	manifest := manifest_v2.Manifest{Extension: manifest_v2.ExtensionMeta{ID: "ext.drift", Version: "1.1.0", Metadata: map[string]any{"migrations": packageMigrationChain("ext.drift", 1)}}}
	_, err := guard.PreflightManifest(context.Background(), manifest, "1.0.0")
	if err == nil {
		t.Fatal("expected applied definition drift rejection")
	}
}

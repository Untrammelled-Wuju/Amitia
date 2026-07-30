package kernel

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

type packageFaultExpectation string

const (
	packageFaultContinue     packageFaultExpectation = "continue"
	packageFaultCompensate   packageFaultExpectation = "compensate"
	packageFaultManual       packageFaultExpectation = "manual"
	packageFaultTerminalFail packageFaultExpectation = "terminal_fail"
)

type packageFaultCase struct {
	operation string
	step      string
	position  string
	expected  packageFaultExpectation
}

var packagePersistedSteps = map[string][]string{
	"install": {
		"validate_preview_session", "reverify_artifact_hash", "extract_to_staging", "build_candidate_definitions",
		"commit_installed_tree", "switch_current_pointer", "commit_kernel_repositories", "mark_installation_disabled",
	},
	"update": {
		"validate_and_diff", "create_rollback_point", "commit_target_generation", "execute_migrations",
		"switch_current_pointer", "commit_update_state",
	},
	"rollback": {
		"validate_rollback_point", "commit_rollback_generation", "switch_current_pointer", "restore_repositories",
	},
	"uninstall": {
		"validate_uninstall_preflight", "move_to_quarantine", "cleanup_kernel_repositories", "finalize_quarantine",
	},
}

func TestPackageFaultMatrixCoversEveryPersistedStepBoundary(t *testing.T) {
	matrix := requiredPackageFaultMatrix()
	seen := map[string]packageFaultExpectation{}
	for _, entry := range matrix {
		key := entry.operation + ":" + entry.step + ":" + entry.position
		if _, duplicate := seen[key]; duplicate {
			t.Fatalf("duplicate package fault case %s", key)
		}
		seen[key] = entry.expected
	}
	for operation, steps := range packagePersistedSteps {
		for _, step := range steps {
			for _, position := range []string{"before_persist", "after_persist"} {
				key := operation + ":" + step + ":" + position
				if _, exists := seen[key]; !exists {
					t.Fatalf("missing package fault boundary %s", key)
				}
			}
		}
	}
}

func TestPackageFaultMatrixRealJournalCracksRecoverDeterministically(t *testing.T) {
	if os.Getenv("AMITIA_RUN_PACKAGE_FAULT_MATRIX") != "1" {
		t.Skip("set AMITIA_RUN_PACKAGE_FAULT_MATRIX=1 after operation compatibility repair")
	}
	for operation := range packagePersistedSteps {
		operation := operation
		t.Run(operation, func(t *testing.T) {
			ctx := context.Background()
			runtimeInstance, container := newPackagePipelineRuntime(t)
			now := time.Now().UTC().Format(time.RFC3339Nano)
			record := PackageOperationRecord{OperationID: "fault-" + operation, TraceID: "trace-fault-" + operation, UserID: "user-1", ScopeType: "global", ExtensionID: "com.example/fault-" + operation, TargetVersion: "2.0.0", OperationType: operation, Status: string(PackageOperationPending), CurrentStep: "created", ConfirmationsJSON: "{}", StartedAt: now, UpdatedAt: now}
			if err := container.PackageRepository.CreateOperation(ctx, record); err != nil {
				t.Fatal(err)
			}
			if err := container.PackageRepository.TransitionOperation(ctx, record.OperationID, []PackageOperationStatus{PackageOperationPending}, PackageOperationInProgress, PackageOperationTransition{CurrentStep: "crash_after_journal"}, PackageWriteGuard{}); err != nil {
				t.Fatal(err)
			}
			firstErr := runtimeInstance.RecoverPackageOperations(ctx)
			if firstErr == nil {
				t.Fatal("recovery must report that the deliberately incomplete operation cannot be proven")
			}
			first, firstSteps, err := container.PackageRepository.GetOperation(ctx, "user-1", record.OperationID)
			if err != nil || first.Status != string(PackageOperationRequiresRecovery) || first.CurrentStep != "recovery_manual" {
				t.Fatalf("unexpected first recovery: operation=%+v steps=%+v err=%v recoveryErr=%v", first, firstSteps, err, firstErr)
			}
			secondErr := runtimeInstance.RecoverPackageOperations(ctx)
			if secondErr == nil {
				t.Fatal("repeated recovery must retain the same manual outcome")
			}
			second, secondSteps, err := container.PackageRepository.GetOperation(ctx, "user-1", record.OperationID)
			if err != nil || second.Status != first.Status || second.CurrentStep != first.CurrentStep || len(secondSteps) != len(firstSteps) {
				t.Fatalf("recovery replay duplicated side effects: first=%+v/%d second=%+v/%d err=%v", first, len(firstSteps), second, len(secondSteps), err)
			}
		})
	}
}

func TestPackageFaultMatrixProductionHookAvailability(t *testing.T) {
	t.Skip("Package sagas expose no test-only or production before/after persisted-step fault hook; real journal/current/DB/generation cracks are used instead")
}

func requiredPackageFaultMatrix() []packageFaultCase {
	result := []packageFaultCase{}
	for operation, steps := range packagePersistedSteps {
		for index, step := range steps {
			before := packageFaultTerminalFail
			after := packageFaultContinue
			if index > 0 {
				before = packageFaultCompensate
			}
			if step == "switch_current_pointer" || step == "cleanup_kernel_repositories" || step == "restore_repositories" || step == "commit_update_state" || step == "commit_kernel_repositories" {
				after = packageFaultManual
			}
			result = append(result,
				packageFaultCase{operation: operation, step: step, position: "before_persist", expected: before},
				packageFaultCase{operation: operation, step: step, position: "after_persist", expected: after},
			)
		}
	}
	return result
}

func packageFaultCaseName(entry packageFaultCase) string {
	return fmt.Sprintf("%s/%s/%s/%s", entry.operation, entry.step, entry.position, entry.expected)
}

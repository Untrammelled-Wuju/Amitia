package kernel

import (
	"reflect"
	"testing"
)

func TestRequiredUninstallConfirmations(t *testing.T) {
	preview := PackageUninstallPreviewResult{
		ArtifactPolicy:          ArtifactPolicyRetainForRollback,
		Dependents:              []string{"dependent-extension"},
		SnapshotRequirementHash: "sha256:snapshot-requirement",
	}
	want := []string{
		"confirm.uninstall",
		"confirm.uninstall.retain_for_rollback",
		"confirm.uninstall.dependents_affected",
		"confirm.uninstall.data_change",
	}
	if got := RequiredUninstallConfirmations(preview); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected required confirmations: got %v, want %v", got, want)
	}
}

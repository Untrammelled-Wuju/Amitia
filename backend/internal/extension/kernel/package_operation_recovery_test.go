package kernel

import "testing"

func TestPackageOperationStoppedBeforeGeneration(t *testing.T) {
	operation := PackageOperationRecord{OperationType: "update"}
	if packageOperationStoppedBeforeGeneration(operation, nil) {
		t.Fatal("operation without prior steps must remain fail-closed")
	}
	completed := map[string]PackageOperationStep{
		"confirmation.authority_evidence": {StepName: "confirmation.authority_evidence", Status: "completed"},
	}
	if !packageOperationStoppedBeforeGeneration(operation, completed) {
		t.Fatal("confirmation-only operation was not recognized as pre-generation")
	}
	operation.CurrentPointerJSON = `{"generationId":"generation-target"}`
	if packageOperationStoppedBeforeGeneration(operation, completed) {
		t.Fatal("operation with generation evidence was treated as pre-generation")
	}
	operation.CurrentPointerJSON = ""
	completed["update.commit_target_generation"] = PackageOperationStep{StepName: "update.commit_target_generation", Status: "completed"}
	if packageOperationStoppedBeforeGeneration(operation, completed) {
		t.Fatal("operation with generation commit step was treated as pre-generation")
	}
}

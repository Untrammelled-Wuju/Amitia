package kernel

import "testing"

func TestCreatedPackageOperationCanStartAndFail(t *testing.T) {
	if !validOperationTransition([]PackageOperationStatus{PackageOperationCreated}, PackageOperationInProgress) {
		t.Fatal("created operation must start")
	}
	if !validOperationTransition([]PackageOperationStatus{PackageOperationCreated}, PackageOperationFailed) {
		t.Fatal("created operation must be fail-closable")
	}
}

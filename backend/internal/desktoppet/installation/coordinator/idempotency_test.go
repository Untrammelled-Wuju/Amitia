package coordinator

import (
	"testing"

	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
)

func TestInstallRequestHashDoesNotIncludeGeneratedInstallationID(t *testing.T) {
	left := &operation.InstallationOperation{
		OperationType:   operation.TypeInstall,
		SpaceID:         "space",
		DeviceID:        "device",
		InstallationID:  "generated-left",
		PetID:           "pet",
		TargetReleaseID: "release",
	}
	right := *left
	right.InstallationID = "generated-right"
	if operationRequestHash(left, nil) != operationRequestHash(&right, nil) {
		t.Fatal("generated installation id changed install request hash")
	}
}

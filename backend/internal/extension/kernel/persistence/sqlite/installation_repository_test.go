package sqlite

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestDecodeInstallationHydratesCommittedVersionPointer(t *testing.T) {
	data := `{"installationId":"installation-1","extensionId":"com.example/plugin","installedVersion":{"major":1,"minor":0,"patch":0},"packageId":"artifact-old","installationState":"installed","enablementState":"enabled","generation":2,"metadata":{"generationId":"generation-old","artifactId":"artifact-old","installedTreeHash":"old-tree"}}`
	installedPath := t.TempDir()
	installation, err := decodeInstallation(
		data,
		"version-new",
		"generation-new",
		"",
		"1.2.1",
		"generation-new",
		"artifact-new",
		installedPath,
		"new-tree",
		"operation-new",
	)
	if err != nil {
		t.Fatal(err)
	}
	if installation.InstalledVersion != (domain.SemanticVersion{Major: 1, Minor: 2, Patch: 1}) {
		t.Fatalf("unexpected version: %s", installation.InstalledVersion)
	}
	if installation.PackageID != "artifact-new" ||
		installation.Metadata["generationId"] != "generation-new" ||
		installation.Metadata["artifactId"] != "artifact-new" ||
		installation.Metadata["installedTreeHash"] != "new-tree" {
		t.Fatalf("normalized pointer was not hydrated: %+v", installation)
	}
}

func TestDecodeInstallationKeepsJSONWhenNormalizedPointerDisagrees(t *testing.T) {
	data := `{"installationId":"installation-1","extensionId":"com.example/plugin","installedVersion":{"major":1,"minor":0,"patch":0},"packageId":"artifact-old","installationState":"installed","enablementState":"enabled","generation":2,"metadata":{"generationId":"generation-old","artifactId":"artifact-old","installedTreeHash":"old-tree"}}`
	installation, err := decodeInstallation(
		data,
		"version-new",
		"generation-new",
		"artifact-new",
		"1.2.1",
		"generation-different",
		"artifact-new",
		"/extensions/plugin/generation-new",
		"new-tree",
		"operation-new",
	)
	if err != nil {
		t.Fatal(err)
	}
	if installation.InstalledVersion != (domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}) ||
		installation.Metadata["generationId"] != "generation-old" {
		t.Fatalf("inconsistent pointer must fail closed: %+v", installation)
	}
}

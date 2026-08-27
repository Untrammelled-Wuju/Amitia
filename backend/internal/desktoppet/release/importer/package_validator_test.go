package importer

import (
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"github.com/u-ai/backend/internal/desktoppet/release"
)

func TestRuntimeCompatibilityCompatibleRange(t *testing.T) {
	max := "2.5.0"
	status, err := checkRuntimeCompatibility(packageformat.ManifestCompatibility{
		MinRuntimeVersion: "1.9.0",
		MaxRuntimeVersion: &max,
	})
	if err != nil {
		t.Fatalf("unexpected compatibility error: %v", err)
	}
	if status != release.ReleaseCompatCompatible {
		t.Fatalf("status=%s want=%s", status, release.ReleaseCompatCompatible)
	}
}

func TestRuntimeCompatibilityOutsideRangeIsIncompatible(t *testing.T) {
	status, err := checkRuntimeCompatibility(packageformat.ManifestCompatibility{MinRuntimeVersion: "3.0.0"})
	if err != nil {
		t.Fatalf("unexpected compatibility error: %v", err)
	}
	if status != release.ReleaseCompatIncompatible {
		t.Fatalf("status=%s want=%s", status, release.ReleaseCompatIncompatible)
	}
}

func TestRuntimeCompatibilityInvalidSemVerIsHardError(t *testing.T) {
	_, err := checkRuntimeCompatibility(packageformat.ManifestCompatibility{MinRuntimeVersion: "not-semver"})
	if err == nil {
		t.Fatal("expected invalid runtime version error")
	}
	var releaseErr *release.ReleaseError
	if !errors.As(err, &releaseErr) {
		t.Fatalf("expected ReleaseError, got %T: %v", err, err)
	}
	if releaseErr.Code != "PACKAGE_RUNTIME_VERSION_INVALID" {
		t.Fatalf("code=%s want=PACKAGE_RUNTIME_VERSION_INVALID", releaseErr.Code)
	}
}

func TestParseSemVerRejectsNegativeAndMalformedValues(t *testing.T) {
	for _, value := range []string{"", "1", "1.2", "1.x.0", "01.2.3", "1.2.03", "1.2.3.4", "-1.2.3", "1.2.3.4.5", "1.2.3-01", "1.2.3+"} {
		if got := parseSemVer(value); got != nil {
			t.Fatalf("parseSemVer(%q)=%v, want nil", value, got)
		}
	}
}

func TestCompareSemVerHonorsPrereleaseOrdering(t *testing.T) {
	alpha := parseSemVer("2.0.0-alpha.1")
	releaseVersion := parseSemVer("2.0.0")
	buildMetadata := parseSemVer("2.0.0+build.7")
	if alpha == nil || releaseVersion == nil || buildMetadata == nil {
		t.Fatal("expected valid strict SemVer values")
	}
	if compareSemVer(alpha, releaseVersion) >= 0 {
		t.Fatal("prerelease must sort before the final release")
	}
	if compareSemVer(releaseVersion, buildMetadata) != 0 {
		t.Fatal("build metadata must not affect precedence")
	}
}

package sandbox

import (
	"fmt"
	"strings"
	"testing"
)

func TestRootfsInstallSpecValidation_ValidBundled(t *testing.T) {
	spec := RootfsInstallSpec{
		RootfsVersion:     "alpine-3.19-x86-v1",
		AlpineVersion:     "3.19",
		GuestArchitecture: "x86",
		SourceType:        RootfsSourceBundled,
		BundleResource:    "rootfs/alpine-3.19-x86.tar.gz",
		SHA256:            strings.Repeat("ab", 32),
		ExpectedSize:      50 * 1024 * 1024,
		ArchiveFormat:     ArchiveFormatTarGz,
		Activate:          false,
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("expected valid spec, got: %v", err)
	}
}

func TestRootfsInstallSpecValidation_ValidRemote(t *testing.T) {
	spec := RootfsInstallSpec{
		RootfsVersion:     "alpine-3.19-x86-v1",
		AlpineVersion:     "3.19",
		GuestArchitecture: "x86",
		SourceType:        RootfsSourceRemote,
		SourceURL:         "https://amitia.example.com/rootfs/alpine-3.19-x86.tar.gz",
		SHA256:            strings.Repeat("cd", 32),
		ExpectedSize:      50 * 1024 * 1024,
		ArchiveFormat:     ArchiveFormatTarGz,
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestRootfsInstallSpecValidation_MissingVersion(t *testing.T) {
	spec := RootfsInstallSpec{
		GuestArchitecture: "x86",
		SourceType:        RootfsSourceBundled,
		BundleResource:    "x",
		SHA256:            strings.Repeat("ab", 32),
		ExpectedSize:      1000,
		ArchiveFormat:     ArchiveFormatTarGz,
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("expected error for missing rootfsVersion")
	}
}

func TestRootfsInstallSpecValidation_InvalidArch(t *testing.T) {
	spec := RootfsInstallSpec{
		RootfsVersion:     "v1",
		GuestArchitecture: "not an arch!",
		SourceType:        RootfsSourceBundled,
		BundleResource:    "x",
		SHA256:            strings.Repeat("ab", 32),
		ExpectedSize:      1000,
		ArchiveFormat:     ArchiveFormatTarGz,
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("expected error for invalid architecture")
	}
}

func TestRootfsInstallSpecValidation_InvalidSource(t *testing.T) {
	spec := RootfsInstallSpec{
		RootfsVersion:     "v1",
		GuestArchitecture: "x86",
		SourceType:        RootfsSourceType("user-url"),
		SHA256:            strings.Repeat("ab", 32),
		ExpectedSize:      1000,
		ArchiveFormat:     ArchiveFormatTarGz,
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("expected error for invalid source type")
	}
}

func TestRootfsInstallSpecValidation_RemoteMissingURL(t *testing.T) {
	spec := RootfsInstallSpec{
		RootfsVersion:     "v1",
		GuestArchitecture: "x86",
		SourceType:        RootfsSourceRemote,
		SHA256:            strings.Repeat("ab", 32),
		ExpectedSize:      1000,
		ArchiveFormat:     ArchiveFormatTarGz,
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("expected error for remote missing url")
	}
}

func TestRootfsInstallSpecValidation_BundledMissingResource(t *testing.T) {
	spec := RootfsInstallSpec{
		RootfsVersion:     "v1",
		GuestArchitecture: "x86",
		SourceType:        RootfsSourceBundled,
		SHA256:            strings.Repeat("ab", 32),
		ExpectedSize:      1000,
		ArchiveFormat:     ArchiveFormatTarGz,
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("expected error for bundled missing resource")
	}
}

func TestRootfsInstallSpecValidation_InvalidSHA256(t *testing.T) {
	spec := RootfsInstallSpec{
		RootfsVersion:     "v1",
		GuestArchitecture: "x86",
		SourceType:        RootfsSourceBundled,
		BundleResource:    "x",
		SHA256:            "tooshort",
		ExpectedSize:      1000,
		ArchiveFormat:     ArchiveFormatTarGz,
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("expected error for invalid sha256")
	}
}

func TestRootfsInstallSpecValidation_SizeTooLarge(t *testing.T) {
	spec := RootfsInstallSpec{
		RootfsVersion:     "v1",
		GuestArchitecture: "x86",
		SourceType:        RootfsSourceBundled,
		BundleResource:    "x",
		SHA256:            strings.Repeat("ab", 32),
		ExpectedSize:      MaxRootfsArchiveBytes + 1,
		ArchiveFormat:     ArchiveFormatTarGz,
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("expected error for oversize spec")
	}
}

func TestRootfsInstallSpecValidation_InvalidFormat(t *testing.T) {
	spec := RootfsInstallSpec{
		RootfsVersion:     "v1",
		GuestArchitecture: "x86",
		SourceType:        RootfsSourceBundled,
		BundleResource:    "x",
		SHA256:            strings.Repeat("ab", 32),
		ExpectedSize:      1000,
		ArchiveFormat:     ArchiveFormat("zip"),
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("expected error for invalid archive format")
	}
}

func TestRootfsInstallSpecValidation_ValidFormats(t *testing.T) {
	formats := []ArchiveFormat{ArchiveFormatTarGz, ArchiveFormatTarXz, ArchiveFormatTarBz2, ArchiveFormatTar}
	for _, fmt := range formats {
		spec := RootfsInstallSpec{
			RootfsVersion:     "v1",
			GuestArchitecture: "x86",
			SourceType:        RootfsSourceBundled,
			BundleResource:    "x",
			SHA256:            strings.Repeat("ab", 32),
			ExpectedSize:      1000,
			ArchiveFormat:     fmt,
		}
		if err := spec.Validate(); err != nil {
			t.Fatalf("format %s should be valid: %v", fmt, err)
		}
	}
}

func TestRootfsURIParsing_Valid(t *testing.T) {
	id, err := ParseRootfsURI("amitia://runtime/ios/rootfs/abc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "abc-123" {
		t.Fatalf("expected abc-123, got %s", id)
	}
}

func TestRootfsURIParsing_InvalidPrefix(t *testing.T) {
	_, err := ParseRootfsURI("https://wrong/prefix")
	if err == nil {
		t.Fatal("expected error for wrong prefix")
	}
	re := err.(*RootfsError)
	if re.Code != RootfsErrNotConfigured {
		t.Fatalf("expected ROOTFS_NOT_CONFIGURED, got %s", re.Code)
	}
}

func TestRootfsURIParsing_TooShort(t *testing.T) {
	_, err := ParseRootfsURI("amitia://x")
	if err == nil {
		t.Fatal("expected error for short URI")
	}
}

func TestRootfsURIParsing_MissingID(t *testing.T) {
	_, err := ParseRootfsURI("amitia://runtime/ios/rootfs/")
	if err == nil {
		t.Fatal("expected error for missing installation ID")
	}
}

func TestRootfsURIBuilding(t *testing.T) {
	uri := BuildRootfsURI("abc-123")
	expected := "amitia://runtime/ios/rootfs/abc-123"
	if uri != expected {
		t.Fatalf("expected %s, got %s", expected, uri)
	}
	id, err := ParseRootfsURI(uri)
	if err != nil {
		t.Fatalf("round-trip failed: %v", err)
	}
	if id != "abc-123" {
		t.Fatalf("round-trip id mismatch: %s", id)
	}
}

func TestRootfsErrorFormat(t *testing.T) {
	err := &RootfsError{Code: RootfsErrDigestMismatch, Message: "digest mismatch"}
	if !strings.Contains(err.Error(), RootfsErrDigestMismatch) {
		t.Fatalf("expected code in error: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("expected message in error: %s", err.Error())
	}
}

func TestRootfsErrorWithCause(t *testing.T) {
	cause := fmt.Errorf("inner error")
	err := &RootfsError{Code: RootfsErrDownloadFailed, Message: "dl failed", Cause: cause}
	if err.Unwrap() != cause {
		t.Fatal("Unwrap should return cause")
	}
	if !strings.Contains(err.Error(), "dl failed") {
		t.Fatalf("expected message: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "inner error") {
		t.Fatalf("expected cause message in error output: %s", err.Error())
	}
}

func TestRootfsStatusDefaults(t *testing.T) {
	var status RootfsStatus
	if status.Installed {
		t.Fatal("default should not be installed")
	}
	if status.RestartRequired {
		t.Fatal("default should not require restart")
	}
	if status.Corrupted {
		t.Fatal("default should not be corrupted")
	}
}

func TestRootfsVersionMetadata_Fields(t *testing.T) {
	m := RootfsVersionMetadata{
		SchemaVersion:     1,
		InstallationID:    "abc",
		RootfsVersion:     "v1",
		AlpineVersion:     "3.19",
		GuestArchitecture: "x86",
		ArchiveDigest:     "sha256:" + strings.Repeat("ab", 32),
		Validated:         true,
		ArchiveSizeBytes:  12345,
		ArchiveFormat:     "tar.gz",
		SourceType:        "bundled",
		SourceRef:         "rootfs/x.tar.gz",
	}
	if !m.Validated {
		t.Fatal("expected validated=true")
	}
	if m.SchemaVersion != 1 {
		t.Fatal("schema version mismatch")
	}
}

func TestRootfsConstants(t *testing.T) {
	if MaxRootfsArchiveBytes <= 0 || MaxRootfsArchiveBytes > 200*1024*1024 {
		t.Fatalf("MaxRootfsArchiveBytes out of range: %d", MaxRootfsArchiveBytes)
	}
	if MaxExpandedBytes <= MaxRootfsArchiveBytes {
		t.Fatal("MaxExpandedBytes should exceed archive size")
	}
	if MaxArchiveEntries < 1000 {
		t.Fatal("MaxArchiveEntries too low")
	}
	if MaxDirectoryDepth < 16 {
		t.Fatal("MaxDirectoryDepth too low")
	}
}

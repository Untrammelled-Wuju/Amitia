package ffmpeg

import (
	"os"
	"testing"
)

func TestDetectArchitecture(t *testing.T) {
	arch := detectArchitecture()
	if arch == ArchUnknown {
		t.Logf("detected architecture: %v (unknown is acceptable)", arch)
	}
	if arch == "" {
		t.Error("expected non-empty architecture")
	}
}

func TestFileExists(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"empty path", "", false},
		{"non-existent", "/tmp/non-existent-path-for-test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileExists(tt.path)
			if got != tt.expected {
				t.Errorf("fileExists(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestIsExecutable(t *testing.T) {
	if isExecutable("") {
		t.Error("empty path should not be executable")
	}
	if isExecutable("/tmp/non-existent") {
		t.Error("non-existent path should not be executable")
	}
}

func TestComputeSHA256(t *testing.T) {
	_, err := computeSHA256("/tmp/non-existent-file")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestBinaryResolver_Resolve_NoPaths(t *testing.T) {
	config := DefaultConfig()
	resolver := NewBinaryResolver(config)
	if resolver == nil {
		t.Fatal("expected non-nil resolver")
	}
}

func TestValidateBinary_NotExist(t *testing.T) {
	resolver := NewBinaryResolver(DefaultConfig())
	err := resolver.validateBinary("/tmp/non-existent-binary")
	if err == nil {
		t.Fatal("expected error")
	}
	mediaErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if mediaErr.Code != FFMPEG_BINARY_NOT_FOUND {
		t.Errorf("expected FFMPEG_BINARY_NOT_FOUND, got %q", mediaErr.Code)
	}
}

func TestValidateBinary_Directory(t *testing.T) {
	resolver := NewBinaryResolver(DefaultConfig())
	err := resolver.validateBinary(os.TempDir())
	if err == nil {
		t.Fatal("expected error for directory")
	}
	mediaErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if mediaErr.Code != FFMPEG_BINARY_INVALID {
		t.Errorf("expected FFMPEG_BINARY_INVALID, got %q", mediaErr.Code)
	}
}

func TestVerifyIntegrity_EmptyHash(t *testing.T) {
	resolver := NewBinaryResolver(DefaultConfig())
	err := resolver.VerifyIntegrity("/tmp/non-existent", "")
	if err != nil {
		t.Errorf("empty hash should skip, got: %v", err)
	}
}

func TestDiscoverFromRuntimePackage(t *testing.T) {
	resolver := NewBinaryResolver(DefaultConfig())
	_ = resolver
}

//go:build linux && !android

package archive

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectByMagic(t *testing.T) {
	tests := []struct {
		name     string
		magic    []byte
		expected Format
	}{
		{"ZIP local header", []byte("PK\x03\x04"), FormatZIP},
		{"ZIP empty archive", []byte("PK\x05\x06"), FormatZIP},
		{"ZIP spanned", []byte("PK\x07\x08"), FormatZIP},
		{"GZIP", []byte{0x1f, 0x8b, 0x08}, FormatTARGZ},
		{"BZIP2", []byte("BZh91AY&SY"), FormatTARBZ2},
		{"XZ", []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}, FormatTARXZ},
		{"ZSTD", []byte{0x28, 0xb5, 0x2f, 0xfd}, FormatTARZST},
		{"Unknown", []byte("hello"), ""},
		{"Too short", []byte("PK"), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectByMagic(tt.magic)
			if result != tt.expected {
				t.Errorf("detectByMagic() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectByExtension(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected Format
	}{
		{"ZIP", "test.zip", FormatZIP},
		{"TAR", "test.tar", FormatTAR},
		{"TAR.GZ", "test.tar.gz", FormatTARGZ},
		{"TGZ", "test.tgz", FormatTARGZ},
		{"TAR.BZ2", "test.tar.bz2", FormatTARBZ2},
		{"TBZ2", "test.tbz2", FormatTARBZ2},
		{"TAR.XZ", "test.tar.xz", FormatTARXZ},
		{"TXZ", "test.txz", FormatTARXZ},
		{"GZIP", "test.gz", FormatGZIP},
		{"BZIP2", "test.bz2", FormatBZIP2},
		{"XZ", "test.xz", FormatXZ},
		{"Unknown", "test.unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectByExtension(tt.path)
			if result != tt.expected {
				t.Errorf("detectByExtension() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsCompressedFormat(t *testing.T) {
	compressed := []Format{FormatGZIP, FormatBZIP2, FormatXZ, FormatZSTD, FormatTARGZ, FormatTARBZ2, FormatTARXZ, FormatTARZST}
	for _, f := range compressed {
		if !isCompressedFormat(f) {
			t.Errorf("isCompressedFormat(%v) = false, want true", f)
		}
	}

	uncompressed := []Format{FormatZIP, FormatTAR, ""}
	for _, f := range uncompressed {
		if isCompressedFormat(f) {
			t.Errorf("isCompressedFormat(%v) = true, want false", f)
		}
	}
}

func TestIsArchiveFormat(t *testing.T) {
	archives := []Format{FormatZIP, FormatTAR, FormatTARGZ, FormatTARBZ2, FormatTARXZ, FormatTARZST}
	for _, f := range archives {
		if !isArchiveFormat(f) {
			t.Errorf("isArchiveFormat(%v) = false, want true", f)
		}
	}

	nonArchives := []Format{FormatGZIP, FormatBZIP2, FormatXZ, FormatZSTD, ""}
	for _, f := range nonArchives {
		if isArchiveFormat(f) {
			t.Errorf("isArchiveFormat(%v) = true, want false", f)
		}
	}
}

func TestFormatToMIME(t *testing.T) {
	tests := []struct {
		format   Format
		expected string
	}{
		{FormatZIP, "application/zip"},
		{FormatTAR, "application/x-tar"},
		{FormatTARGZ, "application/gzip"},
		{FormatGZIP, "application/gzip"},
		{FormatTARBZ2, "application/x-bzip2"},
		{FormatBZIP2, "application/x-bzip2"},
		{FormatTARXZ, "application/x-xz"},
		{FormatXZ, "application/x-xz"},
		{"", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			result := formatToMIME(tt.format)
			if result != tt.expected {
				t.Errorf("formatToMIME() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectorDetect(t *testing.T) {
	dir := t.TempDir()

	zipFile := filepath.Join(dir, "test.zip")
	if err := os.WriteFile(zipFile, []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00"), 0644); err != nil {
		t.Fatal(err)
	}

	tarGzFile := filepath.Join(dir, "test.tar.gz")
	if err := os.WriteFile(tarGzFile, []byte{0x1f, 0x8b, 0x08, 0x00}, 0644); err != nil {
		t.Fatal(err)
	}

	nonArchive := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(nonArchive, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	detector := NewDetector()

	t.Run("detect ZIP", func(t *testing.T) {
		result, err := detector.Detect(zipFile)
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}
		if result.Format != FormatZIP {
			t.Errorf("Format = %v, want %v", result.Format, FormatZIP)
		}
		if !result.Archive {
			t.Error("Archive = false, want true")
		}
	})

	t.Run("detect TAR.GZ", func(t *testing.T) {
		result, err := detector.Detect(tarGzFile)
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}
		if result.Format != FormatTARGZ {
			t.Errorf("Format = %v, want %v", result.Format, FormatTARGZ)
		}
		if !result.Compressed {
			t.Error("Compressed = false, want true")
		}
	})

	t.Run("non-archive by content", func(t *testing.T) {
		result, err := detector.Detect(nonArchive)
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}
		if result.Format != "" {
			t.Errorf("Format = %v, want empty", result.Format)
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := detector.Detect(filepath.Join(dir, "nonexistent.zip"))
		if err == nil {
			t.Error("Detect() expected error for non-existent file")
		}
	})
}

func TestNormalizeEntryPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"foo/bar", "foo/bar"},
		{"foo\\bar", "foo/bar"},
		{"/foo/bar", "foo/bar"},
		{"./foo/bar", "foo/bar"},
		{"foo/../bar", "bar"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeEntryPath(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeEntryPath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsSafeEntryPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"foo/bar", true},
		{"foo/bar/baz.txt", true},
		{"", false},
		{"../etc/passwd", false},
		{"/etc/passwd", false},
		{"..\\windows\\system32", false},
		{"foo:bar", false},
		{"foo\x00bar", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isSafeEntryPath(tt.path)
			if result != tt.expected {
				t.Errorf("isSafeEntryPath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

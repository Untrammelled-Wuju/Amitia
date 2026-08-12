package mediaread

import (
	"encoding/json"
	"testing"
)

func TestFormatToExt(t *testing.T) {
	tests := []struct {
		format   string
		expected string
	}{
		{FormatJPEG, ".jpg"},
		{FormatPNG, ".png"},
		{FormatWebP, ".webp"},
		{FormatGIF, ".gif"},
		{FormatBMP, ".bmp"},
		{FormatHEIC, ".heic"},
		{"", ".bin"},
		{"xyz", ".bin"},
	}

	for _, tt := range tests {
		got := FormatToExt(tt.format)
		if got != tt.expected {
			t.Errorf("FormatToExt(%q) = %q, want %q", tt.format, got, tt.expected)
		}
	}
}

func TestExtToFormat(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{".jpg", FormatJPEG},
		{".jpeg", FormatJPEG},
		{".png", FormatPNG},
		{".webp", FormatWebP},
		{".gif", FormatGIF},
		{".bmp", FormatBMP},
		{".heic", FormatHEIC},
		{".heif", FormatHEIF},
		{".xyz", ""},
	}

	for _, tt := range tests {
		got := ExtToFormat(tt.ext)
		if got != tt.expected {
			t.Errorf("ExtToFormat(%q) = %q, want %q", tt.ext, got, tt.expected)
		}
	}
}

func TestFormatToMIME(t *testing.T) {
	tests := []struct {
		format   string
		expected string
	}{
		{FormatJPEG, "image/jpeg"},
		{FormatPNG, "image/png"},
		{FormatWebP, "image/webp"},
		{FormatGIF, "image/gif"},
		{FormatBMP, "image/bmp"},
		{FormatHEIC, "image/heic"},
		{"xyz", "application/octet-stream"},
	}

	for _, tt := range tests {
		got := FormatToMIME(tt.format)
		if got != tt.expected {
			t.Errorf("FormatToMIME(%q) = %q, want %q", tt.format, got, tt.expected)
		}
	}
}

func TestResolvedResourceIsValid(t *testing.T) {
	tests := []struct {
		name     string
		res      ResolvedResource
		expected bool
	}{
		{"valid", ResolvedResource{URI: "amitia://test", LocalPath: "/tmp/x"}, true},
		{"empty URI", ResolvedResource{LocalPath: "/tmp/x"}, false},
		{"empty path", ResolvedResource{URI: "amitia://test"}, false},
		{"both empty", ResolvedResource{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.res.IsValid(); got != tt.expected {
				t.Errorf("IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestImageInfoJSON(t *testing.T) {
	info := ImageInfo{
		ResourceURI: "amitia://temp/test.jpg",
		MIMEType:    "image/jpeg",
		Format:      "jpeg",
		SizeBytes:   500000,
		Width:       1920,
		Height:      1080,
		Orientation: 0,
		HasAlpha:    false,
		Animated:    false,
		Source:      "camera",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ImageInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ResourceURI != info.ResourceURI {
		t.Fatal("ResourceURI mismatch")
	}
	if decoded.Width != info.Width {
		t.Fatal("Width mismatch")
	}
	if decoded.Source != info.Source {
		t.Fatal("Source mismatch")
	}
}

func TestNormalizedImageJSON(t *testing.T) {
	img := NormalizedImage{
		ResourceURI: "amitia://temp/norm.jpg",
		MIMEType:    "image/jpeg",
		Width:       800,
		Height:      600,
		SizeBytes:    100000,
		Normalized:  true,
		SourceURI:   "amitia://temp/original.jpg",
	}

	data, err := json.Marshal(img)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded NormalizedImage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !decoded.Normalized {
		t.Fatal("expected normalized=true")
	}
	if decoded.SourceURI != img.SourceURI {
		t.Fatal("SourceURI mismatch")
	}
}

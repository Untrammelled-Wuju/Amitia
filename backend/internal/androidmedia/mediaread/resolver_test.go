package mediaread

import (
	"strings"
	"testing"
)

func TestDetectMIMEFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/tmp/test.jpg", "image/jpeg"},
		{"/tmp/test.jpeg", "image/jpeg"},
		{"/tmp/test.png", "image/png"},
		{"/tmp/test.webp", "image/webp"},
		{"/tmp/test.gif", "image/gif"},
		{"/tmp/test.bmp", "image/bmp"},
		{"/tmp/test.heic", "image/heic"},
		{"/tmp/test.xyz", "application/octet-stream"},
		{"/tmp/noext", "application/octet-stream"},
	}

	for _, tt := range tests {
		got := DetectMIMEFromPath(tt.path)
		if got != tt.expected {
			t.Errorf("DetectMIMEFromPath(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}

func TestDetectMIMEFromBytes(t *testing.T) {
	webpData := make([]byte, 16)
	copy(webpData[:4], []byte{'R', 'I', 'F', 'F'})
	copy(webpData[8:12], []byte{'W', 'E', 'B', 'P'})
	copy(webpData[12:], []byte{'V', 'P', '8', ' '})

	bmpData := make([]byte, 14)
	copy(bmpData[:2], []byte{'B', 'M'})

	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0}, "image/jpeg"},
		{"png", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, "image/png"},
		{"webp", webpData, "image/webp"},
		{"gif", []byte{'G', 'I', 'F', '8', '9', 'a'}, "image/gif"},
		{"gif87", []byte{'G', 'I', 'F', '8', '7', 'a'}, "image/gif"},
		{"bmp", bmpData, "image/bmp"},
		{"empty", []byte{}, "application/octet-stream"},
		{"short", []byte{0x01}, "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectMIMEFromBytes(tt.data)
			if got != tt.expected {
				t.Errorf("DetectMIMEFromBytes() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestMIMEToFormat(t *testing.T) {
	tests := []struct {
		mime     string
		expected string
	}{
		{"image/jpeg", FormatJPEG},
		{"image/png", FormatPNG},
		{"image/webp", FormatWebP},
		{"image/gif", FormatGIF},
		{"image/bmp", FormatBMP},
		{"image/heic", FormatHEIC},
		{"image/heif", FormatHEIF},
		{"text/plain", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := MIMEToFormat(tt.mime)
		if got != tt.expected {
			t.Errorf("MIMEToFormat(%q) = %q, want %q", tt.mime, got, tt.expected)
		}
	}
}

func TestClassifySource(t *testing.T) {
	tests := []struct {
		root     string
		relPath  string
		expected string
	}{
		{"workspace", "docs/img.png", SourceWorkspace},
		{"attachments", "img.png", SourceAttachment},
		{"temp", "android-media/camera/cap.jpg", SourceCamera},
		{"temp", "other.png", SourceTemp},
		{"cache", "cached.png", SourceCache},
		{"data", "data.png", SourceUnknown},
		{"unknown", "x.png", SourceUnknown},
	}

	for _, tt := range tests {
		got := classifySource(tt.root, tt.relPath)
		if got != tt.expected {
			t.Errorf("classifySource(%q, %q) = %q, want %q", tt.root, tt.relPath, got, tt.expected)
		}
	}
}

func TestIsWebPSignature(t *testing.T) {
	validWebP := make([]byte, 12)
	copy(validWebP[:4], []byte{'R', 'I', 'F', 'F'})
	copy(validWebP[8:12], []byte{'W', 'E', 'B', 'P'})
	if !isWebPSignature(validWebP) {
		t.Fatal("expected valid WebP")
	}

	invalid := make([]byte, 12)
	copy(invalid[:4], []byte{'R', 'I', 'F', 'F'})
	copy(invalid[8:12], []byte{'X', 'X', 'X', 'X'})
	if isWebPSignature(invalid) {
		t.Fatal("expected invalid WebP")
	}

	if isWebPSignature([]byte{'R', 'I', 'F'}) {
		t.Fatal("short data should not be WebP")
	}
}

func TestIsAnimatedWebP(t *testing.T) {
	animated := make([]byte, 20)
	copy(animated, []byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P'})
	copy(animated[12:], []byte("ANMF"))

	if !isAnimatedWebP(animated) {
		t.Fatal("expected animated WebP")
	}

	notAnimated := make([]byte, 20)
	copy(notAnimated, []byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P'})
	copy(notAnimated[12:], []byte("VP8 "))

	if isAnimatedWebP(notAnimated) {
		t.Fatal("expected non-animated WebP")
	}
}

func TestMediaReadError(t *testing.T) {
	err := &MediaReadError{Code: MediaReadInvalidURI, Message: "invalid"}
	expected := "MEDIA_READ_INVALID_URI: invalid"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}

	var nilErr *MediaReadError
	if nilErr.Error() != "" {
		t.Fatal("expected empty string for nil error")
	}
}

func TestDeclaresMIME(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"image/jpeg", "image/jpeg"},
		{"image/png", "image/png"},
		{"image/webp", "image/webp"},
		{"image/gif", "image/gif"},
		{"image/bmp", "image/bmp"},
		{"image/heic", "image/heic"},
		{"text/plain", "application/octet-stream"},
		{"", "application/octet-stream"},
	}

	for _, tt := range tests {
		got := DeclaredMIME(tt.input)
		if got != tt.expected {
			t.Errorf("DeclaredMIME(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestHasAlphaChannel(t *testing.T) {
	pngData := make([]byte, 8)
	copy(pngData[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	if !hasAlphaChannel(pngData) {
		t.Fatal("expected PNG to have alpha")
	}

	webpData := make([]byte, 12)
	copy(webpData[:4], []byte{'R', 'I', 'F', 'F'})
	copy(webpData[8:12], []byte{'W', 'E', 'B', 'P'})
	if !hasAlphaChannel(webpData) {
		t.Fatal("expected WebP to have alpha")
	}

	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	if hasAlphaChannel(jpegData) {
		t.Fatal("expected JPEG to have no alpha")
	}

	if hasAlphaChannel([]byte{0x01}) {
		t.Fatal("expected short data to have no alpha")
	}
}

func TestSafeDecodeMaxPixels(t *testing.T) {
	if err := safeDecodeMaxPixels(100, 100, 40_000_000); err != nil {
		t.Fatalf("expected no error: %v", err)
	}

	if err := safeDecodeMaxPixels(100000, 100000, 40_000_000); err == nil {
		t.Fatal("expected error for too large image")
	}
}

func TestReadLimited(t *testing.T) {
	data := []byte("hello world")

	reader := strings.NewReader(string(data))
	result, err := readLimited(reader, int64(len(data)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != string(data) {
		t.Fatalf("expected %q, got %q", data, result)
	}

	reader = strings.NewReader(string(data))
	_, err = readLimited(reader, 5)
	if err == nil {
		t.Fatal("expected error for too large read")
	}
}

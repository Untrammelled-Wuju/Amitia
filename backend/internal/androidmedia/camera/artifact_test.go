package camera

import (
	"encoding/json"
	"testing"
	"time"
)

func TestArtifactRecord_IsValid(t *testing.T) {
	record := ArtifactRecord{
		ResourceURI: "amitia://temp/android-media/camera/test.jpg",
		MIMEType:    "image/jpeg",
		Width:       1920,
		Height:      1080,
		SizeBytes:   500000,
	}

	if !record.IsValid() {
		t.Fatal("expected valid record")
	}
}

func TestArtifactRecord_IsValid_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		record ArtifactRecord
	}{
		{"empty URI", ArtifactRecord{MIMEType: "image/jpeg", Width: 1, Height: 1, SizeBytes: 1}},
		{"empty MIME", ArtifactRecord{ResourceURI: "amitia://test", Width: 1, Height: 1, SizeBytes: 1}},
		{"zero width", ArtifactRecord{ResourceURI: "amitia://test", MIMEType: "image/jpeg", Width: 0, Height: 1, SizeBytes: 1}},
		{"zero height", ArtifactRecord{ResourceURI: "amitia://test", MIMEType: "image/jpeg", Width: 1, Height: 0, SizeBytes: 1}},
		{"zero size", ArtifactRecord{ResourceURI: "amitia://test", MIMEType: "image/jpeg", Width: 1, Height: 1, SizeBytes: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.record.IsValid() {
				t.Fatal("expected invalid record")
			}
		})
	}
}

func TestArtifactRecord_IsExpired(t *testing.T) {
	now := time.Now()

	record := ArtifactRecord{
		ExpiresAt: now.Add(1 * time.Hour),
	}
	if record.IsExpired(now) {
		t.Fatal("should not be expired before ExpiresAt")
	}

	record2 := ArtifactRecord{
		ExpiresAt: now.Add(-1 * time.Hour),
	}
	if !record2.IsExpired(now) {
		t.Fatal("should be expired after ExpiresAt")
	}

	record3 := ArtifactRecord{
		ExpiresAt: time.Time{},
	}
	if record3.IsExpired(now) {
		t.Fatal("zero ExpiresAt should not be expired")
	}
}

func TestArtifactURI(t *testing.T) {
	uri := ArtifactURI("req-123", ".jpg")
	expected := "amitia://temp/android-media/camera/req-123.jpg"
	if uri != expected {
		t.Fatalf("expected %q, got %q", expected, uri)
	}
}

func TestSafeResourceName(t *testing.T) {
	tests := []struct {
		requestID string
		ext       string
		expected  string
	}{
		{"req-123", ".jpg", "req-123.jpg"},
		{"abcDEF", "png", "abcDEF.png"},
		{"with spaces", ".webp", "with_spaces.webp"},
		{"special!@#", ".jpg", "special___.jpg"},
		{"", ".jpg", ".jpg"},
	}

	for _, tt := range tests {
		got := SafeResourceName(tt.requestID, tt.ext)
		if got != tt.expected {
			t.Errorf("SafeResourceName(%q, %q) = %q, want %q", tt.requestID, tt.ext, got, tt.expected)
		}
	}
}

func TestFormatToMIME(t *testing.T) {
	tests := []struct {
		format   string
		expected string
	}{
		{"jpeg", "image/jpeg"},
		{"png", "image/png"},
		{"webp", "image/webp"},
		{"bmp", "application/octet-stream"},
		{"", "application/octet-stream"},
	}

	for _, tt := range tests {
		got := FormatToMIME(tt.format)
		if got != tt.expected {
			t.Errorf("FormatToMIME(%q) = %q, want %q", tt.format, got, tt.expected)
		}
	}
}

func TestFormatToExt(t *testing.T) {
	tests := []struct {
		format   string
		expected string
	}{
		{"jpeg", ".jpg"},
		{"png", ".png"},
		{"webp", ".webp"},
		{"bmp", ".bin"},
		{"", ".bin"},
	}

	for _, tt := range tests {
		got := FormatToExt(tt.format)
		if got != tt.expected {
			t.Errorf("FormatToExt(%q) = %q, want %q", tt.format, got, tt.expected)
		}
	}
}

func TestDefaultArtifactURI(t *testing.T) {
	uri := DefaultArtifactURI("req-456", "jpeg")
	expected := "amitia://temp/android-media/camera/req-456.jpg"
	if uri != expected {
		t.Fatalf("expected %q, got %q", expected, uri)
	}
}

func TestArtifactRecordJSON(t *testing.T) {
	record := ArtifactRecord{
		ResourceURI:      "amitia://temp/android-media/camera/test.jpg",
		MIMEType:         "image/jpeg",
		Width:            1920,
		Height:           1080,
		SizeBytes:        500000,
		ContentHash:      "sha256:abc123",
		CameraID:         "0",
		LensFacing:       "back",
		CaptureTimestamp: time.Now().UTC().Truncate(time.Millisecond),
		ExpiresAt:        time.Now().Add(30 * time.Minute).UTC().Truncate(time.Millisecond),
		EXIFStripped:     true,
	}

	data, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ArtifactRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ResourceURI != record.ResourceURI {
		t.Fatalf("URI mismatch: %s vs %s", decoded.ResourceURI, record.ResourceURI)
	}
	if decoded.MIMEType != record.MIMEType {
		t.Fatal("MIMEType mismatch")
	}
	if decoded.Width != record.Width {
		t.Fatal("Width mismatch")
	}
	if decoded.Height != record.Height {
		t.Fatal("Height mismatch")
	}
	if decoded.SizeBytes != record.SizeBytes {
		t.Fatal("SizeBytes mismatch")
	}
	if decoded.ContentHash != record.ContentHash {
		t.Fatal("ContentHash mismatch")
	}
	if decoded.CameraID != record.CameraID {
		t.Fatal("CameraID mismatch")
	}
	if decoded.LensFacing != record.LensFacing {
		t.Fatal("LensFacing mismatch")
	}
	if !decoded.EXIFStripped {
		t.Fatal("EXIFStripped mismatch")
	}
}

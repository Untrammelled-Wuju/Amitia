package mediaread

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"
)

func decodeBase64Helper(s string) []byte {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic("failed to decode base64: " + err.Error())
	}
	return data
}

func tinyJPEG() []byte {
	return decodeBase64Helper("/9j/4AAQSkZJRgABAQEAYABgAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/wAALCAABAAEBAREA/8QAFAABAAAAAAAAAAAAAAAAAAAAAv/EABQQAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQEAAD8AfwD/2Q==")
}

func tinyPNG() []byte {
	return decodeBase64Helper("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")
}

func TestImageDecoder_Inspect_ValidJPEG(t *testing.T) {
	decoder := NewImageDecoder(DefaultPolicy())
	data := tinyJPEG()

	info, err := decoder.Inspect(context.Background(), bytes.NewReader(data), "image/jpeg", int64(len(data)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.MIMEType != "image/jpeg" {
		t.Fatalf("expected image/jpeg, got %s", info.MIMEType)
	}
	if info.Format != FormatJPEG {
		t.Fatalf("expected jpeg format, got %s", info.Format)
	}
	if info.Width != 1 || info.Height != 1 {
		t.Fatalf("expected 1x1, got %dx%d", info.Width, info.Height)
	}
	if info.SizeBytes != int64(len(data)) {
		t.Fatalf("expected size %d, got %d", len(data), info.SizeBytes)
	}
}

func TestImageDecoder_Inspect_ValidPNG(t *testing.T) {
	decoder := NewImageDecoder(DefaultPolicy())
	data := tinyPNG()

	info, err := decoder.Inspect(context.Background(), bytes.NewReader(data), "image/png", int64(len(data)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.MIMEType != "image/png" {
		t.Fatalf("expected image/png, got %s", info.MIMEType)
	}
	if info.Format != FormatPNG {
		t.Fatalf("expected png format, got %s", info.Format)
	}
	if !info.HasAlpha {
		t.Fatal("expected PNG to have alpha")
	}
}

func TestImageDecoder_Inspect_UnsupportedFormat(t *testing.T) {
	decoder := NewImageDecoder(DefaultPolicy())

	_, err := decoder.Inspect(context.Background(), strings.NewReader("not an image"), "application/octet-stream", 12)
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), MediaReadUnsupportedFormat) {
		t.Fatalf("expected unsupported format error, got: %v", err)
	}
}

func TestImageDecoder_Inspect_TooManyPixels(t *testing.T) {
	policy := DefaultPolicy()
	policy.MaxPixels = 0
	decoder := NewImageDecoder(policy)

	data := tinyPNG()

	_, err := decoder.Inspect(context.Background(), bytes.NewReader(data), "image/png", int64(len(data)))
	if err == nil {
		t.Fatal("expected error for too many pixels")
	}
}

func TestImageDecoder_Inspect_TooLargeWidth(t *testing.T) {
	policy := DefaultPolicy()
	policy.MaxWidth = 0
	decoder := NewImageDecoder(policy)

	data := tinyPNG()

	_, err := decoder.Inspect(context.Background(), bytes.NewReader(data), "image/png", int64(len(data)))
	if err == nil {
		t.Fatal("expected error for too large width")
	}
}

func TestImageDecoder_Inspect_EmptyReader(t *testing.T) {
	decoder := NewImageDecoder(DefaultPolicy())

	_, err := decoder.Inspect(context.Background(), bytes.NewReader(nil), "image/jpeg", 0)
	if err == nil {
		t.Fatal("expected error for empty reader")
	}
}

func TestImageDecoder_InspectCancellation(t *testing.T) {
	decoder := NewImageDecoder(DefaultPolicy())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := decoder.Inspect(ctx, bytes.NewReader(tinyJPEG()), "image/jpeg", int64(len(tinyJPEG())))
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestImageDetectorByMagicBytes(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{"jpeg", tinyJPEG(), "image/jpeg"},
		{"png", tinyPNG(), "image/png"},
		{"gif", []byte{'G', 'I', 'F', '8', '9', 'a'}, "image/gif"},
		{"webp", []byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' '}, "image/webp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectMIMEFromBytes(tt.data)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

package camera

import (
	"testing"
	"time"
)

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()

	if p.MaxCapturePixels != DefaultMaxCapturePixels {
		t.Fatalf("expected MaxCapturePixels=%d, got %d", DefaultMaxCapturePixels, p.MaxCapturePixels)
	}
	if p.MaxEncodedBytes != DefaultMaxEncodedBytes {
		t.Fatalf("expected MaxEncodedBytes=%d, got %d", DefaultMaxEncodedBytes, p.MaxEncodedBytes)
	}
	if p.DefaultFormat != "jpeg" {
		t.Fatalf("expected default format jpeg, got %s", p.DefaultFormat)
	}
	if p.DefaultQuality != 90 {
		t.Fatalf("expected default quality 90, got %d", p.DefaultQuality)
	}
	if p.MaxWidth != 3840 {
		t.Fatalf("expected MaxWidth 3840, got %d", p.MaxWidth)
	}
	if p.MaxHeight != 2160 {
		t.Fatalf("expected MaxHeight 2160, got %d", p.MaxHeight)
	}
	if p.MaxCaptureTime != 30*time.Second {
		t.Fatalf("expected MaxCaptureTime 30s, got %v", p.MaxCaptureTime)
	}
	if p.MinInterval != 500*time.Millisecond {
		t.Fatalf("expected MinInterval 500ms, got %v", p.MinInterval)
	}
	if p.MaxConcurrentCaptures != 1 {
		t.Fatalf("expected MaxConcurrentCaptures 1, got %d", p.MaxConcurrentCaptures)
	}
	if !p.StripSensitiveEXIF {
		t.Fatal("expected StripSensitiveEXIF=true")
	}
}

func TestPolicy_ResolveMaxWidth(t *testing.T) {
	p := DefaultPolicy()

	if v := p.ResolveMaxWidth(nil); v != 3840 {
		t.Fatalf("expected default 3840, got %d", v)
	}

	w := 1920
	if v := p.ResolveMaxWidth(&w); v != 1920 {
		t.Fatalf("expected 1920, got %d", v)
	}

	w = 99999
	if v := p.ResolveMaxWidth(&w); v != 3840 {
		t.Fatalf("expected capped to 3840, got %d", v)
	}

	w = -1
	if v := p.ResolveMaxWidth(&w); v != 3840 {
		t.Fatalf("expected default for negative, got %d", v)
	}
}

func TestPolicy_ResolveMaxHeight(t *testing.T) {
	p := DefaultPolicy()

	if v := p.ResolveMaxHeight(nil); v != 2160 {
		t.Fatalf("expected default 2160, got %d", v)
	}

	h := 1080
	if v := p.ResolveMaxHeight(&h); v != 1080 {
		t.Fatalf("expected 1080, got %d", v)
	}
}

func TestPolicy_ResolveFormat(t *testing.T) {
	p := DefaultPolicy()

	if v := p.ResolveFormat(nil); v != "jpeg" {
		t.Fatalf("expected default jpeg, got %s", v)
	}

	f := "png"
	if v := p.ResolveFormat(&f); v != "png" {
		t.Fatalf("expected png, got %s", v)
	}

	f = "bmp"
	if v := p.ResolveFormat(&f); v != "jpeg" {
		t.Fatalf("expected fallback to jpeg for invalid, got %s", v)
	}
}

func TestPolicy_ResolveQuality(t *testing.T) {
	p := DefaultPolicy()

	if v := p.ResolveQuality(nil); v != 90 {
		t.Fatalf("expected default 90, got %d", v)
	}

	q := 75
	if v := p.ResolveQuality(&q); v != 75 {
		t.Fatalf("expected 75, got %d", v)
	}

	q = 0
	if v := p.ResolveQuality(&q); v != 90 {
		t.Fatalf("expected fallback to 90 for 0, got %d", v)
	}

	q = 101
	if v := p.ResolveQuality(&q); v != 90 {
		t.Fatalf("expected fallback to 90 for 101, got %d", v)
	}
}

func TestPolicy_ResolveFlashMode(t *testing.T) {
	p := DefaultPolicy()

	if v := p.ResolveFlashMode(nil); v != FlashOff {
		t.Fatalf("expected default off, got %s", v)
	}

	f := FlashAuto
	if v := p.ResolveFlashMode(&f); v != FlashAuto {
		t.Fatalf("expected auto, got %s", v)
	}

	f = "invalid"
	if v := p.ResolveFlashMode(&f); v != FlashOff {
		t.Fatalf("expected fallback to off for invalid, got %s", v)
	}
}

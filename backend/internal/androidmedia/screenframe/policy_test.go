package screenframe

import (
	"testing"
	"time"
)

func TestDefaultScreenFramePolicy(t *testing.T) {
	p := DefaultScreenFramePolicy()

	if !p.Enabled {
		t.Error("policy should be enabled by default")
	}
	if p.MaxSessions != 1 {
		t.Errorf("expected MaxSessions 1, got %v", p.MaxSessions)
	}
	if p.DefaultFPS != 2 {
		t.Errorf("expected DefaultFPS 2, got %v", p.DefaultFPS)
	}
	if p.MaxFPS != 10 {
		t.Errorf("expected MaxFPS 10, got %v", p.MaxFPS)
	}
	if p.MaxWidth != 1280 {
		t.Errorf("expected MaxWidth 1280, got %v", p.MaxWidth)
	}
	if p.MaxHeight != 1280 {
		t.Errorf("expected MaxHeight 1280, got %v", p.MaxHeight)
	}
	if p.MaxSessionDuration != 5*time.Minute {
		t.Errorf("expected MaxSessionDuration 5m, got %v", p.MaxSessionDuration)
	}
	if p.IdleTimeout != 30*time.Second {
		t.Errorf("expected IdleTimeout 30s, got %v", p.IdleTimeout)
	}
	if p.MaxEncodeConcurrency != 1 {
		t.Errorf("expected MaxEncodeConcurrency 1, got %v", p.MaxEncodeConcurrency)
	}
	if p.MaxLatestWait != 5*time.Second {
		t.Errorf("expected MaxLatestWait 5s, got %v", p.MaxLatestWait)
	}
}

func TestScreenFramePolicy_FrameInterval(t *testing.T) {
	p := DefaultScreenFramePolicy()
	interval := p.FrameInterval()
	expected := 500 * time.Millisecond
	if interval != expected {
		t.Errorf("expected interval %v, got %v", expected, interval)
	}

	p2 := ScreenFramePolicy{DefaultFPS: 10}
	if got := p2.FrameInterval(); got != 100*time.Millisecond {
		t.Errorf("expected 100ms, got %v", got)
	}

	p3 := ScreenFramePolicy{DefaultFPS: 0}
	if got := p3.FrameInterval(); got != 500*time.Millisecond {
		t.Errorf("expected fallback 500ms, got %v", got)
	}
}

func TestScreenFramePolicy_MaxFrameAge(t *testing.T) {
	p := DefaultScreenFramePolicy()
	age := p.MaxFrameAge()
	if age <= 0 || age > 2*time.Second {
		t.Errorf("expected 0 < maxFrameAge <= 2s, got %v", age)
	}
}

func TestScreenshotFormat_MIME(t *testing.T) {
	tests := []struct {
		fmt  ScreenshotFormat
		mime string
	}{
		{FormatPNG, "image/png"},
		{FormatJPEG, "image/jpeg"},
		{FormatWebP, "image/webp"},
		{ScreenshotFormat("unknown"), "application/octet-stream"},
	}
	for _, tt := range tests {
		if got := tt.fmt.MIME(); got != tt.mime {
			t.Errorf("format %s: expected mime %s, got %s", tt.fmt, tt.mime, got)
		}
	}
}

func TestScreenshotFormat_Ext(t *testing.T) {
	if got := FormatPNG.Ext(); got != ".png" {
		t.Errorf("expected .png, got %v", got)
	}
	if got := FormatJPEG.Ext(); got != ".jpg" {
		t.Errorf("expected .jpg, got %v", got)
	}
	if got := FormatWebP.Ext(); got != ".webp" {
		t.Errorf("expected .webp, got %v", got)
	}
}

func TestScreenshotFormat_IsValid(t *testing.T) {
	for _, f := range []ScreenshotFormat{FormatPNG, FormatJPEG, FormatWebP} {
		if !f.IsValid() {
			t.Errorf("expected %s to be valid", f)
		}
	}
	for _, f := range []ScreenshotFormat{ScreenshotFormat("bmp"), ScreenshotFormat(""), ScreenshotFormat("heic")} {
		if f.IsValid() {
			t.Errorf("expected %s to be invalid", f)
		}
	}
}

package mediaread

import (
	"testing"
	"time"
)

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()

	if p.MaxInputBytes != 64*1024*1024 {
		t.Fatalf("expected MaxInputBytes=67108864, got %d", p.MaxInputBytes)
	}
	if p.MaxPixels != 40_000_000 {
		t.Fatalf("expected MaxPixels=40000000, got %d", p.MaxPixels)
	}
	if p.MaxWidth != 12000 {
		t.Fatalf("expected MaxWidth=12000, got %d", p.MaxWidth)
	}
	if p.MaxHeight != 12000 {
		t.Fatalf("expected MaxHeight=12000, got %d", p.MaxHeight)
	}
	if p.MaxNormalizedBytes != 32*1024*1024 {
		t.Fatalf("expected MaxNormalizedBytes=33554432, got %d", p.MaxNormalizedBytes)
	}
	if p.MaxDecodeTime != 10*time.Second {
		t.Fatalf("expected MaxDecodeTime=10s, got %v", p.MaxDecodeTime)
	}
	if p.MaxConcurrentReads != 2 {
		t.Fatalf("expected MaxConcurrentReads=2, got %d", p.MaxConcurrentReads)
	}
	if !p.NormalizeOrientation {
		t.Fatal("expected NormalizeOrientation=true")
	}
	if !p.StripSensitiveMetadata {
		t.Fatal("expected StripSensitiveMetadata=true")
	}
}

func TestPolicy_ResolveMaxWidth(t *testing.T) {
	p := DefaultPolicy()

	if v := p.ResolveMaxWidth(nil); v != 12000 {
		t.Fatalf("expected default 12000, got %d", v)
	}

	w := 1920
	if v := p.ResolveMaxWidth(&w); v != 1920 {
		t.Fatalf("expected 1920, got %d", v)
	}

	w = 99999
	if v := p.ResolveMaxWidth(&w); v != 12000 {
		t.Fatalf("expected capped to 12000, got %d", v)
	}
}

func TestPolicy_ResolveMaxHeight(t *testing.T) {
	p := DefaultPolicy()

	if v := p.ResolveMaxHeight(nil); v != 12000 {
		t.Fatalf("expected default 12000, got %d", v)
	}

	h := 1080
	if v := p.ResolveMaxHeight(&h); v != 1080 {
		t.Fatalf("expected 1080, got %d", v)
	}
}

func TestPolicy_ResolveMaxPixels(t *testing.T) {
	p := DefaultPolicy()

	if v := p.ResolveMaxPixels(nil); v != 40_000_000 {
		t.Fatalf("expected default 40MP, got %d", v)
	}

	mp := int64(10_000_000)
	if v := p.ResolveMaxPixels(&mp); v != 10_000_000 {
		t.Fatalf("expected 10MP, got %d", v)
	}
}

func TestPolicy_EffectiveNormalizeOrientation(t *testing.T) {
	p := DefaultPolicy()

	if !p.EffectiveNormalizeOrientation(nil) {
		t.Fatal("expected true by default")
	}

	b := false
	if p.EffectiveNormalizeOrientation(&b) {
		t.Fatal("expected false when explicitly set")
	}

	b = true
	if !p.EffectiveNormalizeOrientation(&b) {
		t.Fatal("expected true when explicitly set")
	}
}

func TestPolicy_EffectiveStripMetadata(t *testing.T) {
	p := DefaultPolicy()

	if !p.EffectiveStripMetadata(nil) {
		t.Fatal("expected true by default")
	}

	b := false
	if p.EffectiveStripMetadata(&b) {
		t.Fatal("expected false when explicitly set")
	}
}

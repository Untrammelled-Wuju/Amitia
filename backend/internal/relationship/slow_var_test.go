package relationship

import (
	"testing"
	"time"
)

func TestDefaultSlowConfigHasPositiveThresholds(t *testing.T) {
	cfg := DefaultSlowConfig()
	if cfg.TrustThreshold <= 0 {
		t.Fatalf("expected positive trust threshold, got %v", cfg.TrustThreshold)
	}
	if cfg.IntimacyThreshold <= 0 {
		t.Fatalf("expected positive intimacy threshold, got %v", cfg.IntimacyThreshold)
	}
	if cfg.DecayRate <= 0 {
		t.Fatalf("expected positive decay rate, got %v", cfg.DecayRate)
	}
}

func TestDefaultSlowBufferAllZero(t *testing.T) {
	buf := DefaultSlowBuffer()
	if buf.Trust.PendingDelta != 0 || buf.Trust.EvidenceCount != 0 {
		t.Fatalf("expected zero trust buffer, got %v", buf.Trust)
	}
	if buf.Intimacy.PendingDelta != 0 || buf.Intimacy.EvidenceCount != 0 {
		t.Fatalf("expected zero intimacy buffer, got %v", buf.Intimacy)
	}
}

func TestAccumulateSlowEvidenceBelowThresholdDoesNotFlush(t *testing.T) {
	buf := DefaultSlowBuffer()
	cfg := DefaultSlowConfig()
	impacts := []EventImpact{
		{Dimension: "trust", Delta: 2.0, Reason: "test"},
	}
	AccumulateSlowEvidence(&buf, cfg, impacts, time.Now())
	if buf.Trust.PendingDelta <= 0 {
		t.Fatalf("expected pending delta to accumulate, got %v", buf.Trust.PendingDelta)
	}
	flushed := FlushSlowAccumulation(&buf, cfg)
	if len(flushed) > 0 {
		t.Fatalf("expected no flush below threshold, got %v", flushed)
	}
}

func TestAccumulateSlowEvidenceAboveThresholdFlushes(t *testing.T) {
	buf := DefaultSlowBuffer()
	cfg := DefaultSlowConfig()
	cfg.TrustThreshold = 3.0
	impacts := []EventImpact{
		{Dimension: "trust", Delta: 4.0, Reason: "test"},
	}
	AccumulateSlowEvidence(&buf, cfg, impacts, time.Now())
	flushed := FlushSlowAccumulation(&buf, cfg)
	if len(flushed) != 1 {
		t.Fatalf("expected 1 flushed impact, got %d: %v", len(flushed), flushed)
	}
	if flushed[0].Dimension != "trust" {
		t.Fatalf("expected trust flush, got %s", flushed[0].Dimension)
	}
	if flushed[0].Reason != "slow_accumulation_flush" {
		t.Fatalf("expected slow_accumulation_flush reason, got %s", flushed[0].Reason)
	}
	if buf.Trust.PendingDelta != 0 {
		t.Fatalf("expected pending delta reset after flush, got %v", buf.Trust.PendingDelta)
	}
}

func TestMultipleAccumulationBuildsEvidenceCount(t *testing.T) {
	buf := DefaultSlowBuffer()
	cfg := DefaultSlowConfig()
	cfg.TrustThreshold = 10.0

	for i := 0; i < 5; i++ {
		impacts := []EventImpact{
			{Dimension: "trust", Delta: 1.5, Reason: "test"},
		}
		AccumulateSlowEvidence(&buf, cfg, impacts, time.Now())
	}

	if buf.Trust.EvidenceCount != 5 {
		t.Fatalf("expected 5 evidence items, got %d", buf.Trust.EvidenceCount)
	}
	if buf.Trust.PendingDelta < 7.0 {
		t.Fatalf("expected pending delta around 7.5, got %v", buf.Trust.PendingDelta)
	}
}

func TestDecaySlowBufferReducesPendingDelta(t *testing.T) {
	buf := DefaultSlowBuffer()
	cfg := DefaultSlowConfig()
	cfg.DecayRate = 0.05

	impacts := []EventImpact{
		{Dimension: "trust", Delta: 8.0, Reason: "test"},
	}
	AccumulateSlowEvidence(&buf, cfg, impacts, time.Now())

	before := buf.Trust.PendingDelta
	DecaySlowBuffer(&buf, cfg, 10)
	after := buf.Trust.PendingDelta

	if after >= before {
		t.Fatalf("expected decay to reduce delta, before=%v after=%v", before, after)
	}
}

func TestApplySlowToDimensionsUpdatesValues(t *testing.T) {
	dims := DefaultDimensions()
	buf := DefaultSlowBuffer()

	buf.Trust.VisibleChange = 10
	buf.Conflict.VisibleChange = -5

	ApplySlowToDimensions(&dims, &buf)

	if dims.Trust.Value != 60 {
		t.Fatalf("expected trust 60, got %v", dims.Trust.Value)
	}
	if dims.Conflict.Value != 10 {
		t.Fatalf("expected conflict 10, got %v", dims.Conflict.Value)
	}
}

func TestProcessSlowEvidenceFullPipeline(t *testing.T) {
	dims := DefaultDimensions()
	buf := DefaultSlowBuffer()
	cfg := DefaultSlowConfig()
	cfg.TrustThreshold = 3.0
	cfg.IntimacyThreshold = 3.0
	cfg.ConflictThreshold = 2.0

	impacts := []EventImpact{
		{Dimension: "trust", Delta: 4.0, Reason: "test"},
		{Dimension: "intimacy", Delta: 3.5, Reason: "test"},
		{Dimension: "conflict", Delta: 2.0, Reason: "test"},
	}

	flushed := ProcessSlowEvidence(&dims, &buf, cfg, impacts, time.Now())

	if len(flushed) < 2 {
		t.Fatalf("expected at least 2 flushed, got %d: %v", len(flushed), flushed)
	}
	if dims.Trust.Value <= 50 {
		t.Fatalf("expected trust to increase from 50, got %v", dims.Trust.Value)
	}
	if dims.Intimacy.Value <= 35 {
		t.Fatalf("expected intimacy to increase from 35, got %v", dims.Intimacy.Value)
	}
	if dims.Conflict.Value == 15 {
		t.Fatalf("expected conflict to change, got %v", dims.Conflict.Value)
	}
}

func TestSlowEvidenceNilBuffer(t *testing.T) {
	cfg := DefaultSlowConfig()
	impacts := []EventImpact{
		{Dimension: "trust", Delta: 5.0, Reason: "test"},
	}
	AccumulateSlowEvidence(nil, cfg, impacts, time.Now())
	flushed := FlushSlowAccumulation(nil, cfg)
	if flushed != nil {
		t.Fatalf("expected nil from nil buffer flush, got %v", flushed)
	}
	DecaySlowBuffer(nil, cfg, 10)
	ApplySlowToDimensions(nil, nil)
}

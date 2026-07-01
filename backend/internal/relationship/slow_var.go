package relationship

import (
	"math"
	"time"
)

func DefaultSlowConfig() SlowUpdateConfig {
	return SlowUpdateConfig{
		TrustThreshold:      5.0,
		IntimacyThreshold:   4.0,
		DependencyThreshold: 4.5,
		ConflictThreshold:   4.0,
		RepairThreshold:     4.5,
		DecayRate:           0.015,
		MaxEvidenceAge:      72,
	}
}

func DefaultSlowBuffer() SlowEvidenceBuffer {
	now := time.Now()
	return SlowEvidenceBuffer{
		Trust:      SlowDimension{PendingDelta: 0, VisibleChange: 0, EvidenceCount: 0, LastFlushedAt: now},
		Intimacy:   SlowDimension{PendingDelta: 0, VisibleChange: 0, EvidenceCount: 0, LastFlushedAt: now},
		Dependency: SlowDimension{PendingDelta: 0, VisibleChange: 0, EvidenceCount: 0, LastFlushedAt: now},
		Conflict:   SlowDimension{PendingDelta: 0, VisibleChange: 0, EvidenceCount: 0, LastFlushedAt: now},
		Repair:     SlowDimension{PendingDelta: 0, VisibleChange: 0, EvidenceCount: 0, LastFlushedAt: now},
	}
}

func AccumulateSlowEvidence(buffer *SlowEvidenceBuffer, config SlowUpdateConfig, impacts []EventImpact, occurredAt time.Time) {
	if buffer == nil {
		return
	}
	ageHours := time.Since(occurredAt).Hours()
	if ageHours > config.MaxEvidenceAge && config.MaxEvidenceAge > 0 {
		return
	}
	decayFactor := 1.0
	if config.DecayRate > 0 && ageHours > 0 {
		decayFactor = math.Exp(-config.DecayRate * ageHours)
	}
	for _, impact := range impacts {
		dim := getSlowDimension(buffer, impact.Dimension)
		if dim == nil {
			continue
		}
		dim.PendingDelta += impact.Delta * decayFactor
		dim.EvidenceCount++
	}
}

func FlushSlowAccumulation(buffer *SlowEvidenceBuffer, config SlowUpdateConfig) []EventImpact {
	if buffer == nil {
		return nil
	}
	now := time.Now()
	results := make([]EventImpact, 0)
	dims := []struct {
		name      string
		dim       *SlowDimension
		threshold float64
	}{
		{"trust", &buffer.Trust, config.TrustThreshold},
		{"intimacy", &buffer.Intimacy, config.IntimacyThreshold},
		{"dependency", &buffer.Dependency, config.DependencyThreshold},
		{"conflict", &buffer.Conflict, config.ConflictThreshold},
		{"repair", &buffer.Repair, config.RepairThreshold},
	}
	for _, d := range dims {
		if math.Abs(d.dim.PendingDelta) >= d.threshold {
			visible := d.dim.PendingDelta
			d.dim.VisibleChange = visible
			d.dim.LastFlushedAt = now
			d.dim.PendingDelta = 0
			d.dim.EvidenceCount = 0
			results = append(results, EventImpact{
				Dimension: d.name,
				Delta:     round4(visible),
				Reason:    "slow_accumulation_flush",
			})
		} else {
			d.dim.VisibleChange = 0
		}
	}
	return results
}

func ApplySlowToDimensions(dims *RelationshipDimensions, buffer *SlowEvidenceBuffer) {
	if dims == nil || buffer == nil {
		return
	}
	now := time.Now()
	if buffer.Trust.VisibleChange != 0 {
		dims.Trust.Value = round4(clamp01Scale(dims.Trust.Value+buffer.Trust.VisibleChange, 0, 100))
		dims.Trust.LastUpdated = now
	}
	if buffer.Intimacy.VisibleChange != 0 {
		dims.Intimacy.Value = round4(clamp01Scale(dims.Intimacy.Value+buffer.Intimacy.VisibleChange, 0, 100))
		dims.Intimacy.LastUpdated = now
	}
	if buffer.Dependency.VisibleChange != 0 {
		dims.Dependency.Value = round4(clamp01Scale(dims.Dependency.Value+buffer.Dependency.VisibleChange, 0, 100))
		dims.Dependency.LastUpdated = now
	}
	if buffer.Conflict.VisibleChange != 0 {
		dims.Conflict.Value = round4(clamp01Scale(dims.Conflict.Value+buffer.Conflict.VisibleChange, 0, 100))
		dims.Conflict.LastUpdated = now
	}
	if buffer.Repair.VisibleChange != 0 {
		dims.Repair.Value = round4(clamp01Scale(dims.Repair.Value+buffer.Repair.VisibleChange, 0, 100))
		dims.Repair.LastUpdated = now
	}
}

func DecaySlowBuffer(buffer *SlowEvidenceBuffer, config SlowUpdateConfig, hoursElapsed float64) {
	if buffer == nil || hoursElapsed <= 0 {
		return
	}
	decay := math.Exp(-config.DecayRate * hoursElapsed)
	buffer.Trust.PendingDelta *= decay
	buffer.Intimacy.PendingDelta *= decay
	buffer.Dependency.PendingDelta *= decay
	buffer.Conflict.PendingDelta *= decay
	buffer.Repair.PendingDelta *= decay
}

func getSlowDimension(buffer *SlowEvidenceBuffer, dim string) *SlowDimension {
	switch dim {
	case "trust":
		return &buffer.Trust
	case "intimacy":
		return &buffer.Intimacy
	case "dependency":
		return &buffer.Dependency
	case "conflict":
		return &buffer.Conflict
	case "repair":
		return &buffer.Repair
	default:
		return nil
	}
}

func ProcessSlowEvidence(dims *RelationshipDimensions, buffer *SlowEvidenceBuffer, config SlowUpdateConfig, impacts []EventImpact, occurredAt time.Time) []EventImpact {
	DecaySlowBuffer(buffer, config, time.Since(buffer.Trust.LastFlushedAt).Hours())
	AccumulateSlowEvidence(buffer, config, impacts, occurredAt)
	flushed := FlushSlowAccumulation(buffer, config)
	ApplySlowToDimensions(dims, buffer)
	return flushed
}

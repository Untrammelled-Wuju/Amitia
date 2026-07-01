package relationship

import (
	"math"
	"time"
)

func DefaultUnresolvedConfig() UnresolvedConfig {
	return UnresolvedConfig{
		BaseEscalationHours:    24,
		EscalationStepSeverity: 0.12,
		MaxEscalationLevel:     5,
		NaturalDecayPerHour:    0.008,
		ResolutionThreshold:    0.08,
	}
}

func CreateUnresolvedThread(topic string, reason string, severity float64, eventRefs []string) UnresolvedThread {
	now := time.Now()
	return UnresolvedThread{
		ID:              generateThreadID(),
		Topic:           topic,
		EventRefs:       eventRefs,
		Reason:          reason,
		Severity:        clamp01(severity),
		Duration:        0,
		CreatedAt:       now,
		LastEscalatedAt: now,
		ResolvedAt:      nil,
		EscalationLevel: 0,
		DecayPerHour:    0.005,
		RelationImpact:  0,
	}
}

func EscalateUnresolved(thread *UnresolvedThread, config UnresolvedConfig, now time.Time) bool {
	if thread == nil || thread.ResolvedAt != nil {
		return false
	}
	if thread.EscalationLevel >= config.MaxEscalationLevel {
		return false
	}
	hoursSinceLast := now.Sub(thread.LastEscalatedAt).Hours()
	threshold := config.BaseEscalationHours * (1 + float64(thread.EscalationLevel)*0.5)
	if hoursSinceLast < threshold {
		return false
	}
	thread.EscalationLevel++
	thread.Severity += config.EscalationStepSeverity
	thread.Severity = clamp01(thread.Severity)
	thread.LastEscalatedAt = now
	thread.Duration = now.Sub(thread.CreatedAt).Hours()
	thread.RelationImpact = thread.Severity * float64(thread.EscalationLevel) * 0.15
	return true
}

func DecayUnresolved(thread *UnresolvedThread, hoursElapsed float64) bool {
	if thread == nil || thread.ResolvedAt != nil || hoursElapsed <= 0 {
		return false
	}
	decayFactor := math.Exp(-thread.DecayPerHour * hoursElapsed)
	newSeverity := thread.Severity * decayFactor
	if newSeverity < 0.02 && thread.EscalationLevel > 0 {
		thread.EscalationLevel--
		newSeverity = 0.15
	}
	thread.Severity = round4(clamp01(newSeverity))
	thread.Duration = time.Since(thread.CreatedAt).Hours()
	thread.RelationImpact = thread.Severity * float64(thread.EscalationLevel) * 0.15
	return true
}

func ResolveUnresolved(thread *UnresolvedThread) bool {
	if thread == nil || thread.ResolvedAt != nil {
		return false
	}
	now := time.Now()
	thread.ResolvedAt = &now
	thread.Duration = now.Sub(thread.CreatedAt).Hours()
	thread.RelationImpact = 0
	return true
}

func ApplyUnresolvedImpact(dims *RelationshipDimensions, threads []UnresolvedThread) []EventImpact {
	if dims == nil {
		return nil
	}
	impacts := make([]EventImpact, 0)
	totalImpact := 0.0

	for _, t := range threads {
		if t.ResolvedAt != nil {
			continue
		}
		totalImpact += t.RelationImpact
	}

	if totalImpact == 0 {
		return impacts
	}

	capPerDimension := 3.0
	trustDelta := round4(clampDeltaEvent(-totalImpact*0.35, capPerDimension))
	intimacyDelta := round4(clampDeltaEvent(-totalImpact*0.25, capPerDimension))
	dependencyDelta := round4(clampDeltaEvent(-totalImpact*0.20, capPerDimension))
	conflictDelta := round4(clampDeltaEvent(totalImpact*0.60, capPerDimension))
	repairDelta := round4(clampDeltaEvent(-totalImpact*0.30, capPerDimension))

	now := time.Now()
	if trustDelta != 0 {
		impacts = append(impacts, EventImpact{Dimension: "trust", Delta: trustDelta, Reason: "unresolved_threads"})
		dims.Trust.Value = round4(clamp01Scale(dims.Trust.Value+trustDelta, 0, 100))
		dims.Trust.LastUpdated = now
	}
	if intimacyDelta != 0 {
		impacts = append(impacts, EventImpact{Dimension: "intimacy", Delta: intimacyDelta, Reason: "unresolved_threads"})
		dims.Intimacy.Value = round4(clamp01Scale(dims.Intimacy.Value+intimacyDelta, 0, 100))
		dims.Intimacy.LastUpdated = now
	}
	if dependencyDelta != 0 {
		impacts = append(impacts, EventImpact{Dimension: "dependency", Delta: dependencyDelta, Reason: "unresolved_threads"})
		dims.Dependency.Value = round4(clamp01Scale(dims.Dependency.Value+dependencyDelta, 0, 100))
		dims.Dependency.LastUpdated = now
	}
	if conflictDelta != 0 {
		impacts = append(impacts, EventImpact{Dimension: "conflict", Delta: conflictDelta, Reason: "unresolved_threads"})
		dims.Conflict.Value = round4(clamp01Scale(dims.Conflict.Value+conflictDelta, 0, 100))
		dims.Conflict.LastUpdated = now
	}
	if repairDelta != 0 {
		impacts = append(impacts, EventImpact{Dimension: "repair", Delta: repairDelta, Reason: "unresolved_threads"})
		dims.Repair.Value = round4(clamp01Scale(dims.Repair.Value+repairDelta, 0, 100))
		dims.Repair.LastUpdated = now
	}

	return impacts
}

func ProcessUnresolvedThreads(threads []UnresolvedThread, config UnresolvedConfig, dims *RelationshipDimensions, now time.Time) ([]UnresolvedThread, []EventImpact) {
	processed := make([]UnresolvedThread, 0, len(threads))

	for _, t := range threads {
		if t.ResolvedAt != nil {
			processed = append(processed, t)
			continue
		}

		hoursSinceCreate := now.Sub(t.CreatedAt).Hours()
		if hoursSinceCreate > config.BaseEscalationHours*2 && t.EscalationLevel == 0 {
			EscalateUnresolved(&t, config, now)
		}
		DecayUnresolved(&t, hoursSinceCreate*0.1)

		if t.Severity < config.ResolutionThreshold && t.EscalationLevel == 0 {
			ResolveUnresolved(&t)
		}
		processed = append(processed, t)
	}

	impacts := ApplyUnresolvedImpact(dims, processed)
	return processed, impacts
}

var threadIDCounter int

func generateThreadID() string {
	threadIDCounter++
	return "unresolved-" + itoa(threadIDCounter)
}

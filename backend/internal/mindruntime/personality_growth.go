package mindruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"
)

type PersonalityParameter struct {
	Name        string
	Current     float64
	Min         float64
	Max         float64
	Target      float64
	Velocity    float64
}

type PersonalityGrowthConfig struct {
	Enabled             bool
	GrowthRate          float64
	MessageInterval     int
	DecayFactor         float64
	MaxTotalChange      float64
	MinChangePerCycle   float64
	SmoothingWindow     int
	ParameterCount      int
}

func DefaultPersonalityGrowthConfig() PersonalityGrowthConfig {
	return PersonalityGrowthConfig{
		Enabled:            true,
		GrowthRate:         0.001,
		MessageInterval:    200,
		DecayFactor:        0.95,
		MaxTotalChange:     0.3,
		MinChangePerCycle:  0.0001,
		SmoothingWindow:    10,
		ParameterCount:     5,
	}
}

type GrowthTracker struct {
	CharacterID     string
	MessageCount    int
	LastGrowthAt    time.Time
	Parameters      []PersonalityParameter
	GrowthHistory   []GrowthRecord
	Enabled         bool
	TotalChange     float64
	CyclesCompleted int
}

type GrowthRecord struct {
	ID             string
	CycleAt        time.Time
	MessageCount   int
	ParameterDeltas []ParameterDelta
	TotalDelta     float64
}

type ParameterDelta struct {
	Name  string
	Old   float64
	New   float64
	Delta float64
}

func NewGrowthTracker(characterID string, config PersonalityGrowthConfig) GrowthTracker {
	params := []PersonalityParameter{
		{Name: "expressiveness", Current: 0.5, Min: 0.1, Max: 0.9, Target: 0.55, Velocity: 0},
		{Name: "formality", Current: 0.5, Min: 0.1, Max: 0.9, Target: 0.45, Velocity: 0},
		{Name: "empathy", Current: 0.5, Min: 0.1, Max: 0.9, Target: 0.52, Velocity: 0},
		{Name: "curiosity", Current: 0.5, Min: 0.1, Max: 0.9, Target: 0.48, Velocity: 0},
		{Name: "assertiveness", Current: 0.5, Min: 0.1, Max: 0.9, Target: 0.5, Velocity: 0},
	}
	if config.ParameterCount > 0 && config.ParameterCount < len(params) {
		params = params[:config.ParameterCount]
	}
	return GrowthTracker{
		CharacterID:   strings.TrimSpace(characterID),
		Parameters:    params,
		GrowthHistory: make([]GrowthRecord, 0),
		Enabled:       config.Enabled,
	}
}

func (g *GrowthTracker) RecordMessages(count int, config PersonalityGrowthConfig) ([]ParameterDelta, bool) {
	if !g.Enabled || !config.Enabled {
		return nil, false
	}
	g.MessageCount += count
	if g.MessageCount < config.MessageInterval {
		return nil, false
	}
	now := time.Now().UTC()
	cycles := g.MessageCount / config.MessageInterval
	deltas := make([]ParameterDelta, 0)
	totalDelta := 0.0
	updatedParams := make([]PersonalityParameter, len(g.Parameters))
	copy(updatedParams, g.Parameters)
	for cycle := 0; cycle < cycles; cycle++ {
		rawGrowth := config.GrowthRate * float64(cycle+1)
		decayedGrowth := rawGrowth * math.Pow(config.DecayFactor, float64(g.CyclesCompleted+cycle))
		if decayedGrowth < config.MinChangePerCycle {
			continue
		}
		maxRemaining := config.MaxTotalChange - g.TotalChange
		if decayedGrowth > maxRemaining {
			decayedGrowth = maxRemaining
		}
		if decayedGrowth <= 0 {
			break
		}
		for i := range updatedParams {
			p := &updatedParams[i]
			delta := (p.Target - p.Current) * decayedGrowth
			newVal := p.Current + delta
			if newVal < p.Min {
				newVal = p.Min
			}
			if newVal > p.Max {
				newVal = p.Max
			}
			actualDelta := newVal - p.Current
			if math.Abs(actualDelta) >= config.MinChangePerCycle {
				deltas = append(deltas, ParameterDelta{
					Name:  p.Name,
					Old:   p.Current,
					New:   newVal,
					Delta: actualDelta,
				})
				totalDelta += math.Abs(actualDelta)
				p.Current = newVal
				p.Velocity = actualDelta
			}
		}
	}
	if len(deltas) == 0 {
		return nil, false
	}
	g.Parameters = updatedParams
	g.TotalChange += totalDelta
	g.MessageCount = g.MessageCount % config.MessageInterval
	g.CyclesCompleted += cycles
	g.LastGrowthAt = now
	record := GrowthRecord{
		ID:              growthRecordID(g.CharacterID, now),
		CycleAt:         now,
		MessageCount:    g.MessageCount + g.CyclesCompleted*config.MessageInterval,
		ParameterDeltas: deltas,
		TotalDelta:      totalDelta,
	}
	g.GrowthHistory = append(g.GrowthHistory, record)
	return deltas, true
}

func (g *GrowthTracker) GetParameter(name string) (PersonalityParameter, bool) {
	for _, p := range g.Parameters {
		if p.Name == name {
			return p, true
		}
	}
	return PersonalityParameter{}, false
}

func (g *GrowthTracker) GetAllParameters() []PersonalityParameter {
	result := make([]PersonalityParameter, len(g.Parameters))
	copy(result, g.Parameters)
	return result
}

func (g *GrowthTracker) GetTotalChange() float64 {
	return g.TotalChange
}

func (g *GrowthTracker) IsAtTarget(config PersonalityGrowthConfig) bool {
	if !config.Enabled {
		return true
	}
	tolerance := config.MinChangePerCycle * 10
	for _, p := range g.Parameters {
		if math.Abs(p.Target-p.Current) > tolerance {
			return false
		}
	}
	return true
}

func (g *GrowthTracker) Disable() {
	g.Enabled = false
}

func (g *GrowthTracker) Enable() {
	g.Enabled = true
}

func (g *GrowthTracker) TotalMessages() int {
	if g.CyclesCompleted == 0 {
		return g.MessageCount
	}
	return g.MessageCount + g.CyclesCompleted*g.MessageCount
}

func growthRecordID(characterID string, now time.Time) string {
	raw := fmt.Sprintf("growth|%s|%d", characterID, now.UnixNano())
	sum := sha256.Sum256([]byte(raw))
	return "growth-" + hex.EncodeToString(sum[:])[:16]
}

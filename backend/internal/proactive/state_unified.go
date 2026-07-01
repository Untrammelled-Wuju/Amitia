package proactive

import (
	"github.com/u-ai/backend/internal/psyche"
)

type UnifiedState struct {
	Energy    float64 `json:"energy"`
	Fatigue   float64 `json:"fatigue"`
	Busy      bool    `json:"busy"`
	Replyable bool    `json:"replyable"`
}

func UnifiedStateFromRuntime(rs psyche.RuntimeState) UnifiedState {
	energy := 1.0 - (rs.Fatigue*0.6 + rs.Stress*0.3 + rs.MoodPressure*0.1)
	if energy < 0 {
		energy = 0
	}
	if energy > 1 {
		energy = 1
	}

	fatigue := rs.Fatigue*0.8 + rs.Stress*0.2
	if fatigue > 1 {
		fatigue = 1
	}

	busy := rs.SocialLoad > 0.8 || rs.Arousal > 0.9
	replyable := !busy && fatigue < 0.85 && energy > 0.15

	return UnifiedState{
		Energy:    energy,
		Fatigue:   fatigue,
		Busy:      busy,
		Replyable: replyable,
	}
}

func (us UnifiedState) EnergyPercent() int {
	return int(us.Energy * 100)
}

func (us UnifiedState) FatiguePercent() int {
	return int(us.Fatigue * 100)
}

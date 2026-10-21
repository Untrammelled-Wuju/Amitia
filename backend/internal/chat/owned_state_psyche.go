package chat

import (
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/psyche"
)

type AppraisalResultBridge struct {
	EventType         string
	PsycheDelta       float64
	RelationshipDelta float64
	Severity          float64
	EnergyDelta       float64
	StressDelta       float64
}

func computePsycheEnergyDelta(state psyche.PsycheState) float64 {
	now := time.Now().UTC()
	hour := float64(now.Hour()) + float64(now.Minute())/60.0
	timeFactor := 1.0
	if hour >= 22 || hour < 6 {
		timeFactor = 1.5
	} else if hour >= 14 && hour < 16 {
		timeFactor = 1.2
	}
	stressFactor := 1.0 + state.Stress*1.5
	baseCost := -0.03
	return baseCost * timeFactor * stressFactor
}

func buildPsycheEvent(charID string, state psyche.PsycheState, appraisal *AppraisalResultBridge) psyche.PsycheEvent {
	event := psyche.PsycheEvent{
		ID:          uuid.New().String(),
		CharacterID: charID,
		Type:        psyche.EventTypeInteraction,
		Source:      "chat.owned",
		Timestamp:   time.Now().UTC(),
	}

	event.EnergyDelta = computePsycheEnergyDelta(state)

	if appraisal == nil {
		return event
	}

	switch appraisal.EventType {
	case "praise":
		event.ValenceDelta = 0.06 + appraisal.PsycheDelta*0.15
		event.ArousalDelta = 0.04
		event.DominanceDelta = 0.04
		event.StressDelta = -0.04
		event.EnergyDelta += 0.02
	case "cold":
		event.ValenceDelta = -0.05 + appraisal.PsycheDelta*0.12
		event.ArousalDelta = -0.03
		event.DominanceDelta = -0.03
		event.StressDelta = 0.03
		event.EnergyDelta += -0.02
	case "help":
		event.ValenceDelta = 0.02
		event.ArousalDelta = 0.03
		event.DominanceDelta = 0.02
		event.StressDelta = 0.02
		event.EnergyDelta += -0.03
	case "complaint":
		event.ValenceDelta = -0.04
		event.ArousalDelta = 0.02
		event.DominanceDelta = -0.02
		event.StressDelta = 0.05
		event.EnergyDelta += -0.04
	case "boundary_cross":
		event.ValenceDelta = -0.02
		event.ArousalDelta = 0.05
		event.DominanceDelta = -0.04
		event.StressDelta = 0.06
		event.EnergyDelta += -0.02
	case "apology":
		event.ValenceDelta = 0.03
		event.ArousalDelta = -0.02
		event.DominanceDelta = 0.03
		event.StressDelta = -0.02
		event.EnergyDelta += -0.01
	case "emotional":
		event.ArousalDelta = 0.06
		event.ValenceDelta = appraisal.PsycheDelta * 0.18
		event.StressDelta = 0.03
		event.EnergyDelta += -0.03
	case "chat":
		event.ValenceDelta = 0.01
		event.EnergyDelta += -0.01
	default:
		event.EnergyDelta += -0.01
	}

	if appraisal.Severity > 0.6 {
		event.StressDelta += (appraisal.Severity - 0.6) * 0.1
		event.EnergyDelta += -(appraisal.Severity - 0.6) * 0.06
	}
	event.EnergyDelta += appraisal.EnergyDelta
	event.StressDelta += appraisal.StressDelta

	return event
}

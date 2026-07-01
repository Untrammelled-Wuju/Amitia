package decision

func ComputeAffectScore(state AffectSignalInput) float64 {
	if state.Stress > 0.7 {
		return 0.15
	}
	netEmotion := state.Positive - state.Negative
	if netEmotion < -0.3 {
		return 0.10
	}
	if netEmotion > 0.3 {
		return 0.45
	}
	if state.Stress > 0.4 {
		return 0.20
	}
	return 0.30
}

type AffectSignalInput struct {
	Positive float64
	Negative float64
	Stress   float64
}

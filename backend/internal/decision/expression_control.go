package decision

type ExpressionIntensity string

const (
	ExpressionFull       ExpressionIntensity = "full"
	ExpressionModerated  ExpressionIntensity = "moderated"
	ExpressionSuppressed ExpressionIntensity = "suppressed"
	ExpressionMinimal    ExpressionIntensity = "minimal"
)

type ExpressionControlConfig struct {
	SuppressionThreshold float64
	ModerationThreshold  float64
	SafetyCeiling        float64
}

func DefaultExpressionControlConfig() ExpressionControlConfig {
	return ExpressionControlConfig{
		SuppressionThreshold: 0.85,
		ModerationThreshold:  0.60,
		SafetyCeiling:        0.75,
	}
}

type ExpressionControlInput struct {
	EmotionIntensity   float64
	RiskScore          float64
	RelationshipSafety float64
	StressLevel        float64
}

type ExpressionControlResult struct {
	Intensity   ExpressionIntensity
	ScaleFactor float64
	Suppressed  bool
	Reason      string
}

func ControlExpression(input ExpressionControlInput, config ExpressionControlConfig) ExpressionControlResult {
	if input.EmotionIntensity < 0 {
		input.EmotionIntensity = 0
	}
	if input.EmotionIntensity > 1 {
		input.EmotionIntensity = 1
	}
	if input.RiskScore > config.SuppressionThreshold || input.EmotionIntensity > config.SuppressionThreshold {
		if input.RelationshipSafety < 0.3 {
			return ExpressionControlResult{
				Intensity:   ExpressionSuppressed,
				ScaleFactor: 0.1,
				Suppressed:  true,
				Reason:      "high_risk_low_safety",
			}
		}
		return ExpressionControlResult{
			Intensity:   ExpressionSuppressed,
			ScaleFactor: 0.3,
			Suppressed:  true,
			Reason:      "above_suppression_threshold",
		}
	}
	if input.RiskScore > config.ModerationThreshold {
		return ExpressionControlResult{
			Intensity:   ExpressionModerated,
			ScaleFactor: 0.55,
			Suppressed:  false,
			Reason:      "moderate_risk",
		}
	}
	if input.EmotionIntensity > config.ModerationThreshold {
		if input.StressLevel > 0.7 {
			return ExpressionControlResult{
				Intensity:   ExpressionModerated,
				ScaleFactor: 0.50,
				Suppressed:  false,
				Reason:      "high_emotion_high_stress",
			}
		}
		return ExpressionControlResult{
			Intensity:   ExpressionFull,
			ScaleFactor: 0.85,
			Suppressed:  false,
			Reason:      "high_emotion_low_stress",
		}
	}
	return ExpressionControlResult{
		Intensity:   ExpressionFull,
		ScaleFactor: 1.0,
		Suppressed:  false,
		Reason:      "normal",
	}
}

func ComputeExpressionScaleFactor(input ExpressionControlInput, config ExpressionControlConfig) float64 {
	result := ControlExpression(input, config)
	return result.ScaleFactor
}

func IsExpressionSuppressed(input ExpressionControlInput, config ExpressionControlConfig) bool {
	result := ControlExpression(input, config)
	return result.Suppressed
}

func ClampEmotionIntensity(intensity float64, ceiling float64) float64 {
	if ceiling <= 0 {
		ceiling = 0.75
	}
	if intensity > ceiling {
		return ceiling
	}
	if intensity < 0 {
		return 0
	}
	return intensity
}

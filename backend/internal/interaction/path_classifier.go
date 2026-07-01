package interaction

type PathType string

const (
	PathTypeFast     PathType = "fast"
	PathTypeStandard PathType = "standard"
	PathTypeDeep     PathType = "deep"
)

type PathClassifyInput struct {
	MessageContent string
	RoleState      PsycheState
	Urgency        int
	Attachments    int
	IsEmotional    bool
	HasCommands    bool
	PreviousPath   PathType
	MessageLength  int
}

type PathClassifier struct {
	DeepThreshold      float64
	StandardThreshold  float64
	FastUpperBound     float64
	EmotionalBias      float64
	UrgencyBias        float64
	ComplexityBias     float64
	AttachmentBias     float64
	CommandBias        float64
	FatigueWeight      float64
	StressWeight       float64
	PreviousPathWeight float64
}

func NewPathClassifier() *PathClassifier {
	return &PathClassifier{
		DeepThreshold:      0.60,
		StandardThreshold:  0.20,
		FastUpperBound:     0.35,
		EmotionalBias:      0.25,
		UrgencyBias:        0.20,
		ComplexityBias:     0.15,
		AttachmentBias:     0.10,
		CommandBias:        0.05,
		FatigueWeight:      0.10,
		StressWeight:       0.15,
		PreviousPathWeight: 0.05,
	}
}

func (c *PathClassifier) Classify(input PathClassifyInput) PathType {
	score := c.computeDepthScore(input)

	if c.PreviousPathWeight > 0 && input.PreviousPath != "" {
		score += c.pathTypeScore(input.PreviousPath) * c.PreviousPathWeight
	}

	if score >= c.DeepThreshold {
		return PathTypeDeep
	}
	if score >= c.StandardThreshold {
		return PathTypeStandard
	}
	return PathTypeFast
}

func (c *PathClassifier) computeDepthScore(input PathClassifyInput) float64 {
	score := 0.0

	if input.IsEmotional {
		score += c.EmotionalBias
	}

	urgencyScore := clamp01(float64(input.Urgency) / 10.0)
	score += c.UrgencyBias * urgencyScore

	complexityScore := complexityScore(input.MessageContent)
	score += c.ComplexityBias * complexityScore

	if input.Attachments > 0 {
		score += c.AttachmentBias * clamp01(float64(input.Attachments)/5.0)
	}

	if input.HasCommands {
		score += c.CommandBias
	}

	psycheScore := c.psycheDepthScore(input.RoleState)
	score += psycheScore

	return score
}

func (c *PathClassifier) psycheDepthScore(state PsycheState) float64 {
	fatigueScore := state.Fatigue * c.FatigueWeight
	stressScore := state.Stress * c.StressWeight
	arousalScore := (1.0 - state.Arousal) * 0.05
	socialScore := state.SocialLoad * 0.05
	return fatigueScore + stressScore + arousalScore + socialScore
}

func (c *PathClassifier) pathTypeScore(pt PathType) float64 {
	switch pt {
	case PathTypeDeep:
		return 0.1
	case PathTypeStandard:
		return 0.05
	case PathTypeFast:
		return -0.05
	default:
		return 0
	}
}

func complexityScore(content string) float64 {
	runes := []rune(content)
	length := len(runes)
	if length == 0 {
		return 0
	}
	lengthScore := clamp01(float64(length) / 500.0)

	sentenceCount := 0
	questionCount := 0
	exclamationCount := 0
	for _, r := range runes {
		switch r {
		case '。', '.', '\n':
			sentenceCount++
		case '？', '?':
			questionCount++
		case '！', '!':
			exclamationCount++
		}
	}

	structureScore := clamp01(float64(sentenceCount+questionCount+exclamationCount) / 10.0)

	return 0.6*lengthScore + 0.4*structureScore
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

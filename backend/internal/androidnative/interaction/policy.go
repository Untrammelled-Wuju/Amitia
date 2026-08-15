package interaction

import "time"

const (
	DefaultMaxGestureDuration    = 3 * time.Second
	DefaultMaxVisualLocateDuration = 10 * time.Second
	DefaultMaxVisualCandidates   = 10
	DefaultMaxScreenshotAge      = 1 * time.Second

	DefaultMinOCRConfidence      = 0.7
	DefaultMinVisionConfidence   = 0.85

	DefaultAllowCoordinateFallback = true
	DefaultAllowShizukuFallback    = true
	DefaultAllowVisualFallback     = true
	DefaultAllowRootFallback       = false
	DefaultAllowADBFallback        = false

	DefaultNodeClickTimeoutMS    = 5000
	DefaultInputTimeoutMS        = 5000
	DefaultGestureTimeoutMS      = 5000
	DefaultVisualLocateTimeoutMS = 10000
	DefaultVisualClickTimeoutMS  = 12000
	DefaultVerificationTimeoutMS = 3000

	DefaultMaxConcurrentSideEffectsPerDisplay = 1
)

type Policy struct {
	MaxGestureDuration time.Duration

	MaxVisualLocateDuration time.Duration

	MaxVisualCandidates int

	MaxScreenshotAge time.Duration

	MinOCRConfidence float64
	MinVisionConfidence float64

	AllowCoordinateFallback bool
	AllowShizukuFallback    bool
	AllowVisualFallback     bool
	AllowRootFallback       bool
	AllowADBFallback        bool
}

func DefaultPolicy() Policy {
	return Policy{
		MaxGestureDuration:    DefaultMaxGestureDuration,
		MaxVisualLocateDuration: DefaultMaxVisualLocateDuration,
		MaxVisualCandidates:   DefaultMaxVisualCandidates,
		MaxScreenshotAge:      DefaultMaxScreenshotAge,

		MinOCRConfidence:    DefaultMinOCRConfidence,
		MinVisionConfidence: DefaultMinVisionConfidence,

		AllowCoordinateFallback: DefaultAllowCoordinateFallback,
		AllowShizukuFallback:    DefaultAllowShizukuFallback,
		AllowVisualFallback:     DefaultAllowVisualFallback,
		AllowRootFallback:       DefaultAllowRootFallback,
		AllowADBFallback:        DefaultAllowADBFallback,
	}
}

func (p *Policy) ValidateDurationMS(durationMS int, minMS, maxMS int) int {
	if durationMS <= 0 {
		return minMS
	}
	if durationMS < minMS {
		return minMS
	}
	if durationMS > maxMS {
		return maxMS
	}
	return durationMS
}

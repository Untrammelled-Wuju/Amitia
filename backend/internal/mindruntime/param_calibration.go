package mindruntime

import (
	"time"
)

type CalibratedParams struct {
	DecayRate            float64       `json:"decayRate"`
	ChangeBudget         float64       `json:"changeBudget"`
	RelationshipSpeed    float64       `json:"relationshipSpeed"`
	ProactiveThreshold   float64       `json:"proactiveThreshold"`
	QueueConcurrency     int           `json:"queueConcurrency"`
	BackpressureThreshold int          `json:"backpressureThreshold"`
	CircuitBreakerWindow time.Duration `json:"circuitBreakerWindow"`
	ReflectionThreshold  float64       `json:"reflectionThreshold"`
	CalibratedAt         time.Time     `json:"calibratedAt"`
	Version              int           `json:"version"`
	SourceObservedCount  int           `json:"sourceObservedCount"`
}

type CalibrationInput struct {
	ObservedMessageCount int
	ObservedPeakMessages int
	ObservedDuration     time.Duration
	ActiveRoleCount      int
	DefaultConfig        CalibrationConfig
}

type CalibrationConfig struct {
	MinDecayRate            float64
	MaxDecayRate            float64
	MinChangeBudget         float64
	MaxChangeBudget         float64
	MinRelationshipSpeed    float64
	MaxRelationshipSpeed    float64
	MinProactiveThreshold   float64
	MaxProactiveThreshold   float64
	MinQueueConcurrency     int
	MaxQueueConcurrency     int
	MinBackpressureThreshold int
	MaxBackpressureThreshold int
	MinCircuitBreakerWindow time.Duration
	MaxCircuitBreakerWindow time.Duration
	MinReflectionThreshold  float64
	MaxReflectionThreshold  float64
}

func DefaultCalibrationConfig() CalibrationConfig {
	return CalibrationConfig{
		MinDecayRate:            0.01,
		MaxDecayRate:            0.10,
		MinChangeBudget:         0.05,
		MaxChangeBudget:         0.50,
		MinRelationshipSpeed:    0.01,
		MaxRelationshipSpeed:    0.20,
		MinProactiveThreshold:   0.30,
		MaxProactiveThreshold:   0.80,
		MinQueueConcurrency:     1,
		MaxQueueConcurrency:     50,
		MinBackpressureThreshold: 10,
		MaxBackpressureThreshold: 1000,
		MinCircuitBreakerWindow: 5 * time.Second,
		MaxCircuitBreakerWindow: 300 * time.Second,
		MinReflectionThreshold:  0.10,
		MaxReflectionThreshold:  0.70,
	}
}

func CalibrateParameters(input CalibrationInput) CalibratedParams {
	cfg := input.DefaultConfig

	msgRate := safeDiv(float64(input.ObservedMessageCount), input.ObservedDuration.Seconds())
	if msgRate < 1 {
		msgRate = 1
	}

	roleFactor := float64(input.ActiveRoleCount)
	if roleFactor < 1 {
		roleFactor = 1
	}

	peakRatio := safeDiv(float64(input.ObservedPeakMessages), msgRate*60)
	if peakRatio < 1 {
		peakRatio = 1
	}

	decayRate := clampFloat64(cfg.MinDecayRate+msgRate*0.0001, cfg.MinDecayRate, cfg.MaxDecayRate)
	changeBudget := clampFloat64(0.1+roleFactor*0.02, cfg.MinChangeBudget, cfg.MaxChangeBudget)
	relationshipSpeed := clampFloat64(0.02+msgRate*0.00005, cfg.MinRelationshipSpeed, cfg.MaxRelationshipSpeed)
	proactiveThreshold := clampFloat64(0.5-roleFactor*0.02, cfg.MinProactiveThreshold, cfg.MaxProactiveThreshold)

	qConcurrency := clampInt(5+int(msgRate/2), cfg.MinQueueConcurrency, cfg.MaxQueueConcurrency)
	backpressure := clampInt(50+int(msgRate*2), cfg.MinBackpressureThreshold, cfg.MaxBackpressureThreshold)
	cbWindow := clampDuration(time.Duration(10+int(peakRatio*10))*time.Second, cfg.MinCircuitBreakerWindow, cfg.MaxCircuitBreakerWindow)
	reflectionThreshold := clampFloat64(0.3+roleFactor*0.03, cfg.MinReflectionThreshold, cfg.MaxReflectionThreshold)

	params := CalibratedParams{
		DecayRate:             decayRate,
		ChangeBudget:          changeBudget,
		RelationshipSpeed:     relationshipSpeed,
		ProactiveThreshold:    proactiveThreshold,
		QueueConcurrency:      qConcurrency,
		BackpressureThreshold: backpressure,
		CircuitBreakerWindow:  cbWindow,
		ReflectionThreshold:   reflectionThreshold,
		CalibratedAt:          time.Now().UTC(),
		Version:               1,
		SourceObservedCount:   input.ObservedMessageCount,
	}

	return params
}

func RecalibrateWithNewData(previous CalibratedParams, input CalibrationInput) CalibratedParams {
	newParams := CalibrateParameters(input)

	newParams.DecayRate = smoothParam(previous.DecayRate, newParams.DecayRate, 0.3)
	newParams.ChangeBudget = smoothParam(previous.ChangeBudget, newParams.ChangeBudget, 0.3)
	newParams.RelationshipSpeed = smoothParam(previous.RelationshipSpeed, newParams.RelationshipSpeed, 0.3)
	newParams.ProactiveThreshold = smoothParam(previous.ProactiveThreshold, newParams.ProactiveThreshold, 0.3)
	newParams.ReflectionThreshold = smoothParam(previous.ReflectionThreshold, newParams.ReflectionThreshold, 0.3)
	newParams.Version = previous.Version + 1

	return newParams
}

func ValidateParams(params CalibratedParams, config CalibrationConfig) bool {
	if params.DecayRate < config.MinDecayRate || params.DecayRate > config.MaxDecayRate {
		return false
	}
	if params.ChangeBudget < config.MinChangeBudget || params.ChangeBudget > config.MaxChangeBudget {
		return false
	}
	if params.RelationshipSpeed < config.MinRelationshipSpeed || params.RelationshipSpeed > config.MaxRelationshipSpeed {
		return false
	}
	if params.ProactiveThreshold < config.MinProactiveThreshold || params.ProactiveThreshold > config.MaxProactiveThreshold {
		return false
	}
	if params.QueueConcurrency < config.MinQueueConcurrency || params.QueueConcurrency > config.MaxQueueConcurrency {
		return false
	}
	if params.BackpressureThreshold < config.MinBackpressureThreshold || params.BackpressureThreshold > config.MaxBackpressureThreshold {
		return false
	}
	if params.CircuitBreakerWindow < config.MinCircuitBreakerWindow || params.CircuitBreakerWindow > config.MaxCircuitBreakerWindow {
		return false
	}
	if params.ReflectionThreshold < config.MinReflectionThreshold || params.ReflectionThreshold > config.MaxReflectionThreshold {
		return false
	}
	return true
}

func clampFloat64(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func clampInt(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func clampDuration(val, min, max time.Duration) time.Duration {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func smoothParam(prev, next, weight float64) float64 {
	return prev*(1-weight) + next*weight
}

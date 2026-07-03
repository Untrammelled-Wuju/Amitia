package mindruntime

import (
	"testing"
	"time"
)

func TestCalibrateParameters(t *testing.T) {
	input := CalibrationInput{
		ObservedMessageCount: 1000,
		ObservedPeakMessages: 100,
		ObservedDuration:     24 * time.Hour,
		ActiveRoleCount:      3,
		DefaultConfig:        DefaultCalibrationConfig(),
	}

	params := CalibrateParameters(input)

	if params.DecayRate <= 0 || params.DecayRate > 0.10 {
		t.Fatalf("decay rate out of range: %f", params.DecayRate)
	}
	if params.ChangeBudget <= 0 || params.ChangeBudget > 0.50 {
		t.Fatalf("change budget out of range: %f", params.ChangeBudget)
	}
	if params.RelationshipSpeed <= 0 || params.RelationshipSpeed > 0.20 {
		t.Fatalf("relationship speed out of range: %f", params.RelationshipSpeed)
	}
	if params.ProactiveThreshold < 0.30 || params.ProactiveThreshold > 0.80 {
		t.Fatalf("proactive threshold out of range: %f", params.ProactiveThreshold)
	}
	if params.QueueConcurrency < 1 || params.QueueConcurrency > 50 {
		t.Fatalf("queue concurrency out of range: %d", params.QueueConcurrency)
	}
	if params.BackpressureThreshold < 10 || params.BackpressureThreshold > 1000 {
		t.Fatalf("backpressure threshold out of range: %d", params.BackpressureThreshold)
	}
	if params.CircuitBreakerWindow < 5*time.Second || params.CircuitBreakerWindow > 300*time.Second {
		t.Fatalf("circuit breaker window out of range: %v", params.CircuitBreakerWindow)
	}
	if params.ReflectionThreshold < 0.10 || params.ReflectionThreshold > 0.70 {
		t.Fatalf("reflection threshold out of range: %f", params.ReflectionThreshold)
	}
	if params.Version != 1 {
		t.Fatalf("expected version 1, got %d", params.Version)
	}
}

func TestRecalibrateWithNewData(t *testing.T) {
	input1 := CalibrationInput{
		ObservedMessageCount: 500,
		ObservedPeakMessages: 50,
		ObservedDuration:     12 * time.Hour,
		ActiveRoleCount:      2,
		DefaultConfig:        DefaultCalibrationConfig(),
	}
	params1 := CalibrateParameters(input1)

	input2 := CalibrationInput{
		ObservedMessageCount: 2000,
		ObservedPeakMessages: 200,
		ObservedDuration:     24 * time.Hour,
		ActiveRoleCount:      5,
		DefaultConfig:        DefaultCalibrationConfig(),
	}
	params2 := RecalibrateWithNewData(params1, input2)

	if params2.Version != 2 {
		t.Fatalf("expected version 2, got %d", params2.Version)
	}
}

func TestValidateParams(t *testing.T) {
	config := DefaultCalibrationConfig()
	validParams := CalibratedParams{
		DecayRate:             0.05,
		ChangeBudget:          0.20,
		RelationshipSpeed:     0.05,
		ProactiveThreshold:    0.50,
		QueueConcurrency:      10,
		BackpressureThreshold: 100,
		CircuitBreakerWindow:  30 * time.Second,
		ReflectionThreshold:   0.30,
	}

	if !ValidateParams(validParams, config) {
		t.Fatal("expected valid params to pass")
	}

	invalidParams := validParams
	invalidParams.DecayRate = 999.0
	if ValidateParams(invalidParams, config) {
		t.Fatal("expected invalid params to fail")
	}
}

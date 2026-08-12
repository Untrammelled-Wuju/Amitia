package interaction

import (
	"testing"
	"time"
)

func TestDefaultPolicy(t *testing.T) {
	policy := DefaultPolicy()

	if policy.MaxGestureDuration != DefaultMaxGestureDuration {
		t.Fatalf("expected MaxGestureDuration %v, got %v", DefaultMaxGestureDuration, policy.MaxGestureDuration)
	}
	if policy.MaxVisualLocateDuration != DefaultMaxVisualLocateDuration {
		t.Fatalf("expected MaxVisualLocateDuration %v, got %v", DefaultMaxVisualLocateDuration, policy.MaxVisualLocateDuration)
	}
	if policy.MaxVisualCandidates != DefaultMaxVisualCandidates {
		t.Fatalf("expected MaxVisualCandidates %d, got %d", DefaultMaxVisualCandidates, policy.MaxVisualCandidates)
	}
	if policy.MinOCRConfidence != DefaultMinOCRConfidence {
		t.Fatalf("expected MinOCRConfidence %f, got %f", DefaultMinOCRConfidence, policy.MinOCRConfidence)
	}
	if policy.MinVisionConfidence != DefaultMinVisionConfidence {
		t.Fatalf("expected MinVisionConfidence %f, got %f", DefaultMinVisionConfidence, policy.MinVisionConfidence)
	}
	if !policy.AllowCoordinateFallback {
		t.Fatal("expected AllowCoordinateFallback to be true by default")
	}
	if !policy.AllowVisualFallback {
		t.Fatal("expected AllowVisualFallback to be true by default")
	}
	if policy.AllowRootFallback {
		t.Fatal("expected AllowRootFallback to be false by default")
	}
	if policy.AllowADBFallback {
		t.Fatal("expected AllowADBFallback to be false by default")
	}
}

func TestPolicy_ValidateDurationMS(t *testing.T) {
	policy := DefaultPolicy()

	tests := []struct {
		name      string
		duration  int
		minMS     int
		maxMS     int
		expected  int
	}{
		{"zero returns min", 0, 100, 1000, 100},
		{"below min returns min", 50, 100, 1000, 100},
		{"above max returns max", 5000, 100, 1000, 1000},
		{"within range unchanged", 500, 100, 1000, 500},
		{"at min boundary", 100, 100, 1000, 100},
		{"at max boundary", 1000, 100, 1000, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := policy.ValidateDurationMS(tt.duration, tt.minMS, tt.maxMS)
			if got != tt.expected {
				t.Fatalf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestDefaultConstants(t *testing.T) {
	if DefaultMaxGestureDuration != 3*time.Second {
		t.Fatalf("expected 3s, got %v", DefaultMaxGestureDuration)
	}
	if DefaultMaxVisualLocateDuration != 10*time.Second {
		t.Fatalf("expected 10s, got %v", DefaultMaxVisualLocateDuration)
	}
	if DefaultMaxVisualCandidates != 10 {
		t.Fatalf("expected 10, got %d", DefaultMaxVisualCandidates)
	}
	if DefaultMaxScreenshotAge != 1*time.Second {
		t.Fatalf("expected 1s, got %v", DefaultMaxScreenshotAge)
	}
	if DefaultMinOCRConfidence != 0.7 {
		t.Fatalf("expected 0.7, got %f", DefaultMinOCRConfidence)
	}
	if DefaultMinVisionConfidence != 0.85 {
		t.Fatalf("expected 0.85, got %f", DefaultMinVisionConfidence)
	}
	if DefaultLongPressDurationMS != 600 {
		t.Fatalf("expected 600, got %d", DefaultLongPressDurationMS)
	}
	if MinLongPressDurationMS != 300 {
		t.Fatalf("expected 300, got %d", MinLongPressDurationMS)
	}
	if MaxLongPressDurationMS != 3000 {
		t.Fatalf("expected 3000, got %d", MaxLongPressDurationMS)
	}
	if DefaultSwipeDurationMS != 300 {
		t.Fatalf("expected 300, got %d", DefaultSwipeDurationMS)
	}
	if MinSwipeDurationMS != 100 {
		t.Fatalf("expected 100, got %d", MinSwipeDurationMS)
	}
	if MaxSwipeDurationMS != 3000 {
		t.Fatalf("expected 3000, got %d", MaxSwipeDurationMS)
	}
	if MaxInputTextRunes != 10000 {
		t.Fatalf("expected 10000, got %d", MaxInputTextRunes)
	}
}

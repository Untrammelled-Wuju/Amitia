package affect

import (
	"testing"
	"time"
)

type mockRepo struct {
	saved *AffectState
}

func (m *mockRepo) LoadState(characterID string) (*AffectState, error) {
	return nil, nil
}

func (m *mockRepo) SaveState(characterID string, state AffectState) error {
	m.saved = &state
	return nil
}

func TestServiceProcessEventCreatesAndSavesState(t *testing.T) {
	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	repo := &mockRepo{}
	svc := NewService(repo)

	output, err := svc.ProcessEvent("char-1", EngineInput{
		Now: now,
		Appraisal: EventAppraisal{
			EventID: "evt-test",
			Valence: 0.6,
		},
	})
	if err != nil {
		t.Fatalf("process event: %v", err)
	}
	if output.State.Version != StateVersionV1 {
		t.Fatalf("unexpected state version: %v", output.State.Version)
	}
	if output.State.Emotion.Positive <= 0 {
		t.Fatalf("expected positive emotion shift: %#v", output.State.Emotion)
	}
	if repo.saved == nil {
		t.Fatal("expected state to be saved")
	}
	if repo.saved.Emotion.Positive != output.State.Emotion.Positive {
		t.Fatalf("saved state mismatch: %#v vs %#v", repo.saved.Emotion, output.State.Emotion)
	}
}

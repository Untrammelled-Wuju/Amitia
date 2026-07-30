package wiring

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
)

type ActivityAdapter struct {
	clock behavior.Clock
}

func NewActivityAdapter(clock behavior.Clock) *ActivityAdapter {
	if clock == nil {
		clock = behavior.NewRealClock()
	}
	return &ActivityAdapter{clock: clock}
}

func (a *ActivityAdapter) GetActivitySnapshot(ctx context.Context, userID, characterID string) (*behavior.ActivityBehaviorSnapshot, error) {
	now := a.clock.Now()
	hour := now.Hour()

	activityKey, source := inferTimePeriodActivity(hour)

	return &behavior.ActivityBehaviorSnapshot{
		ActivityKey: activityKey,
		Source:      source,
		Confidence:  0.6,
		StartedAt:   truncateToHour(now),
		Version:     "time-v1",
	}, nil
}

func inferTimePeriodActivity(hour int) (string, string) {
	switch {
	case hour >= 0 && hour < 6:
		return "sleeping", "time_inference"
	case hour >= 6 && hour < 9:
		return "morning_routine", "time_inference"
	case hour >= 9 && hour < 12:
		return "working", "time_inference"
	case hour >= 12 && hour < 14:
		return "lunch", "time_inference"
	case hour >= 14 && hour < 18:
		return "working", "time_inference"
	case hour >= 18 && hour < 22:
		return "leisure", "time_inference"
	default:
		return "relaxing", "time_inference"
	}
}

func truncateToHour(t time.Time) time.Time {
	return t.Truncate(time.Hour)
}

var _ behavior.CharacterActivityPort = (*ActivityAdapter)(nil)

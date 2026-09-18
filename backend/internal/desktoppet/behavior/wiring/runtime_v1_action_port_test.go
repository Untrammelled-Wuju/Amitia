package wiring

import (
	"testing"

	runtimev1 "github.com/u-ai/backend/internal/desktoppet/runtime/protocol/v1"
)

func TestRuntimePlaybackStatusOccupiesForeground(t *testing.T) {
	active := []string{
		runtimev1.PlaybackStatusPlaying,
		runtimev1.PlaybackStatusHolding,
		runtimev1.PlaybackStatusPaused,
	}
	for _, status := range active {
		if !runtimePlaybackStatusOccupiesForeground(status) {
			t.Fatalf("status %q must occupy foreground", status)
		}
	}

	inactive := []string{
		"",
		runtimev1.PlaybackStatusIdle,
		runtimev1.PlaybackStatusLoading,
		runtimev1.PlaybackStatusStopped,
		runtimev1.PlaybackStatusFailed,
	}
	for _, status := range inactive {
		if runtimePlaybackStatusOccupiesForeground(status) {
			t.Fatalf("status %q must not preserve stale foreground", status)
		}
	}
}

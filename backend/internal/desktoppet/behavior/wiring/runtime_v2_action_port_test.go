package wiring

import (
	"testing"

	runtimev2 "github.com/u-ai/backend/internal/desktoppet/runtime/protocol/v2"
)

func TestRuntimePlaybackStatusOccupiesForeground(t *testing.T) {
	active := []string{
		runtimev2.PlaybackStatusPlaying,
		runtimev2.PlaybackStatusHolding,
		runtimev2.PlaybackStatusPaused,
	}
	for _, status := range active {
		if !runtimePlaybackStatusOccupiesForeground(status) {
			t.Fatalf("status %q must occupy foreground", status)
		}
	}

	inactive := []string{
		"",
		runtimev2.PlaybackStatusIdle,
		runtimev2.PlaybackStatusLoading,
		runtimev2.PlaybackStatusStopped,
		runtimev2.PlaybackStatusFailed,
	}
	for _, status := range inactive {
		if runtimePlaybackStatusOccupiesForeground(status) {
			t.Fatalf("status %q must not preserve stale foreground", status)
		}
	}
}

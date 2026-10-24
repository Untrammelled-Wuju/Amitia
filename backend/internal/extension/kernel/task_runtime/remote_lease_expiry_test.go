package task_runtime

import (
	"testing"
	"time"
)

func TestRemoteLeaseIsCurrentAtBoundary(t *testing.T) {
	now := time.Date(2026, time.October, 23, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Nanosecond)
	past := now.Add(-time.Nanosecond)
	for _, tc := range []struct {
		name    string
		expires *time.Time
		want    bool
	}{
		{name: "missing", expires: nil, want: false},
		{name: "expired", expires: &past, want: false},
		{name: "boundary", expires: &now, want: false},
		{name: "live", expires: &future, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := remoteLeaseIsCurrent(tc.expires, now); got != tc.want {
				t.Fatalf("remote lease validity=%v, want %v", got, tc.want)
			}
		})
	}
}

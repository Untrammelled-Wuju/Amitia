package task_runtime

import "testing"

func TestRound101OwnerInitialCheckpointGuard(t *testing.T) {
	cases := []struct {
		name            string
		hasCursor       bool
		localCheckpoint bool
		version         int64
		want            bool
	}{
		{"initial_first", false, false, 1, true},
		{"initial_second", false, false, 2, false},
		{"initial_later", false, false, 100, false},
		{"initial_zero", false, false, 0, false},
		{"legacy_local_checkpoint", false, true, 2, false},
		{"legacy_local_first", false, true, 1, false},
		{"existing_cursor_successor", true, true, 2, true},
		{"existing_cursor_initial", true, false, 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ownerInitialCheckpointAllowed(tc.hasCursor, tc.localCheckpoint, tc.version)
			if got != tc.want {
				t.Fatalf("initial checkpoint allowed=%v want=%v", got, tc.want)
			}
		})
	}
}

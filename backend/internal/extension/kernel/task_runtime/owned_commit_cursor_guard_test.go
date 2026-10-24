package task_runtime

import "testing"

func TestRound100AuthoritativeCommitGuards(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		version, committedVersion int64
		id, committedID           string
		want                      bool
	}{
		{"confirmed", 2, 2, "task/cp-2", "task/cp-2", true},
		{"uncommitted", 2, 1, "task/cp-2", "task/cp-1", false},
		{"orphan_without_cursor", 1, 0, "task/cp-1", "", false},
		{"foreign_checkpoint", 2, 2, "task/cp-2", "task/other", false},
		{"invalid_version", 0, 0, "task/cp-0", "task/cp-0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ownerCheckpointConfirmed(tc.version, tc.id, tc.committedVersion, tc.committedID); got != tc.want {
				t.Fatalf("confirmation=%v want=%v", got, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name            string
		terminal        bool
		id, committedID string
		want            bool
	}{
		{"confirmed_terminal", true, "task/outcome/1", "task/outcome/1", true},
		{"orphan", false, "task/outcome/1", "task/outcome/1", false},
		{"foreign_outcome", true, "task/outcome/1", "task/outcome/2", false},
		{"empty_identity", true, "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ownerTerminalConfirmed(tc.id, tc.terminal, tc.committedID); got != tc.want {
				t.Fatalf("confirmation=%v want=%v", got, tc.want)
			}
		})
	}
}

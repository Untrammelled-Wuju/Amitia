package task_runtime

import "testing"

func TestRound99OwnerCheckpointCursorGuard(t *testing.T) {
	cases := []struct {
		name        string
		oldVersion  int64
		newVersion  int64
		oldID       string
		newID       string
		cursorVer   int64
		cursorID    string
		terminal    bool
		wantPending bool
		wantValid   bool
	}{
		{"initial_empty", 0, 1, "", "cp1", 0, "", false, false, true},
		{"first_committed", 0, 1, "", "cp1", 1, "cp1", false, true, true},
		{"matching_previous", 1, 2, "cp1", "cp2", 1, "cp1", false, false, true},
		{"next_committed", 1, 2, "cp1", "cp2", 2, "cp2", false, true, true},
		{"unrelated_previous", 1, 2, "cp1", "cp2", 1, "alien", false, false, false},
		{"orphan_target", 1, 2, "cp1", "cp2", 2, "alien", false, true, false},
		{"cursor_behind", 2, 3, "cp2", "cp3", 1, "cp1", false, false, false},
		{"cursor_ahead", 1, 2, "cp1", "cp2", 3, "cp3", false, false, false},
		{"terminal", 1, 2, "cp1", "cp2", 2, "cp2", true, false, false},
		{"invalid_step", 1, 3, "cp1", "cp3", 3, "cp3", false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pending, valid := ownedPendingCheckpointMatchesCursor(tc.oldVersion, tc.newVersion, tc.oldID, tc.newID, tc.cursorVer, tc.cursorID, tc.terminal)
			if pending != tc.wantPending || valid != tc.wantValid {
				t.Fatalf("pending=%v valid=%v, want pending=%v valid=%v", pending, valid, tc.wantPending, tc.wantValid)
			}
		})
	}
}

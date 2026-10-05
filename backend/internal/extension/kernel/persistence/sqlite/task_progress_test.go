package sqlite

import (
	"encoding/json"
	"testing"
)

func TestTaskProgressRejectsOlderAndDuplicateSequences(t *testing.T) {
	db := openWorkflowTestDB(t)
	repo := NewTaskRepository(db)
	for _, item := range []struct {
		seq  int64
		text string
	}{{2, "new"}, {1, "old"}, {2, "duplicate"}} {
		payload, err := json.Marshal(map[string]string{"message": item.text})
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.PutProgress(t.Context(), "run", item.seq, payload); err != nil {
			t.Fatal(err)
		}
	}
	progress, err := repo.GetProgress(t.Context(), "run")
	if err != nil {
		t.Fatal(err)
	}
	if progress == nil || progress.Sequence != 2 || string(progress.Details) != `{"message":"new"}` {
		t.Fatalf("progress overwritten by stale update: %+v", progress)
	}
}

package sqlite

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
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

func TestTaskProgressRestoresOwnedReferenceAndRejectsCrossTaskMetadata(t *testing.T) {
	db := openWorkflowTestDB(t)
	repo := NewTaskRepository(db)
	current := 2.0
	reference := json.RawMessage(`{"resourceId":"task/progress/run/2/attempt","revision":1,"hash":"confirmed"}`)
	progress := task_runtime.TaskRunProgress{TaskRunID: "run", Sequence: 3, Current: &current, Stage: "confirmed", Details: reference}
	body, err := json.Marshal(progress)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.PutProgress(t.Context(), "run", 3, body); err != nil {
		t.Fatal(err)
	}
	stored, err := repo.GetProgress(t.Context(), "run")
	if err != nil || stored == nil || string(stored.Details) != string(reference) || stored.Current == nil || *stored.Current != current || stored.Stage != progress.Stage {
		t.Fatalf("任务进度没有还原所有者引用与显示内容: %+v %v", stored, err)
	}
	for _, id := range []string{"foreign-task", "foreign-sequence"} {
		sequence := int64(3)
		if id == "foreign-sequence" {
			progress.TaskRunID = id
			sequence = 4
			body, err = json.Marshal(progress)
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := repo.PutProgress(t.Context(), id, sequence, body); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.GetProgress(t.Context(), id); err == nil {
			t.Fatal("任务进度接受了其他任务或不同序列的所有者引用")
		}
	}
}

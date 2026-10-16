package gitworktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestApprovedMergeAcrossProcesses(t *testing.T) {
	if os.Getenv("WORKBENCH_MERGE_CHILD") == "yes" {
		manager, err := New(os.Getenv("WORKBENCH_MERGE_DATA"))
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		result, err := manager.ApplyApprovedMerge(ctx, os.Getenv("WORKBENCH_MERGE_ROOT"), os.Getenv("WORKBENCH_MERGE_ID"), "task-1", "conversation-1", os.Getenv("WORKBENCH_MERGE_PLAN"), "local-owner")
		state := "rejected"
		if err == nil && result != nil && result.Result == "applied" {
			state = "applied"
		}
		if err := os.WriteFile(os.Getenv("WORKBENCH_MERGE_RESULT"), []byte(state), 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	m, root, entry, _ := approvedMergeFixture(t)
	plan, err := m.PreviewApprovedMerge(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	type proc struct {
		cmd  *exec.Cmd
		path string
	}
	processes := make([]proc, 2)
	for i := range processes {
		resultPath := filepath.Join(t.TempDir(), "result")
		cmd := exec.Command(os.Args[0], "-test.run=^TestApprovedMergeAcrossProcesses$")
		cmd.Env = append(os.Environ(), "WORKBENCH_MERGE_CHILD=yes", "WORKBENCH_MERGE_DATA="+m.dataRoot, "WORKBENCH_MERGE_ROOT="+root, "WORKBENCH_MERGE_ID="+entry.ID, "WORKBENCH_MERGE_PLAN="+plan.PlanID, "WORKBENCH_MERGE_RESULT="+resultPath)
		processes[i] = proc{cmd: cmd, path: resultPath}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
	}
	applied := 0
	for _, p := range processes {
		if err := p.cmd.Wait(); err != nil {
			t.Fatalf("merge worker failed: %v", err)
		}
		data, err := os.ReadFile(p.path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) == "applied" {
			applied++
		}
	}
	if applied != 1 || gitTest(t, root, "rev-parse", "HEAD") != plan.SourceHead {
		t.Fatalf("concurrent merges applied %d times", applied)
	}
	audit, err := m.ReadMergeAudit(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil || audit.Result != "applied" {
		t.Fatalf("missing coherent merge audit: %+v %v", audit, err)
	}
}

package gitworktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMergeRecoveryAcrossProcesses(t *testing.T) {
	if os.Getenv("WORKBENCH_RECOVER_CHILD") == "1" {
		m, err := New(os.Getenv("WORKBENCH_RECOVER_DATA"))
		if err != nil {
			t.Fatal(err)
		}
		audit, err := m.ReadMergeAudit(context.Background(), os.Getenv("WORKBENCH_RECOVER_ROOT"), os.Getenv("WORKBENCH_RECOVER_ID"), "task-1", "conversation-1")
		if err != nil || audit.Result != "applied" || audit.RecoveredAt == nil {
			t.Fatalf("worker recovery failed: %+v %v", audit, err)
		}
		return
	}
	m, root, entry, plan := pendingMergeFixture(t)
	gitTest(t, root, "merge", "--ff-only", plan.SourceHead)
	commands := make([]*exec.Cmd, 2)
	for i := range commands {
		commands[i] = exec.Command(os.Args[0], "-test.run=^TestMergeRecoveryAcrossProcesses$")
		commands[i].Env = append(os.Environ(),
			"WORKBENCH_RECOVER_CHILD=1", "WORKBENCH_RECOVER_DATA="+m.dataRoot,
			"WORKBENCH_RECOVER_ROOT="+root, "WORKBENCH_RECOVER_ID="+entry.ID)
		commands[i].Dir = filepath.Dir(root)
		if err := commands[i].Start(); err != nil {
			t.Fatal(err)
		}
	}
	for _, command := range commands {
		if err := command.Wait(); err != nil {
			t.Fatalf("recovery process failed: %v", err)
		}
	}
	audit, err := m.ReadMergeAudit(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil || audit.Result != "applied" || audit.RecoveredAt == nil {
		t.Fatalf("concurrent audit not durable: %+v %v", audit, err)
	}
}

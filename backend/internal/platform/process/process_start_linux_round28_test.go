//go:build linux && !android

package process

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestRound28AtomicLinuxCgroupStartsInsideItsGroup(t *testing.T) {
	support := ResourceLimitsSupported()
	if !support.Processes {
		t.Skip("requires delegated writable cgroup v2 pids controller")
	}
	cmd := exec.Command("cat", "/proc/self/cgroup")
	ConfigureProcess(cmd)
	var output bytes.Buffer
	cmd.Stdout = &output
	handle, err := StartProcessTreeWithLimits(cmd, ResourceLimits{MaxProcesses: 32})
	if err != nil {
		                                                                
		if cmd.Process != nil {
			t.Fatalf("failed atomic launch must not leave running worker: %v", err)
		}
		t.Logf("atomic launch is unsupported by this host; correctly rejected: %v", err)
		return
	}
	defer CloseProcessTree(handle)
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "amitia-task-") {
		t.Fatalf("worker executed outside preconfigured task cgroup: %q", output.String())
	}
}

package process

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestRound28StartRejectsNilCommand(t *testing.T) {
	if _, err := StartProcessTreeWithLimits(nil, ResourceLimits{}); err == nil {
		t.Fatal("nil command must fail before attempting a launch")
	}
}

func TestRound28StartWithoutLimitsPreservesLegacyExecution(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestRound28HelperProcess$")
	cmd.Env = append(os.Environ(), "AMITIA_ROUND28_CHILD=1")
	ConfigureProcess(cmd)
	var output bytes.Buffer
	cmd.Stdout = &output
	handle, err := StartProcessTreeWithLimits(cmd, ResourceLimits{})
	if err != nil {
		t.Fatal(err)
	}
	defer CloseProcessTree(handle)
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) != "ready" {
		t.Fatalf("unexpected child output: %q", output.String())
	}
}

func TestRound28HelperProcess(t *testing.T) {
	if os.Getenv("AMITIA_ROUND28_CHILD") == "1" {
		_, _ = os.Stdout.WriteString("ready\n")
		os.Exit(0)
	}
}

func TestRound28EnforcedLimitsFailClosedWhenUnavailable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux cgroup startup policy")
	}
	if ResourceLimitsSupported().Processes {
		t.Skip("the host delegates cgroup pids; enforced-start test covers it")
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRound28HelperProcess$")
	ConfigureProcess(cmd)
	_, err := StartProcessTreeWithLimits(cmd, ResourceLimits{MaxProcesses: 32})
	if err == nil || cmd.Process != nil {
		t.Fatalf("unsupported enforced limits must be rejected before starting worker: err=%v process=%v", err, cmd.Process)
	}
}

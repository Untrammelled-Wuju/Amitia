//go:build darwin && !ios

package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == ParentBoundChildFlag {
		if err := RunParentBoundChild(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestParentBoundDatabaseProcessExitsAfterOwnerKill(t *testing.T) {
	if os.Getenv("AMITIA_QA_PARENT_BOUND_OWNER") == "1" {
		pids := make(chan string, 1)
		manager := NewDefaultProcessManager()
		proc, err := manager.Start(context.Background(), ProcessConfig{
			Executable:   "/bin/sh",
			Args:         []string{"-c", "trap '' TERM; echo $$; exec /bin/sleep 30"},
			BindToParent: true,
			OnStdout: func(line string) {
				pids <- line
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		select {
		case pid := <-pids:
			fmt.Println(pid)
		case <-time.After(5 * time.Second):
			t.Fatal("database child did not report its PID")
		}
		<-proc.Done()
		return
	}
	owner := exec.Command(os.Args[0], "-test.run=^TestParentBoundDatabaseProcessExitsAfterOwnerKill$")
	owner.Env = append(os.Environ(), "AMITIA_QA_PARENT_BOUND_OWNER=1")
	owner.Stderr = os.Stderr
	stdout, err := owner.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if owner.ProcessState == nil {
			_ = owner.Process.Kill()
			_ = owner.Wait()
		}
	}()
	line := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Scan()
		line <- scanner.Text()
	}()
	var pid int
	select {
	case raw := <-line:
		pid, err = strconv.Atoi(raw)
		if err != nil || pid <= 0 {
			t.Fatalf("invalid database PID %q: %v", raw, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("owner did not report the database PID")
	}
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Wait(); err == nil {
		t.Fatal("owner unexpectedly exited normally")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("database PID %d remained alive after owner SIGKILL", pid)
}

func TestParentBoundHelperReportsUnexpectedCleanChildExit(t *testing.T) {
	cmd := exec.CommandContext(context.Background(), "/usr/bin/true")
	lifetime, err := startParentBoundCommand(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer lifetime.Close()
	if err := cmd.Wait(); err == nil {
		t.Fatal("parent-bound helper reported unexpected child exit as success")
	}
}

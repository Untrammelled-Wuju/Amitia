package gitworktree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCheckpointCancellationStopsChildProcessTree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows process termination requires a live Windows host")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("shell unavailable")
	}
	m, root, entry := checkpointFixture(t)
	location := t.TempDir()
	ready := filepath.Join(location, "ready")
	orphan := filepath.Join(location, "orphan")
	script := "sleep 2; printf leaked > " + strconv.Quote(orphan)
	source := "package worker\nimport (\"os\"; \"os/exec\"; \"testing\"; \"time\")\nfunc TestChild(t *testing.T) {\n" +
		"cmd := exec.Command(\"sh\", \"-c\", " + strconv.Quote(script) + ")\n" +
		"if err := cmd.Start(); err != nil { t.Fatal(err) }\n" +
		"if err := os.WriteFile(" + strconv.Quote(ready) + ", []byte(\"ok\"), 0600); err != nil { t.Fatal(err) }\n" +
		"time.Sleep(15 * time.Second)\n}\n"
	if err := os.WriteFile(filepath.Join(entry.Path, "go.mod"), []byte("module example.test/process\n\ngo 1.23\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entry.Path, "process_test.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, entry.Path, "add", "go.mod", "process_test.go")
	gitTest(t, entry.Path, "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "-m", "process tree fixture")
	checkpoint, err := m.PrepareCheckpoint(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	completed := make(chan error, 1)
	go func() {
		_, runErr := m.RunCheckpointTest(ctx, root, entry.ID, "task-1", "conversation-1", checkpoint.Revision, "go")
		completed <- runErr
	}()
	deadline := time.Now().Add(25 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("Go child did not reach the runnable fixture")
		}
		time.Sleep(30 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-completed:
		if !errors.Is(err, context.Canceled) && err != nil {
			t.Fatalf("unexpected cancellation result: %v", err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("cancelled Go test did not exit")
	}
	time.Sleep(2500 * time.Millisecond)
	if data, err := os.ReadFile(orphan); err == nil || !os.IsNotExist(err) {
		t.Fatalf("child process survived cancellation: %v %s", err, strings.TrimSpace(string(data)))
	}
}

package workbenchterminal

import (
	"encoding/base64"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type realPipeProcess struct {
	command   *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	closeOnce sync.Once
}

func (p *realPipeProcess) Read(b []byte) (int, error)  { return p.stdout.Read(b) }
func (p *realPipeProcess) Write(b []byte) (int, error) { return p.stdin.Write(b) }
func (p *realPipeProcess) Wait() error                 { return p.command.Wait() }
func (p *realPipeProcess) Mode() string                { return "pipe" }
func (p *realPipeProcess) Resize(uint16, uint16) error { return ErrUnavailable }
func (p *realPipeProcess) Close() error {
	p.closeOnce.Do(func() {
		_ = p.stdin.Close()
		_ = p.stdout.Close()
		if p.command.Process != nil {
			_ = p.command.Process.Kill()
		}
	})
	return nil
}

func launchRealPipe(cwd string, rows, cols uint16) (process, error) {
	command := exec.Command("/bin/sh", "-c", "IFS= read -r line; printf 'echo:%s\\n' \"$line\"; exit 7")
	command.Dir = cwd
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	command.Stderr = command.Stdout
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	return &realPipeProcess{command: command, stdin: stdin, stdout: stdout}, nil
}

func TestManagerRealProcessInputOutputAndExit(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("real shell lifecycle test uses Linux /bin/sh")
	}
	manager := NewManager()
	manager.startProcess = launchRealPipe
	defer manager.Close()
	snapshot, err := manager.Open("owner", os.TempDir(), 30, 100)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ID == "" || !snapshot.Running || snapshot.Mode != "pipe" {
		t.Fatalf("unexpected opened process: %+v", snapshot)
	}
	if err := manager.Input("owner", snapshot.ID, []byte("hello-world\n")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		current, err := manager.Read("owner", snapshot.ID, 0)
		if err != nil {
			t.Fatal(err)
		}
		out, err := base64.StdEncoding.DecodeString(current.Data)
		if err != nil {
			t.Fatal(err)
		}
		if !current.Running {
			if current.ExitCode != 7 {
				t.Fatalf("real process exit code not captured: %d", current.ExitCode)
			}
			if !strings.Contains(string(out), "echo:hello-world") {
				t.Fatalf("real process output not captured: %q", out)
			}
			if current.Cursor != int64(len(out)) {
				t.Fatalf("invalid output cursor: %d != %d", current.Cursor, len(out))
			}
			if err := manager.Input("owner", snapshot.ID, []byte("late\n")); !errors.Is(err, ErrExited) {
				t.Fatalf("exited process accepted input: %v", err)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("real shell process did not finish")
}

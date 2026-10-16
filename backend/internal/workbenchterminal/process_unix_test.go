//go:build linux || darwin

package workbenchterminal

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestRealPTYExecAndExit(t *testing.T) {
	manager := NewManager()
	defer manager.Close()
	opened, err := manager.Open("local-user:conversation", t.TempDir(), 30, 100)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Mode != "pty" {
		t.Fatalf("expected real PTY, got %s", opened.Mode)
	}
	if err := manager.Input("local-user:conversation", opened.ID, []byte("printf 'WORKBENCH_PTY_OK\\n'\n")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cursor := int64(0)
	var output strings.Builder
	for {
		if ctx.Err() != nil {
			t.Fatalf("PTY output timeout: %s", output.String())
		}
		snap, readErr := manager.Read("local-user:conversation", opened.ID, cursor)
		if readErr != nil {
			t.Fatal(readErr)
		}
		bytes, decodeErr := base64.StdEncoding.DecodeString(snap.Data)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		output.Write(bytes)
		cursor = snap.Cursor
		if strings.Contains(output.String(), "WORKBENCH_PTY_OK") {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if err := manager.Input("local-user:conversation", opened.ID, []byte("exit\n")); err != nil {
		t.Fatal(err)
	}
	for {
		if ctx.Err() != nil {
			t.Fatal("shell exit not observed")
		}
		snap, readErr := manager.Read("local-user:conversation", opened.ID, cursor)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !snap.Running {
			if snap.ExitCode != 0 {
				t.Fatalf("unexpected exit %d", snap.ExitCode)
			}
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
}

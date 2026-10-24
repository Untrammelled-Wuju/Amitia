//go:build linux && !android

package process

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRound30TerminateUsesExistingCgroupBeforeProcessGroup(t *testing.T) {
	group := t.TempDir()
	killFile := filepath.Join(group, "cgroup.kill")
	if err := os.WriteFile(killFile, []byte("0"), 0o600); err != nil {
		t.Fatal(err)
	}
	handle := ProcessTreeHandle(linuxCgroupSequence.Add(1))
	linuxCgroupMu.Lock()
	linuxCgroupHandles[handle] = group
	linuxCgroupMu.Unlock()
	defer func() {
		linuxCgroupMu.Lock()
		delete(linuxCgroupHandles, handle)
		linuxCgroupMu.Unlock()
	}()
	                                                                       
	                                                                              
	if err := terminateProcessTree(0, handle); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(killFile)
	if err != nil || string(body) != "1" {
		t.Fatalf("cgroup.kill not used: %q, %v", body, err)
	}
}

func TestRound30TerminateFallsBackWithoutCgroup(t *testing.T) {
	if err := terminateProcessTree(0, 0); err != nil {
		t.Fatalf("missing cgroup must preserve local empty-task behavior: %v", err)
	}
}

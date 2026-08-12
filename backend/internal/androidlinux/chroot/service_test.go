//go:build linux && !android

package chroot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()

	if !p.Enabled {
		t.Error("Enabled = false, want true")
	}
	if p.MaxFSBytes != 10*1024*1024*1024 {
		t.Errorf("MaxFSBytes = %d, want %d", p.MaxFSBytes, 10*1024*1024*1024)
	}
	if p.DefaultTimeoutSec != 30 {
		t.Errorf("DefaultTimeoutSec = %d, want 30", p.DefaultTimeoutSec)
	}
	if p.MaxTimeoutSec != 600 {
		t.Errorf("MaxTimeoutSec = %d, want 600", p.MaxTimeoutSec)
	}
}

func TestChrootStatus(t *testing.T) {
	policy := DefaultPolicy()
	svc := NewService(policy)

	status := svc.Status(nil, "/workspace")

	if !status.Enabled {
		t.Error("Status.Enabled = false, want true")
	}
	if status.DefaultRootFSP != "/workspace" {
		t.Errorf("Status.DefaultRootFSP = %s, want /workspace", status.DefaultRootFSP)
	}
	if len(status.ExecBackends) == 0 {
		t.Error("Status.ExecBackends is empty")
	}
}

func TestChrootInspectNonExistent(t *testing.T) {
	policy := DefaultPolicy()
	svc := NewService(policy)

	result, err := svc.Inspect(nil, ChrootInspectRequest{
		RootFSPath: "/nonexistent/path/that/does/not/exist",
	})
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if result.Exists {
		t.Error("Inspect() Exists = true for non-existent path")
	}
}

func TestChrootInspectValid(t *testing.T) {
	dir := t.TempDir()

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}

	shPath := filepath.Join(binDir, "sh")
	if err := os.WriteFile(shPath, []byte("#!/bin/sh\necho hello"), 0755); err != nil {
		t.Fatal(err)
	}

	policy := DefaultPolicy()
	svc := NewService(policy)

	result, err := svc.Inspect(nil, ChrootInspectRequest{
		RootFSPath: dir,
	})
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if !result.Exists {
		t.Error("Inspect() Exists = false, want true")
	}
	if !result.HasBinSH {
		t.Error("Inspect() HasBinSH = false, want true")
	}
	if !result.Valid {
		t.Error("Inspect() Valid = false, want true")
	}
}

func TestChrootExecValidation(t *testing.T) {
	policy := DefaultPolicy()
	svc := NewService(policy)

	t.Run("missing rootfs path", func(t *testing.T) {
		_, err := svc.Exec(nil, ChrootExecRequest{
			Command: "ls",
		})
		if err == nil {
			t.Error("Exec() expected error without rootfsPath")
		}
	})

	t.Run("missing command", func(t *testing.T) {
		_, err := svc.Exec(nil, ChrootExecRequest{
			RootFSPath: "/tmp",
		})
		if err == nil {
			t.Error("Exec() expected error without command")
		}
	})

	t.Run("non-existent rootfs", func(t *testing.T) {
		_, err := svc.Exec(nil, ChrootExecRequest{
			RootFSPath: "/nonexistent/rootfs",
			Command:    "ls",
		})
		if err == nil {
			t.Error("Exec() expected error for non-existent rootfs")
		}
	})
}

func TestVerifyBinariesPath(t *testing.T) {
	dir := t.TempDir()

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}

	policy := DefaultPolicy()
	svc := NewService(policy)

	if !svc.verifyBinariesPath(dir) {
		t.Error("verifyBinariesPath() = false, want true")
	}

	emptyDir := t.TempDir()
	if svc.verifyBinariesPath(emptyDir) {
		t.Error("verifyBinariesPath() = true for empty dir, want false")
	}
}

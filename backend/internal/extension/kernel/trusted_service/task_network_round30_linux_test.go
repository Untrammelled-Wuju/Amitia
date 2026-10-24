//go:build linux && !android

package trusted_service

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRound30IsolatedTaskNetworkLaunchUsesExistingNamespace(t *testing.T) {
	policy := ServiceNetworkPolicy{Mode: "none", Enforce: true}
	if err := ValidateNetworkSandboxPrerequisites(policy); err != nil {
		t.Skipf("host has no usable TrustedService network namespace: %v", err)
	}
	trueExe := firstTrustedLauncher("/usr/bin/true", "/bin/true")
	if trueExe == "" {
		t.Skip("host has no system true binary")
	}
	base := t.TempDir()
	for _, name := range []string{"work", "runtime", "bundle"} {
		if err := os.Mkdir(filepath.Join(base, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	work := filepath.Join(base, "work")
	plan, err := PrepareIsolatedTaskNetworkLaunch(trueExe, nil, work, filepath.Join(base, "runtime"), filepath.Join(base, "bundle"))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Cleanup != nil {
		defer plan.Cleanup()
	}
	if !strings.Contains(strings.Join(plan.Args, " "), "--unshare-net") {
		t.Fatal("sandbox lacks private network namespace")
	}
	if plan.AfterStart != nil {
		t.Fatal("task none policy requires unsafe post-start activation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, plan.Path, plan.Args...)
	cmd.Dir = plan.WorkingDir
	cmd.ExtraFiles = plan.ExtraFiles
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("isolated launch failed: %v: %s", err, out)
	}
}

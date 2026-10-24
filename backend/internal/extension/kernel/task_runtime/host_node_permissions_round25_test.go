package task_runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/runtimeprofile"
)

func round25HostPaths(t *testing.T) ProcessHostConfig {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dirs := []string{"node", "host", "bundle", "workspace"}
	for _, name := range dirs {
		if err := os.Mkdir(filepath.Join(base, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(base, "node", "node"), filepath.Join(base, "host", "runtime.cjs"), filepath.Join(base, "bundle", "entry.cjs")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("process.exit(0)"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return ProcessHostConfig{NodePath: paths[0], HostPath: paths[1], EntryPath: paths[2], EntryHash: strings.Repeat("a", 64), BundleRoot: filepath.Join(base, "bundle"), BundleHash: strings.Repeat("b", 64), WorkDir: filepath.Join(base, "workspace")}
}

func TestRound25CloudHostRestrictsFilesAndProcessesWithoutChangingLocalMode(t *testing.T) {
	cfg := round25HostPaths(t)
	local, err := taskNodeLaunchArgs(runtimeprofile.ProfileLocal, cfg)
	if err != nil || len(local) != 2 || local[1] != cfg.HostPath {
		t.Fatalf("local changed: %v %v", local, err)
	}
	cloud, err := taskNodeLaunchArgs(runtimeprofile.ProfileCloudCore, cfg)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(cloud, "\n")
	for _, required := range []string{"--permission", "--allow-fs-write=" + cfg.WorkDir, "--allow-fs-read=" + cfg.BundleRoot, "--allow-fs-read=" + filepath.Dir(cfg.HostPath)} {
		if !strings.Contains(joined, required) {
			t.Fatalf("cloud missing %q: %v", required, cloud)
		}
	}
	for _, forbidden := range []string{"--allow-child-process", "--allow-worker", "--allow-addons", "--allow-fs-read=*", "--allow-fs-write=*"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("cloud includes dangerous flag %q", forbidden)
		}
	}
	cfg.RequireSandbox = true
	withSandbox, err := taskNodeLaunchArgs(runtimeprofile.ProfileCloudCore, cfg)
	if err != nil || len(withSandbox) != 2 {
		t.Fatalf("trusted sandbox launch changed: %v %v", withSandbox, err)
	}
}

func TestRound25CloudHostRejectsUnsignedAndUnsafePaths(t *testing.T) {
	cfg := round25HostPaths(t)
	for _, mutate := range []func(*ProcessHostConfig){
		func(c *ProcessHostConfig) { c.BundleHash = "" },
		func(c *ProcessHostConfig) { c.BundleRoot = "" },
		func(c *ProcessHostConfig) { c.EntryPath = "relative.js" },
		func(c *ProcessHostConfig) { c.WorkDir = c.WorkDir + "*" },
		func(c *ProcessHostConfig) { c.WorkDir = c.BundleRoot },
		func(c *ProcessHostConfig) { c.WorkDir = filepath.Dir(c.HostPath) },
		func(c *ProcessHostConfig) { c.BundleRoot = filepath.Dir(c.WorkDir) },
	} {
		copy := cfg
		mutate(&copy)
		if _, err := taskNodeLaunchArgs(runtimeprofile.ProfileCloudCore, copy); err == nil {
			t.Fatalf("accepted unsafe path: %+v", copy)
		}
	}
	legacy := cfg
	legacy.BundleRoot, legacy.BundleHash, legacy.EntryHash = "", "", ""
	legacyArgs, legacyErr := taskNodeLaunchArgs(runtimeprofile.ProfileCloudCore, legacy)
	if legacyErr != nil || !strings.Contains(strings.Join(legacyArgs, "\n"), "--allow-fs-read="+filepath.Dir(legacy.EntryPath)) {
		t.Fatalf("legacy task lacks read-only entry permission: %v %v", legacyArgs, legacyErr)
	}
	if runtime.GOOS != "windows" {
		cfg2 := cfg
		if err := os.Chmod(cfg2.WorkDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := taskNodeLaunchArgs(runtimeprofile.ProfileCloudCore, cfg2); err == nil {
			t.Fatal("world-readable workspace accepted")
		}
	}
}

func TestRound25CloudNodePermissionsDenyUnlistedFilesAndChildren(t *testing.T) {
	node := os.Getenv("AMITIA_TEST_NODE")
	if node == "" {
		t.Skip("set AMITIA_TEST_NODE to the bundled Node binary")
	}
	cfg := round25HostPaths(t)
	forbidden := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(forbidden, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(cfg.WorkDir, "output.txt")
	script := `const fs=require('node:fs'); const cp=require('node:child_process');
const secret=process.env.TEST_SECRET; const output=process.env.TEST_OUTPUT;
let denied=0;
try {fs.readFileSync(secret)}catch(e){if(e.code==='ERR_ACCESS_DENIED')denied++}
try {fs.writeFileSync(secret,'oops')}catch(e){if(e.code==='ERR_ACCESS_DENIED')denied++}
try {cp.execFileSync(process.execPath,['-v'])}catch(e){if(e.code==='ERR_ACCESS_DENIED')denied++}
fs.writeFileSync(output, String(denied));
if(denied!==3)process.exitCode=1;`
	if err := os.WriteFile(cfg.HostPath, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.NodePath = node
	args, err := taskNodeLaunchArgs(runtimeprofile.ProfileCloudCore, cfg)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, args...)
	cmd.Dir = cfg.WorkDir
	cmd.Env = append(os.Environ(), "TEST_SECRET="+forbidden, "TEST_OUTPUT="+output)
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Node permission model unavailable or bypassed: %v %s", err, data)
	}
	if result, err := os.ReadFile(output); err != nil || string(result) != "3" {
		t.Fatalf("filesystem/child access not denied: %q %v", result, err)
	}
	if value, err := os.ReadFile(forbidden); err != nil || string(value) != "private" {
		t.Fatalf("secret modified: %s %v", value, err)
	}
}

// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func saveAndClearCache() RuntimePlatform {
	original := current
	current = nil
	return original
}

func restoreCache(p RuntimePlatform) {
	current = p
}

func TestDetectReturnsNonNil(t *testing.T) {
	p := Detect()
	if p == nil {
		t.Fatal("Detect() returned nil")
	}
	if p.Name() == "" {
		t.Fatal("Detect().Name() is empty")
	}
}

func TestDetectWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only runs on windows")
	}
	original := saveAndClearCache()
	defer restoreCache(original)

	t.Setenv(RuntimeModeEnv, "")

	p := Detect()
	if !p.IsWindows() {
		t.Fatalf("expected IsWindows=true, got false (name=%s)", p.Name())
	}
	if p.IsLinux() {
		t.Fatalf("expected IsLinux=false on windows")
	}
	if p.IsAndroid() {
		t.Fatalf("expected IsAndroid=false on windows")
	}
	if p.IsAndroidEmbedded() {
		t.Fatalf("expected IsAndroidEmbedded=false on windows")
	}
	if p.Name() != "desktop-windows" {
		t.Fatalf("expected name=desktop-windows, got %s", p.Name())
	}
}

func TestDetectLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only runs on linux")
	}
	original := saveAndClearCache()
	defer restoreCache(original)

	t.Setenv(RuntimeModeEnv, "")

	p := Detect()
	if !p.IsLinux() {
		t.Fatalf("expected IsLinux=true, got false (name=%s)", p.Name())
	}
	if p.IsWindows() {
		t.Fatalf("expected IsWindows=false on linux")
	}
	if p.IsAndroid() {
		t.Fatalf("expected IsAndroid=false on plain linux")
	}
	if p.IsAndroidEmbedded() {
		t.Fatalf("expected IsAndroidEmbedded=false on plain linux")
	}
	if p.Name() != "desktop-linux" {
		t.Fatalf("expected name=desktop-linux, got %s", p.Name())
	}
}

func TestDetectAndroid(t *testing.T) {
	if runtime.GOOS != "android" {
		t.Skip("only runs on android")
	}
	p := Detect()
	if !p.IsAndroid() {
		t.Fatalf("expected IsAndroid=true, got false (name=%s)", p.Name())
	}
	if !p.IsAndroidEmbedded() {
		t.Fatalf("expected IsAndroidEmbedded=true, got false")
	}
	if !p.IsLinux() {
		t.Fatalf("expected IsLinux=true on android")
	}
	if p.Name() != "android-embedded" {
		t.Fatalf("expected name=android-embedded, got %s", p.Name())
	}
}

func TestGetReturnsDetectWhenUnset(t *testing.T) {
	original := saveAndClearCache()
	defer restoreCache(original)

	if runtime.GOOS == "linux" {
		t.Setenv(RuntimeModeEnv, "")
	}

	p := Get()
	if p == nil {
		t.Fatal("Get() returned nil after reset")
	}
	if p.Name() == "" {
		t.Fatal("Get().Name() is empty")
	}
}

func TestSetOverride(t *testing.T) {
	original := current
	defer func() { current = original }()

	fake := &fakePlatform{}
	Set(fake)
	if got := Get(); got != fake {
		t.Fatalf("Get() did not return injected platform")
	}
}

type fakePlatform struct{}

func (fakePlatform) Name() string                         { return "fake" }
func (fakePlatform) KillExistingServer(addr string) error { return nil }
func (fakePlatform) ExecutableSuffix() string             { return "" }
func (fakePlatform) BinarySuffix() string                 { return "" }
func (fakePlatform) RootFSDir() string                    { return "" }
func (fakePlatform) DefaultDataDir() string               { return "data" }
func (fakePlatform) IsWindows() bool                      { return false }
func (fakePlatform) IsLinux() bool                        { return false }
func (fakePlatform) IsAndroid() bool                      { return false }
func (fakePlatform) IsAndroidEmbedded() bool              { return false }
func (fakePlatform) WritePidFile(string) error            { return nil }
func (fakePlatform) ReadPidFile(string) (int, error)      { return 0, nil }
func (fakePlatform) RemovePidFile(string) error           { return nil }

func TestBinarySuffixMatchesExecutableSuffix(t *testing.T) {
	p := Detect()
	if p.BinarySuffix() != p.ExecutableSuffix() {
		t.Fatalf("BinarySuffix (%q) != ExecutableSuffix (%q)", p.BinarySuffix(), p.ExecutableSuffix())
	}
}

func TestExecutableSuffixByPlatform(t *testing.T) {
	p := Detect()
	switch runtime.GOOS {
	case "windows":
		if p.ExecutableSuffix() != ".exe" {
			t.Fatalf("expected .exe on windows, got %q", p.ExecutableSuffix())
		}
		if p.BinarySuffix() != ".exe" {
			t.Fatalf("expected BinarySuffix .exe on windows, got %q", p.BinarySuffix())
		}
	case "linux", "android":
		if p.ExecutableSuffix() != "" {
			t.Fatalf("expected empty suffix on linux/android, got %q", p.ExecutableSuffix())
		}
		if p.BinarySuffix() != "" {
			t.Fatalf("expected empty BinarySuffix on linux/android, got %q", p.BinarySuffix())
		}
	}
}

func TestRootFSDirEnvVarOnNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("RootFSDir is intentionally empty on windows")
	}

	original := saveAndClearCache()
	defer restoreCache(original)

	if runtime.GOOS == "linux" {
		t.Setenv(RuntimeModeEnv, "")
	}

	const fakeRoot = "/tmp/amitia-rootfs-test"
	t.Setenv("AMITIA_ROOTFS_DIR", fakeRoot)

	p := Detect()
	got := p.RootFSDir()
	if got != fakeRoot {
		t.Fatalf("expected RootFSDir=%s, got %s", fakeRoot, got)
	}
}

func TestRootFSDirEmptyOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only runs on windows")
	}
	t.Setenv("AMITIA_ROOTFS_DIR", "/tmp/should-be-ignored")
	p := Detect()
	if got := p.RootFSDir(); got != "" {
		t.Fatalf("expected empty RootFSDir on windows, got %s", got)
	}
}

func TestKillExistingServerUnoccupiedPort(t *testing.T) {
	p := Detect()
	err := p.KillExistingServer("127.0.0.1:1")
	if err != nil {
		t.Fatalf("KillExistingServer on unoccupied port should return nil, got %v", err)
	}
}

func TestKillExistingServerInvalidAddrNoPanic(t *testing.T) {
	p := Detect()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("KillExistingServer panicked on invalid addr: %v", r)
		}
	}()
	_ = p.KillExistingServer("not-a-valid-addr")
}

func TestPidFileLifecycle(t *testing.T) {
	p := Detect()
	tmp := t.TempDir()
	dataDir := filepath.Join(tmp, p.DefaultDataDir())

	if err := p.WritePidFile(dataDir); err != nil {
		t.Fatalf("WritePidFile failed: %v", err)
	}

	pid, err := p.ReadPidFile(dataDir)
	if err != nil {
		t.Fatalf("ReadPidFile failed: %v", err)
	}
	if pid != os.Getpid() {
		t.Fatalf("expected pid=%d, got %d", os.Getpid(), pid)
	}

	if err := p.RemovePidFile(dataDir); err != nil {
		t.Fatalf("RemovePidFile failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dataDir, ".amitia-backend.pid")); !os.IsNotExist(err) {
		t.Fatalf("pid file still exists after RemovePidFile")
	}
}

func TestAllImplementationsSatisfyInterface(t *testing.T) {
	var _ RuntimePlatform = &ServerRuntime{}

	p := Detect()
	var _ RuntimePlatform = p
}

func TestServerRuntimeDefaultDataDirFromEnv(t *testing.T) {
	original := current
	defer func() { current = original }()

	srv := &ServerRuntime{}
	tmp := t.TempDir()
	t.Setenv("AMITIA_DATA_DIR", tmp)

	got := srv.DefaultDataDir()
	if got != tmp {
		t.Fatalf("expected DefaultDataDir=%s, got %s", tmp, got)
	}
}

func TestServerRuntimeRootFSDirFromEnv(t *testing.T) {
	srv := &ServerRuntime{}
	const fakeRoot = "/tmp/amitia-server-rootfs"
	t.Setenv("AMITIA_ROOTFS_DIR", fakeRoot)
	if got := srv.RootFSDir(); got != fakeRoot {
		t.Fatalf("expected RootFSDir=%s, got %s", fakeRoot, got)
	}
}

func TestAndroidPootModeDetection(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only runs on linux")
	}

	original := saveAndClearCache()
	defer restoreCache(original)

	tmp := t.TempDir()
	t.Setenv(RuntimeModeEnv, AndroidPRootMode)
	t.Setenv("AMITIA_ROOTFS_DIR", tmp)
	t.Setenv("AMITIA_DATA_DIR", tmp)

	p := Detect()
	if p.Name() != "android-proot" {
		t.Fatalf("expected name=android-proot, got %s", p.Name())
	}
	if !p.IsLinux() {
		t.Fatalf("expected IsLinux=true on android-proot")
	}
	if !p.IsAndroid() {
		t.Fatalf("expected IsAndroid=true on android-proot")
	}
	if !p.IsAndroidEmbedded() {
		t.Fatalf("expected IsAndroidEmbedded=true on android-proot")
	}
	if p.IsWindows() {
		t.Fatalf("expected IsWindows=false on android-proot")
	}
	if p.ExecutableSuffix() != "" {
		t.Fatalf("expected empty ExecutableSuffix, got %q", p.ExecutableSuffix())
	}
	if p.BinarySuffix() != "" {
		t.Fatalf("expected empty BinarySuffix, got %q", p.BinarySuffix())
	}
	if p.RootFSDir() != tmp {
		t.Fatalf("expected RootFSDir=%s, got %s", tmp, p.RootFSDir())
	}
	if p.DefaultDataDir() != tmp {
		t.Fatalf("expected DefaultDataDir=%s, got %s", tmp, p.DefaultDataDir())
	}
}

func TestAndroidPootModeNormalization(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only runs on linux")
	}

	cases := []string{
		AndroidPRootMode,
		"ANDROID-PROOT",
		"  android-proot  ",
	}

	for _, tc := range cases {
		original := saveAndClearCache()
		t.Setenv(RuntimeModeEnv, tc)

		p := Detect()
		if p.Name() != "android-proot" {
			restoreCache(original)
			t.Fatalf("expected android-proot for mode=%q, got %s", tc, p.Name())
		}
		restoreCache(original)
	}
}

func TestUnknownRuntimeModeFallsBackToLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only runs on linux")
	}

	original := saveAndClearCache()
	defer restoreCache(original)

	t.Setenv(RuntimeModeEnv, "unknown-mode")

	p := Detect()
	if p.Name() != "desktop-linux" {
		t.Fatalf("expected name=desktop-linux for unknown mode, got %s", p.Name())
	}
	if p.IsAndroid() {
		t.Fatalf("expected IsAndroid=false for unknown mode")
	}
	if p.IsAndroidEmbedded() {
		t.Fatalf("expected IsAndroidEmbedded=false for unknown mode")
	}
}

func TestAndroidPootPidFileUsesDataDir(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only runs on linux")
	}

	original := saveAndClearCache()
	defer restoreCache(original)

	tmp := t.TempDir()
	t.Setenv(RuntimeModeEnv, AndroidPRootMode)
	t.Setenv("AMITIA_DATA_DIR", tmp)

	p := Detect()

	if err := p.WritePidFile(""); err != nil {
		t.Fatalf("WritePidFile failed: %v", err)
	}

	pid, err := p.ReadPidFile("")
	if err != nil {
		t.Fatalf("ReadPidFile failed: %v", err)
	}
	if pid != os.Getpid() {
		t.Fatalf("expected pid=%d, got %d", os.Getpid(), pid)
	}

	pidPath := filepath.Join(tmp, ".amitia-backend.pid")
	if _, err := os.Stat(pidPath); err != nil {
		t.Fatalf("expected pid file at %s, err=%v", pidPath, err)
	}

	cwdPidPath := filepath.Join(".", "data", ".amitia-backend.pid")
	if _, err := os.Stat(cwdPidPath); err == nil {
		t.Fatalf("pid file should not be at %s", cwdPidPath)
	}

	if err := p.RemovePidFile(""); err != nil {
		t.Fatalf("RemovePidFile failed: %v", err)
	}

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("pid file still exists after RemovePidFile")
	}
}

func TestRuntimeModeCacheIsolation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only runs on linux")
	}

	original := saveAndClearCache()
	defer restoreCache(original)

	t.Setenv(RuntimeModeEnv, AndroidPRootMode)
	p1 := Detect()
	if p1.Name() != "android-proot" {
		t.Fatalf("first detect: expected android-proot, got %s", p1.Name())
	}

	current = nil
	t.Setenv(RuntimeModeEnv, "")

	p2 := Detect()
	if p2.Name() != "desktop-linux" {
		t.Fatalf("second detect: expected desktop-linux, got %s", p2.Name())
	}
}

func TestNormalizeRuntimeModeFunction(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"android-proot", "android-proot"},
		{"ANDROID-PROOT", "android-proot"},
		{"  android-proot  ", "android-proot"},
		{"", ""},
		{"unknown", "unknown"},
	}
	for _, tc := range cases {
		got := NormalizeRuntimeMode(tc.input)
		if got != tc.expected {
			t.Fatalf("NormalizeRuntimeMode(%q)=%q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestIsAndroidPRootModeFunction(t *testing.T) {
	cases := []struct {
		input    string
		expected bool
	}{
		{"android-proot", true},
		{"ANDROID-PROOT", true},
		{"  android-proot  ", true},
		{"", false},
		{"unknown", false},
		{"android", false},
	}
	for _, tc := range cases {
		got := IsAndroidPRootMode(tc.input)
		if got != tc.expected {
			t.Fatalf("IsAndroidPRootMode(%q)=%v, want %v", tc.input, got, tc.expected)
		}
	}
}

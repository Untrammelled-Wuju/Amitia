// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package platform

import (
	"runtime"
	"testing"
)

func TestNewRuntimeDescriptor(t *testing.T) {
	d := newRuntimeDescriptor(HostPlatformAndroid, RuntimeKindProot, GuestPlatformLinux)
	if d.Host != HostPlatformAndroid {
		t.Fatalf("expected Host=Android, got %s", d.Host)
	}
	if d.Kind != RuntimeKindProot {
		t.Fatalf("expected Kind=PRoot, got %s", d.Kind)
	}
	if d.Guest != GuestPlatformLinux {
		t.Fatalf("expected Guest=Linux, got %s", d.Guest)
	}
	if d.Architecture != runtime.GOARCH {
		t.Fatalf("expected Architecture=%s, got %s", runtime.GOARCH, d.Architecture)
	}

	d2 := newRuntimeDescriptor(HostPlatformAndroid, RuntimeKindProot, GuestPlatformLinux)
	if d != d2 {
		t.Fatalf("consecutive calls should produce identical results: %+v vs %+v", d, d2)
	}
}

func TestHostPlatformFromGOOS(t *testing.T) {
	cases := []struct {
		goos     string
		expected HostPlatform
	}{
		{"windows", HostPlatformWindows},
		{"linux", HostPlatformLinux},
		{"android", HostPlatformAndroid},
		{"darwin", HostPlatformMacOS},
		{"ios", HostPlatformIOS},
		{"", HostPlatformUnknown},
		{"freebsd", HostPlatformUnknown},
		{"openbsd", HostPlatformUnknown},
	}
	for _, tc := range cases {
		got := hostPlatformFromGOOS(tc.goos)
		if got != tc.expected {
			t.Fatalf("hostPlatformFromGOOS(%q)=%s, want %s", tc.goos, got, tc.expected)
		}
	}
}

func TestGuestPlatformFromGOOS(t *testing.T) {
	cases := []struct {
		goos     string
		expected GuestPlatform
	}{
		{"windows", GuestPlatformWindows},
		{"linux", GuestPlatformLinux},
		{"android", GuestPlatformAndroid},
		{"darwin", GuestPlatformMacOS},
		{"ios", GuestPlatformIOS},
		{"", GuestPlatformUnknown},
		{"freebsd", GuestPlatformUnknown},
		{"openbsd", GuestPlatformUnknown},
	}
	for _, tc := range cases {
		got := guestPlatformFromGOOS(tc.goos)
		if got != tc.expected {
			t.Fatalf("guestPlatformFromGOOS(%q)=%s, want %s", tc.goos, got, tc.expected)
		}
	}
}

func TestLinuxRuntimeDescriptor(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only runs on linux")
	}

	original := saveAndClearCache()
	defer restoreCache(original)

	t.Setenv(RuntimeModeEnv, "")

	p := Detect()
	desc := p.Descriptor()

	if desc.Host != HostPlatformLinux {
		t.Fatalf("expected Host=Linux, got %s", desc.Host)
	}
	if desc.Kind != RuntimeKindNativeProcess {
		t.Fatalf("expected Kind=NativeProcess, got %s", desc.Kind)
	}
	if desc.Guest != GuestPlatformLinux {
		t.Fatalf("expected Guest=Linux, got %s", desc.Guest)
	}
	if desc.Architecture != runtime.GOARCH {
		t.Fatalf("expected Architecture=%s, got %s", runtime.GOARCH, desc.Architecture)
	}
	if p.Name() != "desktop-linux" {
		t.Fatalf("expected Name=desktop-linux, got %s", p.Name())
	}
}

func TestAndroidProotRuntimeDescriptor(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only runs on linux")
	}

	original := saveAndClearCache()
	defer restoreCache(original)

	t.Setenv(RuntimeModeEnv, AndroidPRootMode)

	p := Detect()
	desc := p.Descriptor()

	if desc.Host != HostPlatformAndroid {
		t.Fatalf("expected Host=Android, got %s", desc.Host)
	}
	if desc.Kind != RuntimeKindProot {
		t.Fatalf("expected Kind=PRoot, got %s", desc.Kind)
	}
	if desc.Guest != GuestPlatformLinux {
		t.Fatalf("expected Guest=Linux, got %s", desc.Guest)
	}
	if desc.Architecture != runtime.GOARCH {
		t.Fatalf("expected Architecture=%s, got %s", runtime.GOARCH, desc.Architecture)
	}
	if p.Name() != "android-proot" {
		t.Fatalf("expected Name=android-proot, got %s", p.Name())
	}
	if !p.IsAndroid() {
		t.Fatalf("expected IsAndroid=true")
	}
	if !p.IsLinux() {
		t.Fatalf("expected IsLinux=true")
	}
}

func TestAndroidProotKeepsHostAndGuestSeparate(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only runs on linux")
	}

	original := saveAndClearCache()
	defer restoreCache(original)

	t.Setenv(RuntimeModeEnv, AndroidPRootMode)

	p := Detect()
	desc := p.Descriptor()

	if string(desc.Host) == string(desc.Guest) {
		t.Fatalf("Host (%s) must differ from Guest (%s) for Android PRoot", desc.Host, desc.Guest)
	}
	if desc.Host == HostPlatformLinux {
		t.Fatalf("Android PRoot Host must not be Linux")
	}
	if desc.Guest == GuestPlatformAndroid {
		t.Fatalf("Android PRoot Guest must not be Android")
	}
	if desc.Kind == RuntimeKindEmbedded {
		t.Fatalf("Android PRoot Kind must not be Embedded")
	}
	if desc.Kind == RuntimeKindNativeProcess {
		t.Fatalf("Android PRoot Kind must not be NativeProcess")
	}
}

func TestWindowsRuntimeDescriptor(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only runs on windows")
	}

	original := saveAndClearCache()
	defer restoreCache(original)

	p := Detect()
	desc := p.Descriptor()

	if desc.Host != HostPlatformWindows {
		t.Fatalf("expected Host=Windows, got %s", desc.Host)
	}
	if desc.Kind != RuntimeKindNativeProcess {
		t.Fatalf("expected Kind=NativeProcess, got %s", desc.Kind)
	}
	if desc.Guest != GuestPlatformWindows {
		t.Fatalf("expected Guest=Windows, got %s", desc.Guest)
	}
	if desc.Architecture != runtime.GOARCH {
		t.Fatalf("expected Architecture=%s, got %s", runtime.GOARCH, desc.Architecture)
	}
	if p.Name() != "desktop-windows" {
		t.Fatalf("expected Name=desktop-windows, got %s", p.Name())
	}
}

func TestServerRuntimeDescriptor(t *testing.T) {
	srv := &ServerRuntime{}
	desc := srv.Descriptor()

	if desc.Kind != RuntimeKindRemote {
		t.Fatalf("expected Kind=Remote, got %s", desc.Kind)
	}
	expectedHost := hostPlatformFromGOOS(runtime.GOOS)
	if desc.Host != expectedHost {
		t.Fatalf("expected Host=%s, got %s", expectedHost, desc.Host)
	}
	expectedGuest := guestPlatformFromGOOS(runtime.GOOS)
	if desc.Guest != expectedGuest {
		t.Fatalf("expected Guest=%s, got %s", expectedGuest, desc.Guest)
	}
	if desc.Architecture != runtime.GOARCH {
		t.Fatalf("expected Architecture=%s, got %s", runtime.GOARCH, desc.Architecture)
	}
}

func TestDescriptorDoesNotReadRuntimeMode(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only runs on linux")
	}

	original := saveAndClearCache()
	defer restoreCache(original)

	t.Setenv(RuntimeModeEnv, AndroidPRootMode)
	p1 := Detect()
	d1 := p1.Descriptor()

	p2 := Detect()
	d2 := p2.Descriptor()

	if d1 != d2 {
		t.Fatalf("Descriptor should not depend on env vars: %+v vs %+v", d1, d2)
	}

	if d1.Host != HostPlatformAndroid {
		t.Fatalf("expected Host=Android, got %s", d1.Host)
	}
	if d1.Kind != RuntimeKindProot {
		t.Fatalf("expected Kind=PRoot, got %s", d1.Kind)
	}
}

func TestRuntimeDescriptorDoesNotChangeLegacyFlags(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only runs on linux")
	}

	original := saveAndClearCache()
	defer restoreCache(original)

	t.Setenv(RuntimeModeEnv, "")
	pLinux := Detect()
	if !pLinux.IsLinux() {
		t.Fatalf("plain linux should have IsLinux=true")
	}
	if pLinux.IsWindows() {
		t.Fatalf("plain linux should have IsWindows=false")
	}
	if pLinux.IsAndroid() {
		t.Fatalf("plain linux should have IsAndroid=false")
	}
	if pLinux.IsAndroidEmbedded() {
		t.Fatalf("plain linux should have IsAndroidEmbedded=false")
	}
	pLinux.Descriptor()

	current = nil
	t.Setenv(RuntimeModeEnv, AndroidPRootMode)

	pPRoot := Detect()
	if !pPRoot.IsLinux() {
		t.Fatalf("android-proot should keep IsLinux=true")
	}
	if pPRoot.IsWindows() {
		t.Fatalf("android-proot should keep IsWindows=false")
	}
	if !pPRoot.IsAndroid() {
		t.Fatalf("android-proot should keep IsAndroid=true")
	}
	if !pPRoot.IsAndroidEmbedded() {
		t.Fatalf("android-proot should keep IsAndroidEmbedded=true")
	}
	pPRoot.Descriptor()
}

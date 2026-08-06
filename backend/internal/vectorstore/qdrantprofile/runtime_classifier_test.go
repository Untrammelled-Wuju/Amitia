// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprofile

import (
	"testing"

	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/platform"
)

type fakeDescriptorProvider struct {
	desc platform.RuntimeDescriptor
	caps *runtimehost.HostCapabilities
}

func (f fakeDescriptorProvider) Descriptor() platform.RuntimeDescriptor {
	return f.desc
}

func (f fakeDescriptorProvider) Capabilities() *runtimehost.HostCapabilities {
	return f.caps
}

func makeCaps() *runtimehost.HostCapabilities {
	return runtimehost.NewTestCapabilitiesForTest(map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport{
		runtimehost.CapProcessSpawn:    runtimehost.SupportSupported,
		runtimehost.CapFilesystemLocal: runtimehost.SupportSupported,
	})
}

func TestClassifier_AndroidProotARM64(t *testing.T) {
	c := NewRuntimeClassifier()
	p := fakeDescriptorProvider{
		desc: platform.RuntimeDescriptor{
			Host:         platform.HostPlatformAndroid,
			Kind:         platform.RuntimeKindProot,
			Guest:        platform.GuestPlatformLinux,
			Architecture: "arm64",
		},
		caps: makeCaps(),
	}
	class, err := c.Classify(p)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if class != RuntimeClassAndroidProot {
		t.Errorf("class = %q, want android-proot", class)
	}
}

func TestClassifier_AndroidEmbeddedARM64(t *testing.T) {
	c := NewRuntimeClassifier()
	p := fakeDescriptorProvider{
		desc: platform.RuntimeDescriptor{
			Host:         platform.HostPlatformAndroid,
			Kind:         platform.RuntimeKindEmbedded,
			Guest:        platform.GuestPlatformLinux,
			Architecture: "arm64",
		},
		caps: makeCaps(),
	}
	class, err := c.Classify(p)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if class != RuntimeClassAndroidProot {
		t.Errorf("class = %q, want android-proot", class)
	}
}

func TestClassifier_AndroidGuestNotLinux(t *testing.T) {
	c := NewRuntimeClassifier()
	p := fakeDescriptorProvider{
		desc: platform.RuntimeDescriptor{
			Host:         platform.HostPlatformAndroid,
			Kind:         platform.RuntimeKindProot,
			Guest:        platform.GuestPlatformAndroid,
			Architecture: "arm64",
		},
		caps: makeCaps(),
	}
	class, err := c.Classify(p)
	if err == nil {
		t.Error("expected error for Android guest platform")
	}
	if class != RuntimeClassUnsupported {
		t.Errorf("class = %q, want unsupported", class)
	}
}

func TestClassifier_AndroidGuestNotARM64(t *testing.T) {
	c := NewRuntimeClassifier()
	p := fakeDescriptorProvider{
		desc: platform.RuntimeDescriptor{
			Host:         platform.HostPlatformAndroid,
			Kind:         platform.RuntimeKindProot,
			Guest:        platform.GuestPlatformLinux,
			Architecture: "amd64",
		},
		caps: makeCaps(),
	}
	class, err := c.Classify(p)
	if err == nil {
		t.Error("expected error for non-arm64 guest architecture")
	}
	if class != RuntimeClassUnsupported {
		t.Errorf("class = %q, want unsupported", class)
	}
}

func TestClassifier_LinuxDesktopAMD64(t *testing.T) {
	c := NewRuntimeClassifier()
	p := fakeDescriptorProvider{
		desc: platform.RuntimeDescriptor{
			Host:         platform.HostPlatformLinux,
			Kind:         platform.RuntimeKindNativeProcess,
			Guest:        platform.GuestPlatformLinux,
			Architecture: "amd64",
		},
		caps: makeCaps(),
	}
	class, err := c.Classify(p)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if class != RuntimeClassDesktopProcess {
		t.Errorf("class = %q, want desktop-process", class)
	}
}

func TestClassifier_LinuxARM64Desktop(t *testing.T) {
	c := NewRuntimeClassifier()
	p := fakeDescriptorProvider{
		desc: platform.RuntimeDescriptor{
			Host:         platform.HostPlatformLinux,
			Kind:         platform.RuntimeKindNativeProcess,
			Guest:        platform.GuestPlatformLinux,
			Architecture: "arm64",
		},
		caps: makeCaps(),
	}
	class, err := c.Classify(p)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if class != RuntimeClassDesktopProcess {
		t.Errorf("class = %q, want desktop-process", class)
	}
}

func TestClassifier_WindowsDesktop(t *testing.T) {
	c := NewRuntimeClassifier()
	p := fakeDescriptorProvider{
		desc: platform.RuntimeDescriptor{
			Host:         platform.HostPlatformWindows,
			Kind:         platform.RuntimeKindNativeProcess,
			Guest:        platform.GuestPlatformWindows,
			Architecture: "amd64",
		},
		caps: makeCaps(),
	}
	class, err := c.Classify(p)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if class != RuntimeClassDesktopProcess {
		t.Errorf("class = %q, want desktop-process", class)
	}
}

func TestClassifier_MacOSDesktop(t *testing.T) {
	c := NewRuntimeClassifier()
	p := fakeDescriptorProvider{
		desc: platform.RuntimeDescriptor{
			Host:         platform.HostPlatformMacOS,
			Kind:         platform.RuntimeKindNativeProcess,
			Guest:        platform.GuestPlatformMacOS,
			Architecture: "arm64",
		},
		caps: makeCaps(),
	}
	class, err := c.Classify(p)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if class != RuntimeClassDesktopProcess {
		t.Errorf("class = %q, want desktop-process", class)
	}
}

func TestClassifier_IOSRestricted(t *testing.T) {
	c := NewRuntimeClassifier()
	p := fakeDescriptorProvider{
		desc: platform.RuntimeDescriptor{
			Host:         platform.HostPlatformIOS,
			Kind:         platform.RuntimeKindSandbox,
			Guest:        platform.GuestPlatformNone,
			Architecture: "arm64",
		},
		caps: makeCaps(),
	}
	class, err := c.Classify(p)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if class != RuntimeClassRestricted {
		t.Errorf("class = %q, want restricted", class)
	}
}

func TestClassifier_NoProcessCapability(t *testing.T) {
	c := NewRuntimeClassifier()
	p := fakeDescriptorProvider{
		desc: platform.RuntimeDescriptor{
			Host:         platform.HostPlatformIOS,
			Kind:         platform.RuntimeKindSandbox,
			Guest:        platform.GuestPlatformNone,
			Architecture: "arm64",
		},
		caps: runtimehost.NewTestCapabilitiesForTest(map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport{
			runtimehost.CapProcessSpawn: runtimehost.SupportUnsupported,
		}),
	}
	class, err := c.Classify(p)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if class != RuntimeClassRestricted {
		t.Errorf("class = %q, want restricted", class)
	}
}

func TestClassifier_NilProvider(t *testing.T) {
	c := NewRuntimeClassifier()
	_, err := c.Classify(nil)
	if err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestClassifier_NilCaps(t *testing.T) {
	c := NewRuntimeClassifier()
	p := fakeDescriptorProvider{
		desc: platform.RuntimeDescriptor{
			Host: platform.HostPlatformLinux,
		},
		caps: nil,
	}
	_, err := c.Classify(p)
	if err == nil {
		t.Error("expected error for nil capabilities")
	}
}

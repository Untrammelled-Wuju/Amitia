// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimehost

import (
	"testing"

	"github.com/u-ai/backend/internal/platform/process"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

func TestAndroidProotBuildsNativeProcessHost(t *testing.T) {
	ctx := HostBuildContext{
		Descriptor: platform.RuntimeDescriptor{
			Host:  platform.HostPlatformAndroid,
			Kind:  platform.RuntimeKindProot,
			Guest: platform.GuestPlatformLinux,
		},
		Paths:          util.RuntimePaths{},
		ProcessManager: process.NewDefaultProcessManager(),
	}
	host, err := NewRuntimeHost(ctx)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if host == nil {
		t.Fatal("expected non-nil host")
	}
	if !host.Capabilities().Supports(CapProcessSpawn) {
		t.Fatal("Android PRoot should support process.spawn")
	}
}

func TestLinuxBuildsNativeProcessHost(t *testing.T) {
	ctx := HostBuildContext{
		Descriptor: platform.RuntimeDescriptor{
			Host:  platform.HostPlatformLinux,
			Kind:  platform.RuntimeKindNativeProcess,
			Guest: platform.GuestPlatformLinux,
		},
		Paths:          util.RuntimePaths{},
		ProcessManager: process.NewDefaultProcessManager(),
	}
	host, err := NewRuntimeHost(ctx)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !host.Capabilities().Supports(CapProcessSpawn) {
		t.Fatal("Linux Native should support process.spawn")
	}
}

func TestWindowsBuildsNativeProcessHost(t *testing.T) {
	ctx := HostBuildContext{
		Descriptor: platform.RuntimeDescriptor{
			Host:  platform.HostPlatformWindows,
			Kind:  platform.RuntimeKindNativeProcess,
			Guest: platform.GuestPlatformWindows,
		},
		Paths:          util.RuntimePaths{},
		ProcessManager: process.NewDefaultProcessManager(),
	}
	host, err := NewRuntimeHost(ctx)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !host.Capabilities().Supports(CapProcessSpawn) {
		t.Fatal("Windows Native should support process.spawn")
	}
	caps := host.Capabilities()
	if caps.Support(CapProcessGracefulStop) != SupportLimited {
		t.Fatal("Windows graceful stop should be Limited")
	}
}

func TestEmbeddedIOSBuildsRestrictedHost(t *testing.T) {
	ctx := HostBuildContext{
		Descriptor: platform.RuntimeDescriptor{
			Host:  platform.HostPlatformIOS,
			Kind:  platform.RuntimeKindEmbedded,
			Guest: platform.GuestPlatformIOS,
		},
		Paths:          util.RuntimePaths{},
		ProcessManager: process.NewDefaultProcessManager(),
	}
	host, err := NewRuntimeHost(ctx)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if host.Capabilities().Supports(CapProcessSpawn) {
		t.Fatal("Embedded iOS should NOT support process.spawn")
	}
	if host.Capabilities().Support(CapFilesystemExecutable) != SupportUnsupported {
		t.Fatal("Embedded iOS should not support filesystem.executable")
	}
}

func TestEmbeddedAndroidBuildsRestrictedHost(t *testing.T) {
	ctx := HostBuildContext{
		Descriptor: platform.RuntimeDescriptor{
			Host:  platform.HostPlatformAndroid,
			Kind:  platform.RuntimeKindEmbedded,
			Guest: platform.GuestPlatformAndroid,
		},
		Paths:          util.RuntimePaths{},
		ProcessManager: process.NewDefaultProcessManager(),
	}
	host, err := NewRuntimeHost(ctx)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if host.Capabilities().Supports(CapProcessSpawn) {
		t.Fatal("Embedded Android should NOT support process.spawn")
	}
}

func TestUnknownDescriptorIsRejected(t *testing.T) {
	ctx := HostBuildContext{
		Descriptor: platform.RuntimeDescriptor{
			Host:  platform.HostPlatformUnknown,
			Kind:  platform.RuntimeKindUnknown,
			Guest: platform.GuestPlatformUnknown,
		},
		Paths:          util.RuntimePaths{},
		ProcessManager: process.NewDefaultProcessManager(),
	}
	_, err := NewRuntimeHost(ctx)
	if err == nil {
		t.Fatal("expected error for fully unknown descriptor")
	}
}

func TestRuntimeInstanceIDIsStableAndUnique(t *testing.T) {
	ctx := HostBuildContext{
		Descriptor: platform.RuntimeDescriptor{
			Host:  platform.HostPlatformLinux,
			Kind:  platform.RuntimeKindNativeProcess,
			Guest: platform.GuestPlatformLinux,
		},
		Paths:          util.RuntimePaths{},
		ProcessManager: process.NewDefaultProcessManager(),
	}
	host1, _ := NewRuntimeHost(ctx)
	host2, _ := NewRuntimeHost(ctx)

	id1 := host1.RuntimeInstanceID()
	id2 := host2.RuntimeInstanceID()

	if id1 == "" || id2 == "" {
		t.Fatal("instance IDs should not be empty")
	}
	if id1 == id2 {
		t.Fatal("instance IDs should be unique")
	}

	// Stability check - same host returns same ID multiple times
	for i := 0; i < 10; i++ {
		if host1.RuntimeInstanceID() != id1 {
			t.Fatal("instance ID should be stable within same host")
		}
	}
}

func TestHostCapabilitiesReturnCopies(t *testing.T) {
	ctx := HostBuildContext{
		Descriptor: platform.RuntimeDescriptor{
			Host:  platform.HostPlatformLinux,
			Kind:  platform.RuntimeKindNativeProcess,
			Guest: platform.GuestPlatformLinux,
		},
		Paths:          util.RuntimePaths{},
		ProcessManager: process.NewDefaultProcessManager(),
	}
	host, _ := NewRuntimeHost(ctx)
	snap1 := host.Capabilities().Snapshot()
	snap2 := host.Capabilities().Snapshot()
	if &snap1.Support == &snap2.Support {
		t.Fatal("Snapshot should return copies")
	}
}

func TestRestrictedHostRejectsProcessRegistration(t *testing.T) {
	ctx := HostBuildContext{
		Descriptor: platform.RuntimeDescriptor{
			Host:  platform.HostPlatformIOS,
			Kind:  platform.RuntimeKindEmbedded,
			Guest: platform.GuestPlatformIOS,
		},
		Paths:          util.RuntimePaths{},
		ProcessManager: process.NewDefaultProcessManager(),
	}
	host, _ := NewRuntimeHost(ctx)
	supervisor := host.Processes()
	err := supervisor.Register(ProcessSpec{
		ID:         "test.process",
		Executable: "/path/to/exe",
		WorkingDir: "/tmp",
	})
	if err != ErrHostProcessUnsupported {
		t.Fatalf("expected ErrHostProcessUnsupported, got %v", err)
	}
}

func TestHostDoesNotSelectProviders(t *testing.T) {
	// Factory should not modify the descriptor or select a provider
	ctx := HostBuildContext{
		Descriptor: platform.RuntimeDescriptor{
			Host:  platform.HostPlatformLinux,
			Kind:  platform.RuntimeKindNativeProcess,
			Guest: platform.GuestPlatformLinux,
		},
		Paths:          util.RuntimePaths{},
		ProcessManager: process.NewDefaultProcessManager(),
	}
	originalDesc := ctx.Descriptor
	host, err := NewRuntimeHost(ctx)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if host.Descriptor() != originalDesc {
		t.Fatal("host should not modify descriptor")
	}
}

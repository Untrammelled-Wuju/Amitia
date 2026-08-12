//go:build linux && !android

package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	terminal "github.com/u-ai/backend/internal/androidlinux/terminal"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

func TestBuildLinuxFileTools(t *testing.T) {
	tools := BuildLinuxFileTools()

	require.Len(t, tools, 14)

	expectedIDs := []string{
		"builtin:android_linux:file.stat",
		"builtin:android_linux:file.list",
		"builtin:android_linux:file.read",
		"builtin:android_linux:file.write",
		"builtin:android_linux:file.append",
		"builtin:android_linux:file.mkdir",
		"builtin:android_linux:file.touch",
		"builtin:android_linux:file.copy",
		"builtin:android_linux:file.move",
		"builtin:android_linux:file.delete",
		"builtin:android_linux:file.search",
		"builtin:android_linux:file.chmod",
		"builtin:android_linux:file.readlink",
		"builtin:android_linux:file.symlink",
	}

	for i, tool := range tools {
		assert.Equal(t, expectedIDs[i], tool.ID, "tool %d ID mismatch", i)
		assert.NotEmpty(t, tool.ModelName)
		assert.Equal(t, capability.ToolSourceBuiltin, tool.Source)
		assert.NotNil(t, tool.InputSchema)
		assert.NotNil(t, tool.OutputSchema)
		assert.Equal(t, capability.RuntimeTypeAndroidLinux, tool.Runtime.RuntimeType)
		assert.Equal(t, terminal.RuntimeIDAndroidLinux, tool.Runtime.RuntimeID)
		assert.True(t, tool.Enabled)
		assert.NotEmpty(t, tool.Permissions)
	}
}

func TestBuildLinuxFileTools_PermissionRequirements(t *testing.T) {
	tools := BuildLinuxFileTools()

	for _, tool := range tools {
		switch tool.Runtime.HandlerName {
		case "file.stat", "file.list", "file.read", "file.search", "file.readlink":
			hasReadPerm := false
			for _, perm := range tool.Permissions {
				if perm.Capability == "runtime.linux.file.read" {
					hasReadPerm = true
					break
				}
			}
			assert.True(t, hasReadPerm, "%s should have read permission", tool.ID)
		case "file.write", "file.append", "file.mkdir", "file.touch", "file.copy", "file.move", "file.delete":
			hasWritePerm := false
			for _, perm := range tool.Permissions {
				if perm.Capability == "runtime.linux.file.write" {
					hasWritePerm = true
					break
				}
			}
			assert.True(t, hasWritePerm, "%s should have write permission", tool.ID)
		case "file.chmod", "file.symlink":
			hasControlPerm := false
			for _, perm := range tool.Permissions {
				if perm.Capability == "runtime.linux.file.control" {
					hasControlPerm = true
					break
				}
			}
			assert.True(t, hasControlPerm, "%s should have control permission", tool.ID)
		}
	}
}

func TestBuildLinuxFileTools_SideEffects(t *testing.T) {
	tools := BuildLinuxFileTools()

	readOnlyTools := map[string]bool{
		"file.stat":     true,
		"file.list":     true,
		"file.read":     true,
		"file.search":   true,
		"file.readlink": true,
	}

	for _, tool := range tools {
		if readOnlyTools[tool.Runtime.HandlerName] {
			assert.False(t, tool.HasSideEffects, "%s should not have side effects", tool.ID)
			assert.True(t, tool.Idempotent, "%s should be idempotent", tool.ID)
			assert.Equal(t, capability.SideEffectReadOnly, tool.SideEffect, "%s should have read_only side effect level", tool.ID)
		} else {
			assert.True(t, tool.HasSideEffects, "%s should have side effects", tool.ID)
		}
	}
}

func TestRegisterLinuxFileTools(t *testing.T) {
	registrar := NewToolRegistrar()
	require.NotNil(t, registrar)

	registry := capability.NewToolRegistry()

	host := createTestFileRuntimeHost()

	err := registrar.RegisterLinuxFileTools(host, registry)
	assert.NoError(t, err)

	ctx := context.Background()
	defs := registry.List(ctx, capability.ToolFilter{})

	fileToolCount := 0
	for _, def := range defs {
		if len(def.ID) > 0 && def.ID[:22] == "builtin:android_linux:file" {
			fileToolCount++
		}
	}
	assert.Equal(t, 14, fileToolCount, "should register all 14 file tools")
}

func TestRegisterLinuxFileTools_NonAndroidLinux(t *testing.T) {
	registrar := NewToolRegistrar()
	require.NotNil(t, registrar)

	registry := capability.NewToolRegistry()

	host := createTestToolsRuntimeHost()
	host.desc = platform.NewRuntimeDescriptor(platform.HostPlatformLinux, platform.RuntimeKindNative, platform.GuestPlatformLinux)

	err := registrar.RegisterLinuxFileTools(host, registry)
	assert.NoError(t, err)

	ctx := context.Background()
	defs := registry.List(ctx, capability.ToolFilter{})

	fileToolCount := 0
	for _, def := range defs {
		if len(def.ID) > 0 && def.ID[:22] == "builtin:android_linux:file" {
			fileToolCount++
		}
	}
	assert.Equal(t, 0, fileToolCount, "should not register file tools for non-android-linux runtime")
}

func createTestFileRuntimeHost() *testToolsRuntimeHost {
	return &testToolsRuntimeHost{
		desc: platform.NewRuntimeDescriptor(platform.HostPlatformAndroid, platform.RuntimeKindProot, platform.GuestPlatformLinux),
		paths: util.RuntimePaths{
			WorkspaceDir: "/tmp/test",
		},
		caps: map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport{
			runtimehost.CapProcessSpawn:         runtimehost.SupportSupported,
			runtimehost.CapRuntimeSandboxedExec: runtimehost.SupportLimited,
			runtimehost.CapFilesystemLocal:      runtimehost.SupportSupported,
		},
	}
}

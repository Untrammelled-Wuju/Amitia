//go:build linux && !android

package terminal_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/u-ai/backend/internal/androidlinux/terminal"
	terminaltools "github.com/u-ai/backend/internal/androidlinux/terminal/tools"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/pkg/util"
)

func TestBuildTerminalTools(t *testing.T) {
	tools := terminaltools.BuildTerminalTools()

	require.NotEmpty(t, tools)

	for _, tool := range tools {
		assert.NotEmpty(t, tool.ID)
		assert.NotEmpty(t, tool.ModelName)
		assert.Equal(t, capability.ToolSourceBuiltin, tool.Source)
		assert.NotNil(t, tool.InputSchema)
		assert.NotNil(t, tool.OutputSchema)
		assert.Equal(t, capability.RuntimeTypeAndroidLinux, tool.Runtime.RuntimeType)
		assert.Equal(t, terminal.RuntimeIDAndroidLinux, tool.Runtime.RuntimeID)
	}

	openTool := tools[0]
	assert.Equal(t, "terminal.open", openTool.Runtime.HandlerName)
	assert.Equal(t, capability.RiskMedium, openTool.RiskLevel)
	assert.True(t, openTool.HasSideEffects)
	assert.False(t, openTool.Idempotent)
	assert.False(t, openTool.Retryable)

	readTool := tools[2]
	assert.Equal(t, "terminal.read", readTool.Runtime.HandlerName)
	assert.True(t, readTool.Idempotent)
	assert.NotEmpty(t, readTool.Permissions)
}

func TestToolRegistration(t *testing.T) {
	registrar := terminaltools.NewToolRegistrar()
	require.NotNil(t, registrar)

	registry := capability.NewToolRegistry()

	host := createTestRuntimeHostForTools()

	err := registrar.RegisterTerminalTools(host, registry)
	assert.NoError(t, err)

	ctx := context.Background()
	defs := registry.List(ctx, capability.ToolFilter{})
	assert.NotEmpty(t, defs)

	found := false
	for _, def := range defs {
		if def.ID == capability.BuildToolID(capability.ToolSourceBuiltin, "android_linux", "terminal.open") {
			found = true
			break
		}
	}
	assert.True(t, found, "terminal.open tool should be registered")
}

func TestPermissionRequirements(t *testing.T) {
	tools := terminaltools.BuildTerminalTools()

	for _, tool := range tools {
		switch tool.Runtime.HandlerName {
		case "terminal.open", "terminal.write", "terminal.resize", "terminal.close", "terminal.cancel":
			require.NotEmpty(t, tool.Permissions)
			hasControlPerm := false
			for _, perm := range tool.Permissions {
				if perm.Capability == "runtime.linux.terminal.control" {
					hasControlPerm = true
					break
				}
			}
			assert.True(t, hasControlPerm, "%s should have control permission", tool.ID)
		case "terminal.read", "terminal.status":
			require.NotEmpty(t, tool.Permissions)
			hasReadPerm := false
			for _, perm := range tool.Permissions {
				if perm.Capability == "runtime.linux.terminal.read" {
					hasReadPerm = true
					break
				}
			}
			assert.True(t, hasReadPerm, "%s should have read permission", tool.ID)
		}
	}
}

func createTestRuntimeHostForTools() *testToolsRuntimeHost {
	return &testToolsRuntimeHost{
		desc: platform.NewRuntimeDescriptor(platform.HostPlatformAndroid, platform.RuntimeKindProot, platform.GuestPlatformLinux),
		paths: util.RuntimePaths{
			WorkspaceDir: "/tmp/test",
		},
		caps: map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport{
			runtimehost.CapProcessSpawn:         runtimehost.SupportSupported,
			runtimehost.CapRuntimeSandboxedExec: runtimehost.SupportLimited,
		},
	}
}

type testToolsRuntimeHost struct {
	desc  platform.RuntimeDescriptor
	paths util.RuntimePaths
	caps  map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport
}

func (t *testToolsRuntimeHost) Descriptor() platform.RuntimeDescriptor {
	return t.desc
}

func (t *testToolsRuntimeHost) Capabilities() *testToolsCaps {
	return &testToolsCaps{support: t.caps}
}

func (t *testToolsRuntimeHost) Paths() util.RuntimePaths {
	return t.paths
}

func (t *testToolsRuntimeHost) RuntimeInstanceID() string {
	return "test-instance"
}

func (t *testToolsRuntimeHost) Processes() runtimehost.ProcessSupervisor {
	return nil
}

type testToolsCaps struct {
	support map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport
}

func (c *testToolsCaps) Support(id runtimehost.HostCapabilityID) runtimehost.CapabilitySupport {
	return c.support[id]
}

func (c *testToolsCaps) Supports(id runtimehost.HostCapabilityID) bool {
	return c.support[id] == runtimehost.SupportSupported
}

func (c *testToolsCaps) RequirementSatisfied(req runtimehost.CapabilityRequirement) bool {
	return c.support[req.ID] >= req.Minimum
}

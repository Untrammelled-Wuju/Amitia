//go:build linux && !android

package terminal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

func TestNewProvider(t *testing.T) {
	host := newTestRuntimeHost()
	host.paths.WorkspaceDir = "/tmp/test"

	provider, err := NewProvider(host, "/tmp/test", DefaultPolicy())
	assert.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestNewProviderWrongRuntime(t *testing.T) {
	host := &testRuntimeHost{
		desc: platform.NewRuntimeDescriptor(platform.HostPlatformWindows, platform.RuntimeKindNativeProcess, platform.GuestPlatformWindows),
		paths: util.RuntimePaths{
			WorkspaceDir: "/tmp/test",
		},
		caps: map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport{},
	}

	provider, err := NewProvider(host, "/tmp/test", DefaultPolicy())
	assert.Error(t, err)
	assert.Nil(t, provider)
}

func TestProviderExecuteUnknownOperation(t *testing.T) {
	host := newTestRuntimeHost()
	host.paths.WorkspaceDir = "/tmp/test"

	provider, err := NewProvider(host, "/tmp/test", DefaultPolicy())
	require.NoError(t, err)

	resp := provider.Execute(context.Background(), AndroidLinuxRequest{
		RequestID: "test-123",
		Operation: "unknown.operation",
	})

	assert.Equal(t, "error", resp.Status)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "invalid_operation", resp.Error.Code)
}

func TestProviderHealthNotReadyWithoutPTY(t *testing.T) {
	host := newTestRuntimeHost()
	host.paths.WorkspaceDir = "/tmp/test"

	provider, err := NewProvider(host, "/tmp/test", DefaultPolicy())
	require.NoError(t, err)

	status := provider.Health(context.Background())
	assert.NotEqual(t, HealthUnhealthy, status)
}

func TestProviderHandleOpen(t *testing.T) {
	host := newTestRuntimeHost()
	host.paths.WorkspaceDir = t.TempDir()

	provider, err := NewProvider(host, host.paths.WorkspaceDir, DefaultPolicy())
	require.NoError(t, err)

	resp := provider.Execute(context.Background(), AndroidLinuxRequest{
		RequestID: "open-test",
		Operation: OpOpen,
		Payload: map[string]any{
			"shell": "/bin/sh",
			"rows":  24,
			"cols":  80,
		},
	})

	if resp.Status == "success" {
		require.NotNil(t, resp.Result)
		assert.NotEmpty(t, resp.Result["sessionId"])
		assert.Equal(t, "running", resp.Result["state"])
	}
}

func TestProviderHandleStatus(t *testing.T) {
	host := newTestRuntimeHost()
	host.paths.WorkspaceDir = t.TempDir()

	provider, err := NewProvider(host, host.paths.WorkspaceDir, DefaultPolicy())
	require.NoError(t, err)

	resp := provider.Execute(context.Background(), AndroidLinuxRequest{
		RequestID: "status-test",
		Operation: OpStatus,
		Payload: map[string]any{
			"sessionId": "nonexistent",
		},
	})

	assert.Equal(t, "error", resp.Status)
	require.NotNil(t, resp.Error)
}

func TestExtractOwner(t *testing.T) {
	owner := extractOwner(map[string]any{
		"userId":         "user-123",
		"characterId":    "char-456",
		"conversationId": "conv-789",
	})

	assert.Equal(t, "user-123", owner.UserID)
	assert.Equal(t, "char-456", owner.CharacterID)
	assert.Equal(t, "conv-789", owner.ConversationID)

	owner = extractOwner(map[string]any{})
	assert.Empty(t, owner.UserID)
	assert.Empty(t, owner.CharacterID)
	assert.Empty(t, owner.ConversationID)
}

func TestToAndroidLinuxError(t *testing.T) {
	err := &Error{code: "test.code", message: "test message"}
	result := toAndroidLinuxError(err)
	require.NotNil(t, result)
	assert.Equal(t, "test.code", result.Code)
	assert.Equal(t, "test message", result.Message)

	assert.Nil(t, toAndroidLinuxError(nil))

	stdErr := &stdError{msg: "standard error"}
	result = toAndroidLinuxError(stdErr)
	require.NotNil(t, result)
	assert.Equal(t, "internal_error", result.Code)
	assert.Equal(t, "standard error", result.Message)
}

type stdError struct {
	msg string
}

func (e *stdError) Error() string {
	return e.msg
}

func TestProviderConstantValues(t *testing.T) {
	assert.Equal(t, "android_linux", string(RuntimeTypeAndroidLinux))
	assert.Equal(t, "terminal.open", OpOpen)
	assert.Equal(t, "terminal.write", OpWrite)
	assert.Equal(t, "terminal.read", OpRead)
	assert.Equal(t, "terminal.resize", OpResize)
	assert.Equal(t, "terminal.status", OpStatus)
	assert.Equal(t, "terminal.close", OpClose)
	assert.Equal(t, "terminal.cancel", OpCancel)
}

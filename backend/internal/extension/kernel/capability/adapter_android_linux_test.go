//go:build linux

package capability

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/u-ai/backend/internal/androidlinux/terminal"
)

type mockProvider struct {
	mock.Mock
}

func (m *mockProvider) Execute(ctx context.Context, request terminal.AndroidLinuxRequest) terminal.AndroidLinuxResponse {
	args := m.Called(ctx, request)
	return args.Get(0).(terminal.AndroidLinuxResponse)
}

func (m *mockProvider) Health(ctx context.Context) terminal.HealthStatus {
	args := m.Called(ctx)
	return args.Get(0).(terminal.HealthStatus)
}

func (m *mockProvider) CloseAll(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestAndroidLinuxRuntimeAdapterSupports(t *testing.T) {
	adapter := NewAndroidLinuxRuntimeAdapter(nil)

	assert.True(t, adapter.Supports(RuntimeBinding{RuntimeType: RuntimeTypeAndroidLinux}))
	assert.False(t, adapter.Supports(RuntimeBinding{RuntimeType: RuntimeTypeBuiltin}))
	assert.False(t, adapter.Supports(RuntimeBinding{RuntimeType: RuntimeTypeAndroid_Native}))
}

func TestAndroidLinuxRuntimeAdapterExecuteNilProvider(t *testing.T) {
	adapter := NewAndroidLinuxRuntimeAdapter(nil)
	invocation := ToolInvocationContext{
		InvocationID: "test-123",
		UserID:       "user-1",
	}

	result := adapter.Execute(context.Background(), RuntimeBinding{
		RuntimeType: RuntimeTypeAndroidLinux,
		HandlerName: "terminal.open",
	}, invocation, nil)

	assert.Equal(t, ToolResultStatusFailed, result.Status)
	require.NotNil(t, result.Error)
	assert.Equal(t, ErrorCodeNotAvailable, result.Error.Code)
}

func TestAndroidLinuxRuntimeAdapterExecuteSuccess(t *testing.T) {
	provider := new(mockProvider)
	adapter := NewAndroidLinuxRuntimeAdapter(provider)

	provider.On("Execute", mock.Anything, mock.Anything).Return(terminal.AndroidLinuxResponse{
		RequestID: "test-123",
		Status:    "success",
		Result: map[string]any{
			"sessionId": "sess-abc",
			"state":     "running",
			"rows":      24,
			"cols":      80,
		},
	})

	invocation := ToolInvocationContext{
		InvocationID:   "test-123",
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
	}

	result := adapter.Execute(context.Background(), RuntimeBinding{
		RuntimeType: RuntimeTypeAndroidLinux,
		HandlerName: "terminal.open",
	}, invocation, []byte(`{"shell":"/bin/sh"}`))

	assert.Equal(t, ToolResultStatusSuccess, result.Status)
	require.Nil(t, result.Error)
	assert.NotNil(t, result.Structured)

	provider.AssertCalled(t, "Execute", mock.Anything, mock.MatchedBy(func(req terminal.AndroidLinuxRequest) bool {
		return req.Operation == "terminal.open" &&
			req.Payload["userId"] == "user-1" &&
			req.Payload["characterId"] == "char-1" &&
			req.Payload["conversationId"] == "conv-1"
	}))
}

func TestAndroidLinuxRuntimeAdapterExecuteError(t *testing.T) {
	provider := new(mockProvider)
	adapter := NewAndroidLinuxRuntimeAdapter(provider)

	provider.On("Execute", mock.Anything, mock.Anything).Return(terminal.AndroidLinuxResponse{
		RequestID: "test-456",
		Status:    "error",
		Error: &terminal.AndroidLinuxError{
			Code:    terminal.ErrCodeSessionLimit,
			Message: "session limit reached",
		},
	})

	invocation := ToolInvocationContext{
		InvocationID: "test-456",
		UserID:       "user-2",
	}

	result := adapter.Execute(context.Background(), RuntimeBinding{
		RuntimeType: RuntimeTypeAndroidLinux,
		HandlerName: "terminal.open",
	}, invocation, nil)

	assert.Equal(t, ToolResultStatusFailed, result.Status)
	require.NotNil(t, result.Error)
	assert.Equal(t, ErrorCodeResourceLimitExceeded, result.Error.Code)
}

func TestAndroidLinuxRuntimeAdapterExecuteCancelled(t *testing.T) {
	provider := new(mockProvider)
	adapter := NewAndroidLinuxRuntimeAdapter(provider)

	provider.On("Execute", mock.Anything, mock.Anything).Return(terminal.AndroidLinuxResponse{
		RequestID: "test-789",
		Status:    "cancelled",
	})

	invocation := ToolInvocationContext{
		InvocationID: "test-789",
		UserID:       "user-3",
	}

	result := adapter.Execute(context.Background(), RuntimeBinding{
		RuntimeType: RuntimeTypeAndroidLinux,
		HandlerName: "terminal.open",
	}, invocation, nil)

	assert.Equal(t, ToolResultStatusCancelled, result.Status)
	require.NotNil(t, result.Error)
	assert.Equal(t, ErrorCodeCancelled, result.Error.Code)
}

func TestAndroidLinuxRuntimeAdapterHealth(t *testing.T) {
	provider := new(mockProvider)
	adapter := NewAndroidLinuxRuntimeAdapter(provider)

	provider.On("Health", mock.Anything).Return(terminal.HealthReady)

	status := adapter.Health(context.Background(), RuntimeBinding{
		RuntimeType: RuntimeTypeAndroidLinux,
	})

	assert.Equal(t, HealthReady, status)
}

func TestAndroidLinuxRuntimeAdapterMapTerminalError(t *testing.T) {
	tests := []struct {
		name     string
		err      *terminal.AndroidLinuxError
		expected string
	}{
		{
			name:     "not available",
			err:      &terminal.AndroidLinuxError{Code: terminal.ErrCodeNotAvailable},
			expected: ErrorCodeNotAvailable,
		},
		{
			name:     "session not found",
			err:      &terminal.AndroidLinuxError{Code: terminal.ErrCodeSessionNotFound},
			expected: ErrorCodeNotAvailable,
		},
		{
			name:     "scope denied",
			err:      &terminal.AndroidLinuxError{Code: terminal.ErrCodeScopeDenied},
			expected: ErrorCodeScopeDenied,
		},
		{
			name:     "input too large",
			err:      &terminal.AndroidLinuxError{Code: terminal.ErrCodeInputTooLarge},
			expected: ErrorCodeInvalidInput,
		},
		{
			name:     "session limit",
			err:      &terminal.AndroidLinuxError{Code: terminal.ErrCodeSessionLimit},
			expected: ErrorCodeResourceLimitExceeded,
		},
		{
			name:     "io failed",
			err:      &terminal.AndroidLinuxError{Code: terminal.ErrCodeIOFailed},
			expected: ErrorCodeExecutionFailed,
		},
		{
			name:     "unknown code",
			err:      &terminal.AndroidLinuxError{Code: "unknown.code"},
			expected: ErrorCodeExecutionFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapTerminalError(tt.err)
			assert.Equal(t, tt.expected, result.Code)
		})
	}
}

func TestAndroidLinuxRuntimeAdapterHandleCtxError(t *testing.T) {
	result := handleCtxError("test-123", context.DeadlineExceeded)
	assert.Equal(t, ToolResultStatusTimedOut, result.Status)
	require.NotNil(t, result.Error)
	assert.Equal(t, ErrorCodeTimeout, result.Error.Code)

	result = handleCtxError("test-456", context.Canceled)
	assert.Equal(t, ToolResultStatusCancelled, result.Status)
	require.NotNil(t, result.Error)
	assert.Equal(t, ErrorCodeCancelled, result.Error.Code)

	result = handleCtxError("test-789", errors.New("other error"))
	assert.Equal(t, ToolResultStatusFailed, result.Status)
	require.NotNil(t, result.Error)
}

func TestAndroidLinuxRuntimeAdapterExtractSessionID(t *testing.T) {
	assert.Equal(t, terminal.SessionID("sess-123"), extractSessionID(map[string]any{
		"sessionId": "sess-123",
	}))
	assert.Equal(t, terminal.SessionID(""), extractSessionID(map[string]any{}))
	assert.Equal(t, terminal.SessionID(""), extractSessionID(nil))
}

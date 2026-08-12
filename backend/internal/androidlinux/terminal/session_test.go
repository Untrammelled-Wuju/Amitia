//go:build linux && !android

package terminal

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

type testRuntimeHost struct {
	desc  platform.RuntimeDescriptor
	paths util.RuntimePaths
	caps  map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport
	pid   string
}

func (t *testRuntimeHost) Descriptor() platform.RuntimeDescriptor {
	return t.desc
}

func (t *testRuntimeHost) Capabilities() *testCaps {
	return &testCaps{support: t.caps}
}

func (t *testRuntimeHost) Paths() util.RuntimePaths {
	return t.paths
}

func (t *testRuntimeHost) RuntimeInstanceID() string {
	return t.pid
}

func (t *testRuntimeHost) Processes() runtimehost.ProcessSupervisor {
	return nil
}

type testCaps struct {
	support map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport
}

func (c *testCaps) Support(id runtimehost.HostCapabilityID) runtimehost.CapabilitySupport {
	return c.support[id]
}

func (c *testCaps) Supports(id runtimehost.HostCapabilityID) bool {
	return c.support[id] == runtimehost.SupportSupported
}

func (c *testCaps) RequirementSatisfied(req runtimehost.CapabilityRequirement) bool {
	return c.support[req.ID] >= req.Minimum
}

func newTestRuntimeHost() *testRuntimeHost {
	return &testRuntimeHost{
		desc: platform.NewRuntimeDescriptor(platform.HostPlatformAndroid, platform.RuntimeKindProot, platform.GuestPlatformLinux),
		paths: util.RuntimePaths{WorkspaceDir: "/tmp/test-workspace"},
		caps: map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport{
			runtimehost.CapProcessSpawn:         runtimehost.SupportSupported,
			runtimehost.CapRuntimeSandboxedExec: runtimehost.SupportLimited,
		},
		pid: "test-instance",
	}
}

func TestIsAndroidLinuxRuntime(t *testing.T) {
	androidHost := newTestRuntimeHost()

	nonProotHost := &testRuntimeHost{
		desc:  platform.NewRuntimeDescriptor(platform.HostPlatformAndroid, platform.RuntimeKindNativeProcess, platform.GuestPlatformAndroid),
		paths: util.RuntimePaths{WorkspaceDir: "/tmp"},
		caps: map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport{
			runtimehost.CapProcessSpawn:         runtimehost.SupportSupported,
			runtimehost.CapRuntimeSandboxedExec: runtimehost.SupportLimited,
		},
	}

	linuxHost := &testRuntimeHost{
		desc:  platform.NewRuntimeDescriptor(platform.HostPlatformLinux, platform.RuntimeKindNativeProcess, platform.GuestPlatformLinux),
		paths: util.RuntimePaths{WorkspaceDir: "/tmp"},
		caps: map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport{
			runtimehost.CapProcessSpawn:         runtimehost.SupportSupported,
			runtimehost.CapRuntimeSandboxedExec: runtimehost.SupportLimited,
		},
	}

	assert.True(t, IsAndroidLinuxRuntime(androidHost))
	assert.False(t, IsAndroidLinuxRuntime(nonProotHost))
	assert.False(t, IsAndroidLinuxRuntime(linuxHost))
	assert.False(t, IsAndroidLinuxRuntime(nil))
}

func TestValidateShell(t *testing.T) {
	tests := []struct {
		name    string
		shell   string
		wantErr bool
	}{
		{
			name:    "valid shell /bin/sh",
			shell:   "/bin/sh",
			wantErr: false,
		},
		{
			name:    "valid shell /bin/bash",
			shell:   "/bin/bash",
			wantErr: false,
		},
		{
			name:    "empty shell",
			shell:   "",
			wantErr: true,
		},
		{
			name:    "disallowed shell",
			shell:   "/tmp/malware",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateShell(tt.shell, DefaultShellAllowlist)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResolveWorkingDir(t *testing.T) {
	tests := []struct {
		name         string
		cwd          string
		workspaceDir string
		wantErr      bool
	}{
		{
			name:         "empty cwd returns workspace dir",
			cwd:          "",
			workspaceDir: "/workspace",
			wantErr:      false,
		},
		{
			name:         "valid relative cwd",
			cwd:          "subdir",
			workspaceDir: "/workspace",
			wantErr:      false,
		},
		{
			name:         "path traversal blocked",
			cwd:          "../../etc",
			workspaceDir: "/workspace",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolveWorkingDir(tt.cwd, tt.workspaceDir)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.cwd == "" {
					assert.Equal(t, tt.workspaceDir, result)
				}
			}
		})
	}
}

func TestBuildEnvironment(t *testing.T) {
	env := buildEnvironment("/home/test")

	assert.Contains(t, env, "PATH=/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin")
	assert.Contains(t, env, "HOME=/home/test")
	assert.Contains(t, env, "TERM=xterm-256color")
	assert.Contains(t, env, "LANG=en_US.UTF-8")
	assert.Contains(t, env, "LC_ALL=en_US.UTF-8")

	for _, e := range env {
		assert.NotContains(t, e, "SECRET")
		assert.NotContains(t, e, "TOKEN")
		assert.NotContains(t, e, "API_KEY")
	}
}

func TestDefaultPolicy(t *testing.T) {
	policy := DefaultPolicy()

	assert.Equal(t, DefaultMaxSessions, policy.MaxSessions)
	assert.Equal(t, DefaultMaxSessionsPerUser, policy.MaxSessionsPerUser)
	assert.Equal(t, DefaultMaxSessionsPerConversation, policy.MaxSessionsPerConversation)
	assert.Equal(t, DefaultMaxBufferedOutputBytes, policy.MaxBufferedOutputBytes)
	assert.Equal(t, DefaultIdleTimeout, policy.IdleTimeout)
	assert.Equal(t, DefaultCloseGracePeriod, policy.CloseGracePeriod)
}

func TestSessionStateTransitions(t *testing.T) {
	sess := &Session{
		State: SessionStarting,
	}

	assert.True(t, sess.IsActive())
	assert.Equal(t, SessionStarting, sess.GetState())

	sess.SetState(SessionRunning)
	assert.True(t, sess.IsActive())
	assert.Equal(t, SessionRunning, sess.GetState())

	sess.SetState(SessionExited)
	assert.False(t, sess.IsActive())
	assert.Equal(t, SessionExited, sess.GetState())
}

func TestSessionOwnership(t *testing.T) {
	owner := SessionOwner{
		UserID:         "user123",
		CharacterID:    "char456",
		ConversationID: "conv789",
	}

	sess := &Session{
		Owner: owner,
		SessionOwner: owner,
		State: SessionRunning,
	}

	assert.True(t, sess.BelongsTo(owner))

	otherOwner := SessionOwner{
		UserID:         "user999",
		CharacterID:    "char456",
		ConversationID: "conv789",
	}
	assert.False(t, sess.BelongsTo(otherOwner))
}

func TestNewSessionID(t *testing.T) {
	id1 := NewSessionID()
	id2 := NewSessionID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}

func TestIsAndroidLinuxRuntimeNilCapabilities(t *testing.T) {
	host := newTestRuntimeHost()
	host.caps = nil

	result := IsAndroidLinuxRuntime(host)
	assert.False(t, result)
}

func TestSessionManagerCloseAll(t *testing.T) {
	host := newTestRuntimeHost()
	host.paths.WorkspaceDir = t.TempDir()

	manager := NewSessionManager(host, DefaultPolicy())
	require.NotNil(t, manager)

	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, _, _, _, err := manager.Open(ctx, OpenParams{
			Owner:      SessionOwner{UserID: "testuser"},
			Shell:      "/bin/sh",
			WorkingDir: host.paths.WorkspaceDir,
		})
		require.NoError(t, err)
	}

	assert.Equal(t, 3, manager.ActiveCount())

	err := manager.CloseAll(ctx)
	require.NoError(t, err)

	assert.Equal(t, 0, manager.ActiveCount())
}

func TestOutputBufferAppendAndRead(t *testing.T) {
	buf := NewOutputBuffer(1024)

	now := time.Now()
	chunk1 := buf.Append(TerminalStreamPTY, []byte("hello"), now)
	assert.Equal(t, uint64(1), chunk1.Sequence)

	chunk2 := buf.Append(TerminalStreamPTY, []byte(" world"), now.Add(time.Second))
	assert.Equal(t, uint64(2), chunk2.Sequence)

	chunks, nextSeq, truncated := buf.Read(0, 1024)
	assert.False(t, truncated)
	assert.Equal(t, uint64(2), nextSeq)
	assert.Len(t, chunks, 2)

	chunks, nextSeq, truncated = buf.Read(1, 1024)
	assert.False(t, truncated)
	assert.Len(t, chunks, 1)
	assert.Equal(t, " world", string(chunks[0].Data))
}

func TestOutputBufferTruncation(t *testing.T) {
	buf := NewOutputBuffer(10)

	for i := 0; i < 5; i++ {
		buf.Append(TerminalStreamPTY, []byte("test data"), time.Now())
	}

	chunks, _, truncated := buf.Read(0, 1024)
	assert.True(t, truncated || buf.ChunkCount() < 5)
}

func TestErrorCodes(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		code string
	}{
		{
			name: "not available",
			err:  ErrNotAvailable("test"),
			code: ErrCodeNotAvailable,
		},
		{
			name: "session not found",
			err:  ErrSessionNotFound("test-id"),
			code: ErrCodeSessionNotFound,
		},
		{
			name: "input too large",
			err:  ErrInputTooLarge(100),
			code: ErrCodeInputTooLarge,
		},
		{
			name: "scope denied",
			err:  ErrScopeDenied(),
			code: ErrCodeScopeDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.code, tt.err.Code())
			assert.NotEmpty(t, tt.err.Error())
		})
	}
}

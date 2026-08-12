//go:build linux && !android

package terminal

import (
	"context"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/runtimehost"
)

type SessionID string

func NewSessionID() SessionID {
	return SessionID(uuid.NewString())
}

type SessionState string

const (
	SessionStarting  SessionState = "starting"
	SessionRunning   SessionState = "running"
	SessionExited    SessionState = "exited"
	SessionClosing   SessionState = "closing"
	SessionCancelled SessionState = "cancelled"
	SessionFailed    SessionState = "failed"
)

type TerminalStream string

const (
	TerminalStreamPTY TerminalStream = "pty"
)

type SessionOwner struct {
	UserID         string
	CharacterID    string
	ConversationID string
}

type Session struct {
	ID                   SessionID
	Owner                SessionOwner
	Shell                string
	WorkingDir           string
	State                SessionState
	PID                  int
	Rows                 uint16
	Cols                 uint16
	CreatedAt            time.Time
	StartedAt            time.Time
	ExitedAt             *time.Time
	LastActivity         time.Time
	ExitCode             *int
	CreationInvocationID string

	ptmx      *os.File
	cmd       *exec.Cmd
	cancel    context.CancelFunc

	writeMu   sync.Mutex
	stateMu   sync.RWMutex
	closeCh chan struct{}
	output  *OutputBuffer
}

type Policy struct {
	MaxSessions                int
	MaxSessionsPerUser         int
	MaxSessionsPerConversation int
	MaxBufferedOutputBytes     int
	IdleTimeout                time.Duration
	CloseGracePeriod           time.Duration
}

type Clock interface {
	Now() time.Time
}

type defaultClock struct{}

func (defaultClock) Now() time.Time {
	return time.Now()
}

type SessionManager struct {
	mu       sync.RWMutex
	host     runtimehost.RuntimeHost
	sessions map[SessionID]*Session
	policy   Policy
	clock    Clock
	done     chan struct{}
}

type OutputChunk struct {
	Sequence  uint64
	Stream    TerminalStream
	Data      []byte
	Timestamp time.Time
}

type OutputBuffer struct {
	mu       sync.RWMutex
	chunks   []OutputChunk
	sequence uint64
	maxBytes int
	head     int
	size     int
	startSeq uint64
}

const (
	DefaultMaxSessions                = 16
	DefaultMaxSessionsPerUser         = 8
	DefaultMaxSessionsPerConversation = 4
	DefaultMaxBufferedOutputBytes     = 4 * 1024 * 1024
	DefaultIdleTimeout                = 30 * time.Minute
	DefaultCloseGracePeriod           = 3 * time.Second
	DefaultMaxStdinSize               = 64 * 1024
	DefaultMaxReadOutputSize          = 256 * 1024
	DefaultInitialRows                = 24
	DefaultInitialCols                = 80
)

func DefaultPolicy() Policy {
	return Policy{
		MaxSessions:                DefaultMaxSessions,
		MaxSessionsPerUser:         DefaultMaxSessionsPerUser,
		MaxSessionsPerConversation: DefaultMaxSessionsPerConversation,
		MaxBufferedOutputBytes:     DefaultMaxBufferedOutputBytes,
		IdleTimeout:                DefaultIdleTimeout,
		CloseGracePeriod:           DefaultCloseGracePeriod,
	}
}

func (s *Session) IsActive() bool {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.State == SessionStarting || s.State == SessionRunning
}

func (s *Session) SetState(state SessionState) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.State = state
}

func (s *Session) GetState() SessionState {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.State
}

func (s *Session) BelongsTo(owner SessionOwner) bool {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.Owner.UserID == owner.UserID &&
		s.Owner.CharacterID == owner.CharacterID &&
		s.Owner.ConversationID == owner.ConversationID
}

func (s *Session) RecordActivity() {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.LastActivity = time.Now()
}

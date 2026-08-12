package screenframe

import (
	"context"
	"sync"
	"time"
)

type ScreenFrameSessionID string

type SessionState string

const (
	SessionStateStarting          SessionState = "starting"
	SessionStateAwaitingPermission SessionState = "awaiting_permission"
	SessionStateRunning           SessionState = "running"
	SessionStateStopping          SessionState = "stopping"
	SessionStateStopped           SessionState = "stopped"
	SessionStateProjectionRevoked SessionState = "projection_revoked"
	SessionStateFailed            SessionState = "failed"
)

type SessionOwner struct {
	UserID         string `json:"userId"`
	CharacterID    string `json:"characterId"`
	ConversationID string `json:"conversationId"`
}

type ScreenFrameSession struct {
	ID        ScreenFrameSessionID `json:"id"`
	Owner     SessionOwner         `json:"owner"`
	DisplayID int                  `json:"displayId"`
	Width     int                  `json:"width"`
	Height    int                  `json:"height"`

	TargetFPS float64 `json:"targetFps"`

	State       SessionState `json:"state"`
	CaptureGeneration uint64 `json:"generation"`

	LastFrameSequence uint64    `json:"lastFrameSequence"`
	LastFrameAt       time.Time `json:"lastFrameAt"`

	StartedAt time.Time `json:"startedAt"`
	MaxAge    time.Time `json:"maxAge"`

	cancel context.CancelFunc `json:"-"`
}

func (s *ScreenFrameSession) Active() bool {
	return s.State == SessionStateRunning || s.State == SessionStateStarting || s.State == SessionStateAwaitingPermission
}

type ScreenFrameSessionStore interface {
	Create(ctx context.Context, session ScreenFrameSession) (*ScreenFrameSession, error)
	Get(ctx context.Context, id ScreenFrameSessionID) (*ScreenFrameSession, error)
	ListActive(ctx context.Context) ([]*ScreenFrameSession, error)
	ListByUser(ctx context.Context, userID string) ([]*ScreenFrameSession, error)
	ListByConversation(ctx context.Context, conversationID string) ([]*ScreenFrameSession, error)
	UpdateState(ctx context.Context, id ScreenFrameSessionID, state SessionState, generation uint64) error
	Delete(ctx context.Context, id ScreenFrameSessionID) error
	StopAll(ctx context.Context) error
}

type blockedSessionStore struct {
	mu       sync.RWMutex
	limit    int
	sessions map[ScreenFrameSessionID]*ScreenFrameSession
}

func NewBlockedSessionStore(limit int) ScreenFrameSessionStore {
	if limit <= 0 {
		limit = 1
	}
	return &blockedSessionStore{
		limit:    limit,
		sessions: make(map[ScreenFrameSessionID]*ScreenFrameSession),
	}
}

func (s *blockedSessionStore) Create(ctx context.Context, session ScreenFrameSession) (*ScreenFrameSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	totalActive := 0
	for _, existing := range s.sessions {
		if existing.Active() {
			totalActive++
		}
	}
	if totalActive >= s.limit {
		return nil, NewFrameError(ErrSessionAlreadyActive, "screen frame capture session already active")
	}

	session.State = SessionStateFailed
	session.StartedAt = time.Now()
	session.MaxAge = session.StartedAt.Add(5 * time.Minute)
	session.LastFrameAt = time.Time{}

	s.sessions[session.ID] = &session
	return nil, NewFrameError(ErrBlockedNativeHost, "android native host source not available; screen frame capture blocked")
}

func (s *blockedSessionStore) Get(ctx context.Context, id ScreenFrameSessionID) (*ScreenFrameSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[id]
	if !ok {
		return nil, NewFrameError(ErrSessionNotFound, "frame capture session not found: "+string(id))
	}
	return sess, nil
}

func (s *blockedSessionStore) ListActive(ctx context.Context) ([]*ScreenFrameSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ScreenFrameSession
	for _, sess := range s.sessions {
		if sess.Active() {
			result = append(result, sess)
		}
	}
	return result, nil
}

func (s *blockedSessionStore) ListByUser(ctx context.Context, userID string) ([]*ScreenFrameSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ScreenFrameSession
	for _, sess := range s.sessions {
		if sess.Owner.UserID == userID {
			result = append(result, sess)
		}
	}
	return result, nil
}

func (s *blockedSessionStore) ListByConversation(ctx context.Context, conversationID string) ([]*ScreenFrameSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ScreenFrameSession
	for _, sess := range s.sessions {
		if sess.Owner.ConversationID == conversationID {
			result = append(result, sess)
		}
	}
	return result, nil
}

func (s *blockedSessionStore) UpdateState(ctx context.Context, id ScreenFrameSessionID, state SessionState, generation uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[id]
	if !ok {
		return NewFrameError(ErrSessionNotFound, "frame capture session not found")
	}
	sess.State = state
	if generation > 0 {
		sess.CaptureGeneration = generation
	}
	return nil
}

func (s *blockedSessionStore) Delete(ctx context.Context, id ScreenFrameSessionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[id]; !ok {
		return NewFrameError(ErrSessionNotFound, "frame capture session not found")
	}
	delete(s.sessions, id)
	return nil
}

func (s *blockedSessionStore) StopAll(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sess := range s.sessions {
		if sess.Active() {
			sess.State = SessionStateStopped
		}
	}
	return nil
}

func NewFrameErrorAsValue[T any](value T, err error) (T, error) {
	return value, err
}

type LatestRequest struct {
	SessionID      ScreenFrameSessionID `json:"sessionId"`
	AfterSequence  *uint64              `json:"afterSequence,omitempty"`
	WaitMs         int                  `json:"waitMs,omitempty"`
	Format         *ScreenshotFormat    `json:"format,omitempty"`
	Quality        *int                 `json:"quality,omitempty"`
	MaxWidth       *int                 `json:"maxWidth,omitempty"`
	MaxHeight      *int                 `json:"maxHeight,omitempty"`
	AllowStale     bool                 `json:"allowStale,omitempty"`
}

func (r LatestRequest) WaitDuration() time.Duration {
	if r.WaitMs <= 0 {
		return 0
	}
	if r.WaitMs > 5000 {
		return 5 * time.Second
	}
	return time.Duration(r.WaitMs) * time.Millisecond
}

func (r LatestRequest) ResolveFormat() ScreenshotFormat {
	if r.Format != nil && r.Format.IsValid() {
		return *r.Format
	}
	return FormatJPEG
}

type LatestResult struct {
	HasFrame     bool   `json:"hasFrame"`
	SessionID    ScreenFrameSessionID `json:"sessionId"`
	Sequence     uint64 `json:"sequence,omitempty"`
	Generation   uint64 `json:"generation,omitempty"`
	ResourceURI  string `json:"resourceUri,omitempty"`
	MIMEType     string `json:"mimeType,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	CapturedAt   int64  `json:"capturedAt,omitempty"`
	AgeMs        int64  `json:"ageMs,omitempty"`
	DroppedSincePrevious int64 `json:"droppedSincePrevious,omitempty"`
}

type FrameSnapshot struct {
	Sequence   uint64
	Generation uint64
	Width      int
	Height     int
	Timestamp  time.Time
	Buffer     []byte
	Ref        string
}

func (f FrameSnapshot) IsValid() bool {
	return f.Sequence > 0 && f.Width > 0 && f.Height > 0 && len(f.Buffer) > 0 && !f.Timestamp.IsZero()
}

type StartRequest struct {
	DisplayID *int           `json:"displayId,omitempty"`
	TargetFPS *float64       `json:"targetFps,omitempty"`
	MaxWidth  *int           `json:"maxWidth,omitempty"`
	MaxHeight *int           `json:"maxHeight,omitempty"`
}

func (r StartRequest) Validate(p ScreenFramePolicy) error {
	if r.TargetFPS != nil {
		fps := *r.TargetFPS
		if fps <= 0 {
			return NewFrameError(ErrInvalidFPS, "targetFps must be greater than 0")
		}
		if fps > p.MaxFPS {
			return NewFrameError(ErrInvalidFPS, "targetFps exceeds maximum allowed")
		}
	}
	if r.MaxWidth != nil && *r.MaxWidth <= 0 {
		return NewFrameError(ErrInvalidSize, "maxWidth must be positive")
	}
	if r.MaxHeight != nil && *r.MaxHeight <= 0 {
		return NewFrameError(ErrInvalidSize, "maxHeight must be positive")
	}
	if r.DisplayID != nil && *r.DisplayID < 0 {
		return NewFrameError(ErrInvalidDisplay, "displayId must be non-negative")
	}
	return nil
}

func (r StartRequest) ResolveFPS(p ScreenFramePolicy) float64 {
	if r.TargetFPS != nil && *r.TargetFPS > 0 && *r.TargetFPS <= p.MaxFPS {
		return *r.TargetFPS
	}
	return p.DefaultFPS
}

func (r StartRequest) ResolveMaxWidth(p ScreenFramePolicy) int {
	if r.MaxWidth != nil && *r.MaxWidth > 0 && *r.MaxWidth <= p.MaxWidth {
		return *r.MaxWidth
	}
	return p.MaxWidth
}

func (r StartRequest) ResolveMaxHeight(p ScreenFramePolicy) int {
	if r.MaxHeight != nil && *r.MaxHeight > 0 && *r.MaxHeight <= p.MaxHeight {
		return *r.MaxHeight
	}
	return p.MaxHeight
}

func (r StartRequest) ResolveDisplayID() int {
	if r.DisplayID != nil {
		return *r.DisplayID
	}
	return 0
}

type StartResult struct {
	SessionID ScreenFrameSessionID `json:"sessionId"`
	State     SessionState         `json:"state"`
	DisplayID int                  `json:"displayId"`
	Width     int                  `json:"width"`
	Height    int                  `json:"height"`
	TargetFPS float64              `json:"targetFps"`
	Generation uint64             `json:"generation"`
}

type StopResult struct {
	SessionID ScreenFrameSessionID `json:"sessionId"`
	State     SessionState         `json:"state"`
}

type StatusResult struct {
	Supported          bool    `json:"supported"`
	PermissionState    string  `json:"permissionState"`
	ActiveSession      bool    `json:"activeSession"`
	SessionID          string  `json:"sessionId,omitempty"`
	DisplayID          int     `json:"displayId,omitempty"`
	Width              int     `json:"width,omitempty"`
	Height             int     `json:"height,omitempty"`
	TargetFPS          float64 `json:"targetFps,omitempty"`
	LastFrameSequence  uint64  `json:"lastFrameSequence,omitempty"`
	LastFrameAt        int64   `json:"lastFrameAt,omitempty"`
	UserActionRequired bool    `json:"userActionRequired"`
	State              string  `json:"state,omitempty"`
}

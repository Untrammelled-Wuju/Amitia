package realtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ContinuousVoiceSession struct {
	SessionID          string                        `json:"sessionId"`
	ConversationID     string                        `json:"conversationId"`
	CharacterID        string                        `json:"characterId"`
	Mode               ContinuousVoiceSessionMode    `json:"mode"`
	State              ContinuousVoiceSessionStatus  `json:"state"`
	UserID             string                        `json:"userId"`
	Platform           Platform                      `json:"platform"`
	ProfileID          string                        `json:"profileId"`
	Plan               *VoiceExecutionPlan           `json:"plan,omitempty"`

	CurrentTurnID      string `json:"currentTurnId"`
	CurrentPlaybackID  string `json:"currentPlaybackId"`
	CaptureGeneration  uint64 `json:"captureGeneration"`
	PlaybackGeneration uint64 `json:"playbackGeneration"`

	WakeArmed          bool      `json:"wakeArmed"`
	LastActivityAt     time.Time `json:"lastActivityAt"`
	CreatedAt          time.Time `json:"createdAt"`
	EndedAt            time.Time `json:"endedAt,omitempty"`

	mu                 sync.RWMutex
	cancelFn           context.CancelFunc

	vad                VoiceActivityDetector
	endpoint           *EndpointDetector
	wake               WakeDetector
	stateBeforeSuspend ContinuousVoiceSessionStatus
}

type Service interface {
	CreateSession(ctx context.Context, req VoiceSessionRequest) (*ContinuousVoiceSession, error)
	GetSession(sessionID string) (*ContinuousVoiceSession, error)
	StartSession(ctx context.Context, sessionID string) error
	StopSession(ctx context.Context, sessionID string) error
	InterruptSession(ctx context.Context, sessionID string) error
	ArmWake(ctx context.Context, sessionID string) error
	DisarmWake(ctx context.Context, sessionID string) error
	ListActiveSessions() []*ContinuousVoiceSession
	HandleAudioFrame(ctx context.Context, sessionID string, frame *VoiceAudioFrame) error
	Status() ServiceStatus
}

type ServiceStatus struct {
	ActiveSessions    int       `json:"activeSessions"`
	WakeArmedSessions int       `json:"wakeArmedSessions"`
	Healthy           bool      `json:"healthy"`
	UptimeSeconds     int64     `json:"uptimeSeconds"`
}

type service struct {
	mu        sync.RWMutex
	sessions  map[string]*ContinuousVoiceSession
	planner   VoiceRoutePlanner
	startedAt time.Time
}

func NewService() Service {
	return &service{
		sessions:  make(map[string]*ContinuousVoiceSession),
		planner:   NewRoutePlanner(),
		startedAt: time.Now(),
	}
}

func (s *service) CreateSession(ctx context.Context, req VoiceSessionRequest) (*ContinuousVoiceSession, error) {
	if req.SessionID == "" {
		req.SessionID = uuid.New().String()
	}
	if req.Mode == "" {
		req.Mode = ContinuousVoiceSessionModePushToTalk
	}

	sess := &ContinuousVoiceSession{
		SessionID:      req.SessionID,
		ConversationID: req.ConversationID,
		CharacterID:    req.CharacterID,
		Mode:           req.Mode,
		UserID:         req.UserID,
		Platform:       req.Platform,
		ProfileID:      req.ProfileID,
		State:          ContinuousVoiceSessionStatusIdle,
		WakeArmed:      req.Mode == ContinuousVoiceSessionModeWakeArmed,
		CreatedAt:      time.Now(),
		LastActivityAt: time.Now(),
	}

	sess.vad = NewSoftwareVAD(0.02)
	sess.endpoint = NewEndpointDetector(DefaultEndpointConfig())

	if sess.WakeArmed {
		sess.wake = NewSoftwareWake()
	}

	s.mu.Lock()
	s.sessions[req.SessionID] = sess
	s.mu.Unlock()

	return sess, nil
}

func (s *service) GetSession(sessionID string) (*ContinuousVoiceSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("voice session not found: %s", sessionID)
	}
	return sess, nil
}

func (s *service) StartSession(ctx context.Context, sessionID string) error {
	sess, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	if sess.State != ContinuousVoiceSessionStatusIdle {
		return fmt.Errorf("voice session: cannot start from state=%s", sess.State)
	}

	sess.State = ContinuousVoiceSessionStatusStarting
	sess.Plan = &VoiceExecutionPlan{
		Path:            "local_vad_segment_asr_full_tts",
		UseLocalVAD:     true,
		UseSegmentASR:   true,
		UseFullTTS:      true,
		RequiresNetwork: true,
	}

	switch sess.Mode {
	case ContinuousVoiceSessionModePushToTalk, ContinuousVoiceSessionModeOpenMic:
		sess.State = ContinuousVoiceSessionStatusListening
	case ContinuousVoiceSessionModeWakeArmed:
		sess.State = ContinuousVoiceSessionStatusArmed
	case ContinuousVoiceSessionModeProviderRealtime:
		sess.State = ContinuousVoiceSessionStatusTranscribing
	}

	sess.LastActivityAt = time.Now()
	return nil
}

func (s *service) StopSession(ctx context.Context, sessionID string) error {
	sess, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	if sess.State.IsTerminal() {
		return nil
	}

	sess.State = ContinuousVoiceSessionStatusStopping

	if sess.vad != nil {
		sess.vad.Reset()
	}
	if sess.endpoint != nil {
		sess.endpoint.Reset()
	}
	if sess.wake != nil {
		sess.wake.Reset()
	}
	if sess.cancelFn != nil {
		sess.cancelFn()
	}

	sess.State = ContinuousVoiceSessionStatusEnded
	sess.EndedAt = time.Now()

	return nil
}

func (s *service) InterruptSession(ctx context.Context, sessionID string) error {
	sess, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	if sess.State == ContinuousVoiceSessionStatusSpeaking {
		sess.PlaybackGeneration++
		sess.State = ContinuousVoiceSessionStatusListening
		sess.LastActivityAt = time.Now()
		return nil
	}

	if sess.State == ContinuousVoiceSessionStatusProcessing {
		sess.State = ContinuousVoiceSessionStatusListening
		sess.LastActivityAt = time.Now()
		return nil
	}

	return nil
}

func (s *service) ArmWake(ctx context.Context, sessionID string) error {
	sess, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	sess.WakeArmed = true
	if sess.State == ContinuousVoiceSessionStatusIdle || sess.State == ContinuousVoiceSessionStatusListening {
		sess.State = ContinuousVoiceSessionStatusArmed
	}
	return nil
}

func (s *service) DisarmWake(ctx context.Context, sessionID string) error {
	sess, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	sess.WakeArmed = false
	if sess.State == ContinuousVoiceSessionStatusArmed {
		sess.State = ContinuousVoiceSessionStatusIdle
	}
	return nil
}

func (s *service) ListActiveSessions() []*ContinuousVoiceSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ContinuousVoiceSession, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sess.mu.RLock()
		if !sess.State.IsTerminal() {
			result = append(result, sess)
		}
		sess.mu.RUnlock()
	}
	return result
}

func (s *service) HandleAudioFrame(ctx context.Context, sessionID string, frame *VoiceAudioFrame) error {
	sess, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	if sess.State.IsTerminal() {
		return fmt.Errorf("voice session: terminal state=%s", sess.State)
	}

	sess.LastActivityAt = time.Now()

	switch sess.State {
	case ContinuousVoiceSessionStatusArmed:
		if sess.wake != nil {
			result, err := sess.wake.Process(frame)
			if err == nil && result.Detected {
				sess.State = ContinuousVoiceSessionStatusListening
				sess.WakeArmed = false
			}
		}

	case ContinuousVoiceSessionStatusListening:
		if sess.vad != nil {
			vadResult, err := sess.vad.Process(frame)
			if err == nil && vadResult.SpeechStarted {
				sess.State = ContinuousVoiceSessionStatusTranscribing
				sess.CurrentTurnID = uuid.New().String()
				sess.CaptureGeneration++
			}
		}

	case ContinuousVoiceSessionStatusTranscribing:
		if sess.vad != nil {
			vadResult, err := sess.vad.Process(frame)
			if err == nil && vadResult.SpeechEnded {
				sess.State = ContinuousVoiceSessionStatusProcessing
				sess.PlaybackGeneration++
			}
		}

	case ContinuousVoiceSessionStatusSpeaking:
		if sess.vad != nil {
			vadResult, err := sess.vad.Process(frame)
			if err == nil && vadResult.Speech {
				sess.State = ContinuousVoiceSessionStatusListening
				sess.PlaybackGeneration++
				sess.CurrentTurnID = uuid.New().String()
				sess.CaptureGeneration++
			}
		}
	}

	return nil
}

func (s *service) Status() ServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wakeArmed := 0
	for _, sess := range s.sessions {
		if sess.WakeArmed {
			wakeArmed++
		}
	}

	return ServiceStatus{
		ActiveSessions:    len(s.sessions),
		WakeArmedSessions: wakeArmed,
		Healthy:           true,
		UptimeSeconds:     int64(time.Since(s.startedAt).Seconds()),
	}
}

func (sess *ContinuousVoiceSession) IsActive() bool {
	sess.mu.RLock()
	defer sess.mu.RUnlock()
	return sess.State.IsActive()
}

func (sess *ContinuousVoiceSession) TransitionTo(target ContinuousVoiceSessionStatus) error {
	sess.mu.Lock()
	defer sess.mu.Unlock()

	if !sess.State.CanTransitionTo(target) {
		return fmt.Errorf("voice session: invalid transition %s -> %s", sess.State, target)
	}
	sess.State = target
	sess.LastActivityAt = time.Now()
	return nil
}

package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ContinuousVoiceSession struct {
	SessionID      string                       `json:"sessionId"`
	ConversationID string                       `json:"conversationId"`
	CharacterID    string                       `json:"characterId"`
	Mode           ContinuousVoiceSessionMode   `json:"mode"`
	State          ContinuousVoiceSessionStatus `json:"state"`
	UserID         string                       `json:"userId"`
	Platform       Platform                     `json:"platform"`
	ProfileID      string                       `json:"profileId"`
	WakeConfigID   string                       `json:"wakeConfigId,omitempty"`
	Plan           *VoiceExecutionPlan          `json:"plan,omitempty"`

	CurrentTurnID      string `json:"currentTurnId"`
	CurrentPlaybackID  string `json:"currentPlaybackId"`
	CaptureGeneration  uint64 `json:"captureGeneration"`
	PlaybackGeneration uint64 `json:"playbackGeneration"`

	WakeArmed      bool      `json:"wakeArmed"`
	LastActivityAt time.Time `json:"lastActivityAt"`
	CreatedAt      time.Time `json:"createdAt"`
	EndedAt        time.Time `json:"endedAt,omitempty"`

	mu       sync.RWMutex
	cancelFn context.CancelFunc

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
	PublishASRFinal(ctx context.Context, sessionID, transcript, eventID string) error
	Status() ServiceStatus
}

type ServiceStatus struct {
	ActiveSessions    int   `json:"activeSessions"`
	WakeArmedSessions int   `json:"wakeArmedSessions"`
	Healthy           bool  `json:"healthy"`
	UptimeSeconds     int64 `json:"uptimeSeconds"`
}

type service struct {
	mu        sync.RWMutex
	sessions  map[string]*ContinuousVoiceSession
	planner   VoiceRoutePlanner
	events    *VoiceEventBus
	startedAt time.Time
}

func NewService() Service {
	return &service{
		sessions:  make(map[string]*ContinuousVoiceSession),
		planner:   NewRoutePlanner(),
		events:    NewVoiceEventBus(),
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
		if err := s.loadWakeDetector(ctx, sess); err != nil {
			return nil, err
		}
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
	if sess.Mode == ContinuousVoiceSessionModeWakeArmed && sess.wake == nil {
		if err := s.loadWakeDetector(ctx, sess); err != nil {
			return err
		}
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
	emitDesktopPetVoice(ctx, sess, "session.started")
	if sess.State == ContinuousVoiceSessionStatusListening {
		emitDesktopPetVoice(ctx, sess, "listening.started")
	}
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
	emitDesktopPetVoice(ctx, sess, "session.ended")

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
		emitDesktopPetVoice(ctx, sess, "turn.interrupted")
		emitDesktopPetVoice(ctx, sess, "listening.started")
		return nil
	}

	if sess.State == ContinuousVoiceSessionStatusProcessing {
		sess.State = ContinuousVoiceSessionStatusListening
		sess.LastActivityAt = time.Now()
		emitDesktopPetVoice(ctx, sess, "turn.interrupted")
		emitDesktopPetVoice(ctx, sess, "listening.started")
		return nil
	}

	return nil
}

func (s *service) ArmWake(ctx context.Context, sessionID string) error {
	sess, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}
	if err := s.loadWakeDetector(ctx, sess); err != nil {
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

	var detected *WakeDetectionResult
	sess.mu.Lock()
	if sess.State.IsTerminal() {
		sess.mu.Unlock()
		return fmt.Errorf("voice session: terminal state=%s", sess.State)
	}

	sess.LastActivityAt = time.Now()

	switch sess.State {
	case ContinuousVoiceSessionStatusArmed:
		if sess.wake != nil {
			result, processErr := sess.wake.Process(frame)
			if processErr != nil {
				sess.mu.Unlock()
				return processErr
			}
			if result.Detected {
				detected = &result
				sess.State = ContinuousVoiceSessionStatusListening
				sess.WakeArmed = false
			}
		}

	case ContinuousVoiceSessionStatusListening:
		if sess.vad != nil {
			vadResult, processErr := sess.vad.Process(frame)
			if processErr == nil && vadResult.SpeechStarted {
				sess.State = ContinuousVoiceSessionStatusTranscribing
				sess.CurrentTurnID = uuid.New().String()
				sess.CaptureGeneration++
				emitDesktopPetVoice(ctx, sess, "listening.activity")
			}
		}

	case ContinuousVoiceSessionStatusTranscribing:
		if sess.vad != nil {
			vadResult, processErr := sess.vad.Process(frame)
			if processErr == nil && vadResult.SpeechEnded {
				sess.State = ContinuousVoiceSessionStatusProcessing
				sess.PlaybackGeneration++
				emitDesktopPetVoice(ctx, sess, "listening.ended")
				emitDesktopPetVoice(ctx, sess, "processing.started")
			}
		}

	case ContinuousVoiceSessionStatusSpeaking:
		if sess.vad != nil {
			vadResult, processErr := sess.vad.Process(frame)
			if processErr == nil && vadResult.Speech {
				sess.State = ContinuousVoiceSessionStatusListening
				sess.PlaybackGeneration++
				sess.CurrentTurnID = uuid.New().String()
				sess.CaptureGeneration++
				emitDesktopPetVoice(ctx, sess, "turn.interrupted")
				emitDesktopPetVoice(ctx, sess, "listening.started")
			}
		}
	}
	wakeConfigID := sess.WakeConfigID
	userID := sess.UserID
	turnID := sess.CurrentTurnID
	sess.mu.Unlock()

	if detected != nil {
		payload := map[string]any{
			"sessionId":    sessionID,
			"turnId":       turnID,
			"wakeConfigId": wakeConfigID,
			"phraseId":     detected.PhraseID,
			"confidence":   detected.Confidence,
			"detectedAtNs": detected.DetectedAtNS,
		}
		s.events.PublishSessionEvent(sessionID, VoiceEventWakeDetected, payload)
		raw, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return marshalErr
		}
		wakeEventID := makeVoiceWorkflowEventID("voice-wake", fmt.Sprintf("%s\n%s\n%d", sessionID, wakeConfigID, detected.DetectedAtNS))
		if err := publishVoiceWorkflowTrigger(ctx, VoiceWorkflowTriggerEvent{
			EventID:   wakeEventID,
			EventType: string(VoiceEventWakeDetected),
			UserID:    userID,
			Source:    "voice.wake",
			Payload:   raw,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *service) PublishASRFinal(ctx context.Context, sessionID, transcript, eventID string) error {
	transcript = strings.TrimSpace(transcript)
	if transcript == "" {
		return fmt.Errorf("voice asr final transcript is required")
	}
	if len([]rune(transcript)) > maxWorkflowASRTranscriptChars {
		return fmt.Errorf("voice asr final transcript exceeds %d characters", maxWorkflowASRTranscriptChars)
	}
	sess, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}
	sess.mu.RLock()
	userID := sess.UserID
	turnID := sess.CurrentTurnID
	conversationID := sess.ConversationID
	characterID := sess.CharacterID
	sess.mu.RUnlock()
	payload := map[string]any{
		"sessionId":      sessionID,
		"turnId":         turnID,
		"conversationId": conversationID,
		"characterId":    characterID,
		"transcript":     transcript,
		"final":          true,
	}
	s.events.PublishSessionEvent(sessionID, VoiceEventASRFinal, payload)
	return PublishASRWorkflowFinal(ctx, userID, sessionID, turnID, conversationID, characterID, transcript, eventID)
}

func (s *service) loadWakeDetector(ctx context.Context, sess *ContinuousVoiceSession) error {
	if dbInstance == nil {
		return fmt.Errorf("wake config storage unavailable")
	}
	var profile VoiceProfile
	profileID := strings.TrimSpace(sess.ProfileID)
	profileQuery := dbInstance.WithContext(ctx).Table("voice_profiles")
	var profileErr error
	if profileID != "" && profileID != "default" {
		profileErr = profileQuery.Where("id = ?", profileID).First(&profile).Error
	} else {
		profileErr = profileQuery.Where("is_default = 1").First(&profile).Error
	}
	wakeConfigID := strings.TrimSpace(profile.WakeConfigID)
	if profileErr != nil || wakeConfigID == "" {
		var fallback WakeConfig
		if err := dbInstance.WithContext(ctx).Table("wake_configs").Where("enabled = 1").Where("id NOT LIKE ?", "workflow-wake-%").Order("updated_at DESC").First(&fallback).Error; err != nil {
			if profileErr != nil {
				return fmt.Errorf("load voice profile wake config: %w", profileErr)
			}
			return fmt.Errorf("voice profile has no wake config")
		}
		return s.installWakeDetector(ctx, sess, fallback)
	}
	var config WakeConfig
	if err := dbInstance.WithContext(ctx).Table("wake_configs").Where("id = ? AND enabled = 1", wakeConfigID).First(&config).Error; err != nil {
		return fmt.Errorf("load wake config %s: %w", wakeConfigID, err)
	}
	return s.installWakeDetector(ctx, sess, config)
}

func (s *service) installWakeDetector(ctx context.Context, sess *ContinuousVoiceSession, config WakeConfig) error {
	backend := strings.TrimSpace(config.Backend)
	if backend == "" {
		backend = "software"
	}
	factory, ok := GetWakeBackend(backend)
	if !ok {
		return fmt.Errorf("wake backend unavailable: %s", backend)
	}
	detector, err := factory("{}")
	if err != nil {
		return err
	}
	phrases, err := decodeWakePhrases(config.Phrases)
	if err != nil {
		return err
	}
	if err := detector.Load(ctx, WakeDetectorConfig{
		Enabled:          config.Enabled,
		Backend:          backend,
		ModelResourceURI: config.ModelResourceURI,
		Phrases:          phrases,
		Threshold:        config.Threshold,
		CooldownMS:       config.CooldownMS,
	}); err != nil {
		return err
	}
	sess.mu.Lock()
	old := sess.wake
	sess.wake = detector
	sess.WakeConfigID = config.ID
	sess.mu.Unlock()
	if old != nil && old != detector {
		_ = old.Unload()
	}
	return nil
}

func decodeWakePhrases(raw string) ([]WakePhrase, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("wake config phrases are empty")
	}
	var objects []struct {
		ID          string `json:"id"`
		DisplayText string `json:"displayText"`
		Locale      string `json:"locale"`
	}
	if err := json.Unmarshal([]byte(raw), &objects); err == nil && len(objects) > 0 {
		result := make([]WakePhrase, 0, len(objects))
		for i, item := range objects {
			text := strings.TrimSpace(item.DisplayText)
			if text == "" {
				continue
			}
			id := strings.TrimSpace(item.ID)
			if id == "" {
				id = fmt.Sprintf("wake-%d", i+1)
			}
			result = append(result, WakePhrase{ID: id, DisplayText: text, Locale: strings.TrimSpace(item.Locale)})
		}
		if len(result) > 0 {
			return result, nil
		}
	}
	var stringsOnly []string
	if err := json.Unmarshal([]byte(raw), &stringsOnly); err != nil {
		return nil, fmt.Errorf("decode wake phrases: %w", err)
	}
	result := make([]WakePhrase, 0, len(stringsOnly))
	for i, value := range stringsOnly {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, WakePhrase{ID: fmt.Sprintf("wake-%d", i+1), DisplayText: value})
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("wake config phrases are empty")
	}
	return result, nil
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

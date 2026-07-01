package interaction

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

type VoiceInterruptPolicy string

const (
	VoiceInterruptPolicyImmediate VoiceInterruptPolicy = "immediate"
	VoiceInterruptPolicyDeferred  VoiceInterruptPolicy = "deferred"
)

type VoiceTurnState string

const (
	VoiceTurnStateListening  VoiceTurnState = "listening"
	VoiceTurnStateProcessing VoiceTurnState = "processing"
	VoiceTurnStateResponding VoiceTurnState = "responding"
	VoiceTurnStateIdle       VoiceTurnState = "idle"
)

var (
	ErrVoiceSessionNotFound = errors.New("voice_entry: session not found")
	ErrVoiceTurnCancelled   = errors.New("voice_entry: turn cancelled")
	ErrVoiceBusy            = errors.New("voice_entry: orchestrator busy for voice input")
)

type VoiceTurnRequest struct {
	SessionID      string `json:"sessionId"`
	TurnID         string `json:"turnId"`
	Text           string `json:"text"`
	IsFinal        bool   `json:"isFinal"`
	ConversationID string `json:"conversationId"`
	CharacterID    string `json:"characterId"`
	Channel        string `json:"channel"`
}

type VoiceSession struct {
	SessionID       string              `json:"sessionId"`
	ConversationID  string              `json:"conversationId"`
	CharacterID     string              `json:"characterId"`
	State           VoiceTurnState      `json:"state"`
	CurrentTurnID   string              `json:"currentTurnId"`
	CurrentText     string              `json:"currentText"`
	InterruptPolicy VoiceInterruptPolicy `json:"interruptPolicy"`
	CreatedAt       time.Time           `json:"createdAt"`
	LastActivity    time.Time           `json:"lastActivity"`
	mu              sync.RWMutex
	cancelFn        context.CancelFunc
}

type VoiceEntry struct {
	orchestrator *Orchestrator
	sessions     map[string]*VoiceSession
	mu           sync.RWMutex
}

func NewVoiceEntry(orchestrator *Orchestrator) *VoiceEntry {
	return &VoiceEntry{
		orchestrator: orchestrator,
		sessions:     make(map[string]*VoiceSession),
	}
}

func (v *VoiceEntry) CreateSession(sessionID, conversationID, characterID string) *VoiceSession {
	v.mu.Lock()
	defer v.mu.Unlock()
	session := &VoiceSession{
		SessionID:       sessionID,
		ConversationID:  conversationID,
		CharacterID:     characterID,
		State:           VoiceTurnStateIdle,
		InterruptPolicy: VoiceInterruptPolicyImmediate,
		CreatedAt:       time.Now(),
		LastActivity:    time.Now(),
	}
	v.sessions[sessionID] = session
	return session
}

func (v *VoiceEntry) GetSession(sessionID string) *VoiceSession {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.sessions[sessionID]
}

func (v *VoiceEntry) RemoveSession(sessionID string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.sessions, sessionID)
}

func (v *VoiceEntry) HandleTurn(ctx context.Context, req *VoiceTurnRequest) (*OrchestrationResult, error) {
	session := v.GetSession(req.SessionID)
	if session == nil {
		session = v.CreateSession(req.SessionID, req.ConversationID, req.CharacterID)
	}

	session.mu.Lock()
	session.LastActivity = time.Now()

	if !req.IsFinal {
		session.CurrentTurnID = req.TurnID
		session.CurrentText = req.Text
		session.State = VoiceTurnStateListening
		session.mu.Unlock()
		return nil, nil
	}

	if session.State == VoiceTurnStateProcessing || session.State == VoiceTurnStateResponding {
		switch session.InterruptPolicy {
		case VoiceInterruptPolicyImmediate:
			v.cancelCurrentProcessing(session)
		case VoiceInterruptPolicyDeferred:
			session.mu.Unlock()
			return nil, ErrVoiceBusy
		}
	}

	session.CurrentTurnID = req.TurnID
	session.CurrentText = req.Text
	session.State = VoiceTurnStateProcessing
	session.mu.Unlock()

	v.cancelPreviousForScope(session)

	procReq := &ProcessRequest{
		CharacterID:    session.CharacterID,
		ConversationID: session.ConversationID,
		Message:        req.Text,
		Channel:        req.Channel,
		Source:         "voice",
		RequestID:      req.TurnID,
	}

	result, err := v.orchestrator.Process(ctx, procReq)

	session.mu.Lock()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			session.State = VoiceTurnStateIdle
			session.mu.Unlock()
			return nil, ErrVoiceTurnCancelled
		}
		session.State = VoiceTurnStateIdle
		session.mu.Unlock()
		return nil, err
	}
	session.State = VoiceTurnStateIdle
	session.mu.Unlock()

	return result, nil
}

func (v *VoiceEntry) cancelCurrentProcessing(session *VoiceSession) {
	v.orchestrator.CancelByScope(InteractionScope{
		ConversationID: session.ConversationID,
		CharacterID:    session.CharacterID,
	})
}

func (v *VoiceEntry) cancelPreviousForScope(session *VoiceSession) {
	scope := InteractionScope{
		ConversationID: session.ConversationID,
		CharacterID:    session.CharacterID,
	}.Normalize()
	count := v.orchestrator.CancelByScope(scope)
	if count > 0 {
		log.Printf("[voice_entry] cancelled %d previous interactions for conversation=%s", count, session.ConversationID)
	}
}

func (v *VoiceEntry) SetInterruptPolicy(sessionID string, policy VoiceInterruptPolicy) error {
	session := v.GetSession(sessionID)
	if session == nil {
		return ErrVoiceSessionNotFound
	}
	session.mu.Lock()
	session.InterruptPolicy = policy
	session.mu.Unlock()
	return nil
}

func (v *VoiceEntry) CleanupStaleSessions(maxAge time.Duration) int {
	v.mu.Lock()
	defer v.mu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	count := 0
	for id, session := range v.sessions {
		session.mu.RLock()
		last := session.LastActivity
		session.mu.RUnlock()
		if last.Before(cutoff) {
			delete(v.sessions, id)
			count++
		}
	}
	return count
}

func (s *VoiceSession) GetState() VoiceTurnState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

func (s *VoiceSession) GetCurrentText() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CurrentText
}

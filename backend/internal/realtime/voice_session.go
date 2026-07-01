package realtime

import (
	"sync"
	"time"
)

type TranscriptionStatus string

const (
	TranscriptionInterim TranscriptionStatus = "interim"
	TranscriptionFinal   TranscriptionStatus = "final"
	TranscriptionCancel  TranscriptionStatus = "cancel"
)

type VoiceTurn struct {
	TurnID       string              `json:"turnId"`
	SessionID    string              `json:"sessionId"`
	ConversationID string            `json:"conversationId"`
	CharacterID  string              `json:"characterId"`
	Text         string              `json:"text"`
	Status       TranscriptionStatus `json:"status"`
	StartedAt    time.Time           `json:"startedAt"`
	EndedAt      time.Time           `json:"endedAt,omitempty"`
	SupersededBy string              `json:"supersededBy,omitempty"`
	Cancelled    bool                `json:"cancelled"`
}

type VoiceSessionState struct {
	SessionID          string        `json:"sessionId"`
	ConversationID     string        `json:"conversationId"`
	CharacterID        string        `json:"characterId"`
	LastCommittedEvent string        `json:"lastCommittedEvent"`
	CurrentTurn        *VoiceTurn    `json:"currentTurn,omitempty"`
	CompletedTurns     []VoiceTurn   `json:"completedTurns"`
	StateVersion       int64         `json:"stateVersion"`
	CreatedAt          time.Time     `json:"createdAt"`
	EndedAt            time.Time     `json:"endedAt,omitempty"`
	mu                 sync.RWMutex  `json:"-"`
}

var activeVoiceSessions sync.Map

func NewVoiceSession(sessionID, conversationID, characterID string) *VoiceSessionState {
	return &VoiceSessionState{
		SessionID:      sessionID,
		ConversationID: conversationID,
		CharacterID:    characterID,
		StateVersion:   1,
		CreatedAt:      time.Now(),
		CompletedTurns: make([]VoiceTurn, 0),
	}
}

func GetOrCreateVoiceSession(sessionID, conversationID, characterID string) *VoiceSessionState {
	val, ok := activeVoiceSessions.Load(sessionID)
	if ok {
		return val.(*VoiceSessionState)
	}
	s := NewVoiceSession(sessionID, conversationID, characterID)
	activeVoiceSessions.Store(sessionID, s)
	return s
}

func GetVoiceSession(sessionID string) *VoiceSessionState {
	val, ok := activeVoiceSessions.Load(sessionID)
	if !ok {
		return nil
	}
	return val.(*VoiceSessionState)
}

func RemoveVoiceSession(sessionID string) {
	activeVoiceSessions.Delete(sessionID)
}

func (s *VoiceSessionState) BeginTurn(turnID, text string) *VoiceTurn {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.CurrentTurn != nil && s.CurrentTurn.Status != TranscriptionFinal {
		s.CurrentTurn.Cancelled = true
		s.CurrentTurn.Status = TranscriptionCancel
		s.CompletedTurns = append(s.CompletedTurns, *s.CurrentTurn)
	}
	turn := &VoiceTurn{
		TurnID:       turnID,
		SessionID:    s.SessionID,
		ConversationID: s.ConversationID,
		CharacterID:  s.CharacterID,
		Text:         text,
		Status:       TranscriptionInterim,
		StartedAt:    time.Now(),
	}
	s.CurrentTurn = turn
	return turn
}

func (s *VoiceSessionState) UpdateTurnText(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.CurrentTurn != nil && s.CurrentTurn.Status == TranscriptionInterim {
		s.CurrentTurn.Text = text
	}
}

func (s *VoiceSessionState) CommitTurn(turnID string) *VoiceTurn {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.CurrentTurn == nil || s.CurrentTurn.TurnID != turnID {
		return nil
	}
	s.CurrentTurn.Status = TranscriptionFinal
	s.CurrentTurn.EndedAt = time.Now()
	s.LastCommittedEvent = turnID
	s.StateVersion++
	committed := *s.CurrentTurn
	s.CompletedTurns = append(s.CompletedTurns, committed)
	s.CurrentTurn = nil
	return &committed
}

func (s *VoiceSessionState) CancelTurn(turnID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.CurrentTurn == nil || s.CurrentTurn.TurnID != turnID {
		return
	}
	s.CurrentTurn.Status = TranscriptionCancel
	s.CurrentTurn.Cancelled = true
	s.CurrentTurn.EndedAt = time.Now()
	cancelled := *s.CurrentTurn
	s.CompletedTurns = append(s.CompletedTurns, cancelled)
	s.CurrentTurn = nil
}

func (s *VoiceSessionState) EndSession() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.CurrentTurn != nil && s.CurrentTurn.Status != TranscriptionFinal {
		s.CurrentTurn.Status = TranscriptionCancel
		s.CurrentTurn.Cancelled = true
		s.CurrentTurn.EndedAt = time.Now()
		s.CompletedTurns = append(s.CompletedTurns, *s.CurrentTurn)
		s.CurrentTurn = nil
	}
	s.EndedAt = time.Now()
}

func (s *VoiceSessionState) GetCompletedTurns() []VoiceTurn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]VoiceTurn, len(s.CompletedTurns))
	copy(result, s.CompletedTurns)
	return result
}

func (s *VoiceSessionState) GetFinalTurns() []VoiceTurn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]VoiceTurn, 0)
	for _, t := range s.CompletedTurns {
		if t.Status == TranscriptionFinal && !t.Cancelled {
			result = append(result, t)
		}
	}
	return result
}

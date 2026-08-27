package realtime

import (
	"context"
	"sync"
	"time"
)

type DesktopPetVoiceLifecycle struct {
	SessionID      string
	TurnID         string
	CharacterID    string
	UserID         string
	ConversationID string
	Phase          string
	StateVersion   int64
	OccurredAt     time.Time
}

type DesktopPetVoiceObserver interface {
	OnVoiceLifecycle(context.Context, DesktopPetVoiceLifecycle)
}

var desktopPetVoiceObserver struct {
	sync.RWMutex
	observer DesktopPetVoiceObserver
}

func SetDesktopPetVoiceObserver(observer DesktopPetVoiceObserver) {
	desktopPetVoiceObserver.Lock()
	desktopPetVoiceObserver.observer = observer
	desktopPetVoiceObserver.Unlock()
}

func emitDesktopPetVoice(ctx context.Context, sess *ContinuousVoiceSession, phase string) {
	if sess == nil {
		return
	}
	desktopPetVoiceObserver.RLock()
	observer := desktopPetVoiceObserver.observer
	desktopPetVoiceObserver.RUnlock()
	if observer == nil || sess.CharacterID == "" {
		return
	}
	observer.OnVoiceLifecycle(ctx, DesktopPetVoiceLifecycle{
		SessionID: sess.SessionID, TurnID: sess.CurrentTurnID, CharacterID: sess.CharacterID,
		UserID: sess.UserID, ConversationID: sess.ConversationID, Phase: phase,
		StateVersion: int64(sess.CaptureGeneration + sess.PlaybackGeneration + 1), OccurredAt: time.Now().UTC(),
	})
}

package realtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

type VoiceWorkflowTriggerEvent struct {
	EventID   string
	EventType string
	UserID    string
	Source    string
	Payload   json.RawMessage
}

const maxWorkflowASRTranscriptChars = 16_384

type VoiceWorkflowTriggerPublisher func(context.Context, VoiceWorkflowTriggerEvent) error

var voiceWorkflowTriggerPublisher struct {
	sync.RWMutex
	fn VoiceWorkflowTriggerPublisher
}

func SetWorkflowTriggerPublisher(publisher VoiceWorkflowTriggerPublisher) {
	voiceWorkflowTriggerPublisher.Lock()
	voiceWorkflowTriggerPublisher.fn = publisher
	voiceWorkflowTriggerPublisher.Unlock()
}

func publishVoiceWorkflowTrigger(ctx context.Context, event VoiceWorkflowTriggerEvent) error {
	voiceWorkflowTriggerPublisher.RLock()
	publisher := voiceWorkflowTriggerPublisher.fn
	voiceWorkflowTriggerPublisher.RUnlock()
	if publisher == nil {
		return nil
	}
	return publisher(ctx, event)
}

func makeVoiceWorkflowEventID(prefix, seed string) string {
	digest := sha256.Sum256([]byte(seed))
	return prefix + ":" + hex.EncodeToString(digest[:16])
}

func validVoiceWorkflowEventID(value string) bool {
	if value == "" || len(value) > 200 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == ':' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func PublishASRWorkflowFinal(ctx context.Context, userID, sessionID, turnID, conversationID, characterID, transcript, eventID string) error {
	userID = strings.TrimSpace(userID)
	transcript = strings.TrimSpace(transcript)
	if userID == "" {
		return fmt.Errorf("voice asr workflow user is required")
	}
	if transcript == "" {
		return fmt.Errorf("voice asr final transcript is required")
	}
	if len([]rune(transcript)) > maxWorkflowASRTranscriptChars {
		return fmt.Errorf("voice asr final transcript exceeds %d characters", maxWorkflowASRTranscriptChars)
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		eventID = makeVoiceWorkflowEventID("asr-final", sessionID+"\n"+turnID+"\n"+transcript)
	} else if !validVoiceWorkflowEventID(eventID) {
		return fmt.Errorf("invalid voice workflow event id")
	}
	payload, err := json.Marshal(map[string]any{
		"sessionId":      sessionID,
		"turnId":         turnID,
		"conversationId": conversationID,
		"characterId":    characterID,
		"transcript":     transcript,
		"final":          true,
	})
	if err != nil {
		return err
	}
	return publishVoiceWorkflowTrigger(ctx, VoiceWorkflowTriggerEvent{
		EventID:   eventID,
		EventType: string(VoiceEventASRFinal),
		UserID:    userID,
		Source:    "voice.asr",
		Payload:   payload,
	})
}

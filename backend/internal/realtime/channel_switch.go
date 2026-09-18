package realtime

import (
	"log"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/outputlease"
)

type ChannelGroup string

const (
	ChannelGroupText  ChannelGroup = "text"
	ChannelGroupVoice ChannelGroup = "voice"
	ChannelGroupAll   ChannelGroup = "all"
)

type ChannelSwitchEvent struct {
	EventID     string       `json:"eventId"`
	CharacterID string       `json:"characterId"`
	FromChannel string       `json:"fromChannel"`
	ToChannel   string       `json:"toChannel"`
	FromGroup   ChannelGroup `json:"fromGroup"`
	ToGroup     ChannelGroup `json:"toGroup"`
	Reason      string       `json:"reason"`
	SwitchedAt  time.Time    `json:"switchedAt"`
}

type ChannelBelief struct {
	CharacterID   string               `json:"characterId"`
	ActiveChannel string               `json:"activeChannel"`
	ActiveGroup   ChannelGroup         `json:"activeGroup"`
	LastSwitch    time.Time            `json:"lastSwitch"`
	SwitchHistory []ChannelSwitchEvent `json:"switchHistory"`
	mu            sync.RWMutex
}

var channelBeliefs sync.Map

func resolveChannelGroup(channel string) ChannelGroup {
	switch channel {
	case "voice", "tts":
		return ChannelGroupVoice
	default:
		return ChannelGroupText
	}
}

func GetOrCreateChannelBelief(characterID string) *ChannelBelief {
	val, ok := channelBeliefs.Load(characterID)
	if ok {
		return val.(*ChannelBelief)
	}
	belief := &ChannelBelief{
		CharacterID:   characterID,
		ActiveChannel: "web",
		ActiveGroup:   ChannelGroupText,
		SwitchHistory: make([]ChannelSwitchEvent, 0),
	}
	channelBeliefs.Store(characterID, belief)
	return belief
}

func (cb *ChannelBelief) GetActiveChannel() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.ActiveChannel
}

func (cb *ChannelBelief) GetActiveGroup() ChannelGroup {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.ActiveGroup
}

func (cb *ChannelBelief) RecordSwitch(event ChannelSwitchEvent) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.ActiveChannel = event.ToChannel
	cb.ActiveGroup = event.ToGroup
	cb.LastSwitch = event.SwitchedAt
	cb.SwitchHistory = append(cb.SwitchHistory, event)
	if len(cb.SwitchHistory) > 20 {
		cb.SwitchHistory = cb.SwitchHistory[len(cb.SwitchHistory)-20:]
	}
	log.Printf("[ChannelBelief] char=%s switch %s->%s reason=%s", cb.CharacterID, event.FromChannel, event.ToChannel, event.Reason)
}

func SwitchChannel(characterID, fromChannel, toChannel, reason string) *ChannelSwitchEvent {
	fromGroup := resolveChannelGroup(fromChannel)
	toGroup := resolveChannelGroup(toChannel)
	event := ChannelSwitchEvent{
		EventID:     generateEventID(),
		CharacterID: characterID,
		FromChannel: fromChannel,
		ToChannel:   toChannel,
		FromGroup:   fromGroup,
		ToGroup:     toGroup,
		Reason:      reason,
		SwitchedAt:  time.Now(),
	}
	belief := GetOrCreateChannelBelief(characterID)
	belief.RecordSwitch(event)
	return &event
}

func generateEventID() string {
	return "evt-" + time.Now().Format("20060102150405") + "-" + randomSuffix(6)
}

func randomSuffix(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

func AcquireChannelLease(characterID, conversationID, channel string, correlationID string, priority outputlease.OutputPriority, ttl time.Duration) *outputlease.OutputLease {
	group := resolveChannelGroup(channel)
	lease := outputlease.AcquireLeaseForGroup(priority, characterID, conversationID, string(group), correlationID, ttl)
	return lease
}

func CancelLowPriorityLeasesOnUserInput(characterID string) int {
	return outputlease.CancelLowPriorityOnUserInput(characterID)
}

func CancelLeasesForChannelGroup(characterID string, group ChannelGroup) int {
	return outputlease.CancelLeasesForGroup(characterID, string(group))
}

func HasActiveLeaseForChannel(characterID, channel string) bool {
	group := resolveChannelGroup(channel)
	leases := outputlease.GetActiveLeasesForGroup(characterID, string(group))
	return len(leases) > 0
}

package system

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	applog "github.com/u-ai/backend/log"
)

type MessageEventType string

const (
	EventMessageCreated      MessageEventType = "message_created"
	EventMessageUpdated      MessageEventType = "message_updated"
	EventConversationUpdated MessageEventType = "conversation_updated"
)

type MessageEvent struct {
	Type           MessageEventType `json:"type"`
	ConversationID string           `json:"conversationId"`
	MessageID      string           `json:"messageId,omitempty"`
	Channel        string           `json:"channel"`
	Direction      string           `json:"direction,omitempty"`
	Role           string           `json:"role,omitempty"`
	Content        string           `json:"content,omitempty"`
	CreatedAt      string           `json:"createdAt,omitempty"`
	Status         string           `json:"status,omitempty"`
	Data           interface{}      `json:"data,omitempty"`
}

type MessageEventSubscriber struct {
	ID       string
	Channels map[string]bool
	Events   chan MessageEvent
}

// MessageEventBus 是 ephemeral in-process projection / notification bus。
// channel 满时允许丢事件是现有实时 UI 行为。
// 它绝不能被用作 Cloud durable sync、Device Mesh event log 或任务恢复的数据源。
// 后续 durable sync 统一复用 extension/kernel/event。
type MessageEventBus struct {
	mu          sync.RWMutex
	subscribers map[string]*MessageEventSubscriber
	durable     MessageDurablePublisher
}

type MessageDurablePublisher interface {
	PublishMessageEvent(
		ctx context.Context,
		event MessageEvent,
	) error
}

var globalMessageEventBus *MessageEventBus
var messageEventBusOnce sync.Once

func GetMessageEventBus() *MessageEventBus {
	messageEventBusOnce.Do(func() {
		globalMessageEventBus = &MessageEventBus{
			subscribers: make(map[string]*MessageEventSubscriber),
		}
	})
	return globalMessageEventBus
}

func (bus *MessageEventBus) SetDurablePublisher(publisher MessageDurablePublisher) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.durable = publisher
}

func (bus *MessageEventBus) Subscribe(id string, channels []string) *MessageEventSubscriber {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	chMap := make(map[string]bool)
	for _, ch := range channels {
		chMap[ch] = true
	}
	sub := &MessageEventSubscriber{
		ID:       id,
		Channels: chMap,
		Events:   make(chan MessageEvent, 256),
	}
	bus.subscribers[id] = sub
	applog.Info(fmt.Sprintf("[MessageEventBus] subscriber %s registered for channels: %v", id, channels))
	return sub
}

func (bus *MessageEventBus) Unsubscribe(id string) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	if sub, ok := bus.subscribers[id]; ok {
		close(sub.Events)
		delete(bus.subscribers, id)
		applog.Info(fmt.Sprintf("[MessageEventBus] subscriber %s unregistered", id))
	}
}

func (bus *MessageEventBus) Publish(event MessageEvent) {
	bus.mu.RLock()
	defer bus.mu.RUnlock()
	payload, _ := json.Marshal(event)
	applog.Info(fmt.Sprintf("[MessageEventBus] publish event=%s channel=%s", event.Type, event.Channel))
	for _, sub := range bus.subscribers {
		if len(sub.Channels) == 0 || sub.Channels[event.Channel] {
			select {
			case sub.Events <- event:
			default:
				applog.Warn(fmt.Sprintf("[MessageEventBus] subscriber %s channel full, dropping event", sub.ID))
			}
		}
	}
	_ = payload
}

func (bus *MessageEventBus) PublishMessageCreated(convID, msgID, channel, direction, role, content, createdAt string) {
	bus.Publish(MessageEvent{
		Type:           EventMessageCreated,
		ConversationID: convID,
		MessageID:      msgID,
		Channel:        channel,
		Direction:      direction,
		Role:           role,
		Content:        content,
		CreatedAt:      createdAt,
	})
}

func (bus *MessageEventBus) PublishMessageUpdated(convID, msgID, channel, status string) {
	bus.Publish(MessageEvent{
		Type:           EventMessageUpdated,
		ConversationID: convID,
		MessageID:      msgID,
		Channel:        channel,
		Status:         status,
	})
}

func (bus *MessageEventBus) PublishConversationUpdated(convID, channel string, data interface{}) {
	bus.Publish(MessageEvent{
		Type:           EventConversationUpdated,
		ConversationID: convID,
		Channel:        channel,
		Data:           data,
	})
}

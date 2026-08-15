package desktoppet

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/log"
)

const (
	PetOutputChannel   = "pet_output"
	PetInvokeMethod    = "pet.invoke"
	SubscribeAckMethod = "notification.subscribe.ack"
)

type SubscriptionState string

const (
	SubscriptionStateNone       SubscriptionState = "none"
	SubscriptionStatePending    SubscriptionState = "pending"
	SubscriptionStateSubscribed SubscriptionState = "subscribed"
)

type PetOutputSubscription struct {
	mu           sync.Mutex
	sessionID    string
	runtimeID    string
	deviceID     string
	userID       string
	state        SubscriptionState
	createdAt    time.Time
	subscribedAt time.Time
	conn         SubscriptionConnection
}

type SubscriptionConnection interface {
	Send(msg contracts.RuntimeMessage) error
}

func NewPetOutputSubscription(sessionID, runtimeID, deviceID, userID string) *PetOutputSubscription {
	return &PetOutputSubscription{
		sessionID: sessionID,
		runtimeID: runtimeID,
		deviceID:  deviceID,
		userID:    userID,
		state:     SubscriptionStateNone,
		createdAt: time.Now(),
	}
}

func (s *PetOutputSubscription) SetConnection(conn SubscriptionConnection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conn = conn
}

func (s *PetOutputSubscription) State() SubscriptionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *PetOutputSubscription) RuntimeID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runtimeID
}

func (s *PetOutputSubscription) SessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionID
}

func (s *PetOutputSubscription) CanInvoke() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == SubscriptionStateSubscribed
}

func (s *PetOutputSubscription) MarkSubscribed() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == SubscriptionStateSubscribed {
		return nil
	}
	s.state = SubscriptionStateSubscribed
	s.subscribedAt = time.Now()
	return nil
}

func (s *PetOutputSubscription) SendSubscribeAck() error {
	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("pet output subscription: no connection for session %s", s.sessionID)
	}
	ackPayload := map[string]interface{}{
		"channel":    PetOutputChannel,
		"status":     "subscribed",
		"sessionId":  s.sessionID,
		"serverTime": time.Now().UTC(),
	}
	rawPayload, err := json.Marshal(ackPayload)
	if err != nil {
		return fmt.Errorf("pet output subscription: marshal ack failed: %w", err)
	}
	msg := contracts.RuntimeMessage{
		SchemaVersion:   contracts.SchemaVersion,
		ProtocolVersion: contracts.ProtocolMax,
		Kind:            contracts.KindEvent,
		Name:            SubscribeAckMethod,
		MessageID:       uuid.New().String(),
		RuntimeID:       s.runtimeID,
		SessionID:       s.sessionID,
		Sequence:        0,
		SentAt:          time.Now(),
		Payload:         rawPayload,
	}
	return conn.Send(msg)
}

type PetOutputBus struct {
	mu            sync.RWMutex
	subscriptions map[string]*PetOutputSubscription
	byRuntime     map[string]*PetOutputSubscription
}

func NewPetOutputBus() *PetOutputBus {
	return &PetOutputBus{
		subscriptions: make(map[string]*PetOutputSubscription),
		byRuntime:     make(map[string]*PetOutputSubscription),
	}
}

func (b *PetOutputBus) Subscribe(sub *PetOutputSubscription) error {
	if sub == nil {
		return fmt.Errorf("pet output bus: subscription is nil")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.subscriptions[sub.sessionID]; exists {
		return fmt.Errorf("pet output bus: session %s already subscribed", sub.sessionID)
	}
	b.subscriptions[sub.sessionID] = sub
	b.byRuntime[sub.runtimeID] = sub
	return nil
}

func (b *PetOutputBus) Unsubscribe(sessionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	sub, ok := b.subscriptions[sessionID]
	if !ok {
		return
	}
	delete(b.subscriptions, sessionID)
	if existing, ok := b.byRuntime[sub.runtimeID]; ok && existing.sessionID == sessionID {
		delete(b.byRuntime, sub.runtimeID)
	}
}

func (b *PetOutputBus) GetBySession(sessionID string) (*PetOutputSubscription, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	sub, ok := b.subscriptions[sessionID]
	return sub, ok
}

func (b *PetOutputBus) GetByRuntime(runtimeID string) (*PetOutputSubscription, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	sub, ok := b.byRuntime[runtimeID]
	return sub, ok
}

func (b *PetOutputBus) HandleSubscribe(sessionID, runtimeID, deviceID, userID string, conn SubscriptionConnection) error {
	b.mu.Lock()
	existing, found := b.subscriptions[sessionID]
	if found {
		existing.SetConnection(conn)
		_ = existing.MarkSubscribed()
		b.mu.Unlock()
		return existing.SendSubscribeAck()
	}
	sub := NewPetOutputSubscription(sessionID, runtimeID, deviceID, userID)
	sub.SetConnection(conn)
	_ = sub.MarkSubscribed()
	b.subscriptions[sessionID] = sub
	b.byRuntime[runtimeID] = sub
	b.mu.Unlock()
	log.Logger.Infof("pet output bus: subscribed sessionID=%s runtimeID=%s", sessionID, runtimeID)
	return sub.SendSubscribeAck()
}

func (b *PetOutputBus) CheckInvokeAllowed(sessionID string) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	sub, ok := b.subscriptions[sessionID]
	if !ok {
		return fmt.Errorf("pet output bus: session %s has not subscribed to %s", sessionID, PetOutputChannel)
	}
	if !sub.CanInvoke() {
		return fmt.Errorf("pet output bus: session %s is not subscribed (state=%s), must subscribe to %s before invoking %s", sessionID, sub.state, PetOutputChannel, PetInvokeMethod)
	}
	return nil
}

func (b *PetOutputBus) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscriptions)
}

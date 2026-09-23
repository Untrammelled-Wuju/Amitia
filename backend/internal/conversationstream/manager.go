package conversationstream

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const ProtocolVersion = 1

type AgentUIEvent struct {
	Version         int            `json:"version"`
	EventID         string         `json:"eventId"`
	EventSequence   int64          `json:"eventSequence"`
	ConversationID  string         `json:"conversationId"`
	RequestID       string         `json:"requestId,omitempty"`
	ExecutionID     string         `json:"executionId,omitempty"`
	TurnID          string         `json:"turnId,omitempty"`
	ParentTurnID    string         `json:"parentTurnId,omitempty"`
	ParentBlockID   string         `json:"parentBlockId,omitempty"`
	AgentID         string         `json:"agentId,omitempty"`
	TurnSequence    int64          `json:"turnSequence,omitempty"`
	BlockID         string         `json:"blockId,omitempty"`
	BlockSequence   int64          `json:"blockSequence,omitempty"`
	MessageID       string         `json:"messageId,omitempty"`
	MessageSequence int64          `json:"messageSequence,omitempty"`
	CallID          string         `json:"callId,omitempty"`
	Revision        int64          `json:"revision,omitempty"`
	Type            string         `json:"type"`
	Status          string         `json:"status,omitempty"`
	Payload         map[string]any `json:"payload,omitempty"`
	CreatedAt       string         `json:"createdAt"`
}

type BlockState struct {
	BlockID       string         `json:"blockId"`
	BlockSequence int64          `json:"blockSequence"`
	Type          string         `json:"type"`
	Status        string         `json:"status"`
	CallID        string         `json:"callId,omitempty"`
	Revision      int64          `json:"revision,omitempty"`
	Content       string         `json:"content,omitempty"`
	Payload       map[string]any `json:"payload,omitempty"`
}

type ActiveTurnState struct {
	TurnID         string       `json:"turnId"`
	TurnSequence   int64        `json:"turnSequence"`
	RequestID      string       `json:"requestId,omitempty"`
	ExecutionID    string       `json:"executionId,omitempty"`
	ParentTurnID   string       `json:"parentTurnId,omitempty"`
	ParentBlockID  string       `json:"parentBlockId,omitempty"`
	AgentID        string       `json:"agentId,omitempty"`
	Status         string       `json:"status"`
	LastSequence   int64        `json:"lastEventSequence"`
	Blocks         []BlockState `json:"blocks"`
	StartedAt      string       `json:"startedAt,omitempty"`
	LastModifiedAt string       `json:"lastModifiedAt,omitempty"`
}

type RuntimeSnapshot struct {
	LastEventSequence int64            `json:"lastEventSequence"`
	ActiveTurn        *ActiveTurnState `json:"activeTurn,omitempty"`
}

type DurableStore interface {
	Persist(context.Context, AgentUIEvent) error
	ListAfter(context.Context, string, int64, int) ([]AgentUIEvent, error)
	LatestSequence(context.Context, string) (int64, error)
}

type executionState struct {
	turnID         string
	ctx            context.Context
	cancel         context.CancelFunc
	providerCancel context.CancelFunc
	release        func()
	steers         []string
}

type conversationState struct {
	mu           sync.Mutex
	initialized  bool
	lastSequence int64
	ring         []AgentUIEvent
	subscribers  map[string]chan AgentUIEvent
	activeTurn   *ActiveTurnState
	execution    *executionState
}

type Manager struct {
	mu             sync.RWMutex
	states         map[string]*conversationState
	ringSize       int
	durable        DurableStore
	executionSlots chan struct{}
}

var defaultManager = NewManager()

func NewManager() *Manager {
	return &Manager{states: map[string]*conversationState{}, ringSize: 4096, executionSlots: make(chan struct{}, 16)}
}

func DefaultManager() *Manager {
	return defaultManager
}

func (m *Manager) SetRingSize(size int) {
	if size <= 0 {
		size = 4096
	}
	m.mu.Lock()
	m.ringSize = size
	m.mu.Unlock()
}

func (m *Manager) SetDurableStore(store DurableStore) {
	m.mu.Lock()
	m.durable = store
	m.mu.Unlock()
}

func (m *Manager) SetMaxConcurrentExecutions(size int) {
	if size <= 0 {
		size = 16
	}
	m.mu.Lock()
	m.executionSlots = make(chan struct{}, size)
	m.mu.Unlock()
}

func (m *Manager) Publish(ctx context.Context, event AgentUIEvent, durable bool) (AgentUIEvent, error) {
	conversationID := strings.TrimSpace(event.ConversationID)
	if conversationID == "" {
		return AgentUIEvent{}, errors.New("conversation id required")
	}
	event.ConversationID = conversationID
	if strings.TrimSpace(event.Type) == "" {
		return AgentUIEvent{}, errors.New("event type required")
	}
	state := m.state(conversationID)
	store := m.durableStore()
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.initialized {
		if store != nil {
			latest, err := store.LatestSequence(ctx, conversationID)
			if err != nil {
				return AgentUIEvent{}, err
			}
			state.lastSequence = latest
		}
		state.initialized = true
	}
	event.Version = ProtocolVersion
	if strings.TrimSpace(event.EventID) == "" {
		event.EventID = newEventID()
	}
	event.EventSequence = state.lastSequence + 1
	if strings.TrimSpace(event.CreatedAt) == "" {
		event.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if event.Payload == nil {
		event.Payload = map[string]any{}
	}
	if durable && store != nil {
		if err := store.Persist(ctx, event); err != nil {
			return AgentUIEvent{}, err
		}
	}
	state.lastSequence = event.EventSequence
	active, terminal := reduceActiveTurn(state.activeTurn, event)
	state.activeTurn = active
	if terminal {
		state.activeTurn = nil
	}
	state.ring = append(state.ring, cloneEvent(event))
	ringSize := m.currentRingSize()
	if len(state.ring) > ringSize {
		state.ring = append([]AgentUIEvent(nil), state.ring[len(state.ring)-ringSize:]...)
	}
	for _, subscriber := range state.subscribers {
		select {
		case subscriber <- cloneEvent(event):
		default:
		}
	}
	return event, nil
}

func (m *Manager) Subscribe(conversationID, subscriberID string, afterSequence int64) (<-chan AgentUIEvent, []AgentUIEvent, bool, func()) {
	conversationID = strings.TrimSpace(conversationID)
	subscriberID = strings.TrimSpace(subscriberID)
	if subscriberID == "" {
		subscriberID = newEventID()
	}
	state := m.state(conversationID)
	state.mu.Lock()
	bufferSize := m.currentRingSize()
	if bufferSize > 1024 {
		bufferSize = 1024
	}
	if bufferSize < 64 {
		bufferSize = 64
	}
	ch := make(chan AgentUIEvent, bufferSize)
	state.subscribers[subscriberID] = ch
	replay := make([]AgentUIEvent, 0)
	for _, event := range state.ring {
		if event.EventSequence > afterSequence {
			replay = append(replay, cloneEvent(event))
		}
	}
	covered := afterSequence == 0 || afterSequence == state.lastSequence
	if afterSequence > state.lastSequence {
		covered = false
	} else if afterSequence > 0 && len(state.ring) > 0 {
		oldest := state.ring[0].EventSequence
		covered = afterSequence >= oldest-1
	} else if afterSequence > 0 && len(state.ring) == 0 {
		covered = afterSequence == state.lastSequence
	}
	state.mu.Unlock()
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			state.mu.Lock()
			if current, ok := state.subscribers[subscriberID]; ok && current == ch {
				delete(state.subscribers, subscriberID)
				close(ch)
			}
			state.mu.Unlock()
		})
	}
	return ch, replay, covered, cancel
}

func (m *Manager) RuntimeSnapshot(conversationID string) RuntimeSnapshot {
	state := m.state(strings.TrimSpace(conversationID))
	state.mu.Lock()
	defer state.mu.Unlock()
	snapshot := RuntimeSnapshot{LastEventSequence: state.lastSequence}
	if state.activeTurn != nil {
		snapshot.ActiveTurn = cloneActiveTurn(state.activeTurn)
	}
	return snapshot
}

func (m *Manager) LatestSequence(ctx context.Context, conversationID string) (int64, error) {
	conversationID = strings.TrimSpace(conversationID)
	state := m.state(conversationID)
	store := m.durableStore()
	state.mu.Lock()
	defer state.mu.Unlock()
	latest := state.lastSequence
	if store != nil {
		persisted, err := store.LatestSequence(ctx, conversationID)
		if err != nil {
			return latest, err
		}
		if persisted > latest {
			latest = persisted
			if !state.initialized {
				state.lastSequence = persisted
			}
		}
	}
	return latest, nil
}

func (m *Manager) ListDurableAfter(ctx context.Context, conversationID string, afterSequence int64, limit int) ([]AgentUIEvent, error) {
	store := m.durableStore()
	if store == nil {
		return []AgentUIEvent{}, nil
	}
	return store.ListAfter(ctx, strings.TrimSpace(conversationID), afterSequence, limit)
}

func (m *Manager) BeginExecution(conversationID, turnID string) (context.Context, context.CancelFunc, bool) {
	conversationID = strings.TrimSpace(conversationID)
	turnID = strings.TrimSpace(turnID)
	state := m.state(conversationID)
	state.mu.Lock()
	if state.execution != nil {
		state.mu.Unlock()
		return nil, func() {}, false
	}
	state.mu.Unlock()
	slots := m.currentExecutionSlots()
	if slots != nil {
		slots <- struct{}{}
	}
	state.mu.Lock()
	if state.execution != nil {
		state.mu.Unlock()
		if slots != nil {
			<-slots
		}
		return nil, func() {}, false
	}
	ctx, cancel := context.WithCancel(context.Background())
	release := func() {
		if slots != nil {
			<-slots
		}
	}
	state.execution = &executionState{turnID: turnID, ctx: ctx, cancel: cancel, release: release}
	state.mu.Unlock()
	return ctx, cancel, true
}

func (m *Manager) ClearExecution(conversationID, turnID string) {
	state := m.state(strings.TrimSpace(conversationID))
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.execution == nil || state.execution.turnID != strings.TrimSpace(turnID) {
		return
	}
	if state.execution.providerCancel != nil {
		state.execution.providerCancel()
	}
	state.execution.cancel()
	if state.execution.release != nil {
		state.execution.release()
	}
	state.execution = nil
}

func (m *Manager) HasExecution(conversationID, turnID string) bool {
	state := m.state(strings.TrimSpace(conversationID))
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.execution != nil && state.execution.turnID == strings.TrimSpace(turnID)
}

func (m *Manager) Interrupt(conversationID, turnID string) bool {
	state := m.state(strings.TrimSpace(conversationID))
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.execution == nil || state.execution.turnID != strings.TrimSpace(turnID) {
		return false
	}
	if state.execution.providerCancel != nil {
		state.execution.providerCancel()
	}
	state.execution.cancel()
	return true
}

func (m *Manager) Steer(conversationID, turnID, input string) bool {
	input = strings.TrimSpace(input)
	if input == "" {
		return false
	}
	state := m.state(strings.TrimSpace(conversationID))
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.execution == nil || state.execution.turnID != strings.TrimSpace(turnID) {
		return false
	}
	state.execution.steers = append(state.execution.steers, input)
	if state.execution.providerCancel != nil {
		state.execution.providerCancel()
	}
	return true
}

func (m *Manager) ConsumeSteer(conversationID, turnID string) []string {
	state := m.state(strings.TrimSpace(conversationID))
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.execution == nil || state.execution.turnID != strings.TrimSpace(turnID) || len(state.execution.steers) == 0 {
		return nil
	}
	values := append([]string(nil), state.execution.steers...)
	state.execution.steers = nil
	return values
}

func (m *Manager) RegisterProviderCancel(conversationID, turnID string, cancel context.CancelFunc) {
	state := m.state(strings.TrimSpace(conversationID))
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.execution != nil && state.execution.turnID == strings.TrimSpace(turnID) {
		state.execution.providerCancel = cancel
	}
}

func (m *Manager) ClearProviderCancel(conversationID, turnID string) {
	state := m.state(strings.TrimSpace(conversationID))
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.execution != nil && state.execution.turnID == strings.TrimSpace(turnID) {
		state.execution.providerCancel = nil
	}
}

func (m *Manager) state(conversationID string) *conversationState {
	m.mu.RLock()
	state := m.states[conversationID]
	m.mu.RUnlock()
	if state != nil {
		return state
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if state = m.states[conversationID]; state == nil {
		state = &conversationState{subscribers: map[string]chan AgentUIEvent{}}
		m.states[conversationID] = state
	}
	return state
}

func (m *Manager) durableStore() DurableStore {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.durable
}

func (m *Manager) currentExecutionSlots() chan struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.executionSlots
}

func (m *Manager) currentRingSize() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ringSize <= 0 {
		return 4096
	}
	return m.ringSize
}

func reduceActiveTurn(active *ActiveTurnState, event AgentUIEvent) (*ActiveTurnState, bool) {
	if strings.TrimSpace(event.TurnID) == "" {
		return active, false
	}
	if active == nil || active.TurnID != event.TurnID {
		active = &ActiveTurnState{TurnID: event.TurnID, TurnSequence: event.TurnSequence, RequestID: event.RequestID, ExecutionID: event.ExecutionID, ParentTurnID: event.ParentTurnID, ParentBlockID: event.ParentBlockID, AgentID: event.AgentID, Status: initialTurnStatus(event.Type, event.Status), StartedAt: event.CreatedAt, LastModifiedAt: event.CreatedAt, Blocks: []BlockState{}}
	}
	active.LastSequence = event.EventSequence
	active.LastModifiedAt = event.CreatedAt
	if event.RequestID != "" {
		active.RequestID = event.RequestID
	}
	if event.ExecutionID != "" {
		active.ExecutionID = event.ExecutionID
	}
	if event.ParentTurnID != "" {
		active.ParentTurnID = event.ParentTurnID
	}
	if event.ParentBlockID != "" {
		active.ParentBlockID = event.ParentBlockID
	}
	if event.AgentID != "" {
		active.AgentID = event.AgentID
	}
	if event.TurnSequence > 0 {
		active.TurnSequence = event.TurnSequence
	}
	switch event.Type {
	case "turn.queued":
		active.Status = "queued"
		return active, false
	case "turn.started", "turn.steered":
		active.Status = "running"
		return active, false
	case "turn.cancelling":
		active.Status = "cancelling"
		return active, false
	case "approval.requested":
		active.Status = "waiting_approval"
		return active, false
	case "approval.approved", "approval.denied", "approval.expired":
		active.Status = "running"
		return active, false
	case "turn.completed":
		active.Status = "completed"
		if strings.TrimSpace(event.MessageID) != "" {
			for index := len(active.Blocks) - 1; index >= 0; index-- {
				if active.Blocks[index].Type == "text" {
					active.Blocks[index].Status = "completed"
					break
				}
			}
		}
		return active, true
	case "turn.failed":
		active.Status = "failed"
		finalizeOpenBlocks(active, "failed")
		return active, true
	case "turn.interrupted":
		active.Status = "interrupted"
		finalizeOpenBlocks(active, "interrupted")
		return active, true
	}
	if strings.TrimSpace(event.BlockID) == "" {
		return active, false
	}
	reduceBlock(active, event)
	if active.Status != "waiting_approval" && active.Status != "cancelling" {
		hasRunningTool := false
		for _, block := range active.Blocks {
			if block.Type != "tool_call" {
				continue
			}
			switch block.Status {
			case "completed", "failed", "interrupted":
			default:
				hasRunningTool = true
			}
		}
		if hasRunningTool {
			active.Status = "waiting_tool"
		} else if active.Status == "waiting_tool" {
			active.Status = "running"
		}
	}
	return active, false
}

func finalizeOpenBlocks(turn *ActiveTurnState, status string) {
	if turn == nil {
		return
	}
	for index := range turn.Blocks {
		switch turn.Blocks[index].Status {
		case "completed", "failed", "interrupted":
		default:
			turn.Blocks[index].Status = status
		}
	}
}

func reduceBlock(turn *ActiveTurnState, event AgentUIEvent) {
	index := -1
	for i := range turn.Blocks {
		if turn.Blocks[i].BlockID == event.BlockID {
			index = i
			break
		}
	}
	blockType := eventBlockType(event)
	if index < 0 {
		turn.Blocks = append(turn.Blocks, BlockState{BlockID: event.BlockID, BlockSequence: event.BlockSequence, Type: blockType, Status: event.Status, CallID: event.CallID, Revision: event.Revision, Payload: map[string]any{}})
		index = len(turn.Blocks) - 1
	}
	block := turn.Blocks[index]
	if event.Revision > 0 && block.Revision > event.Revision {
		return
	}
	if event.BlockSequence > 0 {
		block.BlockSequence = event.BlockSequence
	}
	if blockType != "" {
		block.Type = blockType
	}
	if event.CallID != "" {
		block.CallID = event.CallID
	}
	if event.Revision > 0 {
		block.Revision = event.Revision
	}
	if block.Payload == nil {
		block.Payload = map[string]any{}
	}
	payload := event.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	if delta, ok := payload["delta"].(string); ok && strings.HasSuffix(event.Type, ".delta") {
		if event.Type == "tool.arguments.delta" || stringValue(payload["field"]) == "arguments" {
			block.Payload["arguments"] = stringValue(block.Payload["arguments"]) + delta
		} else {
			block.Content += delta
		}
	}
	if !strings.HasSuffix(event.Type, ".delta") {
		if value, exists := payload["content"]; exists {
			block.Content = stringValue(value)
		}
	}
	if value, exists := payload["arguments"]; exists {
		block.Payload["arguments"] = stringValue(value)
	}
	if value, exists := payload["result"]; exists {
		block.Payload["result"] = stringValue(value)
	}
	for _, key := range []string{"toolName", "errorCode", "durationMs", "recoveryCheckpoint"} {
		if value, exists := payload[key]; exists {
			block.Payload[key] = value
		}
	}
	block.Status = blockEventStatus(event.Type, event.Status, block.Status)
	turn.Blocks[index] = block
	sort.SliceStable(turn.Blocks, func(i, j int) bool { return turn.Blocks[i].BlockSequence < turn.Blocks[j].BlockSequence })
}

func eventBlockType(event AgentUIEvent) string {
	if value := strings.TrimSpace(stringValue(event.Payload["blockType"])); value != "" {
		return value
	}
	switch {
	case strings.HasPrefix(event.Type, "reasoning."):
		return "reasoning"
	case strings.HasPrefix(event.Type, "text."):
		return "text"
	case strings.HasPrefix(event.Type, "tool."):
		return "tool_call"
	}
	parts := strings.SplitN(event.Type, ".", 2)
	return parts[0]
}

func blockEventStatus(eventType, eventStatus, current string) string {
	if eventType == "tool.arguments.completed" {
		if strings.TrimSpace(eventStatus) != "" {
			return eventStatus
		}
		if current != "" {
			return current
		}
		return "running"
	}
	if strings.HasSuffix(eventType, ".failed") {
		return "failed"
	}
	if strings.HasSuffix(eventType, ".interrupted") {
		return "interrupted"
	}
	if strings.HasSuffix(eventType, ".completed") {
		return "completed"
	}
	if eventType == "tool.running" {
		return "running"
	}
	if strings.TrimSpace(eventStatus) != "" {
		return eventStatus
	}
	if current != "" {
		return current
	}
	return "running"
}

func initialTurnStatus(eventType, status string) string {
	switch eventType {
	case "turn.queued":
		return "queued"
	case "turn.cancelling":
		return "cancelling"
	case "turn.completed":
		return "completed"
	case "turn.failed":
		return "failed"
	case "turn.interrupted":
		return "interrupted"
	case "approval.requested":
		return "waiting_approval"
	}
	if value := strings.TrimSpace(status); value != "" && value != "streaming" {
		return value
	}
	return "running"
}

func cloneEvent(event AgentUIEvent) AgentUIEvent {
	copyEvent := event
	copyEvent.Payload = cloneMap(event.Payload)
	return copyEvent
}

func cloneActiveTurn(turn *ActiveTurnState) *ActiveTurnState {
	if turn == nil {
		return nil
	}
	copyTurn := *turn
	copyTurn.Blocks = make([]BlockState, len(turn.Blocks))
	for i := range turn.Blocks {
		copyTurn.Blocks[i] = turn.Blocks[i]
		copyTurn.Blocks[i].Payload = cloneMap(turn.Blocks[i].Payload)
	}
	return &copyTurn
}

func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return strings.TrimSpace(strings.ReplaceAll(fmt.Sprint(value), "<nil>", ""))
}

func newEventID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return time.Now().UTC().Format("20060102T150405.000000000")
	}
	return hex.EncodeToString(raw[:])
}

package v2

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrConnectionClosed = errors.New("connection closed")
)

type ConnectionState string

const (
	ConnStateHandshake  ConnectionState = "handshake"
	ConnStateConnected  ConnectionState = "connected"
	ConnStateDegraded   ConnectionState = "degraded"
	ConnStateClosing    ConnectionState = "closing"
	ConnStateClosed     ConnectionState = "closed"
)

type Connection struct {
	ID       string
	UserID   string
	DeviceID string
	RuntimeID string
	SessionID string
	State     ConnectionState
	LastSeq   int64
	LastBeat  time.Time

	mu       sync.RWMutex
	sendCh   []byte
}

func (c *Connection) GetState() ConnectionState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.State
}

func (c *Connection) SetState(s ConnectionState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.State = s
}

type Handler struct {
	sessions  SessionService
	commands  CommandService
	events    EventService
	states    ActualStateService

	connections map[string]*Connection
	mu          sync.RWMutex
}

func NewHandler(services *Services) *Handler {
	return &Handler{
		sessions:    services.Sessions,
		commands:    services.Commands,
		events:      services.Events,
		states:      services.ActualStates,
		connections: make(map[string]*Connection),
	}
}

func (h *Handler) HandleConnect(userID, deviceID, runtimeID string) (*Connection, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := userID + ":" + deviceID + ":" + runtimeID
	if existing, ok := h.connections[key]; ok && existing.GetState() == ConnStateConnected {
		_ = h.sessions.SupersedeSession(existing.SessionID, "new_connection")
	}

	conn := &Connection{
		ID:        "conn_" + uuid.NewString(),
		UserID:    userID,
		DeviceID:  deviceID,
		RuntimeID: runtimeID,
		State:     ConnStateHandshake,
		LastSeq:   0,
		LastBeat:  time.Now(),
	}

	h.connections[key] = conn
	return conn, nil
}

func (h *Handler) HandleHello(conn *Connection, payload *HelloPayload) (*HelloAckPayload, error) {
	if conn == nil {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "connection is nil")
	}

	if payload.RuntimeID != conn.RuntimeID {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "runtime_id mismatch")
	}

	newSession, oldSession, err := h.sessions.AcquireSession(
		nil,
		conn.UserID, conn.DeviceID, conn.RuntimeID,
		payload.Capabilities, payload.RuntimeContractVersion,
		payload.LastAppliedDesiredRevision,
		payload.LastProcessedCommandSequence,
		payload.LastEventSequence,
		payload.RuntimeContractVersion,
	)
	if err != nil {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, fmt.Sprintf("session acquire failed: %v", err))
	}
	_ = oldSession

	conn.SessionID = newSession.ID
	conn.State = ConnStateConnected
	conn.LastSeq = newSession.LastEventSequence

	return &HelloAckPayload{
		Accepted:         true,
		SessionID:        newSession.ID,
		ServerTime:       time.Now(),
		DesiredRevision:  newSession.LastAppliedDesiredRevision,
		ResumeMode:       "resume_or_full",
	}, nil
}

func (h *Handler) HandleCommandAck(conn *Connection, env *Envelope, ack *CommandAckPayload) error {
	if conn == nil || conn.SessionID == "" {
		return ErrConnectionClosed
	}

	newSeq := conn.LastSeq + 1
	if env.Sequence <= conn.LastSeq {
		return nil
	}

	event := &EventRecord{
		ID:               "rtevtv2_" + uuid.NewString(),
		EventType:        "command.ack",
		Payload:          env.Payload,
		PayloadHash:      env.PayloadHash,
		RuntimeSessionID: conn.SessionID,
		Sequence:         newSeq,
		OccurredAt:       time.Now().Format("2006-01-02 15:04:05"),
		Delivered:        0,
		InsertedAt:       time.Now().Format("2006-01-02 15:04:05"),
	}

	if _, err := h.events.Append(event.EventType, event.Payload, conn.SessionID, newSeq, "runtime_command", nil); err != nil {
		return fmt.Errorf("append event failed: %v", err)
	}

	conn.LastSeq = newSeq
	return nil
}

func (h *Handler) HandleEvent(conn *Connection, eventType string, payload []byte) (*EventRecord, error) {
	if conn == nil || conn.SessionID == "" {
		return nil, ErrConnectionClosed
	}

	newSeq := conn.LastSeq + 1

	event, err := h.events.Append(eventType, payload, conn.SessionID, newSeq, TriggerSourceRuntimeCommand, nil)
	if err != nil {
		return nil, fmt.Errorf("append event failed: %v", err)
	}

	conn.LastSeq = newSeq
	return event, nil
}

func (h *Handler) HandleDisconnect(conn *Connection) error {
	if conn == nil {
		return nil
	}

	conn.SetState(ConnStateClosed)
	return h.sessions.SupersedeSession(conn.SessionID, "client_disconnect")
}

func (h *Handler) HandleHeartbeat(conn *Connection) error {
	if conn == nil {
		return ErrConnectionClosed
	}

	now := time.Now()
	conn.LastBeat = now

	return h.sessions.UpdateLastHeartbeat(conn.SessionID)
}

func (h *Handler) CreateEnvelope(msgType MessageType, msgName, runtimeID, sessionID string, payload interface{}, userID, deviceID string) (*Envelope, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload failed: %v", err)
	}

	env := &Envelope{
		EnvelopeVersion:      EnvelopeVersion,
		Protocol:             ProtocolName,
		MessageType:          msgType,
		MessageName:          msgName,
		MessageID:            "msg_" + uuid.NewString(),
		UserID:               userID,
		DeviceID:             deviceID,
		RuntimeID:            runtimeID,
		RuntimeSessionID:     sessionID,
		ConnectionGeneration: 1,
		Sequence:             0,
		PayloadSchemaVersion: 1,
		PayloadHash:          ComputePayloadHash(payloadBytes),
		SentAt:               time.Now(),
		Payload:              payloadBytes,
	}

	return env, nil
}

func (h *Handler) GetConnection(userID, deviceID, runtimeID string) *Connection {
	h.mu.RLock()
	defer h.mu.RUnlock()
	key := userID + ":" + deviceID + ":" + runtimeID
	return h.connections[key]
}

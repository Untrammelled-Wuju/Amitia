package v2

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/deviceruntime"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/log"
)

var (
	ErrConnectionClosed = errors.New("connection closed")
)

type ConnectionState = protocol.ConnectionState

const (
	ConnStateHandshake = protocol.ConnectionStateHandshake
	ConnStateConnected = protocol.ConnectionStateConnected
	ConnStateDegraded  = protocol.ConnectionStateDegraded
	ConnStateClosing   = protocol.ConnectionStateClosing
	ConnStateClosed    = protocol.ConnectionStateClosed
)

type Connection struct {
	ID         string
	UserID     runtimeidentity.UserID
	DeviceID   runtimeidentity.DeviceID
	RuntimeID  runtimeidentity.RuntimeID
	SessionID  string
	Generation int64
	State      ConnectionState
	LastSeq    int64
	LastBeat   time.Time

	mu     sync.RWMutex
	sendCh []byte
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

func (c *Connection) SessionIdentity() protocol.SessionIdentity {
	return protocol.SessionIdentity{
		UserID:           c.UserID,
		DeviceID:         c.DeviceID,
		RuntimeID:        c.RuntimeID,
		RuntimeSessionID: runtimeidentity.ParseRuntimeSessionID(c.SessionID),
	}
}

func (c *Connection) TouchHeartbeat(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LastBeat = now
}

type Handler struct {
	sessions SessionService
	commands CommandService
	events   EventService
	states   ActualStateService

	deviceRuntimeSessions *deviceruntime.Service

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

func NewHandlerWithDeviceRuntime(services *Services, deviceRuntimeSessions *deviceruntime.Service) *Handler {
	h := NewHandler(services)
	h.deviceRuntimeSessions = deviceRuntimeSessions
	return h
}

func RuntimeConnectionKey(
	userID runtimeidentity.UserID,
	deviceID runtimeidentity.DeviceID,
	runtimeID runtimeidentity.RuntimeID,
) string {
	parts := []string{
		strings.TrimSpace(userID.String()),
		strings.TrimSpace(deviceID.String()),
		strings.TrimSpace(runtimeID.String()),
	}
	return strings.Join(parts, "\x00")
}

func (h *Handler) HandleConnect(userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) (*Connection, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := RuntimeConnectionKey(userID, deviceID, runtimeID)
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

	if payload.DeviceID != conn.DeviceID {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "device_id mismatch")
	}

	if payload.RuntimeID != conn.RuntimeID {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "runtime_id mismatch")
	}

	now := time.Now().UTC()

	if h.deviceRuntimeSessions != nil {
		acqReq := HelloToAcquireRequest(*payload, conn.UserID, runtimeidentity.PlatformUnknown, now)

		result, err := h.deviceRuntimeSessions.Acquire(nil, acqReq)
		if err != nil {
			return nil, NewProtocolError(ErrCodeEnvelopeInvalid, fmt.Sprintf("session acquire failed: %v", err))
		}

		projection, projectionErr := h.sessions.SyncFromDeviceRuntimeSession(nil, result.Session, *payload)
		if projectionErr != nil {
			_ = h.deviceRuntimeSessions.Close(nil, result.Session.ID, result.Session.ConnectionGeneration, "desktop_pet_projection_failed", now)
			return nil, NewProtocolError(ErrCodeEnvelopeInvalid, fmt.Sprintf("sync projection failed: %v", projectionErr))
		}

		var markErr error
		if result.Session.Status != protocol.SessionStatusReady {
			_, markErr = h.deviceRuntimeSessions.MarkReady(nil, result.Session.ID, result.Session.ConnectionGeneration, now)
		}

		conn.SessionID = result.Session.ID.String()
		conn.Generation = result.Session.ConnectionGeneration
		conn.State = ConnStateConnected
		conn.LastSeq = result.Session.LastEventSequence

		helloAck := SessionResultToHelloAck(result, projection.LastAppliedDesiredRevision)

		if markErr != nil {
			_ = h.deviceRuntimeSessions.Close(nil, result.Session.ID, result.Session.ConnectionGeneration, "presence_projection_failed", now)
			return nil, NewProtocolError(ErrCodeEnvelopeInvalid, fmt.Sprintf("session mark ready failed: %v", markErr))
		}

		return helloAck, nil
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
	conn.Generation = newSession.ConnectionGeneration
	conn.State = ConnStateConnected
	conn.LastSeq = newSession.LastEventSequence

	return &HelloAckPayload{
		Accepted:        true,
		SessionID:       runtimeidentity.ParseRuntimeSessionID(newSession.ID),
		ServerTime:      time.Now(),
		DesiredRevision: newSession.LastAppliedDesiredRevision,
		ResumeMode:      string(protocol.ResumeModeResumeOrFull),
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

	ackCmdID := ack.CommandID
	if ackCmdID == "" {
		return nil
	}

	now := time.Now()
	switch CommandStatus(ack.Status) {
	case CommandStatusRuntimeReceived:
		if err := h.commands.MarkRuntimeReceived(ackCmdID, string(conn.RuntimeID), conn.SessionID, now); err != nil {
			log.Warn("[v2] MarkRuntimeReceived failed: ", err)
		}
	case CommandStatusRuntimeAccepted:
		if err := h.commands.MarkRuntimeAccepted(ackCmdID, string(conn.RuntimeID), conn.SessionID, now); err != nil {
			log.Warn("[v2] MarkRuntimeAccepted failed: ", err)
		}
	case CommandStatusRendererAccepted:
		if err := h.commands.MarkRendererAccepted(ackCmdID, string(conn.RuntimeID), conn.SessionID, now); err != nil {
			log.Warn("[v2] MarkRendererAccepted failed: ", err)
		}
	case CommandStatusCompleted:
		if err := h.commands.MarkCompleted(ackCmdID, "", now); err != nil {
			log.Warn("[v2] MarkCompleted failed: ", err)
		}
	case CommandStatusFailedTerminal:
		if err := h.commands.MarkFailed(ackCmdID, ack.RejectErrorCode, ack.RejectReason, now); err != nil {
			log.Warn("[v2] MarkFailed failed: ", err)
		}
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

	if h.deviceRuntimeSessions != nil {
		if conn.SessionID != "" && conn.Generation > 0 {
			err := h.deviceRuntimeSessions.Close(nil, runtimeidentity.ParseRuntimeSessionID(conn.SessionID), conn.Generation, "client_disconnect", time.Now().UTC())
			if err != nil && !errors.Is(err, deviceruntime.ErrConnectionSuperseded) {
				log.Warn("[v2] device runtime close failed: ", err)
			}
		}
		return h.sessions.SupersedeSession(conn.SessionID, "client_disconnect")
	}

	return h.sessions.SupersedeSession(conn.SessionID, "client_disconnect")
}

func (h *Handler) HandleHeartbeat(conn *Connection) error {
	if conn == nil {
		return ErrConnectionClosed
	}

	now := time.Now().UTC()
	conn.TouchHeartbeat(now)

	if h.deviceRuntimeSessions != nil && conn.SessionID != "" && conn.Generation > 0 {
		_, err := h.deviceRuntimeSessions.Heartbeat(nil, runtimeidentity.ParseRuntimeSessionID(conn.SessionID), conn.Generation, now)
		if err != nil {
			return err
		}
		return h.sessions.UpdateLastHeartbeat(conn.SessionID)
	}

	return h.sessions.UpdateLastHeartbeat(conn.SessionID)
}

func (h *Handler) CreateEnvelope(msgType MessageType, msgName string, runtimeID runtimeidentity.RuntimeID, sessionID runtimeidentity.RuntimeSessionID, payload interface{}, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID) (*Envelope, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload failed: %v", err)
	}

	coreEnv, err := protocol.BuildEnvelope(protocol.EnvelopeInput{
		Descriptor:           Descriptor,
		MessageType:          msgType,
		MessageName:          msgName,
		MessageID:            "msg_" + uuid.NewString(),
		Identity:             protocol.SessionIdentity{UserID: userID, DeviceID: deviceID, RuntimeID: runtimeID, RuntimeSessionID: sessionID},
		ConnectionGeneration: 1,
		Sequence:             0,
		PayloadSchemaVersion: 1,
		SentAt:               time.Now(),
		Payload:              payloadBytes,
	})
	if err != nil {
		return nil, err
	}

	return (*Envelope)(coreEnv), nil
}

func (h *Handler) GetConnection(userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) *Connection {
	h.mu.RLock()
	defer h.mu.RUnlock()
	key := RuntimeConnectionKey(userID, deviceID, runtimeID)
	return h.connections[key]
}

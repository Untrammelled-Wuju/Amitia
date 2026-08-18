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
	// LastSeq tracks the last successfully persisted inbound runtime envelope sequence.
	// OutboundSeq is independent: server->runtime envelopes must never advance the
	// replay cursor used for runtime->server envelopes.
	LastSeq     int64
	OutboundSeq int64
	LastBeat    time.Time

	mu     sync.RWMutex
	sendCh []byte
}

func (c *Connection) ActivateSession(sessionID string, generation, lastInboundSeq int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.SessionID = sessionID
	c.Generation = generation
	c.LastSeq = lastInboundSeq
	c.State = ConnStateConnected
}

func (c *Connection) SessionSnapshot() (string, int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.SessionID, c.Generation
}

func (c *Connection) SessionIDValue() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.SessionID
}

func (c *Connection) LastHeartbeat() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.LastBeat
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

func (c *Connection) LastInboundSequence() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.LastSeq
}

func (c *Connection) AcceptInboundSequence(seq int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if seq <= c.LastSeq {
		return false
	}
	c.LastSeq = seq
	return true
}

func (c *Connection) IsInboundSequenceNew(seq int64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return seq > c.LastSeq
}

func (c *Connection) NextOutboundSequence() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.OutboundSeq++
	return c.OutboundSeq
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
		existingSessionID := existing.SessionIDValue()
		if existingSessionID != "" {
			if err := h.sessions.SupersedeSession(existingSessionID, "new_connection"); err != nil {
				return nil, fmt.Errorf("supersede existing runtime session: %w", err)
			}
		}
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
			closeErr := h.deviceRuntimeSessions.Close(nil, result.Session.ID, result.Session.ConnectionGeneration, "desktop_pet_projection_failed", now)
			if closeErr != nil {
				projectionErr = errors.Join(projectionErr, fmt.Errorf("close failed session after projection error: %w", closeErr))
			}
			return nil, NewProtocolError(ErrCodeEnvelopeInvalid, fmt.Sprintf("sync projection failed: %v", projectionErr))
		}

		if result.Session.Status != protocol.SessionStatusReady {
			if _, markErr := h.deviceRuntimeSessions.MarkReady(nil, result.Session.ID, result.Session.ConnectionGeneration, now); markErr != nil {
				closeErr := h.deviceRuntimeSessions.Close(nil, result.Session.ID, result.Session.ConnectionGeneration, "presence_projection_failed", now)
				if closeErr != nil {
					markErr = errors.Join(markErr, fmt.Errorf("close failed session after mark-ready error: %w", closeErr))
				}
				return nil, NewProtocolError(ErrCodeEnvelopeInvalid, fmt.Sprintf("session mark ready failed: %v", markErr))
			}
		}

		conn.ActivateSession(result.Session.ID.String(), result.Session.ConnectionGeneration, result.Session.LastEventSequence)
		helloAck := SessionResultToHelloAck(result, projection.LastAppliedDesiredRevision)
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

	conn.ActivateSession(newSession.ID, newSession.ConnectionGeneration, newSession.LastEventSequence)

	return &HelloAckPayload{
		Accepted:        true,
		SessionID:       runtimeidentity.ParseRuntimeSessionID(newSession.ID),
		ServerTime:      time.Now(),
		DesiredRevision: newSession.LastAppliedDesiredRevision,
		ResumeMode:      string(protocol.ResumeModeResumeOrFull),
	}, nil
}

func (h *Handler) HandleCommandAck(conn *Connection, env *Envelope, ack *CommandAckPayload) error {
	if conn == nil {
		return ErrConnectionClosed
	}
	sessionID := conn.SessionIDValue()
	if sessionID == "" {
		return ErrConnectionClosed
	}
	if env == nil || ack == nil {
		return NewProtocolError(ErrCodeEnvelopeInvalid, "command ack envelope is required")
	}
	if !conn.IsInboundSequenceNew(env.Sequence) {
		// Exact/older retries are idempotently ignored. The unique event sequence
		// constraint provides a second persistence-level guard.
		return nil
	}

	ackCmdID := strings.TrimSpace(ack.CommandID)
	if ackCmdID == "" {
		return NewProtocolError(ErrCodeEnvelopeInvalid, "commandId is required")
	}
	status := CommandStatus(ack.Status)
	switch status {
	case CommandStatusRuntimeReceived, CommandStatusRuntimeAccepted, CommandStatusRendererAccepted,
		CommandStatusCompleted, CommandStatusFailedTerminal:
	default:
		return NewProtocolError(ErrCodeEnvelopeInvalid, fmt.Sprintf("unsupported command ack status: %s", ack.Status))
	}

	now := time.Now().UTC()
	switch status {
	case CommandStatusRuntimeReceived:
		if err := h.commands.MarkRuntimeReceived(ackCmdID, string(conn.RuntimeID), sessionID, now); err != nil {
			return fmt.Errorf("mark runtime received: %w", err)
		}
	case CommandStatusRuntimeAccepted:
		if err := h.commands.MarkRuntimeAccepted(ackCmdID, string(conn.RuntimeID), sessionID, now); err != nil {
			return fmt.Errorf("mark runtime accepted: %w", err)
		}
	case CommandStatusRendererAccepted:
		if err := h.commands.MarkRendererAccepted(ackCmdID, string(conn.RuntimeID), sessionID, now); err != nil {
			return fmt.Errorf("mark renderer accepted: %w", err)
		}
	case CommandStatusCompleted:
		if err := h.commands.MarkCompleted(ackCmdID, "", now); err != nil {
			return fmt.Errorf("mark command completed: %w", err)
		}
	case CommandStatusFailedTerminal:
		if err := h.commands.MarkFailed(ackCmdID, ack.RejectErrorCode, ack.RejectReason, now); err != nil {
			return fmt.Errorf("mark command failed: %w", err)
		}
	}

	if _, err := h.events.Append(
		EventTypeCommandAck,
		env.Payload,
		sessionID,
		env.Sequence,
		TriggerSourceRuntimeCommand,
		&ackCmdID,
	); err != nil {
		return fmt.Errorf("append command ack event: %w", err)
	}
	processedSeq := int64(0)
	if status.IsTerminal() {
		processedSeq = ack.CommandSequence
	}
	if err := h.persistAuthoritativeCursor(conn, env.Sequence, processedSeq, 0, ""); err != nil {
		return err
	}
	if !conn.AcceptInboundSequence(env.Sequence) {
		return nil
	}
	if err := h.sessions.UpdateLastEventSequence(sessionID, env.Sequence); err != nil {
		return fmt.Errorf("persist command ack sequence: %w", err)
	}
	return nil
}

func (h *Handler) HandleEvent(conn *Connection, env *Envelope) (*EventRecord, error) {
	if conn == nil {
		return nil, ErrConnectionClosed
	}
	sessionID := conn.SessionIDValue()
	if sessionID == "" {
		return nil, ErrConnectionClosed
	}
	if env == nil {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "runtime event envelope is required")
	}
	if !IsEventType(env.MessageName) {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, fmt.Sprintf("unsupported runtime event type: %s", env.MessageName))
	}
	if !conn.IsInboundSequenceNew(env.Sequence) {
		return nil, nil
	}

	var commandID *string
	if id := extractEventCommandID(env.Payload); id != "" {
		commandID = &id
	}
	event, err := h.events.Append(
		env.MessageName,
		env.Payload,
		sessionID,
		env.Sequence,
		TriggerSourceRuntimeCommand,
		commandID,
	)
	if err != nil {
		return nil, fmt.Errorf("append runtime event: %w", err)
	}
	processedSeq, appliedRevision, actualStateHash := runtimeEventCursor(env)
	if err := h.persistAuthoritativeCursor(conn, env.Sequence, processedSeq, appliedRevision, actualStateHash); err != nil {
		return nil, err
	}
	if !conn.AcceptInboundSequence(env.Sequence) {
		return event, nil
	}
	if err := h.sessions.UpdateLastEventSequence(sessionID, env.Sequence); err != nil {
		return nil, fmt.Errorf("persist runtime event sequence: %w", err)
	}
	return event, nil
}

func extractEventCommandID(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	var body struct {
		CommandID string `json:"commandId"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return ""
	}
	return strings.TrimSpace(body.CommandID)
}

func runtimeEventCursor(env *Envelope) (processedSeq, appliedRevision int64, actualStateHash string) {
	if env == nil || env.MessageName != EventStateSnapshot {
		return 0, 0, ""
	}
	var snapshot StateSnapshotPayload
	if err := json.Unmarshal(env.Payload, &snapshot); err != nil {
		return 0, 0, ""
	}
	return snapshot.LastProcessedCommandSequence, snapshot.AppliedDesiredRevision, snapshot.ActualStateHash
}

func (h *Handler) persistAuthoritativeCursor(conn *Connection, eventSeq, processedSeq, appliedRevision int64, actualStateHash string) error {
	if h.deviceRuntimeSessions == nil || conn == nil {
		return nil
	}
	sessionID, generation := conn.SessionSnapshot()
	if sessionID == "" || generation <= 0 {
		return nil
	}
	id := runtimeidentity.ParseRuntimeSessionID(sessionID)
	session, err := h.deviceRuntimeSessions.GetSession(nil, id)
	if err != nil {
		return fmt.Errorf("load authoritative runtime cursor: %w", err)
	}
	cursor := protocol.SessionCursor{
		ConnectionGeneration:         generation,
		LastAppliedStateRevision:     session.LastAppliedStateRevision,
		LastProcessedCommandSequence: session.LastProcessedCommandSequence,
		LastEventSequence:            session.LastEventSequence,
		ActualStateHash:              session.ActualStateHash,
	}
	if eventSeq > cursor.LastEventSequence {
		cursor.LastEventSequence = eventSeq
	}
	if processedSeq > cursor.LastProcessedCommandSequence {
		cursor.LastProcessedCommandSequence = processedSeq
	}
	if appliedRevision > cursor.LastAppliedStateRevision {
		cursor.LastAppliedStateRevision = appliedRevision
	}
	if actualStateHash != "" {
		cursor.ActualStateHash = actualStateHash
	}
	if err := h.deviceRuntimeSessions.UpdateCursor(nil, id, generation, cursor, time.Now().UTC()); err != nil {
		return fmt.Errorf("persist authoritative runtime cursor: %w", err)
	}
	return nil
}

func (h *Handler) HandleDisconnect(conn *Connection) error {
	if conn == nil {
		return nil
	}

	conn.SetState(ConnStateClosed)
	sessionID, generation := conn.SessionSnapshot()

	if h.deviceRuntimeSessions != nil {
		if sessionID != "" && generation > 0 {
			err := h.deviceRuntimeSessions.Close(nil, runtimeidentity.ParseRuntimeSessionID(sessionID), generation, "client_disconnect", time.Now().UTC())
			if err != nil && !errors.Is(err, deviceruntime.ErrConnectionSuperseded) {
				log.Warn("[v2] device runtime close failed: ", err)
			}
		}
		if sessionID == "" {
			return nil
		}
		return h.sessions.SupersedeSession(sessionID, "client_disconnect")
	}

	if sessionID == "" {
		return nil
	}
	return h.sessions.SupersedeSession(sessionID, "client_disconnect")
}

func (h *Handler) HandleHeartbeat(conn *Connection) error {
	if conn == nil {
		return ErrConnectionClosed
	}

	now := time.Now().UTC()
	conn.TouchHeartbeat(now)
	sessionID, generation := conn.SessionSnapshot()

	if h.deviceRuntimeSessions != nil && sessionID != "" && generation > 0 {
		_, err := h.deviceRuntimeSessions.Heartbeat(nil, runtimeidentity.ParseRuntimeSessionID(sessionID), generation, now)
		if err != nil {
			return err
		}
		return h.sessions.UpdateLastHeartbeat(sessionID)
	}

	return h.sessions.UpdateLastHeartbeat(sessionID)
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

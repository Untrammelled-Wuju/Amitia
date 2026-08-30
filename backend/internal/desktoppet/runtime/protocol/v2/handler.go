package v2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
	LastSeq        int64
	OutboundSeq    int64
	LastBeat       time.Time
	HandshakeAcked bool

	mu             sync.RWMutex
	fenceMu        sync.RWMutex
	sendCh         []byte
	runtimeVersion string
	capabilities   map[string]struct{}
	closeTransport func()
}

func (c *Connection) ActivateSession(sessionID string, generation, lastInboundSeq int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.SessionID = sessionID
	c.Generation = generation
	c.LastSeq = lastInboundSeq
	c.HandshakeAcked = false
	c.State = ConnStateConnected
}

func (c *Connection) MarkHandshakeAcked() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.HandshakeAcked = true
}

func (c *Connection) IsHandshakeAcked() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.HandshakeAcked
}

func (c *Connection) SetNegotiatedRuntime(version string, capabilities []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runtimeVersion = strings.TrimSpace(version)
	c.capabilities = make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		capability = strings.TrimSpace(capability)
		if capability != "" {
			c.capabilities[capability] = struct{}{}
		}
	}
}

func (c *Connection) HasCapability(capability string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.capabilities[strings.TrimSpace(capability)]
	return ok
}

func (c *Connection) SetTransportCloser(closeFn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeTransport = closeFn
}

func (c *Connection) CloseTransport() {
	c.mu.RLock()
	closeFn := c.closeTransport
	c.mu.RUnlock()
	if closeFn != nil {
		closeFn()
	}
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
	if existing, ok := h.connections[key]; ok && existing != nil {
		// A reconnect must fence the old websocket before replacing the registry
		// entry. In-flight mutations finish first; once this exclusive fence is
		// acquired, no stale ACK/event/heartbeat can begin after supersession.
		existing.fenceMu.Lock()
		existingState := existing.GetState()
		existingSessionID := existing.SessionIDValue()
		if existingState != ConnStateClosed && existingState != ConnStateClosing {
			if h.commands != nil {
				if err := h.commands.SupersedeEphemeralCommands(string(userID), string(deviceID), string(runtimeID), existingSessionID, "runtime connection replaced", time.Now().UTC()); err != nil {
					existing.fenceMu.Unlock()
					return nil, fmt.Errorf("supersede ephemeral runtime commands: %w", err)
				}
			}
			if existingSessionID != "" {
				if err := h.sessions.SupersedeSession(existingSessionID, "new_connection"); err != nil {
					existing.fenceMu.Unlock()
					return nil, fmt.Errorf("supersede existing runtime session: %w", err)
				}
			}
			existing.SetState(ConnStateClosing)
		}
		existing.fenceMu.Unlock()
		existing.CloseTransport()
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
	if payload == nil {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "hello payload is required")
	}

	conn.fenceMu.RLock()
	defer conn.fenceMu.RUnlock()
	if conn.GetState() != ConnStateHandshake {
		return nil, deviceruntime.ErrConnectionSuperseded
	}

	if payload.DeviceID != conn.DeviceID {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "device_id mismatch")
	}

	if payload.RuntimeID != conn.RuntimeID {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "runtime_id mismatch")
	}

	if strings.TrimSpace(payload.RuntimeVersion) == "" {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "runtimeVersion is required")
	}
	if strings.TrimSpace(payload.RuntimeVersion) != CurrentRuntimeVersion {
		return nil, NewProtocolError(
			ErrCodeProtocolUnsupported,
			fmt.Sprintf("unsupported desktop-pet runtime version: got %q, want %q", payload.RuntimeVersion, CurrentRuntimeVersion),
		)
	}

	if payload.RuntimeContractVersion != CurrentSchemaVersion {
		return nil, NewProtocolError(
			ErrCodeProtocolUnsupported,
			fmt.Sprintf("unsupported desktop-pet runtime contract version: got %q, want %q", payload.RuntimeContractVersion, CurrentSchemaVersion),
		)
	}
	mandatoryCapabilities := mandatoryRuntimeCapabilities()
	capabilitySet := make(map[string]struct{}, len(payload.Capabilities))
	for _, capability := range payload.Capabilities {
		capabilitySet[strings.TrimSpace(capability)] = struct{}{}
	}
	for _, required := range mandatoryCapabilities {
		if _, ok := capabilitySet[required]; !ok {
			return nil, NewProtocolError(ErrCodeProtocolUnsupported, "runtime missing mandatory capability: "+required)
		}
	}
	conn.SetNegotiatedRuntime(payload.RuntimeVersion, payload.Capabilities)

	now := time.Now().UTC()

	if h.deviceRuntimeSessions != nil {
		acqReq := HelloToAcquireRequest(*payload, conn.UserID, runtimeidentity.PlatformUnknown, now)

		result, err := h.deviceRuntimeSessions.Acquire(context.Background(), acqReq)
		if err != nil {
			return nil, NewProtocolError(ErrCodeEnvelopeInvalid, fmt.Sprintf("session acquire failed: %v", err))
		}

		_, projectionErr := h.sessions.SyncFromDeviceRuntimeSession(nil, result.Session, *payload)
		if projectionErr != nil {
			closeErr := h.deviceRuntimeSessions.Close(context.Background(), result.Session.ID, result.Session.ConnectionGeneration, "desktop_pet_projection_failed", now)
			if closeErr != nil {
				projectionErr = errors.Join(projectionErr, fmt.Errorf("close failed session after projection error: %w", closeErr))
			}
			return nil, NewProtocolError(ErrCodeEnvelopeInvalid, fmt.Sprintf("sync projection failed: %v", projectionErr))
		}

		if result.Session.Status != protocol.SessionStatusReady {
			if _, markErr := h.deviceRuntimeSessions.MarkReady(context.Background(), result.Session.ID, result.Session.ConnectionGeneration, now); markErr != nil {
				closeErr := h.deviceRuntimeSessions.Close(context.Background(), result.Session.ID, result.Session.ConnectionGeneration, "presence_projection_failed", now)
				if closeErr != nil {
					markErr = errors.Join(markErr, fmt.Errorf("close failed session after mark-ready error: %w", closeErr))
				}
				return nil, NewProtocolError(ErrCodeEnvelopeInvalid, fmt.Sprintf("session mark ready failed: %v", markErr))
			}
		}

		conn.ActivateSession(result.Session.ID.String(), result.Session.ConnectionGeneration, result.Session.LastEventSequence)
		authoritativeRevision, reconcileErr := h.commands.ReconcileDesiredStateOnHello(
			string(conn.UserID), string(conn.DeviceID), string(conn.RuntimeID),
			payload.LastAppliedDesiredRevision, result.Session.ConnectionGeneration,
		)
		if reconcileErr != nil {
			return nil, NewProtocolError(ErrCodeEnvelopeInvalid, fmt.Sprintf("desired-state reconciliation failed: %v", reconcileErr))
		}
		helloAck := SessionResultToHelloAck(result, authoritativeRevision)
		if payload.LastAppliedDesiredRevision < authoritativeRevision {
			helloAck.ResumeMode = string(protocol.ResumeModeFull)
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

	conn.ActivateSession(newSession.ID, newSession.ConnectionGeneration, newSession.LastEventSequence)
	authoritativeRevision, reconcileErr := h.commands.ReconcileDesiredStateOnHello(
		string(conn.UserID), string(conn.DeviceID), string(conn.RuntimeID),
		payload.LastAppliedDesiredRevision, newSession.ConnectionGeneration,
	)
	if reconcileErr != nil {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, fmt.Sprintf("desired-state reconciliation failed: %v", reconcileErr))
	}
	resumeMode := protocol.ResumeModeResumeOrFull
	if payload.LastAppliedDesiredRevision < authoritativeRevision {
		resumeMode = protocol.ResumeModeFull
	}

	return &HelloAckPayload{
		Accepted:                           true,
		SessionID:                          runtimeidentity.ParseRuntimeSessionID(newSession.ID),
		ServerTime:                         time.Now(),
		DesiredRevision:                    authoritativeRevision,
		ResumeMode:                         string(resumeMode),
		ServerLastAppliedDesiredRevision:   newSession.LastAppliedDesiredRevision,
		ServerLastProcessedCommandSequence: newSession.LastProcessedCommandSequence,
		LastCommittedClientEventSequence:   newSession.LastEventSequence,
	}, nil
}

func validateEstablishedInboundEnvelope(conn *Connection, env *Envelope) (string, int64, error) {
	if conn == nil {
		return "", 0, ErrConnectionClosed
	}
	if env == nil {
		return "", 0, NewProtocolError(ErrCodeEnvelopeInvalid, "runtime envelope is required")
	}
	if conn.GetState() != ConnStateConnected {
		return "", 0, deviceruntime.ErrConnectionSuperseded
	}
	sessionID, generation := conn.SessionSnapshot()
	if sessionID == "" || generation <= 0 {
		return "", 0, ErrConnectionClosed
	}
	if env.UserID != conn.UserID || env.DeviceID != conn.DeviceID || env.RuntimeID != conn.RuntimeID {
		return "", 0, NewProtocolError(ErrCodeEnvelopeInvalid, "runtime envelope identity mismatch")
	}
	if env.RuntimeSessionID != runtimeidentity.ParseRuntimeSessionID(sessionID) {
		return "", 0, NewProtocolError(ErrCodeEnvelopeInvalid, "runtime envelope session mismatch")
	}
	if env.ConnectionGeneration != generation {
		return "", 0, deviceruntime.ErrConnectionSuperseded
	}
	return sessionID, generation, nil
}

func (h *Handler) validateCommandOwnership(conn *Connection, sessionID, commandID string) (*RuntimeCommand, error) {
	cmd, err := h.commands.GetCommand(commandID)
	if err != nil {
		return nil, err
	}
	if !cmd.HasValidClassification() {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "stored runtime command classification is invalid")
	}
	if cmd.UserID != string(conn.UserID) || cmd.DeviceID != string(conn.DeviceID) {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "command ownership mismatch")
	}
	if cmd.RuntimeID != "" && cmd.RuntimeID != string(conn.RuntimeID) {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "command runtime mismatch")
	}
	if cmd.RuntimeSessionID != "" && cmd.RuntimeSessionID != sessionID {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "command session mismatch")
	}
	return cmd, nil
}

func (h *Handler) HandleCommandAck(conn *Connection, env *Envelope, ack *CommandAckPayload) error {
	if conn == nil {
		return ErrConnectionClosed
	}
	conn.fenceMu.RLock()
	defer conn.fenceMu.RUnlock()

	if ack == nil {
		return NewProtocolError(ErrCodeEnvelopeInvalid, "command ack payload is required")
	}
	sessionID, _, err := validateEstablishedInboundEnvelope(conn, env)
	if err != nil {
		return err
	}
	if ack.RuntimeSessionID != sessionID {
		return NewProtocolError(ErrCodeEnvelopeInvalid, "command ack runtimeSessionId mismatch")
	}
	if ack.CommandSequence < 0 {
		return NewProtocolError(ErrCodeEnvelopeInvalid, "commandSequence must be non-negative")
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
	case CommandStatusRuntimeReceived, CommandStatusRuntimeAccepted, CommandStatusCompleted, CommandStatusFailedTerminal, CommandStatusExpired:
	default:
		return NewProtocolError(ErrCodeEnvelopeInvalid, fmt.Sprintf("unsupported command ack status: %s", ack.Status))
	}

	cmd, err := h.validateCommandOwnership(conn, sessionID, ackCmdID)
	if err != nil {
		return fmt.Errorf("validate command ack ownership: %w", err)
	}
	if cmd.DeviceSequence > 0 && ack.CommandSequence != cmd.DeviceSequence {
		return NewProtocolError(ErrCodeEnvelopeInvalid, "command ack sequence mismatch")
	}
	commandType := CommandType(cmd.CommandType)
	if status == CommandStatusCompleted {
		if commandType == CommandTypePlayAction || commandType.IsDurable() {
			return NewProtocolError(ErrCodeEnvelopeInvalid, "completed command_ack is forbidden for renderer/durable desired commands")
		}
	}
	if status == CommandStatusFailedTerminal {
		if commandType.IsDurable() {
			return NewProtocolError(ErrCodeEnvelopeInvalid, "failed_terminal command_ack is forbidden for durable desired commands; use desired_rejected")
		}
		if commandType == CommandTypePlayAction && strings.TrimSpace(cmd.PlaybackRequestID) != "" {
			return NewProtocolError(ErrCodeEnvelopeInvalid, "failed_terminal command_ack is forbidden after renderer acceptance; use playback.action_failed")
		}
	}
	if status == CommandStatusExpired {
		if commandType.IsDurable() {
			return NewProtocolError(ErrCodeEnvelopeInvalid, "expired command_ack is forbidden for durable desired commands")
		}
		if commandType == CommandTypePlayAction && strings.TrimSpace(cmd.PlaybackRequestID) != "" {
			return NewProtocolError(ErrCodeEnvelopeInvalid, "expired command_ack is forbidden after renderer acceptance; use playback failure event")
		}
	}

	now := time.Now().UTC()
	ignoreTransition := cmd.Status == string(CommandStatusSuperseded) ||
		cmd.Status == string(CommandStatusCancelled) ||
		cmd.Status == string(CommandStatusExpired)
	if !ignoreTransition {
		switch status {
		case CommandStatusRuntimeReceived:
			if err := h.commands.MarkRuntimeReceived(ackCmdID, string(conn.RuntimeID), sessionID, now); err != nil {
				return fmt.Errorf("mark runtime received: %w", err)
			}
		case CommandStatusRuntimeAccepted:
			if err := h.commands.MarkRuntimeAccepted(ackCmdID, string(conn.RuntimeID), sessionID, now); err != nil {
				return fmt.Errorf("mark runtime accepted: %w", err)
			}
		case CommandStatusCompleted:
			if err := h.commands.MarkCompleted(ackCmdID, "", now); err != nil {
				return fmt.Errorf("mark runtime command completed: %w", err)
			}
		case CommandStatusExpired:
			if err := h.commands.MarkExpired(ackCmdID, now); err != nil {
				return fmt.Errorf("mark runtime command expired: %w", err)
			}
		case CommandStatusFailedTerminal:
			if err := h.commands.MarkFailed(ackCmdID, ack.RejectErrorCode, ack.RejectReason, now); err != nil {
				return fmt.Errorf("mark command failed: %w", err)
			}
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
	conn.fenceMu.RLock()
	defer conn.fenceMu.RUnlock()

	sessionID, _, err := validateEstablishedInboundEnvelope(conn, env)
	if err != nil {
		return nil, err
	}
	if !IsEventType(env.MessageName) {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, fmt.Sprintf("unsupported runtime event type: %s", env.MessageName))
	}
	if !conn.IsInboundSequenceNew(env.Sequence) {
		return nil, nil
	}
	// Validate state snapshots before appending the event record. Otherwise an
	// invalid snapshot could consume its sequence in the event store and become
	// impossible to repair by retrying the same envelope.
	if env.MessageName == EventStateSnapshot {
		if _, err := h.runtimeActualStateFromSnapshot(conn, env); err != nil {
			return nil, err
		}
	}
	if err := h.validateRuntimeEventCommandOwnership(conn, sessionID, env); err != nil {
		return nil, err
	}
	// Reject semantically impossible lifecycle events before EventRecord
	// persistence. Otherwise an invalid event could permanently occupy its
	// session sequence and a corrected retry would conflict with that record.
	if err := h.validateRuntimeEventProgressPrecondition(env); err != nil {
		return nil, err
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
	if err := h.applyRuntimeEventCommandProgress(conn, sessionID, env); err != nil {
		return nil, fmt.Errorf("apply runtime playback progress: %w", err)
	}
	if err := h.appendRuntimeDomainEvent(conn, env); err != nil {
		return nil, fmt.Errorf("append runtime behavior event: %w", err)
	}
	if err := h.persistActualStateSnapshot(conn, env); err != nil {
		return nil, err
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

type runtimeEventMetadata struct {
	CommandID          string `json:"commandId"`
	PlaybackInstanceID string `json:"playbackInstanceId"`
	InstallationID     string `json:"installationId"`
	CharacterID        string `json:"characterId"`
	PetInstanceID      string `json:"petInstanceId"`
	DecisionID         string `json:"decisionId"`
	ErrorCode          string `json:"errorCode"`
	ErrorMessage       string `json:"errorMessage"`
	Reason             string `json:"reason"`
	InterruptReason    string `json:"interruptReason"`
	OccurredAt         string `json:"occurredAt"`
	DesiredRevision    int64  `json:"desiredRevision"`
	DesiredHash        string `json:"desiredHash"`
	AppliedDesiredHash string `json:"appliedDesiredHash"`
}

func decodeRuntimeEventMetadata(payload []byte) runtimeEventMetadata {
	var meta runtimeEventMetadata
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &meta)
	}
	meta.CommandID = strings.TrimSpace(meta.CommandID)
	meta.PlaybackInstanceID = strings.TrimSpace(meta.PlaybackInstanceID)
	meta.InstallationID = strings.TrimSpace(meta.InstallationID)
	meta.CharacterID = strings.TrimSpace(meta.CharacterID)
	meta.PetInstanceID = strings.TrimSpace(meta.PetInstanceID)
	meta.Reason = strings.TrimSpace(meta.Reason)
	meta.InterruptReason = strings.TrimSpace(meta.InterruptReason)
	meta.DesiredHash = strings.TrimSpace(meta.DesiredHash)
	meta.AppliedDesiredHash = strings.TrimSpace(meta.AppliedDesiredHash)
	if meta.DesiredHash == "" {
		meta.DesiredHash = meta.AppliedDesiredHash
	}
	if meta.Reason == "" {
		meta.Reason = meta.InterruptReason
	}
	return meta
}

func runtimeEventOccurredAt(meta runtimeEventMetadata) time.Time {
	if meta.OccurredAt != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, meta.OccurredAt); err == nil {
			return parsed.UTC()
		}
		if parsed, err := time.Parse(time.RFC3339, meta.OccurredAt); err == nil {
			return parsed.UTC()
		}
	}
	return time.Now().UTC()
}

func (h *Handler) validateRuntimeEventCommandOwnership(conn *Connection, sessionID string, env *Envelope) error {
	if env == nil {
		return nil
	}
	meta := decodeRuntimeEventMetadata(env.Payload)
	if meta.CommandID == "" {
		return nil
	}
	switch env.MessageName {
	case EventPlaybackCommandAccepted, EventPlaybackActionStarted, EventPlaybackActionFirstCycle, EventPlaybackActionHolding, EventPlaybackActionCompleted, EventPlaybackActionInterrupted, EventPlaybackActionFailed:
		cmd, err := h.validateCommandOwnership(conn, sessionID, meta.CommandID)
		if err != nil {
			return fmt.Errorf("validate runtime event command ownership: %w", err)
		}
		if CommandType(cmd.CommandType) != CommandTypePlayAction {
			return NewProtocolError(ErrCodeEnvelopeInvalid, "playback event is bound to a non-play_action command")
		}
		if meta.PlaybackInstanceID == "" {
			return NewProtocolError(ErrCodeEnvelopeInvalid, "playback event requires playbackInstanceId")
		}
		boundPlaybackID := strings.TrimSpace(cmd.PlaybackRequestID)
		if env.MessageName == EventPlaybackCommandAccepted {
			if boundPlaybackID != "" && boundPlaybackID != meta.PlaybackInstanceID {
				return NewProtocolError(ErrCodeEnvelopeInvalid, "renderer playback identity mismatch")
			}
			return nil
		}
		if boundPlaybackID == "" {
			// A renderer can reject/fail a submitted command before command_accepted.
			// That pre-accept failure is the only physical event allowed without an
			// already bound playback identity.
			if env.MessageName == EventPlaybackActionFailed && CommandStatus(cmd.Status) == CommandStatusRuntimeAccepted {
				return nil
			}
			return NewProtocolError(ErrCodeEnvelopeInvalid, "playback identity is not bound by command_accepted")
		}
		if boundPlaybackID != meta.PlaybackInstanceID {
			return NewProtocolError(ErrCodeEnvelopeInvalid, "playback event identity does not match renderer-accepted playback")
		}
	case EventStateDesiredApplied, EventStateDesiredRejected:
		cmd, err := h.validateCommandOwnership(conn, sessionID, meta.CommandID)
		if err != nil {
			return fmt.Errorf("validate runtime event command ownership: %w", err)
		}
		if !CommandType(cmd.CommandType).IsDurable() {
			return NewProtocolError(ErrCodeEnvelopeInvalid, "desired-state event is bound to a non-durable command")
		}
		var desired SyncDesiredStatePayload
		if err := json.Unmarshal([]byte(cmd.PayloadJSON), &desired); err != nil {
			return NewProtocolError(ErrCodeEnvelopeInvalid, "stored desired-state command payload is invalid")
		}
		if meta.DesiredRevision != desired.DesiredRevision || meta.DesiredRevision <= 0 {
			return NewProtocolError(ErrCodeEnvelopeInvalid, "desired-state event revision mismatch")
		}
		if desired.DesiredHash == "" || meta.DesiredHash == "" || meta.DesiredHash != desired.DesiredHash {
			return NewProtocolError(ErrCodeDesiredHashMismatch, "desired-state event hash mismatch")
		}
	}
	return nil
}

func (h *Handler) validateRuntimeEventProgressPrecondition(env *Envelope) error {
	if env == nil || h.commands == nil {
		return nil
	}
	meta := decodeRuntimeEventMetadata(env.Payload)
	if meta.CommandID == "" {
		return nil
	}
	cmd, err := h.commands.GetCommand(meta.CommandID)
	if err != nil {
		return err
	}
	current := CommandStatus(cmd.Status)
	valid := func(ok bool, expected string) error {
		if ok {
			return nil
		}
		return NewProtocolError(
			ErrCodeEnvelopeInvalid,
			fmt.Sprintf("invalid %s lifecycle event from command status %s", expected, current),
		)
	}

	switch env.MessageName {
	case EventPlaybackCommandAccepted:
		return valid(current == CommandStatusRendererAccepted || isValidProgressTransition(cmd, CommandStatusRendererAccepted), env.MessageName)
	case EventPlaybackActionStarted:
		return valid(current == CommandStatusPlaybackStarted || isValidProgressTransition(cmd, CommandStatusPlaybackStarted), env.MessageName)
	case EventPlaybackActionFirstCycle, EventPlaybackActionHolding:
		// on_started completion may already have moved the logical command to
		// completed while the same renderer playback continues physically.
		return valid(current == CommandStatusPlaybackStarted || current == CommandStatusCompleted, env.MessageName)
	case EventPlaybackActionCompleted:
		return valid(current == CommandStatusCompleted || isValidProgressTransition(cmd, CommandStatusCompleted), env.MessageName)
	case EventPlaybackActionInterrupted:
		return valid(
			current == CommandStatusRendererAccepted || current == CommandStatusPlaybackStarted || current == CommandStatusCompleted || current == CommandStatusCancelled,
			env.MessageName,
		)
	case EventPlaybackActionFailed:
		return valid(
			current == CommandStatusRuntimeAccepted || current == CommandStatusRendererAccepted || current == CommandStatusPlaybackStarted || current == CommandStatusCompleted || current == CommandStatusFailedTerminal,
			env.MessageName,
		)
	case EventStateDesiredApplied:
		return valid(current == CommandStatusRuntimeAccepted || current == CommandStatusCompleted, env.MessageName)
	case EventStateDesiredRejected:
		return valid(current == CommandStatusRuntimeAccepted || current == CommandStatusFailedTerminal, env.MessageName)
	default:
		return nil
	}
}

func (h *Handler) applyRuntimeEventCommandProgress(conn *Connection, sessionID string, env *Envelope) error {
	if env == nil {
		return nil
	}
	meta := decodeRuntimeEventMetadata(env.Payload)
	if meta.CommandID == "" {
		return nil
	}
	// Ownership is validated before EventRecord persistence in HandleEvent. Keep
	// the parameters explicit so this function cannot accidentally be detached
	// from the established session context during future refactors.
	_ = conn
	_ = sessionID
	now := runtimeEventOccurredAt(meta)
	switch env.MessageName {
	case EventPlaybackCommandAccepted:
		return h.commands.MarkRendererAccepted(meta.CommandID, string(conn.RuntimeID), sessionID, meta.PlaybackInstanceID, now)
	case EventStateDesiredApplied:
		return h.commands.MarkCompleted(meta.CommandID, "", now)
	case EventStateDesiredRejected:
		code := strings.TrimSpace(meta.ErrorCode)
		if code == "" {
			code = "DESIRED_STATE_REJECTED"
		}
		message := strings.TrimSpace(meta.ErrorMessage)
		if message == "" {
			message = "runtime rejected desired state"
		}
		return h.commands.MarkFailed(meta.CommandID, code, message, now)
	case EventPlaybackActionStarted:
		if err := h.commands.MarkPlaybackStarted(meta.CommandID, meta.PlaybackInstanceID, now); err != nil {
			return err
		}
		if h.playActionCompletionPolicy(meta.CommandID) == PlayActionCompletionOnStarted {
			return h.commands.MarkCompleted(meta.CommandID, meta.PlaybackInstanceID, now)
		}
		return nil
	case EventPlaybackActionFirstCycle:
		if h.playActionCompletionPolicy(meta.CommandID) == PlayActionCompletionOnFirstCycle {
			return h.commands.MarkCompleted(meta.CommandID, meta.PlaybackInstanceID, now)
		}
		return nil
	case EventPlaybackActionCompleted:
		// Natural physical completion is a safe terminal fallback for every policy.
		// Policies such as on_started/on_first_cycle may already have completed the
		// command; commandService treats this late terminal event idempotently.
		return h.commands.MarkCompleted(meta.CommandID, meta.PlaybackInstanceID, now)
	case EventPlaybackActionInterrupted:
		policy := h.playActionCompletionPolicy(meta.CommandID)
		if policy == PlayActionCompletionOnInterrupted ||
			(policy == PlayActionCompletionManualStop && strings.TrimSpace(meta.Reason) == "runtime_stop") {
			return h.commands.MarkCompleted(meta.CommandID, meta.PlaybackInstanceID, now)
		}
		// Interruption is cancellation unless the contract explicitly declares the
		// interruption itself (or an explicit manual stop) to be successful completion.
		return h.commands.MarkCancelled(meta.CommandID, now)
	case EventPlaybackActionFailed:
		if strings.EqualFold(strings.TrimSpace(meta.ErrorCode), "PLAYBACK_COMMAND_EXPIRED") {
			return h.commands.MarkExpired(meta.CommandID, now)
		}
		// Completion policies such as on_started/on_first_cycle can terminalize
		// the logical command while the physical playback continues. A later
		// renderer failure is still valid telemetry but must not rewrite a
		// successfully completed command.
		if cmd, getErr := h.commands.GetCommand(meta.CommandID); getErr == nil && cmd != nil && CommandStatus(cmd.Status) == CommandStatusCompleted {
			return nil
		}
		code := strings.TrimSpace(meta.ErrorCode)
		if code == "" {
			code = "PLAYBACK_FAILED"
		}
		message := strings.TrimSpace(meta.ErrorMessage)
		if message == "" {
			message = "desktop pet playback failed"
		}
		return h.commands.MarkFailed(meta.CommandID, code, message, now)
	default:
		return nil
	}
}

func (h *Handler) playActionCompletionPolicy(commandID string) string {
	if h == nil || h.commands == nil || strings.TrimSpace(commandID) == "" {
		return ""
	}
	cmd, err := h.commands.GetCommand(commandID)
	if err != nil || cmd == nil || CommandType(cmd.CommandType) != CommandTypePlayAction {
		return ""
	}
	var payload PlayActionPayload
	if err := json.Unmarshal([]byte(cmd.PayloadJSON), &payload); err != nil {
		return ""
	}
	switch strings.TrimSpace(payload.CompletionPolicy) {
	case PlayActionCompletionOnStarted, PlayActionCompletionOnFirstCycle, PlayActionCompletionOnInterrupted, PlayActionCompletionManualStop:
		return strings.TrimSpace(payload.CompletionPolicy)
	default:
		return ""
	}
}

func (h *Handler) appendRuntimeDomainEvent(conn *Connection, env *Envelope) error {
	if h.states == nil || conn == nil || env == nil {
		return nil
	}
	sessionID := conn.SessionIDValue()
	if sessionID == "" {
		return ErrConnectionClosed
	}
	meta := decodeRuntimeEventMetadata(env.Payload)
	if meta.InstallationID == "" && meta.CommandID != "" {
		if cmd, err := h.commands.GetCommand(meta.CommandID); err == nil && cmd != nil {
			meta.InstallationID = strings.TrimSpace(cmd.InstallationID)
		}
	}
	occurredAt := runtimeEventOccurredAt(meta)
	payload := struct {
		EventType      string          `json:"EventType"`
		RuntimeID      string          `json:"RuntimeID"`
		SessionID      string          `json:"SessionID"`
		DeviceID       string          `json:"DeviceID"`
		InstallationID string          `json:"InstallationID"`
		UserID         string          `json:"UserID"`
		CharacterID    string          `json:"CharacterID"`
		Sequence       int64           `json:"Sequence"`
		Timestamp      time.Time       `json:"Timestamp"`
		Payload        json.RawMessage `json:"Payload"`
	}{
		EventType:      env.MessageName,
		RuntimeID:      string(conn.RuntimeID),
		SessionID:      sessionID,
		DeviceID:       string(conn.DeviceID),
		InstallationID: meta.InstallationID,
		UserID:         string(conn.UserID),
		CharacterID:    meta.CharacterID,
		Sequence:       env.Sequence,
		Timestamp:      occurredAt,
		Payload:        json.RawMessage(env.Payload),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	idempotencyKey := fmt.Sprintf("runtime_event:%s:%d", sessionID, env.Sequence)
	_, err = h.states.AppendDomainEvent(
		env.MessageName,
		string(conn.RuntimeID),
		body,
		occurredAt,
		&idempotencyKey,
	)
	return err
}

func (h *Handler) persistActualStateSnapshot(conn *Connection, env *Envelope) error {
	if conn == nil || env == nil || env.MessageName != EventStateSnapshot {
		return nil
	}
	state, err := h.runtimeActualStateFromSnapshot(conn, env)
	if err != nil {
		return err
	}
	if h.states == nil {
		return fmt.Errorf("runtime actual state service is unavailable")
	}
	if err := h.states.Upsert(state); err != nil {
		return fmt.Errorf("persist runtime actual state snapshot: %w", err)
	}
	return nil
}

func (h *Handler) runtimeActualStateFromSnapshot(conn *Connection, env *Envelope) (*RuntimeActualState, error) {
	if conn == nil {
		return nil, ErrConnectionClosed
	}
	if env == nil || env.MessageName != EventStateSnapshot {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "runtime.state.snapshot envelope is required")
	}
	var snapshot StateSnapshotPayload
	if err := json.Unmarshal(env.Payload, &snapshot); err != nil {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "invalid runtime.state.snapshot payload")
	}
	sessionID, generation := conn.SessionSnapshot()
	if sessionID == "" || generation <= 0 {
		return nil, ErrConnectionClosed
	}
	if snapshot.ConnectionGeneration != generation {
		return nil, NewProtocolError(
			ErrCodeEnvelopeInvalid,
			fmt.Sprintf("state snapshot generation mismatch: payload=%d envelope=%d", snapshot.ConnectionGeneration, generation),
		)
	}
	if snapshot.EventSequence != env.Sequence {
		return nil, NewProtocolError(
			ErrCodeEnvelopeInvalid,
			fmt.Sprintf("state snapshot event sequence mismatch: payload=%d envelope=%d", snapshot.EventSequence, env.Sequence),
		)
	}
	if strings.TrimSpace(snapshot.ActualStateHash) == "" {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "state snapshot actualStateHash is required")
	}
	if snapshot.AppliedDesiredRevision < 0 || snapshot.AppliedSettingsRevision < 0 || snapshot.LastProcessedCommandSequence < 0 {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "state snapshot revisions and command cursor must be non-negative")
	}
	if snapshot.WindowWidth < 0 || snapshot.WindowHeight < 0 || math.IsNaN(snapshot.Scale) || math.IsInf(snapshot.Scale, 0) || snapshot.Scale < 0 {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "state snapshot contains invalid window geometry")
	}
	if snapshot.AppliedDesiredRevision > 0 && strings.TrimSpace(snapshot.AppliedDesiredHash) == "" {
		return nil, NewProtocolError(ErrCodeDesiredHashMismatch, "state snapshot appliedDesiredHash is required when appliedDesiredRevision > 0")
	}
	if !validInstanceStatus(snapshot.InstanceStatus) || !validWindowStatus(snapshot.WindowStatus) ||
		!validRendererStatus(snapshot.RendererStatus) || !validPlaybackStatus(snapshot.PlaybackStatus) {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "state snapshot contains unsupported runtime status")
	}

	instanceStatus := strings.TrimSpace(snapshot.InstanceStatus)
	windowStatus := strings.TrimSpace(snapshot.WindowStatus)
	rendererStatus := strings.TrimSpace(snapshot.RendererStatus)
	playbackStatus := strings.TrimSpace(snapshot.PlaybackStatus)
	if snapshot.Visible != (windowStatus == WindowStatusVisible) {
		return nil, NewProtocolError(ErrCodeEnvelopeInvalid, "state snapshot visible flag does not match windowStatus")
	}
	state := &RuntimeActualState{
		UserID:                  string(conn.UserID),
		DeviceID:                string(conn.DeviceID),
		RuntimeID:               string(conn.RuntimeID),
		RuntimeSessionID:        sessionID,
		ConnectionGeneration:    generation,
		LastEventSequence:       env.Sequence,
		AppliedDesiredRevision:  snapshot.AppliedDesiredRevision,
		AppliedDesiredHash:      strings.TrimSpace(snapshot.AppliedDesiredHash),
		AppliedSettingsRevision: snapshot.AppliedSettingsRevision,
		InstallationID:          strings.TrimSpace(snapshot.InstallationID),
		PetID:                   strings.TrimSpace(snapshot.PetID),
		ReleaseID:               strings.TrimSpace(snapshot.ReleaseID),
		InstanceStatus:          instanceStatus,
		WindowStatus:            windowStatus,
		RendererStatus:          rendererStatus,
		PlaybackStatus:          playbackStatus,
		Visible:                 snapshot.Visible,
		PositionX:               snapshot.PositionX,
		PositionY:               snapshot.PositionY,
		ScreenID:                strings.TrimSpace(snapshot.ScreenID),
		WindowWidth:             snapshot.WindowWidth,
		WindowHeight:            snapshot.WindowHeight,
		Scale:                   snapshot.Scale,
		StableActionKey:         strings.TrimSpace(snapshot.StableActionKey),
		CurrentActionKey:        strings.TrimSpace(snapshot.CurrentActionKey),
		PlaybackInstanceID:      strings.TrimSpace(snapshot.PlaybackInstanceID),
		CurrentCommandID:        strings.TrimSpace(snapshot.CurrentCommandID),
		ActualStateHash:         strings.TrimSpace(snapshot.ActualStateHash),
	}
	state.HealthStatus = deriveActualStateHealth(state)
	return state, nil
}

func validInstanceStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case InstanceStatusAbsent, InstanceStatusStarting, InstanceStatusLoadingRelease,
		InstanceStatusWindowCreated, InstanceStatusRendererInitializing, InstanceStatusReady,
		InstanceStatusStopping, InstanceStatusStopped, InstanceStatusFailed:
		return true
	default:
		return false
	}
}

func validWindowStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case WindowStatusAbsent, WindowStatusHidden, WindowStatusVisible, WindowStatusDestroyed, WindowStatusFailed:
		return true
	default:
		return false
	}
}

func validRendererStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case RendererStatusAbsent, RendererStatusBootstrapped, RendererStatusRuntimeReady,
		RendererStatusUnresponsive, RendererStatusCrashed, RendererStatusFailed:
		return true
	default:
		return false
	}
}

func validPlaybackStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case PlaybackStatusIdle, PlaybackStatusLoading, PlaybackStatusPlaying, PlaybackStatusHolding,
		PlaybackStatusPaused, PlaybackStatusStopped, PlaybackStatusFailed:
		return true
	default:
		return false
	}
}

func deriveActualStateHealth(state *RuntimeActualState) string {
	if state == nil {
		return HealthStatusFailed
	}
	if state.InstanceStatus == InstanceStatusAbsent || state.InstanceStatus == InstanceStatusStopped {
		return HealthStatusOnlineNoPet
	}
	if state.InstanceStatus == InstanceStatusFailed || state.WindowStatus == WindowStatusFailed ||
		state.RendererStatus == RendererStatusCrashed || state.RendererStatus == RendererStatusFailed ||
		state.PlaybackStatus == PlaybackStatusFailed {
		return HealthStatusFailed
	}
	if state.RendererStatus == RendererStatusUnresponsive {
		return HealthStatusDegraded
	}
	if state.InstanceStatus == InstanceStatusReady && state.RendererStatus == RendererStatusRuntimeReady &&
		(state.WindowStatus == WindowStatusVisible || state.WindowStatus == WindowStatusHidden) {
		return HealthStatusHealthy
	}
	return HealthStatusSyncing
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
	session, err := h.deviceRuntimeSessions.GetSession(context.Background(), id)
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
	if err := h.deviceRuntimeSessions.UpdateCursor(context.Background(), id, generation, cursor, time.Now().UTC()); err != nil {
		return fmt.Errorf("persist authoritative runtime cursor: %w", err)
	}
	return nil
}

func (h *Handler) HandleDisconnect(conn *Connection) error {
	if conn == nil {
		return nil
	}

	conn.fenceMu.Lock()
	defer conn.fenceMu.Unlock()
	previousState := conn.GetState()
	conn.SetState(ConnStateClosed)
	// A connection explicitly fenced by HandleConnect is already superseded.
	// Its deferred websocket cleanup must not close the authoritative session
	// that the replacement connection is about to resume.
	if previousState == ConnStateClosing {
		return nil
	}
	sessionID, generation := conn.SessionSnapshot()
	if sessionID != "" && h.commands != nil {
		if err := h.commands.SupersedeEphemeralCommands(
			string(conn.UserID), string(conn.DeviceID), string(conn.RuntimeID), sessionID,
			"runtime connection disconnected", time.Now().UTC(),
		); err != nil {
			log.Warn("[v2] supersede disconnected ephemeral commands failed: ", err)
		}
	}

	if h.deviceRuntimeSessions != nil {
		if sessionID != "" && generation > 0 {
			err := h.deviceRuntimeSessions.Close(context.Background(), runtimeidentity.ParseRuntimeSessionID(sessionID), generation, "client_disconnect", time.Now().UTC())
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

	conn.fenceMu.RLock()
	defer conn.fenceMu.RUnlock()
	if conn.GetState() != ConnStateConnected {
		return deviceruntime.ErrConnectionSuperseded
	}

	now := time.Now().UTC()
	conn.TouchHeartbeat(now)
	sessionID, generation := conn.SessionSnapshot()

	if h.deviceRuntimeSessions != nil && sessionID != "" && generation > 0 {
		_, err := h.deviceRuntimeSessions.Heartbeat(context.Background(), runtimeidentity.ParseRuntimeSessionID(sessionID), generation, now)
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

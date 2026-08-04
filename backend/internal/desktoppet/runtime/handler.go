// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/log"
)

type RuntimeDomainEvent struct {
	EventType      string
	RuntimeID      string
	SessionID      string
	DeviceID       string
	InstallationID string
	UserID         string
	CharacterID    string
	Timestamp      time.Time
	Payload        json.RawMessage
}

type RuntimeEventSink interface {
	OnRuntimeEvent(ctx context.Context, event RuntimeDomainEvent) error
}

type NoopEventSink struct{}

func (NoopEventSink) OnRuntimeEvent(_ context.Context, _ RuntimeDomainEvent) error { return nil }

type Handler struct {
	config        *DesktopPetRuntimeConfig
	auth          *Auth
	bootstrapRepo *BootstrapTicketRepository
	registry      *RuntimeRegistry
	pending       *PendingTracker
	state         StateStore
	eventSink     RuntimeEventSink
	deduper       *eventDeduplicator
	upgrader      websocket.Upgrader
	onRegister    func(conn *Connection, payload *contracts.RegisterPayload) (*contracts.WelcomePayload, error)
	onResult      func(msg *contracts.RuntimeMessage, payload *contracts.ResultPayload)
	onEvent       func(conn *Connection, msg *contracts.RuntimeMessage, payload *contracts.EventPayload)
	onHeartbeat   func(runtimeID, sessionID string, payload *contracts.HeartbeatPayload)
}

func NewHandler(
	config *DesktopPetRuntimeConfig,
	auth *Auth,
	registry *RuntimeRegistry,
	pending *PendingTracker,
	state StateStore,
	eventSink RuntimeEventSink,
) *Handler {
	return &Handler{
		config:        config,
		auth:          auth,
		registry:      registry,
		pending:       pending,
		state:         state,
		eventSink:     eventSink,
		bootstrapRepo: nil,
		deduper:       newEventDeduplicator(defaultEventDedupCapacity),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     auth.CheckOrigin,
		},
	}
}

func (h *Handler) SetBootstrapRepo(repo *BootstrapTicketRepository) {
	h.bootstrapRepo = repo
}

func (h *Handler) SetCallbacks(
	onRegister func(conn *Connection, payload *contracts.RegisterPayload) (*contracts.WelcomePayload, error),
	onResult func(msg *contracts.RuntimeMessage, payload *contracts.ResultPayload),
	onEvent func(conn *Connection, msg *contracts.RuntimeMessage, payload *contracts.EventPayload),
	onHeartbeat func(runtimeID, sessionID string, payload *contracts.HeartbeatPayload),
) {
	h.onRegister = onRegister
	h.onResult = onResult
	h.onEvent = onEvent
	h.onHeartbeat = onHeartbeat
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.config.Enabled {
		writeHTTPError(w, http.StatusServiceUnavailable, ErrCodeRuntimeDisabled, "runtime bridge is disabled")
		return
	}

	if err := h.auth.ValidateRequest(r); err != nil {
		writeRuntimeHTTPError(w, err)
		return
	}

	conn, err := h.upgrade(w, r)
	if err != nil {
		return
	}

	h.serveConnection(conn)
}

func (h *Handler) upgrade(w http.ResponseWriter, r *http.Request) (*Connection, error) {
	wsConn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Logger.Errorf("runtime handler: websocket upgrade failed: %v", err)
		return nil, err
	}

	wsConn.SetReadLimit(int64(h.config.MaxMessageBytes))

	conn := NewConnection(wsConn, h.config)
	conn.onRegister = h.onRegister
	conn.onResult = h.onResult
	conn.onEvent = h.onEvent
	conn.onHeartbeat = h.onHeartbeat
	conn.onClose = func(sessionID, runtimeID string, code int, reason string) {
		h.registry.Unregister(sessionID)
		h.pending.FailForSession(sessionID, contracts.ResultCancelled, ErrCodeRuntimeDisconnected, "connection closed: "+reason, ErrRuntimeDisconnected)
		if runtimeID != "" && h.state != nil {
			if err := h.state.MarkClientDisconnected(runtimeID); err != nil {
				log.Logger.Errorf("runtime handler: mark client disconnected failed runtimeID=%s err=%v", runtimeID, err)
			}
			if err := h.state.UpdateActualStateHealth(runtimeID, "offline"); err != nil {
				log.Logger.Errorf("runtime handler: mark actual state offline failed runtimeID=%s err=%v", runtimeID, err)
			}
		}
		log.Logger.Infof("runtime handler: session closed sessionID=%s runtimeID=%s code=%d reason=%s", sessionID, runtimeID, code, reason)
	}

	return conn, nil
}

func (h *Handler) serveConnection(conn *Connection) {
	go conn.Start(context.Background())
	log.Logger.Infof("runtime handler: new connection established")
}

func (h *Handler) HandleRegister(conn *Connection, payload *contracts.RegisterPayload) (*contracts.WelcomePayload, error) {
	if payload.RuntimeID == "" {
		return nil, NewRuntimeError(ErrCodeRuntimeProtocolError, "runtimeId is required", ErrRuntimeProtocolError)
	}
	if payload.DeviceID == "" {
		return nil, NewRuntimeError(ErrCodeRuntimeProtocolError, "deviceId is required", ErrRuntimeProtocolError)
	}

	if h.bootstrapRepo == nil {
		return nil, NewRuntimeError(ErrCodeRuntimeUnauthorized, "runtime bootstrap not configured", ErrRuntimeUnauthorized)
	}
	if payload.BootstrapTicket == "" {
		return nil, NewRuntimeError(ErrCodeRuntimeUnauthorized, "bootstrap ticket is required", ErrRuntimeUnauthorized)
	}

	selectedProtocol, err := negotiateProtocol(payload.ProtocolMin, payload.ProtocolMax)
	if err != nil {
		return nil, NewRuntimeError(ErrCodeRuntimeProtocolIncompatible, "protocol negotiation failed", err)
	}

	sessionID := "rtsess_" + uuid.New().String()
	conn.sessionID = sessionID
	conn.runtimeID = payload.RuntimeID
	conn.deviceID = payload.DeviceID

	ticket, ticketErr := h.bootstrapRepo.ConsumeWithValidation(context.Background(), payload.BootstrapTicket, payload.RuntimeID, payload.DeviceID)
	if ticketErr != nil {
		switch ticketErr {
		case ErrTicketExpired:
			return nil, NewRuntimeError(ErrCodeRuntimeUnauthorized, "bootstrap ticket expired", ErrRuntimeUnauthorized)
		case ErrTicketConsumed:
			return nil, NewRuntimeError(ErrCodeRuntimeUnauthorized, "bootstrap ticket already used", ErrRuntimeUnauthorized)
		case ErrTicketRevoked:
			return nil, NewRuntimeError(ErrCodeRuntimeUnauthorized, "bootstrap ticket revoked", ErrRuntimeUnauthorized)
		case ErrTicketNotFound:
			return nil, NewRuntimeError(ErrCodeRuntimeUnauthorized, "invalid bootstrap ticket", ErrRuntimeUnauthorized)
		default:
			log.Logger.Warnf("runtime handler: bootstrap ticket validation failed err=%v", ticketErr)
			return nil, NewRuntimeError(ErrCodeRuntimeUnauthorized, "bootstrap ticket validation failed", ErrRuntimeUnauthorized)
		}
	}
	conn.userID = ticket.UserID
	conn.ticketDeviceID = ticket.DeviceID
	conn.ticketRuntimeID = ticket.RuntimeID
	log.Logger.Infof("runtime handler: bootstrap ticket consumed ticketId=%s userID=%s deviceID=%s", ticket.ID, ticket.UserID, ticket.DeviceID)

	conn.SetCapabilities(payload.Capabilities)

	fullSyncRequired := true
	if payload.LastAppliedDesiredRevision > 0 {
		fullSyncRequired = false
	}

	superseded := h.registry.Register(conn)
	if superseded != nil {
		oldSessionID := superseded.SessionID()
		supersededMsg, _ := buildMessage(
			contracts.KindControl,
			contracts.MsgControlSuperseded,
			payload.RuntimeID,
			oldSessionID,
			contracts.SupersededPayload{
				NewSessionID: sessionID,
				Reason:       "replaced by new process",
			},
		)
		_ = superseded.Send(supersededMsg)
		superseded.SetState(SessionStateClosing)
		go func() {
			time.Sleep(2 * time.Second)
			superseded.Close(websocket.ClosePolicyViolation, "session superseded")
		}()
		h.pending.FailForSession(oldSessionID, contracts.ResultCancelled, ErrCodeRuntimeSessionReplaced, "session replaced by new connection", ErrRuntimeSessionReplaced)
		log.Logger.Infof("runtime handler: session superseded oldSession=%s newSession=%s runtimeID=%s", oldSessionID, sessionID, payload.RuntimeID)
	}

	conn.SetState(SessionStateSyncing)

	h.persistClientRegistration(conn, payload, selectedProtocol, sessionID)

	welcome := &contracts.WelcomePayload{
		SessionID:           sessionID,
		SelectedProtocol:    selectedProtocol,
		BackendInstanceID:   h.config.BackendInstanceID,
		HeartbeatIntervalMs: h.config.HeartbeatIntervalMs,
		HeartbeatTimeoutMs:  h.config.HeartbeatTimeoutMs,
		MaxMessageBytes:     h.config.MaxMessageBytes,
		FullSyncRequired:    fullSyncRequired,
		ServerTime:          time.Now().UTC(),
	}

	log.Logger.Infof("runtime handler: registered runtimeID=%s sessionID=%s protocol=%s fullSync=%v", payload.RuntimeID, sessionID, selectedProtocol, fullSyncRequired)

	return welcome, nil
}

func (h *Handler) HandleResult(msg *contracts.RuntimeMessage, payload *contracts.ResultPayload) {
	if payload.Status == contracts.ResultAccepted {
		log.Logger.Infof("runtime handler: command accepted commandId=%s runtimeID=%s", payload.CommandID, msg.RuntimeID)
		return
	}

	result := &PendingResult{
		CommandID:      payload.CommandID,
		Status:         payload.Status,
		ErrorCode:      payload.ErrorCode,
		ErrorMsg:       payload.ErrorMessage,
		AppliedRev:     payload.AppliedRevision,
		ActualState:    payload.ActualState,
		AcceptedAction: payload.AcceptedAction,
		PlaybackReqID:  payload.PlaybackRequestID,
	}
	if payload.Status == contracts.ResultFailed || payload.Status == contracts.ResultRejected {
		result.Err = NewRuntimeError(payload.ErrorCode, payload.ErrorMessage, nil)
		if payload.ErrorCode == "" {
			result.Err = ErrRuntimeCommandFailed
		}
	}
	h.pending.Complete(result)

	if payload.Status != contracts.ResultApplied {
		return
	}

	if payload.ActualState != nil && msg.RuntimeID != "" && h.state != nil {
		actual := payload.ActualState
		state := &RuntimeActualState{
			RuntimeID:               msg.RuntimeID,
			InstallationID:          actual.InstallationID,
			PetInstanceID:           actual.PetInstanceID,
			SessionID:               msg.SessionID,
			AppliedSettingsRevision: payload.AppliedRevision,
			Visible:                 boolToInt(actual.Visible),
			CurrentActionKey:        actual.CurrentActionKey,
			PositionX:               actual.PositionX,
			PositionY:               actual.PositionY,
			ScreenID:                actual.ScreenID,
			Scale:                   actual.Scale,
			Health:                  "healthy",
			ObservedAt:              time.Now().Format(runtimeTimeFormat),
		}
		if payload.Status == contracts.ResultFailed || payload.Status == contracts.ResultRejected {
			state.Health = "degraded"
		}
		if err := h.state.UpsertActualState(state); err != nil {
			log.Logger.Errorf("runtime handler: upsert actual state from result failed runtimeID=%s err=%v", msg.RuntimeID, err)
		}
	}
}

func validateConnectionEnvelope(
	conn *Connection,
	msg *contracts.RuntimeMessage,
) error {
	if msg.RuntimeID != "" &&
		msg.RuntimeID != conn.RuntimeID() {
		return ErrRuntimeProtocolError
	}
	if msg.SessionID != "" &&
		msg.SessionID != conn.SessionID() {
		return ErrRuntimeProtocolError
	}
	if msg.UserID != "" &&
		msg.UserID != conn.UserID() {
		return ErrRuntimeProtocolError
	}
	if msg.DeviceID != "" &&
		msg.DeviceID != conn.DeviceID() {
		return ErrRuntimeProtocolError
	}
	return nil
}

func (h *Handler) HandleEvent(
	conn *Connection,
	msg *contracts.RuntimeMessage,
	payload *contracts.EventPayload,
) {
	if conn != nil && validateConnectionEnvelope(conn, msg) != nil {
		conn.Close(
			websocket.ClosePolicyViolation,
			"runtime identity mismatch",
		)
		return
	}
	log.Logger.Debugf("runtime event: type=%s petInstance=%s", payload.EventType, payload.PetInstanceID)

	var summary contracts.PetInstanceSummary
	if len(payload.Data) > 0 {
		if err := json.Unmarshal(payload.Data, &summary); err != nil {
			return
		}
	} else {
		summary.PetInstanceID = payload.PetInstanceID
		summary.InstallationID = msg.InstallationID
	}

	if summary.InstallationID == "" {
		summary.InstallationID = msg.InstallationID
	}
	if summary.PetInstanceID == "" {
		summary.PetInstanceID = payload.PetInstanceID
	}

	if h.state != nil && conn != nil && payload.PetInstanceID != "" {
		state := &RuntimeActualState{
			RuntimeID:        conn.RuntimeID(),
			InstallationID:   summary.InstallationID,
			PetInstanceID:    summary.PetInstanceID,
			SessionID:        conn.SessionID(),
			Visible:          boolToInt(summary.Visible),
			CurrentActionKey: summary.CurrentActionKey,
			PositionX:        summary.PositionX,
			PositionY:        summary.PositionY,
			ScreenID:         summary.ScreenID,
			Scale:            summary.Scale,
			Health:           "healthy",
			ObservedAt:       time.Now().Format(runtimeTimeFormat),
		}
		if err := h.state.UpsertActualState(state); err != nil {
			log.Logger.Errorf("runtime handler: upsert actual state from event failed runtimeID=%s err=%v", conn.RuntimeID(), err)
		}
	}

	h.dispatchEvent(conn, msg, payload, summary)
}

func (h *Handler) dispatchEvent(conn *Connection, msg *contracts.RuntimeMessage, payload *contracts.EventPayload, summary contracts.PetInstanceSummary) {
	if h.eventSink == nil {
		return
	}
	standardType, ok := normalizeRuntimeEventType(payload.EventType)
	if !ok {
		return
	}
	if h.deduper != nil && h.deduper.isDuplicate(msg.MessageID) {
		log.Logger.Debugf("runtime handler: dropping duplicate event type=%s messageId=%s", standardType, msg.MessageID)
		return
	}
	event := RuntimeDomainEvent{
		EventType:      standardType,
		InstallationID: summary.InstallationID,
		UserID:         conn.UserID(),
		RuntimeID:      conn.RuntimeID(),
		SessionID:      conn.SessionID(),
		DeviceID:       conn.DeviceID(),
		Timestamp:      time.Now(),
		Payload:        payload.Data,
	}
	if err := h.eventSink.OnRuntimeEvent(context.Background(), event); err != nil {
		log.Logger.Errorf("runtime handler: event sink delivery failed type=%s err=%v", standardType, err)
	}
}

var runtimeEventAliases = map[string]string{
	"clicked":              "desktop.pet.clicked",
	"double_clicked":       "desktop.pet.double_clicked",
	"hovered":              "desktop.pet.hovered",
	"dragged":              "desktop.pet.drag.ended",
	"drag_started":         "desktop.pet.drag.started",
	"drag_moved":           "desktop.pet.drag.moved",
	"playback_completed":   "playback.action.completed",
	"playback_interrupted": "playback.action.interrupted",
	"action_switch":        "playback.action.started",
}

var runtimeStandardEventTypes = map[string]struct{}{
	"desktop.pet.clicked":         {},
	"desktop.pet.double_clicked":  {},
	"desktop.pet.hovered":         {},
	"desktop.pet.drag.started":    {},
	"desktop.pet.drag.moved":      {},
	"desktop.pet.drag.ended":      {},
	"desktop.pet.fall.started":    {},
	"desktop.pet.edge.reached":    {},
	"desktop.pet.interacted":      {},
	"playback.action.started":     {},
	"playback.action.completed":   {},
	"playback.action.interrupted": {},
	"playback.action.failed":      {},
}

func normalizeRuntimeEventType(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	if mapped, ok := runtimeEventAliases[raw]; ok {
		return mapped, true
	}
	if _, ok := runtimeStandardEventTypes[raw]; ok {
		return raw, true
	}
	return "", false
}

func (h *Handler) HandleHeartbeat(runtimeID, sessionID string, payload *contracts.HeartbeatPayload) {
	if payload == nil {
		return
	}

	if h.state != nil && runtimeID != "" {
		if err := h.state.UpdateClientSession(runtimeID, sessionID, ""); err != nil {
			log.Logger.Errorf("runtime handler: update client session on heartbeat failed runtimeID=%s err=%v", runtimeID, err)
		}
	}

	health := "healthy"
	if !payload.RendererHealthy {
		health = "degraded"
	}

	for _, inst := range payload.PetInstances {
		state := &RuntimeActualState{
			RuntimeID:               runtimeID,
			InstallationID:          inst.InstallationID,
			PetInstanceID:           inst.PetInstanceID,
			SessionID:               sessionID,
			DesiredRevision:         payload.LastAppliedDesiredRevision,
			AppliedSettingsRevision: payload.LastAppliedDesiredRevision,
			Visible:                 boolToInt(inst.Visible),
			CurrentActionKey:        inst.CurrentActionKey,
			PositionX:               inst.PositionX,
			PositionY:               inst.PositionY,
			ScreenID:                inst.ScreenID,
			Scale:                   inst.Scale,
			Health:                  health,
			ObservedAt:              time.Now().Format(runtimeTimeFormat),
		}
		if err := h.state.UpsertActualState(state); err != nil {
			log.Logger.Errorf("runtime handler: upsert actual state from heartbeat failed runtimeID=%s err=%v", runtimeID, err)
		}
	}

	if payload.ErrorSummary != "" {
		log.Logger.Warnf("runtime handler: heartbeat error summary runtimeID=%s: %s", runtimeID, payload.ErrorSummary)
	}
}

func (h *Handler) persistClientRegistration(conn *Connection, payload *contracts.RegisterPayload, protocol, sessionID string) {
	if h.state == nil {
		return
	}
	capsJSON, _ := json.Marshal(payload.Capabilities)
	nowStr := time.Now().Format(runtimeTimeFormat)
	client := &RuntimeClient{
		RuntimeID:             payload.RuntimeID,
		DeviceID:              payload.DeviceID,
		UserID:                conn.UserID(),
		Platform:              payload.Platform,
		Arch:                  payload.Arch,
		AppVersion:            payload.AppVersion,
		ProtocolVersion:       protocol,
		CapabilitiesJSON:      string(capsJSON),
		LastProcessInstanceID: payload.ProcessInstanceID,
		LastSessionID:         sessionID,
		LastSeenAt:            nowStr,
		LastConnectedAt:       nowStr,
	}
	if err := h.state.UpsertClient(client); err != nil {
		log.Logger.Errorf("runtime handler: upsert client failed runtimeID=%s err=%v", payload.RuntimeID, err)
	}
}

func writeHTTPError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

func writeRuntimeHTTPError(w http.ResponseWriter, err error) {
	if re, ok := err.(*RuntimeError); ok {
		writeHTTPError(w, MapRuntimeErrorCodeToHTTP(re.Code), re.Code, re.Message)
		return
	}
	writeHTTPError(w, http.StatusUnauthorized, ErrCodeRuntimeUnauthorized, err.Error())
}

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/credential"
	meshprotocol "github.com/u-ai/backend/internal/devicemesh/protocol"
	"github.com/u-ai/backend/internal/deviceruntime"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type CommandAckHandler func(invocationID string, generation int64, status string, result json.RawMessage) bool
type DisconnectHandler func(sessionID string, generation int64)
type ReadyHandler func(spaceID runtimeidentity.SpaceID, deviceID runtimeidentity.DeviceID)
type InvocationResultHandler func(result protocol.RuntimeResultPayload)
type InvocationErrorHandler func(errResult protocol.RuntimeErrorPayload)
type InvocationCancelAckHandler func(invocationID string, reason string)
type TaskClaimHandler func(taskRunID string, attemptID string, workerID string, leaseDuration time.Duration) bool
type TaskCompleteHandler func(taskRunID string, attemptID string, success bool, result json.RawMessage, errMsg string)
type TaskProgressHandler func(taskRunID string, attemptID string, progress json.RawMessage)
type TaskCheckpointHandler func(taskRunID string, attemptID string, checkpoint json.RawMessage)
type TaskHeartbeatHandler func(taskRunID string, attemptID string, reportedAt time.Time)
type TaskClaimPayloadHandler func(claim protocol.TaskClaimPayload) bool
type TaskCompletePayloadHandler func(complete protocol.TaskCompletePayload)
type TaskProgressPayloadHandler func(progress protocol.TaskProgressPayload)
type TaskCheckpointPayloadHandler func(checkpoint protocol.TaskCheckpointPayload)
type TaskHeartbeatPayloadHandler func(heartbeat protocol.TaskHeartbeatPayload) bool

type Handler struct {
	sessions                *deviceruntime.Service
	hub                     *ConnectionHub
	dispatcher              agent.RuntimeDispatcher
	upgrader                websocket.Upgrader
	onCommandAck            CommandAckHandler
	onDisconnect            DisconnectHandler
	readyMu                 sync.RWMutex
	onReady                 ReadyHandler
	onReadyObservers        []ReadyHandler
	onInvocationResult      InvocationResultHandler
	onInvocationError       InvocationErrorHandler
	onInvocationCancelAck   InvocationCancelAckHandler
	onTaskClaim             TaskClaimHandler
	onTaskComplete          TaskCompleteHandler
	onTaskProgress          TaskProgressHandler
	onTaskCheckpoint        TaskCheckpointHandler
	onTaskHeartbeat         TaskHeartbeatHandler
	onTaskClaimPayload      TaskClaimPayloadHandler
	onTaskCompletePayload   TaskCompletePayloadHandler
	onTaskProgressPayload   TaskProgressPayloadHandler
	onTaskCheckpointPayload TaskCheckpointPayloadHandler
	onTaskHeartbeatPayload  TaskHeartbeatPayloadHandler
}

func NewHandler(sessions *deviceruntime.Service, hub *ConnectionHub) *Handler {
	return &Handler{
		sessions: sessions,
		hub:      hub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  64 * 1024,
			WriteBufferSize: 64 * 1024,
			CheckOrigin:     nil,
		},
	}
}

func (h *Handler) SetSessions(sessions *deviceruntime.Service) {
	h.sessions = sessions
}

func (h *Handler) SetDispatcher(dispatcher agent.RuntimeDispatcher) {
	h.dispatcher = dispatcher
}

func (h *Handler) SetOnCommandAck(handler CommandAckHandler) {
	h.onCommandAck = handler
}

func (h *Handler) SetOnDisconnect(handler DisconnectHandler) {
	h.onDisconnect = handler
}

func (h *Handler) SetOnReady(handler ReadyHandler) {
	h.readyMu.Lock()
	h.onReady = handler
	h.readyMu.Unlock()
}

func (h *Handler) AddOnReady(handler ReadyHandler) {
	if handler == nil {
		return
	}
	h.readyMu.Lock()
	h.onReadyObservers = append(h.onReadyObservers, handler)
	h.readyMu.Unlock()
}

func (h *Handler) SetOnInvocationResult(handler InvocationResultHandler) {
	h.onInvocationResult = handler
}

func (h *Handler) SetOnInvocationError(handler InvocationErrorHandler) {
	h.onInvocationError = handler
}

func (h *Handler) SetOnTaskClaim(handler TaskClaimHandler) {
	h.onTaskClaim = handler
}

func (h *Handler) SetOnTaskComplete(handler TaskCompleteHandler) {
	h.onTaskComplete = handler
}

func (h *Handler) SetOnTaskProgress(handler TaskProgressHandler) {
	h.onTaskProgress = handler
}

func (h *Handler) SetOnTaskCheckpoint(handler TaskCheckpointHandler) {
	h.onTaskCheckpoint = handler
}

func (h *Handler) SetOnInvocationCancelAck(handler InvocationCancelAckHandler) {
	h.onInvocationCancelAck = handler
}

func (h *Handler) SetOnTaskHeartbeat(handler TaskHeartbeatHandler) {
	h.onTaskHeartbeat = handler
}

func (h *Handler) SetOnTaskClaimPayload(handler TaskClaimPayloadHandler) {
	h.onTaskClaimPayload = handler
}
func (h *Handler) SetOnTaskCompletePayload(handler TaskCompletePayloadHandler) {
	h.onTaskCompletePayload = handler
}
func (h *Handler) SetOnTaskProgressPayload(handler TaskProgressPayloadHandler) {
	h.onTaskProgressPayload = handler
}
func (h *Handler) SetOnTaskCheckpointPayload(handler TaskCheckpointPayloadHandler) {
	h.onTaskCheckpointPayload = handler
}
func (h *Handler) SetOnTaskHeartbeatPayload(handler TaskHeartbeatPayloadHandler) {
	h.onTaskHeartbeatPayload = handler
}

func (h *Handler) HandleWS(c *gin.Context) {
	principal, ok := credential.GinPrincipal(c)
	if !ok {
		c.JSON(401, gin.H{"code": "mesh.credential_invalid", "message": "unauthorized"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("devicemesh: ws upgrade failed: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	if err := h.handleConnection(ctx, conn, principal); err != nil {
		log.Printf("devicemesh: connection error: spaceId=%s deviceId=%s runtimeId=%s err=%v",
			principal.SpaceID, principal.DeviceID, principal.RuntimeID, err)
	}
}

func (h *Handler) handleConnection(ctx context.Context, conn *websocket.Conn, principal credential.DeviceRuntimePrincipal) error {
	defer conn.Close()

	conn.SetReadLimit(meshprotocol.MaxMessageSizeBytes)
	conn.SetReadDeadline(time.Now().Add(meshprotocol.ReadDeadlineSeconds * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(meshprotocol.ReadDeadlineSeconds * time.Second))
		return nil
	})

	rawHello, err := h.readHello(conn)
	if err != nil {
		h.sendError(conn, "mesh.hello_timeout", err.Error(), true)
		return err
	}

	env, hello, err := h.parseHello(rawHello, principal)
	if err != nil {
		h.sendError(conn, "mesh.protocol_incompatible", err.Error(), true)
		return err
	}

	if env.SpaceID != principal.SpaceID || env.DeviceID != principal.DeviceID || env.RuntimeID != principal.RuntimeID {
		h.sendError(conn, "mesh.identity_mismatch", "envelope identity does not match credential", true)
		return fmt.Errorf("identity mismatch")
	}

	acquireReq := deviceruntime.AcquireRequest{
		Identity: protocol.SessionIdentity{
			SpaceID:          principal.SpaceID,
			DeviceID:         principal.DeviceID,
			RuntimeID:        principal.RuntimeID,
			RuntimeSessionID: env.RuntimeSessionID,
		},
		Platform:               runtimeidentity.PlatformUnknown,
		RuntimeVersion:         hello.RuntimeVersion,
		RuntimeContractVersion: hello.RuntimeContractVersion,
		Capabilities:           hello.RuntimeCapabilities,
		Cursor:                 hello.ResumeCursor(env.ConnectionGeneration),
		Now:                    time.Now().UTC(),
	}

	result, err := h.sessions.Acquire(ctx, acquireReq)
	if err != nil {
		h.sendError(conn, "mesh.session_error", err.Error(), true)
		return fmt.Errorf("acquire session: %w", err)
	}

	if result.Resume.Mode == protocol.ResumeModeFull && result.Resume.Reason == "client_cursor_ahead" {
		h.sendError(conn, "mesh.cursor_reset_required", "client cursor ahead of server state", true)
		return fmt.Errorf("cursor reset required")
	}

	session := result.Session
	meshConn := NewMeshConnection(conn, session.ID, session.ConnectionGeneration, principal.SpaceID, principal.DeviceID, principal.RuntimeID)

	stored, attached := h.hub.Attach(session.ID, meshConn)
	if !attached {
		_ = meshConn.Close(4001, "session_superseded")
		return fmt.Errorf("session superseded by newer connection")
	}
	if stored != nil {
		_ = stored.Close(4001, "session_superseded")
	}

	defer h.hub.Detach(session.ID, session.ConnectionGeneration)
	if h.onDisconnect != nil {
		h.onDisconnect(session.ID.String(), session.ConnectionGeneration)
	}
	defer func() {
		now := time.Now().UTC()
		if closeErr := h.sessions.Close(ctx, session.ID, session.ConnectionGeneration, "connection_closed", now); closeErr != nil {
			log.Printf("devicemesh: session close on disconnect: sessionId=%s gen=%d err=%v", session.ID, session.ConnectionGeneration, closeErr)
		}
	}()

	if _, err := h.sessions.MarkReady(ctx, session.ID, session.ConnectionGeneration, time.Now().UTC()); err != nil {
		return fmt.Errorf("mark ready: %w", err)
	}

	if err := h.sendHelloAck(meshConn, session.ID, session.ConnectionGeneration, result.Resume.Mode); err != nil {
		return fmt.Errorf("send hello ack: %w", err)
	}
	h.readyMu.RLock()
	onReady := h.onReady
	readyObservers := append([]ReadyHandler(nil), h.onReadyObservers...)
	h.readyMu.RUnlock()
	if onReady != nil {
		onReady(principal.SpaceID, principal.DeviceID)
	}
	for _, observer := range readyObservers {
		if observer != nil {
			observer(principal.SpaceID, principal.DeviceID)
		}
	}

	return h.messageLoop(ctx, meshConn, session.ID, session.ConnectionGeneration, env.Sequence)
}

func (h *Handler) readHello(conn *websocket.Conn) ([]byte, error) {
	conn.SetReadDeadline(time.Now().Add(meshprotocol.HelloTimeoutSeconds * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("read hello: %w", err)
	}
	return data, nil
}

func (h *Handler) parseHello(data []byte, principal credential.DeviceRuntimePrincipal) (*protocol.Envelope, *protocol.HelloPayload, error) {
	var env protocol.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, nil, fmt.Errorf("parse envelope: %w", err)
	}

	if err := env.ValidateBase(meshprotocol.RuntimeProtocolDescriptor); err != nil {
		return nil, nil, fmt.Errorf("validate envelope: %w", err)
	}
	if !env.VerifyPayloadHash() {
		return nil, nil, fmt.Errorf("hello payload hash mismatch")
	}

	if env.MessageType != protocol.MessageTypeHello {
		return nil, nil, fmt.Errorf("expected hello, got %s", env.MessageType)
	}

	var hello protocol.HelloPayload
	if err := json.Unmarshal(env.Payload, &hello); err != nil {
		return nil, nil, fmt.Errorf("parse hello payload: %w", err)
	}

	if hello.RuntimeContractVersion != meshprotocol.RuntimeContractVersion {
		return nil, nil, fmt.Errorf("unsupported contract version: %s", hello.RuntimeContractVersion)
	}
	if hello.DeviceID != principal.DeviceID || hello.RuntimeID != principal.RuntimeID {
		return nil, nil, fmt.Errorf("hello payload identity does not match credential")
	}

	return &env, &hello, nil
}

func (h *Handler) sendHelloAck(conn *MeshConnection, sessionID runtimeidentity.RuntimeSessionID, generation int64, resumeMode protocol.ResumeMode) error {
	helloAck := protocol.HelloAckPayload{
		Accepted:   true,
		SessionID:  sessionID,
		ServerTime: time.Now().UTC(),
		ResumeMode: resumeMode,
	}
	return h.sendEnvelope(conn, protocol.MessageTypeHelloAck, helloAck)
}

func (h *Handler) messageLoop(ctx context.Context, conn *MeshConnection, sessionID runtimeidentity.RuntimeSessionID, generation int64, clientSequence int64) error {
	for {
		conn.Conn.SetReadDeadline(time.Now().Add(meshprotocol.ReadDeadlineSeconds * time.Second))
		_, data, err := conn.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				return fmt.Errorf("read message: %w", err)
			}
			return nil
		}

		if len(data) > meshprotocol.MaxMessageSizeBytes {
			h.sendErrorConn(conn, "mesh.message_too_big", "message exceeds size limit", false)
			continue
		}

		var env protocol.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			h.sendErrorConn(conn, "mesh.protocol_error", "invalid envelope", false)
			continue
		}
		if env.Protocol != meshprotocol.ProtocolName || env.EnvelopeVersion != meshprotocol.EnvelopeVersion || env.PayloadSchemaVersion != 1 || env.SpaceID != conn.SpaceID || env.DeviceID != conn.DeviceID || env.RuntimeID != conn.RuntimeID || env.Sequence < 1 {
			h.sendErrorConn(conn, "mesh.protocol_error", "invalid protocol or device identity", false)
			return fmt.Errorf("设备通道收到不匹配的协议或身份")
		}

		if !env.VerifyPayloadHash() {
			h.sendErrorConn(conn, "mesh.protocol_error", "payload hash mismatch", false)
			continue
		}

		if env.RuntimeSessionID != sessionID {
			h.sendErrorConn(conn, "mesh.session_superseded", "session id mismatch", true)
			return nil
		}

		if env.ConnectionGeneration != generation {
			h.sendErrorConn(conn, "mesh.generation_mismatch", "connection generation mismatch", true)
			return nil
		}

		if env.Sequence > 0 {
			last := atomic.LoadInt64(&clientSequence)
			switch protocol.ClassifySequence(protocol.Sequence(last), protocol.Sequence(env.Sequence)) {
			case protocol.SequenceDispositionDuplicate:
				h.sendErrorConn(conn, "mesh.sequence_duplicate", "duplicate sequence", false)
				continue
			case protocol.SequenceDispositionStale:
				h.sendErrorConn(conn, "mesh.sequence_stale", "stale sequence", false)
				continue
			case protocol.SequenceDispositionGap:
				h.sendErrorConn(conn, "mesh.sequence_gap", "sequence gap", false)
				continue
			case protocol.SequenceDispositionNext:
				atomic.StoreInt64(&clientSequence, env.Sequence)
			}
		}

		switch env.MessageType {
		case protocol.MessageTypePing:
			var ping protocol.PingPayload
			if err := json.Unmarshal(env.Payload, &ping); err == nil {
				conn.LastPongAt = time.Now().UTC()
				pong := protocol.PongPayload{Time: ping.Time}
				_ = h.sendEnvelope(conn, protocol.MessageTypePong, pong)
				h.sessions.Heartbeat(ctx, sessionID, generation, time.Now().UTC())
			}

		case protocol.MessageTypePong:
			conn.LastPongAt = time.Now().UTC()
			h.sessions.Heartbeat(ctx, sessionID, generation, time.Now().UTC())

		case protocol.MessageTypeCommand:
			h.sendErrorConn(conn, "mesh.unsupported_command", "commands not supported in G21", false)

		case protocol.MessageTypeCommandAck:
			var ack protocol.CommandAckPayload
			if err := json.Unmarshal(env.Payload, &ack); err == nil {
				if h.onCommandAck != nil {
					h.onCommandAck(ack.CommandID, int64(env.ConnectionGeneration), ack.Status, nil)
				}
			}

		case protocol.MessageTypeRuntimeInvoke:
			var invoke protocol.RuntimeInvokePayload
			if err := json.Unmarshal(env.Payload, &invoke); err != nil {
				h.sendErrorConn(conn, "mesh.protocol_error", "invalid runtime invoke payload", false)
				break
			}
			if h.dispatcher != nil {
				handler := h.dispatcher.Resolve(invoke.Handler)
				if handler == nil {
					h.sendErrorConn(conn, "mesh.unsupported_runtime_handler", "runtime handler not registered", false)
					break
				}
				result, invokeErr := handler(invoke)
				if invokeErr != nil {
					now := time.Now().UTC()
					errPayload := protocol.RuntimeErrorPayload{
						InvocationID:         invoke.InvocationID,
						RuntimeSessionID:     env.RuntimeSessionID,
						ConnectionGeneration: env.ConnectionGeneration,
						DeviceID:             conn.DeviceID,
						RuntimeID:            conn.RuntimeID,
						ErrorCode:            "mesh.invoke_error",
						Message:              invokeErr.Error(),
						Retryable:            false,
						IdempotencyKey:       invoke.IdempotencyKey,
						FencingToken:         invoke.FencingToken,
						FailedAt:             now,
					}
					_ = h.sendEnvelope(conn, protocol.MessageTypeRuntimeError, errPayload)
				} else if result != nil {
					result.IdempotencyKey = invoke.IdempotencyKey
					result.FencingToken = invoke.FencingToken
					_ = h.sendEnvelope(conn, protocol.MessageTypeRuntimeResult, *result)
				}
			} else {
				h.sendErrorConn(conn, "mesh.unsupported_command", "runtime.invoke not accepted on this connection", false)
			}

		case protocol.MessageTypeRuntimeResult:
			var result protocol.RuntimeResultPayload
			if err := json.Unmarshal(env.Payload, &result); err == nil {
				if h.onInvocationResult != nil {
					h.onInvocationResult(result)
				}
			}

		case protocol.MessageTypeRuntimeError:
			var errResult protocol.RuntimeErrorPayload
			if err := json.Unmarshal(env.Payload, &errResult); err == nil {
				if h.onInvocationError != nil {
					h.onInvocationError(errResult)
				}
			}

		case protocol.MessageTypeTaskClaim:
			var claim protocol.TaskClaimPayload
			if err := json.Unmarshal(env.Payload, &claim); err == nil {
				accepted := false
				valid := claim.DeviceID == conn.DeviceID && claim.RuntimeID == conn.RuntimeID && claim.RuntimeSessionID == conn.SessionID && claim.ConnectionGeneration == conn.Generation && claim.TaskRunID != "" && claim.AttemptID != "" && claim.LeaseID != "" && claim.WorkerID == conn.RuntimeID.String() && claim.LeaseDurationMs > 0 && claim.LeaseDurationMs <= 300000
				if valid && h.onTaskClaimPayload != nil {
					accepted = h.onTaskClaimPayload(claim)
				} else if valid && h.onTaskClaim != nil {
					accepted = h.onTaskClaim(claim.TaskRunID, claim.AttemptID, claim.WorkerID, time.Duration(claim.LeaseDurationMs)*time.Millisecond)
				}
				_ = h.sendEnvelope(conn, protocol.MessageTypeTaskLeaseAck, protocol.TaskLeaseAckPayload{TaskRunID: claim.TaskRunID, AttemptID: claim.AttemptID, LeaseID: claim.LeaseID, Accepted: accepted, LeaseDurationMs: claim.LeaseDurationMs, RuntimeSessionID: conn.SessionID, ConnectionGeneration: conn.Generation})
			}

		case protocol.MessageTypeTaskComplete:
			var complete protocol.TaskCompletePayload
			if err := json.Unmarshal(env.Payload, &complete); err == nil {
				if complete.DeviceID != conn.DeviceID || complete.RuntimeID != conn.RuntimeID || complete.RuntimeSessionID != conn.SessionID || complete.ConnectionGeneration != conn.Generation {
					continue
				}
				if h.onTaskCompletePayload != nil {
					h.onTaskCompletePayload(complete)
				} else if h.onTaskComplete != nil {
					h.onTaskComplete(complete.TaskRunID, complete.AttemptID, complete.Success, complete.Result, complete.Error)
				}
			}

		case protocol.MessageTypeTaskProgress:
			var progress protocol.TaskProgressPayload
			if err := json.Unmarshal(env.Payload, &progress); err == nil {
				if progress.DeviceID != conn.DeviceID || progress.RuntimeID != conn.RuntimeID || progress.RuntimeSessionID != conn.SessionID || progress.ConnectionGeneration != conn.Generation {
					continue
				}
				if h.onTaskProgressPayload != nil {
					h.onTaskProgressPayload(progress)
				} else if h.onTaskProgress != nil {
					h.onTaskProgress(progress.TaskRunID, progress.AttemptID, mustMarshal(progress))
				}
			}

		case protocol.MessageTypeTaskCheckpoint:
			var checkpoint protocol.TaskCheckpointPayload
			if err := json.Unmarshal(env.Payload, &checkpoint); err == nil {
				if checkpoint.DeviceID != conn.DeviceID || checkpoint.RuntimeID != conn.RuntimeID || checkpoint.RuntimeSessionID != conn.SessionID || checkpoint.ConnectionGeneration != conn.Generation {
					continue
				}
				if h.onTaskCheckpointPayload != nil {
					h.onTaskCheckpointPayload(checkpoint)
				} else if h.onTaskCheckpoint != nil {
					h.onTaskCheckpoint(checkpoint.TaskRunID, checkpoint.AttemptID, mustMarshal(checkpoint))
				}
			}

		case protocol.MessageTypeTaskHeartbeat:
			var heartbeat protocol.TaskHeartbeatPayload
			if err := json.Unmarshal(env.Payload, &heartbeat); err == nil {
				accepted := false
				valid := heartbeat.DeviceID == conn.DeviceID && heartbeat.RuntimeID == conn.RuntimeID && heartbeat.RuntimeSessionID == conn.SessionID && heartbeat.ConnectionGeneration == conn.Generation && heartbeat.Sequence > 0
				if valid && h.onTaskHeartbeatPayload != nil {
					accepted = h.onTaskHeartbeatPayload(heartbeat)
				}
				_ = h.sendEnvelope(conn, protocol.MessageTypeTaskLeaseAck, protocol.TaskLeaseAckPayload{TaskRunID: heartbeat.TaskRunID, AttemptID: heartbeat.AttemptID, LeaseID: heartbeat.LeaseID, Sequence: heartbeat.Sequence, Accepted: accepted, LeaseDurationMs: 300000, RuntimeSessionID: conn.SessionID, ConnectionGeneration: conn.Generation})
			}

		case protocol.MessageTypeRuntimeCancel:
			var cancel protocol.RuntimeCancelPayload
			if err := json.Unmarshal(env.Payload, &cancel); err == nil {
				if h.onInvocationCancelAck != nil {
					h.onInvocationCancelAck(cancel.InvocationID, cancel.Reason)
				}
			}

		default:
			h.sendErrorConn(conn, "mesh.protocol_error", "unsupported message type: "+string(env.MessageType), false)
		}
	}
}

func (h *Handler) sendEnvelope(conn *MeshConnection, msgType protocol.MessageType, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	env := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          msgType,
		MessageID:            uuid.New().String(),
		SpaceID:              conn.SpaceID,
		DeviceID:             conn.DeviceID,
		RuntimeID:            conn.RuntimeID,
		RuntimeSessionID:     conn.SessionID,
		ConnectionGeneration: conn.Generation,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	data, err := json.Marshal(env)
	if err != nil {
		return err
	}

	return conn.Send(data)
}

func (h *Handler) sendError(conn *websocket.Conn, code, message string, fatal bool) {
	payload := protocol.ErrorPayload{
		Code:    code,
		Message: message,
	}
	payloadBytes, _ := json.Marshal(payload)

	env := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypeError,
		MessageID:            uuid.New().String(),
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	data, err := json.Marshal(env)
	if err != nil {
		return
	}

	_ = conn.WriteMessage(websocket.TextMessage, data)
	if fatal {
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(4000, code),
			time.Now().Add(2*time.Second))
	}
}

func (h *Handler) sendErrorConn(conn *MeshConnection, code, message string, fatal bool) {
	h.sendError(conn.Conn, code, message, fatal)
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

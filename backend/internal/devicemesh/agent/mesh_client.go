package agent

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	meshprotocol "github.com/u-ai/backend/internal/devicemesh/protocol"
	protocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type MeshClientConfig struct {
	CloudBaseURL      string
	Credential        string
	UserID            runtimeidentity.UserID
	Identity          *LocalIdentity
	Cursor            *SessionCursor
	OnState           func(AgentState)
	RuntimeDispatcher RuntimeDispatcher
	TaskWorker        TaskWorkerIface
}

type TaskWorkerIface interface {
	ExecuteTask(ctx context.Context, dispatch protocol.TaskDispatchPayload) error
	CancelTask(ctx context.Context, taskRunID, attemptID, leaseID string) error
}

type MeshClient struct {
	conf    MeshClientConfig
	dialer  *websocket.Dialer
	mu      sync.Mutex
	conn    *websocket.Conn
	state   *ConnectionManager
	stopCh  chan struct{}
	backoff *Backoff

	handshakeOnce sync.Once
	handshakeDone chan struct{}
	handshakeErr  error

	seqMu          sync.Mutex
	localSequence  int64
	remoteSequence int64

	sessionID     runtimeidentity.RuntimeSessionID
	connectionGen int64

	credentialStore *CredentialStore
}

func NewMeshClient(conf MeshClientConfig) *MeshClient {
	return &MeshClient{
		conf: conf,
		dialer: &websocket.Dialer{
			HandshakeTimeout: meshprotocol.HelloTimeoutSeconds * time.Second,
			TLSClientConfig:  &tls.Config{},
			Proxy:            http.ProxyFromEnvironment,
		},
		state:         NewConnectionManager(),
		stopCh:        make(chan struct{}),
		backoff:       NewBackoff(),
		handshakeDone: make(chan struct{}),
	}
}

func (c *MeshClient) SetCredentialStore(store *CredentialStore) {
	c.credentialStore = store
}

func (c *MeshClient) SetTaskWorker(w TaskWorkerIface) {
	c.conf.TaskWorker = w
}

func (c *MeshClient) Start() {
	go c.runLoop()
}

func (c *MeshClient) Stop() {
	close(c.stopCh)
}

func (c *MeshClient) State() AgentState {
	return c.state.Get()
}

func (c *MeshClient) runLoop() {
	for {
		select {
		case <-c.stopCh:
			c.setState(StateStopped)
			c.closeSocket(websocket.CloseGoingAway, "stopped")
			return
		default:
		}

		if c.state.Get() == StateRevoked {
			c.setState(StateStopped)
			return
		}

		if c.state.Get() == StateUnprovisioned {
			select {
			case <-c.stopCh:
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}

		err := c.connectAndServe()
		if err != nil {
			log.Printf("devicemesh: agent: connection error: %v", err)
		}

		if c.state.Get() == StateRevoked {
			return
		}

		select {
		case <-c.stopCh:
			return
		case <-time.After(c.backoff.Duration()):
		}
	}
}

func (c *MeshClient) connectAndServe() error {
	c.setState(StateConnecting)

	wsURL := c.wsURL()
	header := http.Header{}
	header.Set("Authorization", "AmitiaDevice "+c.conf.Credential)

	conn, _, err := c.dialer.DialContext(context.Background(), wsURL, header)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	defer func() {
		gen := c.connectionGen
		c.closeSocketWithGen(websocket.CloseGoingAway, "", gen)
	}()

	c.handshakeOnce = sync.Once{}
	c.handshakeDone = make(chan struct{})
	c.handshakeErr = nil
	c.localSequence = 0
	c.remoteSequence = 0

	c.setState(StateHandshaking)
	if err := c.sendHello(); err != nil {
		c.completeHandshake(fmt.Errorf("hello: %w", err))
		return err
	}

	select {
	case <-c.handshakeDone:
		if c.handshakeErr != nil {
			return c.handshakeErr
		}
	case <-c.stopCh:
		return nil
	case <-time.After(meshprotocol.HelloTimeoutSeconds * time.Second):
		return fmt.Errorf("handshake timeout: no HelloAck received")
	}

	c.setState(StateHelloAck)

	c.setState(StateReady)
	c.backoff.Reset()

	return c.readLoop()
}

func (c *MeshClient) completeHandshake(err error) {
	c.handshakeOnce.Do(func() {
		c.handshakeErr = err
		close(c.handshakeDone)
	})
}

func deviceWorkflowRuntimeCapabilities() []string {
	// ASR-final phrase events are a platform-neutral Workflow ingress capability:
	// official Flutter and Electron clients both forward final realtime ASR into
	// the local Device Agent. Android-only native producers are advertised only
	// when the backend is running inside the Android runtime.
	capabilities := []string{"workflow.trigger.voice_phrase.v1"}
	if strings.TrimSpace(os.Getenv("ANDROID_ROOT")) == "" {
		return capabilities
	}
	return append(capabilities,
		"workflow.trigger.android_intent.v1",
		"workflow.trigger.tasker.v1",
		"workflow.trigger.voice_wake.v1",
		"workflow.trigger.app_foreground.v1",
	)
}

func (c *MeshClient) sendHello() error {
	cursor := c.conf.Cursor
	lastGen := int64(1)
	var (
		lastAppliedStateRev int64
		lastProcessedCmdSeq int64
		lastEventSeq        int64
		actualStateHash     string
		lastSessionID       runtimeidentity.RuntimeSessionID
	)
	if cursor != nil {
		lastGen = cursor.ConnectionGeneration
		lastAppliedStateRev = cursor.LastAppliedStateRevision
		lastProcessedCmdSeq = cursor.LastProcessedCommandSeq
		lastEventSeq = cursor.LastEventSequence
		actualStateHash = cursor.ActualStateHash
		lastSessionID = cursor.RuntimeSessionID
	}

	hello := protocol.HelloPayload{
		RuntimeVersion:               "1.0.0",
		RuntimeContractVersion:       meshprotocol.RuntimeContractVersion,
		DeviceID:                     c.conf.Identity.DeviceID,
		RuntimeID:                    c.conf.Identity.RuntimeID,
		RuntimeCapabilities:          deviceWorkflowRuntimeCapabilities(),
		LastAppliedStateRevision:     lastAppliedStateRev,
		LastProcessedCommandSequence: lastProcessedCmdSeq,
		LastEventSequence:            lastEventSeq,
		ActualStateHash:              actualStateHash,
	}

	payloadBytes, err := json.Marshal(hello)
	if err != nil {
		return err
	}

	seq := c.nextLocalSequence()

	env := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypeHello,
		MessageID:            uuid.New().String(),
		UserID:               c.conf.UserID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     lastSessionID,
		ConnectionGeneration: lastGen,
		Sequence:             seq,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	return c.writeEnvelope(env)
}

func (c *MeshClient) nextLocalSequence() int64 {
	c.seqMu.Lock()
	defer c.seqMu.Unlock()
	c.localSequence++
	return c.localSequence
}

func (c *MeshClient) readLoop() error {
	c.conn.SetReadLimit(meshprotocol.MaxMessageSizeBytes)
	c.conn.SetPongHandler(func(_ string) error {
		c.conn.SetReadDeadline(time.Now().Add(meshprotocol.ReadDeadlineSeconds * time.Second))
		return nil
	})

	heartbeatTicker := time.NewTicker(meshprotocol.HeartbeatInterval * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-c.stopCh:
			return nil
		case <-heartbeatTicker.C:
			if err := c.sendPing(); err != nil {
				return fmt.Errorf("ping: %w", err)
			}
		default:
		}

		c.conn.SetReadDeadline(time.Now().Add(time.Duration(meshprotocol.ReadDeadlineSeconds) * time.Second))
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				return fmt.Errorf("read: %w", err)
			}
			return nil
		}

		if len(data) > meshprotocol.MaxMessageSizeBytes {
			continue
		}

		var env protocol.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}

		if !env.VerifyPayloadHash() {
			continue
		}

		if env.MessageType != protocol.MessageTypeHelloAck {
			if env.Sequence > 0 {
				if env.Sequence <= c.remoteSequence {
					continue
				}
				c.remoteSequence = env.Sequence
			}
		}

		switch env.MessageType {
		case protocol.MessageTypeHelloAck:
			c.handleHelloAck(&env)
		case protocol.MessageTypePing:
			c.handlePing(&env)
		case protocol.MessageTypeError:
			c.handleError(&env)
		case protocol.MessageTypePong:
		case protocol.MessageTypeRuntimeInvoke:
			c.handleRuntimeInvoke(&env)
		case protocol.MessageTypeRuntimeCancel:
			c.handleRuntimeCancel(&env)
		case protocol.MessageTypeCommand:
			c.sendUnsupportedCommand(&env)
		case protocol.MessageTypeTaskDispatch:
			c.handleTaskDispatch(&env)
		case protocol.MessageTypeTaskCancel:
			c.handleTaskCancel(&env)
		}
	}
}

func (c *MeshClient) handleHelloAck(env *protocol.Envelope) {
	var ack protocol.HelloAckPayload
	if err := json.Unmarshal(env.Payload, &ack); err != nil {
		c.completeHandshake(fmt.Errorf("helloAck parse: %w", err))
		return
	}

	if !ack.Accepted {
		c.completeHandshake(fmt.Errorf("hello rejected by server"))
		c.setState(StateBackoff)
		return
	}

	if env.Sequence < 1 {
		c.completeHandshake(fmt.Errorf("invalid remote sequence: %d", env.Sequence))
		return
	}
	c.remoteSequence = int64(env.Sequence)

	if ack.ResumeMode == protocol.ResumeModeFull {
		c.conf.Cursor = &SessionCursor{
			ConnectionGeneration: env.ConnectionGeneration,
			RuntimeSessionID:     ack.SessionID,
		}
		c.sessionID = ack.SessionID
		c.connectionGen = env.ConnectionGeneration
		c.setState(StateDegraded)
		c.persistCursor()
		c.completeHandshake(nil)
		return
	}

	if c.conf.Cursor == nil {
		c.conf.Cursor = &SessionCursor{
			ConnectionGeneration: env.ConnectionGeneration,
			RuntimeSessionID:     ack.SessionID,
		}
	} else {
		c.conf.Cursor.RuntimeSessionID = ack.SessionID
		c.conf.Cursor.ConnectionGeneration = env.ConnectionGeneration
	}
	c.sessionID = ack.SessionID
	c.connectionGen = env.ConnectionGeneration

	c.persistCursor()
	c.completeHandshake(nil)
}

func (c *MeshClient) persistCursor() {
	if c.credentialStore == nil || c.conf.Cursor == nil {
		return
	}
	if err := c.credentialStore.SaveCursor(c.conf.Cursor); err != nil {
		log.Printf("devicemesh: agent: save cursor failed: %v", err)
	}
}

func (c *MeshClient) handlePing(env *protocol.Envelope) {
	var ping protocol.PingPayload
	if err := json.Unmarshal(env.Payload, &ping); err != nil {
		return
	}

	pong := protocol.PongPayload{Time: ping.Time}
	payloadBytes, err := json.Marshal(pong)
	if err != nil {
		log.Printf("devicemesh: agent: marshal pong failed: %v", err)
		return
	}

	seq := c.nextLocalSequence()

	pongEnv := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypePong,
		MessageID:            env.MessageID,
		UserID:               c.conf.UserID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     c.sessionID,
		ConnectionGeneration: c.connectionGen,
		Sequence:             seq,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	if err := c.writeEnvelope(pongEnv); err != nil {
		log.Printf("devicemesh: agent: send pong failed: %v", err)
	}
}

func (c *MeshClient) handleError(env *protocol.Envelope) {
	var errPayload protocol.ErrorPayload
	if err := json.Unmarshal(env.Payload, &errPayload); err != nil {
		return
	}

	if env.Sequence > 0 {
		if env.Sequence <= c.remoteSequence {
			return
		}
		c.remoteSequence = env.Sequence
	}

	switch errPayload.Code {
	case "mesh.credential_revoked", "mesh.credential_expired":
		c.setState(StateRevoked)
	case "mesh.session_superseded":
		c.closeSocketGen(websocket.CloseNormalClosure, "superseded", env.ConnectionGeneration)
	case "mesh.cursor_reset_required":
		c.conf.Cursor = &SessionCursor{}
		c.setState(StateDegraded)
	}
}

func (c *MeshClient) sendPing() error {
	ping := protocol.PingPayload{Time: time.Now().UTC()}
	payloadBytes, err := json.Marshal(ping)
	if err != nil {
		log.Printf("devicemesh: agent: marshal ping failed: %v", err)
		return err
	}

	seq := c.nextLocalSequence()

	env := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypePing,
		MessageID:            uuid.New().String(),
		UserID:               c.conf.UserID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     c.sessionID,
		ConnectionGeneration: c.connectionGen,
		Sequence:             seq,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	return c.writeEnvelope(env)
}

func (c *MeshClient) sendUnsupportedCommand(env *protocol.Envelope) {
	var cmd protocol.CommandPayload
	if err := json.Unmarshal(env.Payload, &cmd); err != nil {
		c.sendCommandReject(env, "invalid_command", "failed to parse command")
		return
	}

	result, err := c.executeCommand(cmd)
	if err != nil {
		c.sendCommandReject(env, "command_failed", err.Error())
		return
	}

	c.sendCommandAck(&cmd, result)
}

func (c *MeshClient) handleRuntimeInvoke(env *protocol.Envelope) {
	var invoke protocol.RuntimeInvokePayload
	if err := json.Unmarshal(env.Payload, &invoke); err != nil {
		c.sendRuntimeError(&protocol.RuntimeErrorPayload{
			InvocationID:         "",
			RuntimeSessionID:     c.sessionID,
			ConnectionGeneration: c.connectionGen,
			DeviceID:             c.conf.Identity.DeviceID,
			RuntimeID:            c.conf.Identity.RuntimeID,
			ErrorCode:            "invalid_invoke_payload",
			Message:              fmt.Sprintf("failed to parse invoke payload: %v", err),
			Retryable:            false,
			FailedAt:             time.Now().UTC(),
		})
		return
	}

	go func() {
		result, err := c.executeRuntimeInvoke(invoke)
		if err != nil {
			c.sendRuntimeError(&protocol.RuntimeErrorPayload{
				InvocationID:         invoke.InvocationID,
				RuntimeSessionID:     c.sessionID,
				ConnectionGeneration: c.connectionGen,
				DeviceID:             c.conf.Identity.DeviceID,
				RuntimeID:            c.conf.Identity.RuntimeID,
				ErrorCode:            "invoke_execution_failed",
				Message:              err.Error(),
				Retryable:            true,
				FailedAt:             time.Now().UTC(),
			})
			return
		}
		c.sendRuntimeResult(result)
	}()
}

func (c *MeshClient) handleRuntimeCancel(env *protocol.Envelope) {
	var cancelPayload protocol.RuntimeCancelPayload
	if err := json.Unmarshal(env.Payload, &cancelPayload); err != nil {
		return
	}
	canceller, ok := c.conf.RuntimeDispatcher.(RuntimeCancelDispatcher)
	if !ok || canceller == nil {
		return
	}
	canceller.CancelInvocation(cancelPayload.InvocationID)
}

func (c *MeshClient) executeRuntimeInvoke(invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	handler := c.resolveHandler(invoke.Handler)
	if handler == nil {
		return nil, fmt.Errorf("unsupported handler: %s", invoke.Handler)
	}
	return handler(invoke)
}

func (c *MeshClient) resolveHandler(handlerName string) RuntimeInvokeHandler {
	return c.conf.RuntimeDispatcher.Resolve(handlerName)
}

func (c *MeshClient) sendRuntimeResult(result *protocol.RuntimeResultPayload) {
	payloadBytes, err := json.Marshal(result)
	if err != nil {
		return
	}

	seq := c.nextLocalSequence()

	env := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypeRuntimeResult,
		MessageID:            uuid.New().String(),
		UserID:               c.conf.UserID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     c.sessionID,
		ConnectionGeneration: c.connectionGen,
		Sequence:             seq,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	if err := c.writeEnvelope(env); err != nil {
		log.Printf("devicemesh: agent: send runtime result failed: %v", err)
	}
}

func (c *MeshClient) sendRuntimeError(errPayload *protocol.RuntimeErrorPayload) {
	payloadBytes, err := json.Marshal(errPayload)
	if err != nil {
		return
	}

	seq := c.nextLocalSequence()

	env := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypeRuntimeError,
		MessageID:            uuid.New().String(),
		UserID:               c.conf.UserID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     c.sessionID,
		ConnectionGeneration: c.connectionGen,
		Sequence:             seq,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	if err := c.writeEnvelope(env); err != nil {
		log.Printf("devicemesh: agent: send runtime error failed: %v", err)
	}
}

func (c *MeshClient) handleTaskDispatch(env *protocol.Envelope) {
	var dispatch protocol.TaskDispatchPayload
	if err := json.Unmarshal(env.Payload, &dispatch); err != nil {
		c.sendTaskError(env.MessageID, "", "invalid_dispatch_payload", fmt.Sprintf("failed to parse dispatch: %v", err))
		return
	}

	if c.conf.TaskWorker == nil {
		c.sendTaskError(env.MessageID, dispatch.TaskRunID, "task_worker_unavailable", "no task worker configured")
		return
	}

	if err := c.conf.TaskWorker.ExecuteTask(context.Background(), dispatch); err != nil {
		c.sendTaskError(env.MessageID, dispatch.TaskRunID, "task_execution_failed", err.Error())
		return
	}
}

func (c *MeshClient) handleTaskCancel(env *protocol.Envelope) {
	var cancel protocol.TaskCancelPayload
	if err := json.Unmarshal(env.Payload, &cancel); err != nil {
		c.sendTaskError(env.MessageID, cancel.TaskRunID, "invalid_cancel_payload", err.Error())
		return
	}

	if c.conf.TaskWorker != nil {
		if err := c.conf.TaskWorker.CancelTask(context.Background(), cancel.TaskRunID, cancel.AttemptID, cancel.LeaseID); err != nil {
			log.Printf("devicemesh: agent: cancel task failed: %v", err)
			c.sendTaskError(env.MessageID, cancel.TaskRunID, "cancel_failed", err.Error())
			return
		}
	}
}

func (c *MeshClient) sendTaskClaim(taskRunID, attemptID, leaseID, workerID string, leaseDurationMs int64) {
	claim := protocol.TaskClaimPayload{
		TaskRunID:            taskRunID,
		AttemptID:            attemptID,
		LeaseID:              leaseID,
		WorkerID:             workerID,
		LeaseDurationMs:      leaseDurationMs,
		RuntimeSessionID:     c.sessionID,
		ConnectionGeneration: c.connectionGen,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		ClaimedAt:            time.Now().UTC(),
	}
	c.sendTaskEnvelope(protocol.MessageTypeTaskClaim, claim)
}

func (c *MeshClient) sendTaskComplete(taskRunID, attemptID, leaseID string, success bool, result json.RawMessage, errMsg string) {
	complete := protocol.TaskCompletePayload{
		TaskRunID:            taskRunID,
		AttemptID:            attemptID,
		LeaseID:              leaseID,
		Success:              success,
		Result:               result,
		Error:                errMsg,
		RuntimeSessionID:     c.sessionID,
		ConnectionGeneration: c.connectionGen,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		CompletedAt:          time.Now().UTC(),
	}
	c.sendTaskEnvelope(protocol.MessageTypeTaskComplete, complete)
}

func (c *MeshClient) sendTaskProgress(taskRunID, attemptID, leaseID string, seq int64, current, total, percentage *float64, stage, message string) {
	progress := protocol.TaskProgressPayload{
		TaskRunID:            taskRunID,
		AttemptID:            attemptID,
		LeaseID:              leaseID,
		Sequence:             seq,
		Current:              current,
		Total:                total,
		Percentage:           percentage,
		Stage:                stage,
		Message:              message,
		RuntimeSessionID:     c.sessionID,
		ConnectionGeneration: c.connectionGen,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		ReportedAt:           time.Now().UTC(),
	}
	c.sendTaskEnvelope(protocol.MessageTypeTaskProgress, progress)
}

func (c *MeshClient) sendTaskHeartbeat(taskRunID, attemptID, leaseID string, seq int64) {
	heartbeat := protocol.TaskHeartbeatPayload{
		TaskRunID:            taskRunID,
		AttemptID:            attemptID,
		LeaseID:              leaseID,
		Sequence:             seq,
		RuntimeSessionID:     c.sessionID,
		ConnectionGeneration: c.connectionGen,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		ReportedAt:           time.Now().UTC(),
	}
	c.sendTaskEnvelope(protocol.MessageTypeTaskHeartbeat, heartbeat)
}

func (c *MeshClient) sendTaskCheckpoint(taskRunID, attemptID, leaseID, checkpointID string, version int64, payload json.RawMessage, payloadHash string) {
	checkpoint := protocol.TaskCheckpointPayload{
		TaskRunID:            taskRunID,
		AttemptID:            attemptID,
		LeaseID:              leaseID,
		CheckpointID:         checkpointID,
		Version:              version,
		Payload:              payload,
		PayloadHash:          payloadHash,
		RuntimeSessionID:     c.sessionID,
		ConnectionGeneration: c.connectionGen,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		CheckpointAt:         time.Now().UTC(),
	}
	c.sendTaskEnvelope(protocol.MessageTypeTaskCheckpoint, checkpoint)
}

func (c *MeshClient) sendTaskEnvelope(msgType protocol.MessageType, payload interface{}) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return
	}

	seq := c.nextLocalSequence()

	env := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          msgType,
		MessageID:            uuid.New().String(),
		UserID:               c.conf.UserID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     c.sessionID,
		ConnectionGeneration: c.connectionGen,
		Sequence:             seq,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	if err := c.writeEnvelope(env); err != nil {
		log.Printf("devicemesh: agent: send task envelope failed: %v", err)
	}
}

func (c *MeshClient) sendTaskError(messageID, taskRunID, code, message string) {
	seq := c.nextLocalSequence()

	errPayload := protocol.ErrorPayload{
		Code:    code,
		Message: message,
	}
	payloadBytes, _ := json.Marshal(errPayload)

	resp := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypeError,
		MessageID:            messageID,
		UserID:               c.conf.UserID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     c.sessionID,
		ConnectionGeneration: c.connectionGen,
		Sequence:             seq,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	if err := c.writeEnvelope(resp); err != nil {
		log.Printf("devicemesh: agent: send task error failed: %v", err)
	}
}

func (c *MeshClient) executeCommand(cmd protocol.CommandPayload) (*CommandResult, error) {
	switch cmd.CommandName {
	case "status":
		return c.execStatusCommand(cmd)
	case "ping":
		return &CommandResult{
			CommandID:       cmd.CommandID,
			CommandName:     cmd.CommandName,
			CommandSequence: cmd.CommandSequence,
			Status:          "completed",
			CompletedAt:     time.Now().UTC(),
		}, nil
	default:
		return nil, fmt.Errorf("unknown command: %s", cmd.CommandName)
	}
}

type CommandResult struct {
	CommandID       string
	CommandName     string
	CommandSequence int64
	Status          string
	Result          map[string]interface{}
	CompletedAt     time.Time
}

func (c *MeshClient) execStatusCommand(cmd protocol.CommandPayload) (*CommandResult, error) {
	result := map[string]interface{}{
		"state":            string(c.state.Get()),
		"connectionGen":    c.connectionGen,
		"localSequence":    c.localSequence,
		"remoteSequence":   c.remoteSequence,
		"runtimeSessionId": c.sessionID.String(),
		"deviceId":         c.conf.Identity.DeviceID.String(),
		"runtimeId":        c.conf.Identity.RuntimeID.String(),
	}

	return &CommandResult{
		CommandID:       cmd.CommandID,
		CommandName:     cmd.CommandName,
		CommandSequence: cmd.CommandSequence,
		Status:          "completed",
		Result:          result,
		CompletedAt:     time.Now().UTC(),
	}, nil
}

func (c *MeshClient) sendCommandAck(cmd *protocol.CommandPayload, result *CommandResult) {
	ack := protocol.CommandAckPayload{
		CommandID:        result.CommandID,
		CommandSequence:  result.CommandSequence,
		Status:           "completed",
		RuntimeSessionID: c.sessionID,
		ReceivedAt:       time.Now().UTC(),
	}

	resultBytes, _ := json.Marshal(result.Result)
	ack.PayloadHash = protocol.ComputePayloadHash(resultBytes)

	seq := c.nextLocalSequence()

	env := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypeCommandAck,
		MessageID:            uuid.New().String(),
		UserID:               c.conf.UserID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     c.sessionID,
		ConnectionGeneration: c.connectionGen,
		Sequence:             seq,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(mustMarshal(ack)),
		SentAt:               time.Now().UTC(),
		Payload:              mustMarshal(ack),
	}

	if err := c.writeEnvelope(env); err != nil {
		log.Printf("devicemesh: agent: send command ack failed: %v", err)
	}
}

func (c *MeshClient) sendCommandReject(env *protocol.Envelope, code, reason string) {
	seq := c.nextLocalSequence()

	errPayload := protocol.ErrorPayload{
		Code:    code,
		Message: reason,
	}
	payloadBytes, _ := json.Marshal(errPayload)

	resp := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypeError,
		MessageID:            env.MessageID,
		UserID:               c.conf.UserID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     c.sessionID,
		ConnectionGeneration: c.connectionGen,
		Sequence:             seq,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	if err := c.writeEnvelope(resp); err != nil {
		log.Printf("devicemesh: agent: send command reject failed: %v", err)
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func (c *MeshClient) writeEnvelope(env protocol.Envelope) error {
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

func (c *MeshClient) closeSocket(code int, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		_ = c.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(code, reason),
			time.Now().Add(2*time.Second))
		_ = c.conn.Close()
		c.conn = nil
	}
}

func (c *MeshClient) closeSocketWithGen(code int, reason string, gen int64) {
	c.connectionGen = gen
	c.closeSocketGen(code, reason, gen)
}

func (c *MeshClient) closeSocketGen(code int, reason string, gen int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		closeReason := reason
		if gen > 0 {
			closeReason = fmt.Sprintf("gen=%d;%s", gen, reason)
		}
		_ = c.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(code, closeReason),
			time.Now().Add(2*time.Second))
		_ = c.conn.Close()
		c.conn = nil
	}
}

func (c *MeshClient) setState(s AgentState) {
	c.state.Set(s)
	if c.conf.OnState != nil {
		c.conf.OnState(s)
	}
}

func (c *MeshClient) wsURL() string {
	base := strings.TrimRight(c.conf.CloudBaseURL, "/")
	u, err := url.Parse(base)
	if err != nil {
		return strings.Replace(base, "http", "ws", 1) + meshprotocol.WebSocketPath
	}

	scheme := "wss"
	if u.Scheme == "http" {
		scheme = "ws"
	}

	return scheme + "://" + u.Host + meshprotocol.WebSocketPath
}

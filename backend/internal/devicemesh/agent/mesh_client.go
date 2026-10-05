package agent

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	meshprotocol "github.com/u-ai/backend/internal/devicemesh/protocol"
	protocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type MeshClientConfig struct {
	CloudBaseURL      string
	Credential        string
	SpaceID           runtimeidentity.SpaceID
	Identity          *LocalIdentity
	Cursor            *SessionCursor
	OnState           func(AgentState)
	RuntimeDispatcher RuntimeDispatcher
	TaskWorker        TaskWorkerIface
	TLSConfig         *tls.Config
	SignRequest       func(*http.Request, string) error
	ExecutionJournal  *executionjournal.Store
	ExecutionGuard    RuntimeExecutionGuard
}

type TaskWorkerIface interface {
	ExecuteTask(ctx context.Context, dispatch protocol.TaskDispatchPayload) error
	CancelTask(ctx context.Context, taskRunID, attemptID, leaseID string) error
}

type MeshClient struct {
	conf         MeshClientConfig
	dialer       *websocket.Dialer
	mu           sync.Mutex
	conn         *websocket.Conn
	state        *ConnectionManager
	stopCh       chan struct{}
	stopOnce     sync.Once
	startOnce    sync.Once
	clientCtx    context.Context
	cancelClient context.CancelFunc
	doneCh       chan struct{}
	backoff      *Backoff

	handshakeOnce sync.Once
	handshakeDone chan struct{}
	handshakeErr  error

	seqMu          sync.Mutex
	localSequence  int64
	remoteSequence int64

	sessionID     runtimeidentity.RuntimeSessionID
	sessionMu     sync.RWMutex
	connectionGen int64

	credentialStore atomic.Pointer[CredentialStore]
	taskLeaseMu     sync.Mutex
	taskLeases      map[string]chan protocol.TaskLeaseAckPayload
}

func (c *MeshClient) sessionIdentity() runtimeidentity.RuntimeSessionID {
	c.sessionMu.RLock()
	defer c.sessionMu.RUnlock()
	return c.sessionID
}
func (c *MeshClient) sessionGeneration() int64 {
	c.sessionMu.RLock()
	defer c.sessionMu.RUnlock()
	return c.connectionGen
}

func NewMeshClient(conf MeshClientConfig) *MeshClient {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if conf.TLSConfig != nil {
		tlsConfig = conf.TLSConfig.Clone()
	}
	clientCtx, cancelClient := context.WithCancel(context.Background())
	return &MeshClient{
		conf: conf,
		dialer: &websocket.Dialer{
			HandshakeTimeout: meshprotocol.HelloTimeoutSeconds * time.Second,
			TLSClientConfig:  tlsConfig,
			Proxy:            http.ProxyFromEnvironment,
		},
		state:         NewConnectionManager(),
		stopCh:        make(chan struct{}),
		clientCtx:     clientCtx,
		cancelClient:  cancelClient,
		doneCh:        make(chan struct{}),
		backoff:       NewBackoff(),
		handshakeDone: make(chan struct{}),
		taskLeases:    make(map[string]chan protocol.TaskLeaseAckPayload),
	}
}

func (c *MeshClient) SetCredentialStore(store *CredentialStore) {
	c.credentialStore.Store(store)
}

func (c *MeshClient) SetTaskWorker(w TaskWorkerIface) {
	c.conf.TaskWorker = w
}

func (c *MeshClient) Start() {
	c.startOnce.Do(func() {
		if c.conf.Credential != "" && c.conf.Identity != nil && c.conf.SpaceID != "" {
			c.setState(StateConnecting)
		}
		go c.runLoop()
	})
}

func (c *MeshClient) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
		c.cancelClient()
		c.closeSocket(websocket.CloseGoingAway, "stopped")
	})
}

func (c *MeshClient) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		state := c.State()
		if state == StateReady {
			return nil
		}
		if state == StateRevoked || state == StateStopped {
			return fmt.Errorf("设备通道不可用: %s", state)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.stopCh:
			return fmt.Errorf("设备通道已停止")
		case <-ticker.C:
		}
	}
}

func (c *MeshClient) WaitStopped(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.doneCh:
		return nil
	}
}

func (c *MeshClient) State() AgentState {
	return c.state.Get()
}

func (c *MeshClient) runLoop() {
	defer close(c.doneCh)
	defer func() {
		if worker, ok := c.conf.TaskWorker.(interface{ CancelAllTasks() }); ok {
			worker.CancelAllTasks()
		}
		if c.State() != StateRevoked {
			c.setState(StateStopped)
		}
	}()
	for {
		select {
		case <-c.stopCh:
			c.setState(StateStopped)
			c.closeSocket(websocket.CloseGoingAway, "stopped")
			return
		default:
		}

		if c.state.Get() == StateRevoked {
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
	if c.conf.SignRequest != nil {
		request, err := http.NewRequestWithContext(c.clientCtx, http.MethodGet, wsURL, nil)
		if err != nil {
			return err
		}
		request.Header = header
		if err := c.conf.SignRequest(request, c.conf.SpaceID.String()); err != nil {
			return err
		}
	}

	conn, response, err := c.dialer.DialContext(c.clientCtx, wsURL, header)
	if err != nil {
		if response != nil && response.Body != nil {
			body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
			_ = response.Body.Close()
			var denied struct {
				Code string `json:"code"`
			}
			if response.StatusCode == http.StatusUnauthorized && json.Unmarshal(body, &denied) == nil {
				switch denied.Code {
				case "mesh.credential_revoked", "mesh.credential_expired", "mesh.credential_invalid", "mesh.device_not_trusted", "mesh.identity_proof_invalid", "mesh.identity_mismatch":
					c.setState(StateRevoked)
					c.cancelClient()
				}
			}
		}
		return fmt.Errorf("dial: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	defer func() {
		if c.State() == StateReady {
			c.setState(StateDegraded)
		}
		if dispatcher, ok := c.conf.RuntimeDispatcher.(RuntimeDisconnectDispatcher); ok && dispatcher != nil {
			dispatcher.CancelAllInvocations("device mesh disconnected")
		}
		if worker, ok := c.conf.TaskWorker.(interface{ CancelAllTasks() }); ok {
			worker.CancelAllTasks()
		}
		gen := c.sessionGeneration()
		c.closeSocketWithGen(websocket.CloseGoingAway, "", gen)
	}()

	c.handshakeOnce = sync.Once{}
	c.handshakeDone = make(chan struct{})
	c.handshakeErr = nil
	c.seqMu.Lock()
	c.localSequence = 0
	c.remoteSequence = 0
	c.seqMu.Unlock()

	c.setState(StateHandshaking)
	if err := c.sendHello(); err != nil {
		c.completeHandshake(fmt.Errorf("hello: %w", err))
		return err
	}

	conn.SetReadLimit(meshprotocol.MaxMessageSizeBytes)
	if err := conn.SetReadDeadline(time.Now().Add(meshprotocol.HelloTimeoutSeconds * time.Second)); err != nil {
		return err
	}
	_, data, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("握手确认读取失败: %w", err)
	}
	var acknowledgement protocol.Envelope
	if err := json.Unmarshal(data, &acknowledgement); err != nil || !acknowledgement.VerifyPayloadHash() || acknowledgement.Protocol != meshprotocol.ProtocolName || acknowledgement.EnvelopeVersion != meshprotocol.EnvelopeVersion || acknowledgement.PayloadSchemaVersion != 1 || acknowledgement.MessageType != protocol.MessageTypeHelloAck || acknowledgement.SpaceID != c.conf.SpaceID || acknowledgement.DeviceID != c.conf.Identity.DeviceID || acknowledgement.RuntimeID != c.conf.Identity.RuntimeID || acknowledgement.ConnectionGeneration < 1 {
		return fmt.Errorf("设备握手确认身份无效")
	}
	c.handleHelloAck(&acknowledgement)
	if c.handshakeErr != nil {
		return c.handshakeErr
	}

	c.setState(StateHelloAck)

	c.setState(StateReady)
	c.backoff.Reset()

	return c.readLoop(conn)
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
	capabilities := []string{
		protocol.WorkflowProtocolCapability(),
		protocol.WorkflowSchemaCapability(protocol.WorkflowSchemaVersion),
		protocol.ToolProtocolCapability(),
		"workflow.trigger.voice_phrase.v1",
	}
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

	env := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypeHello,
		MessageID:            uuid.New().String(),
		SpaceID:              c.conf.SpaceID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     lastSessionID,
		ConnectionGeneration: lastGen,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	return c.writeEnvelope(env)
}

func (c *MeshClient) readLoop(conn *websocket.Conn) error {
	conn.SetReadLimit(meshprotocol.MaxMessageSizeBytes)
	conn.SetPongHandler(func(_ string) error {
		conn.SetReadDeadline(time.Now().Add(meshprotocol.ReadDeadlineSeconds * time.Second))
		return nil
	})

	heartbeatTicker := time.NewTicker(meshprotocol.HeartbeatInterval * time.Second)
	defer heartbeatTicker.Stop()
	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)
	go func() {
		for {
			select {
			case <-heartbeatDone:
				return
			case <-c.stopCh:
				return
			case <-heartbeatTicker.C:
				if err := c.sendPing(); err != nil {
					_ = conn.Close()
					return
				}
			}
		}
	}()

	for {
		select {
		case <-c.stopCh:
			return nil
		default:
		}

		conn.SetReadDeadline(time.Now().Add(time.Duration(meshprotocol.ReadDeadlineSeconds) * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, 4003) {
				c.setState(StateRevoked)
				c.cancelClient()
				return nil
			}
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
		if env.Protocol != meshprotocol.ProtocolName || env.EnvelopeVersion != meshprotocol.EnvelopeVersion || env.PayloadSchemaVersion != 1 || env.SpaceID != c.conf.SpaceID || env.DeviceID != c.conf.Identity.DeviceID || env.RuntimeID != c.conf.Identity.RuntimeID || env.RuntimeSessionID != c.sessionIdentity() || env.ConnectionGeneration != c.sessionGeneration() || env.Sequence < 1 || env.MessageType == protocol.MessageTypeHelloAck {
			return fmt.Errorf("设备通道收到身份、协议或会话版本不一致的消息")
		}

		c.seqMu.Lock()
		accepted := env.Sequence > c.remoteSequence
		if accepted {
			c.remoteSequence = env.Sequence
		}
		c.seqMu.Unlock()
		if !accepted {
			continue
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
		case protocol.MessageTypeTaskLeaseAck:
			c.handleTaskLeaseAck(&env)
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
	if ack.SessionID == "" || (ack.ResumeMode != protocol.ResumeModeFresh && ack.ResumeMode != protocol.ResumeModeResume && ack.ResumeMode != protocol.ResumeModeFull) {
		c.completeHandshake(fmt.Errorf("握手会话或恢复模式无效"))
		return
	}
	if env.RuntimeSessionID != "" && env.RuntimeSessionID != ack.SessionID {
		c.completeHandshake(fmt.Errorf("握手确认中的会话身份不一致"))
		return
	}

	if env.Sequence < 1 {
		c.completeHandshake(fmt.Errorf("invalid remote sequence: %d", env.Sequence))
		return
	}
	c.seqMu.Lock()
	c.remoteSequence = int64(env.Sequence)
	c.seqMu.Unlock()

	if ack.ResumeMode == protocol.ResumeModeFull {
		c.conf.Cursor = &SessionCursor{}
		c.setState(StateDegraded)
		c.persistCursor()
		c.completeHandshake(fmt.Errorf("设备游标需要重置，将建立全新会话；不会重放旧动作"))
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
	c.sessionMu.Lock()
	c.sessionID = ack.SessionID
	c.connectionGen = env.ConnectionGeneration
	c.sessionMu.Unlock()

	c.persistCursor()
	c.completeHandshake(nil)
}

func (c *MeshClient) persistCursor() {
	store := c.credentialStore.Load()
	if store == nil || c.conf.Cursor == nil {
		return
	}
	if err := store.SaveCursor(c.conf.Cursor); err != nil {
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

	pongEnv := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypePong,
		MessageID:            env.MessageID,
		SpaceID:              c.conf.SpaceID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     c.sessionIdentity(),
		ConnectionGeneration: c.sessionGeneration(),
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

	switch errPayload.Code {
	case "mesh.credential_revoked", "mesh.credential_expired":
		c.setState(StateRevoked)
		c.cancelClient()
		if worker, ok := c.conf.TaskWorker.(interface{ CancelAllTasks() }); ok {
			worker.CancelAllTasks()
		}
		c.closeSocket(websocket.ClosePolicyViolation, "credential unavailable")
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

	env := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypePing,
		MessageID:            uuid.New().String(),
		SpaceID:              c.conf.SpaceID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     c.sessionIdentity(),
		ConnectionGeneration: c.sessionGeneration(),
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
			RuntimeSessionID:     c.sessionIdentity(),
			ConnectionGeneration: c.sessionGeneration(),
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
			code := "invoke_execution_failed"
			retryable := invoke.Handler == "coordination.data"
			if errors.Is(err, executionjournal.ErrUncertain) {
				code = "device_execution_unknown"
				retryable = false
			} else if ownedCode := coordination.ProtocolErrorCode(err); ownedCode != "" {
				code = ownedCode
				retryable = false
			}
			c.sendRuntimeError(&protocol.RuntimeErrorPayload{
				InvocationID:         invoke.InvocationID,
				RuntimeSessionID:     invoke.RuntimeSessionID,
				ConnectionGeneration: invoke.ConnectionGeneration,
				DeviceID:             c.conf.Identity.DeviceID,
				RuntimeID:            c.conf.Identity.RuntimeID,
				ErrorCode:            code,
				Message:              err.Error(),
				Retryable:            retryable,
				IdempotencyKey:       invoke.IdempotencyKey,
				FencingToken:         invoke.FencingToken,
				FailedAt:             time.Now().UTC(),
			})
			return
		}
		if result != nil {
			result.IdempotencyKey = invoke.IdempotencyKey
			result.FencingToken = invoke.FencingToken
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
	if invoke.Handler != "coordination.data" && len(invoke.OwnedExecutionScope) > 0 && c.conf.ExecutionJournal == nil {
		return nil, fmt.Errorf("设备缺少持久化工具执行记录")
	}
	if invoke.InvocationID == "" || len(invoke.InvocationID) > 512 || invoke.SpaceID != c.conf.SpaceID || invoke.DeviceID != c.conf.Identity.DeviceID || invoke.RuntimeID != c.conf.Identity.RuntimeID || invoke.RuntimeSessionID != c.sessionIdentity() || invoke.ConnectionGeneration != c.sessionGeneration() || c.State() != StateReady {
		return nil, fmt.Errorf("设备调用身份或会话已失效")
	}
	handler := c.resolveHandler(invoke.Handler)
	if handler == nil {
		return nil, fmt.Errorf("unsupported handler: %s", invoke.Handler)
	}
	run := func() (*protocol.RuntimeResultPayload, error) {
		if c.State() != StateReady || invoke.RuntimeSessionID != c.sessionIdentity() || invoke.ConnectionGeneration != c.sessionGeneration() {
			return nil, fmt.Errorf("设备调用执行前会话已失效")
		}
		return handler(invoke)
	}
	var result *protocol.RuntimeResultPayload
	var err error
	execute := func(current context.Context) (*protocol.RuntimeResultPayload, error) {
		if deadline, ok := current.Deadline(); ok {
			remaining := time.Until(deadline).Milliseconds()
			if remaining <= 0 {
				return nil, context.Cause(current)
			}
			if invoke.DeadlineMs <= 0 || remaining < invoke.DeadlineMs {
				invoke.DeadlineMs = remaining
			}
		}
		stop := context.AfterFunc(current, func() {
			if dispatcher, ok := c.conf.RuntimeDispatcher.(RuntimeCancelDispatcher); ok {
				dispatcher.CancelInvocation(invoke.InvocationID)
			}
		})
		defer stop()
		if c.conf.ExecutionJournal != nil && invoke.Handler != "coordination.data" {
			return c.conf.ExecutionJournal.Execute(current, invoke, run)
		}
		return run()
	}
	if c.conf.ExecutionGuard != nil && invoke.Handler != "coordination.data" {
		result, err = c.conf.ExecutionGuard(c.clientCtx, invoke, execute)
	} else if len(invoke.OwnedExecutionScope) != 0 || invoke.AuthorityCallID != "" {
		if invoke.Handler != "coordination.data" {
			return nil, fmt.Errorf("设备缺少工具授权校验端口")
		}
		result, err = execute(c.clientCtx)
	} else {
		result, err = execute(c.clientCtx)
	}
	if err != nil {
		return nil, err
	}
	if c.State() != StateReady || invoke.RuntimeSessionID != c.sessionIdentity() || invoke.ConnectionGeneration != c.sessionGeneration() {
		return nil, fmt.Errorf("设备调用结束时会话已失效")
	}
	if result != nil {
		copyResult := *result
		result = &copyResult
		result.InvocationID = invoke.InvocationID
		result.DeviceID = invoke.DeviceID
		result.RuntimeID = invoke.RuntimeID
		result.RuntimeSessionID = invoke.RuntimeSessionID
		result.ConnectionGeneration = invoke.ConnectionGeneration
	}
	return result, nil
}

func (c *MeshClient) resolveHandler(handlerName string) RuntimeInvokeHandler {
	return c.conf.RuntimeDispatcher.Resolve(handlerName)
}

func (c *MeshClient) sendRuntimeResult(result *protocol.RuntimeResultPayload) {
	payloadBytes, err := json.Marshal(result)
	if err != nil {
		return
	}

	env := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypeRuntimeResult,
		MessageID:            uuid.New().String(),
		SpaceID:              c.conf.SpaceID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     result.RuntimeSessionID,
		ConnectionGeneration: result.ConnectionGeneration,
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

	env := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypeRuntimeError,
		MessageID:            uuid.New().String(),
		SpaceID:              c.conf.SpaceID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     errPayload.RuntimeSessionID,
		ConnectionGeneration: errPayload.ConnectionGeneration,
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

	if dispatch.DeviceID != c.conf.Identity.DeviceID || dispatch.RuntimeID != c.conf.Identity.RuntimeID || dispatch.RuntimeSessionID != env.RuntimeSessionID || dispatch.ConnectionGeneration != env.ConnectionGeneration || dispatch.TaskRunID == "" || dispatch.AttemptID == "" || dispatch.LeaseID == "" {
		c.sendTaskError(env.MessageID, dispatch.TaskRunID, "task_identity_mismatch", "任务身份或租约无效")
		return
	}
	messageID := env.MessageID
	go func() {
		if err := c.conf.TaskWorker.ExecuteTask(c.clientCtx, dispatch); err != nil {
			c.sendTaskError(messageID, dispatch.TaskRunID, "task_execution_failed", err.Error())
		}
	}()
}

func (c *MeshClient) handleTaskCancel(env *protocol.Envelope) {
	var cancel protocol.TaskCancelPayload
	if err := json.Unmarshal(env.Payload, &cancel); err != nil {
		c.sendTaskError(env.MessageID, cancel.TaskRunID, "invalid_cancel_payload", err.Error())
		return
	}
	if cancel.RuntimeSessionID != env.RuntimeSessionID || cancel.ConnectionGeneration != env.ConnectionGeneration {
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

func (c *MeshClient) taskSource(source []protocol.TaskDispatchPayload) (runtimeidentity.RuntimeSessionID, int64) {
	if len(source) > 0 {
		return source[0].RuntimeSessionID, source[0].ConnectionGeneration
	}
	c.sessionMu.RLock()
	defer c.sessionMu.RUnlock()
	return c.sessionID, c.connectionGen
}

func (c *MeshClient) sendTaskClaim(taskRunID, attemptID, leaseID, workerID string, leaseDurationMs int64, source ...protocol.TaskDispatchPayload) {
	session, generation := c.taskSource(source)
	claim := protocol.TaskClaimPayload{
		TaskRunID:            taskRunID,
		AttemptID:            attemptID,
		LeaseID:              leaseID,
		WorkerID:             workerID,
		LeaseDurationMs:      leaseDurationMs,
		RuntimeSessionID:     session,
		ConnectionGeneration: generation,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		ClaimedAt:            time.Now().UTC(),
	}
	c.sendTaskEnvelope(protocol.MessageTypeTaskClaim, claim)
}

func (c *MeshClient) sendTaskComplete(taskRunID, attemptID, leaseID string, success bool, result json.RawMessage, errMsg string, source ...protocol.TaskDispatchPayload) {
	session, generation := c.taskSource(source)
	complete := protocol.TaskCompletePayload{
		TaskRunID:            taskRunID,
		AttemptID:            attemptID,
		LeaseID:              leaseID,
		Success:              success,
		Result:               result,
		Error:                errMsg,
		RuntimeSessionID:     session,
		ConnectionGeneration: generation,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		CompletedAt:          time.Now().UTC(),
	}
	c.sendTaskEnvelope(protocol.MessageTypeTaskComplete, complete)
}

func (c *MeshClient) sendTaskProgress(taskRunID, attemptID, leaseID string, seq int64, current, total, percentage *float64, stage, message string, source ...protocol.TaskDispatchPayload) {
	session, generation := c.taskSource(source)
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
		RuntimeSessionID:     session,
		ConnectionGeneration: generation,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		ReportedAt:           time.Now().UTC(),
	}
	c.sendTaskEnvelope(protocol.MessageTypeTaskProgress, progress)
}

func (c *MeshClient) sendOwnedTaskComplete(dispatch protocol.TaskDispatchPayload, outcome protocol.OwnedTaskExecutionOutcome) {
	c.sendTaskEnvelope(protocol.MessageTypeTaskComplete, protocol.TaskCompletePayload{TaskRunID: dispatch.TaskRunID, AttemptID: dispatch.AttemptID, LeaseID: dispatch.LeaseID, Success: true, Result: outcome.Result, ResultArtifactID: outcome.ResultArtifactID, RuntimeSessionID: dispatch.RuntimeSessionID, ConnectionGeneration: dispatch.ConnectionGeneration, DeviceID: c.conf.Identity.DeviceID, RuntimeID: c.conf.Identity.RuntimeID, CompletedAt: time.Now().UTC()})
}

func (c *MeshClient) sendOwnedTaskUnknown(dispatch protocol.TaskDispatchPayload) {
	c.sendTaskEnvelope(protocol.MessageTypeTaskComplete, protocol.TaskCompletePayload{TaskRunID: dispatch.TaskRunID, AttemptID: dispatch.AttemptID, LeaseID: dispatch.LeaseID, OutcomeUnknown: true, RuntimeSessionID: dispatch.RuntimeSessionID, ConnectionGeneration: dispatch.ConnectionGeneration, DeviceID: c.conf.Identity.DeviceID, RuntimeID: c.conf.Identity.RuntimeID, CompletedAt: time.Now().UTC()})
}

func (c *MeshClient) sendTaskHeartbeat(taskRunID, attemptID, leaseID string, seq int64, source ...protocol.TaskDispatchPayload) {
	session, generation := c.taskSource(source)
	heartbeat := protocol.TaskHeartbeatPayload{
		TaskRunID:            taskRunID,
		AttemptID:            attemptID,
		LeaseID:              leaseID,
		Sequence:             seq,
		RuntimeSessionID:     session,
		ConnectionGeneration: generation,
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
		RuntimeSessionID:     c.sessionIdentity(),
		ConnectionGeneration: c.sessionGeneration(),
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		CheckpointAt:         time.Now().UTC(),
	}
	c.sendTaskEnvelope(protocol.MessageTypeTaskCheckpoint, checkpoint)
}

func (c *MeshClient) sendTaskEnvelope(msgType protocol.MessageType, payload interface{}) {
	if c.State() != StateReady {
		return
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return
	}
	var session struct {
		RuntimeSessionID     runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
		ConnectionGeneration int64                            `json:"connectionGeneration"`
	}
	if json.Unmarshal(payloadBytes, &session) != nil || session.RuntimeSessionID == "" || session.ConnectionGeneration < 1 {
		return
	}

	env := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          msgType,
		MessageID:            uuid.New().String(),
		SpaceID:              c.conf.SpaceID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     session.RuntimeSessionID,
		ConnectionGeneration: session.ConnectionGeneration,
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
		SpaceID:              c.conf.SpaceID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     c.sessionIdentity(),
		ConnectionGeneration: c.sessionGeneration(),
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
	c.seqMu.Lock()
	localSequence, remoteSequence := c.localSequence, c.remoteSequence
	c.seqMu.Unlock()
	result := map[string]interface{}{
		"state":            string(c.state.Get()),
		"connectionGen":    c.sessionGeneration(),
		"localSequence":    localSequence,
		"remoteSequence":   remoteSequence,
		"runtimeSessionId": c.sessionIdentity().String(),
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
		RuntimeSessionID: c.sessionIdentity(),
		ReceivedAt:       time.Now().UTC(),
	}

	resultBytes, _ := json.Marshal(result.Result)
	ack.PayloadHash = protocol.ComputePayloadHash(resultBytes)

	env := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypeCommandAck,
		MessageID:            uuid.New().String(),
		SpaceID:              c.conf.SpaceID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     c.sessionIdentity(),
		ConnectionGeneration: c.sessionGeneration(),
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
		SpaceID:              c.conf.SpaceID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     c.sessionIdentity(),
		ConnectionGeneration: c.sessionGeneration(),
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
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	if env.MessageType != protocol.MessageTypeHello {
		c.sessionMu.RLock()
		current := env.RuntimeSessionID == c.sessionID && env.ConnectionGeneration == c.connectionGen
		c.sessionMu.RUnlock()
		if !current {
			return fmt.Errorf("执行会话已失效，迟到消息已拦截")
		}
	}
	c.seqMu.Lock()
	env.Sequence = c.localSequence + 1
	data, err := json.Marshal(env)
	if err != nil {
		c.seqMu.Unlock()
		return err
	}
	c.localSequence = env.Sequence
	c.seqMu.Unlock()
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
	c.sessionMu.Lock()
	c.connectionGen = gen
	c.sessionMu.Unlock()
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

package agent

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	protocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
	meshprotocol "github.com/u-ai/backend/internal/devicemesh/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type MeshClientConfig struct {
	CloudBaseURL string
	Credential   string
	UserID       runtimeidentity.UserID
	Identity     *LocalIdentity
	Cursor       *SessionCursor
	OnState      func(AgentState)
}

type MeshClient struct {
	conf     MeshClientConfig
	dialer   *websocket.Dialer
	mu       sync.Mutex
	conn     *websocket.Conn
	state    *ConnectionManager
	stopCh   chan struct{}
	backoff  *Backoff

	// R15: handshake synchronization
	handshakeOnce  sync.Once
	handshakeDone  chan struct{}
	handshakeErr   error

	// R17: bidirectional monotonic sequences
	localSequence  int64
	remoteSequence int64

	// R16/R18: session state after HelloAck
	sessionID      runtimeidentity.RuntimeSessionID
	connectionGen  int64
}

func NewMeshClient(conf MeshClientConfig) *MeshClient {
	return &MeshClient{
		conf:  conf,
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

	// R18: deferred generation-aware close
	defer func() {
		gen := c.connectionGen
		c.closeSocketWithGen(websocket.CloseGoingAway, "", gen)
	}()

	// R15: Reset handshake channel for this connection
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

	// R15: Wait for HelloAck before becoming Ready
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
		RuntimeCapabilities:          []string{},
		LastAppliedStateRevision:     lastAppliedStateRev,
		LastProcessedCommandSequence: lastProcessedCmdSeq,
		LastEventSequence:            lastEventSeq,
		ActualStateHash:              actualStateHash,
	}

	payloadBytes, err := json.Marshal(hello)
	if err != nil {
		return err
	}

	// R17: First outbound message uses sequence 1
	c.localSequence = 1

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
		Sequence:             int64(c.localSequence),
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	return c.writeEnvelope(env)
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

		// R17: Validate bidirectional monotonic sequence (except HelloAck which sets initial)
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
		case protocol.MessageTypeCommand:
			c.sendUnsupportedCommand(&env)
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

	// R17: Validate remote sequence (first message should be sequence 1)
	if env.Sequence < 1 {
		c.completeHandshake(fmt.Errorf("invalid remote sequence: %d", env.Sequence))
		return
	}
	c.remoteSequence = int64(env.Sequence)

	if ack.ResumeMode == protocol.ResumeModeFull {
		// R16: Full reset creates fresh cursor
		c.conf.Cursor = &SessionCursor{
			ConnectionGeneration: env.ConnectionGeneration,
			RuntimeSessionID:     ack.SessionID,
		}
		c.sessionID = ack.SessionID
		c.connectionGen = env.ConnectionGeneration
		c.setState(StateDegraded)
		c.completeHandshake(nil)
		return
	}

	// R16: First Ack creates cursor if nil, always updates SessionID
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

	c.completeHandshake(nil)
}

func (c *MeshClient) handlePing(env *protocol.Envelope) {
	var ping protocol.PingPayload
	if err := json.Unmarshal(env.Payload, &ping); err != nil {
		return
	}

	pong := protocol.PongPayload{Time: ping.Time}
	payloadBytes, _ := json.Marshal(pong)

	// R17: Pong carries monotonic sequence
	c.localSequence++

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
		Sequence:             c.localSequence,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	_ = c.writeEnvelope(pongEnv)
}

func (c *MeshClient) handleError(env *protocol.Envelope) {
	var errPayload protocol.ErrorPayload
	if err := json.Unmarshal(env.Payload, &errPayload); err != nil {
		return
	}

	// R17: Validate remote sequence on error too
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
		// R18: generation-aware close
		c.closeSocketGen(websocket.CloseNormalClosure, "superseded", env.ConnectionGeneration)
	case "mesh.cursor_reset_required":
		c.conf.Cursor = &SessionCursor{}
		c.setState(StateDegraded)
	}
}

func (c *MeshClient) sendPing() error {
	ping := protocol.PingPayload{Time: time.Now().UTC()}
	payloadBytes, _ := json.Marshal(ping)

		// R17: Increment local sequence for each outbound message
	c.localSequence++

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
		Sequence:             c.localSequence,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	return c.writeEnvelope(env)
}

func (c *MeshClient) sendUnsupportedCommand(env *protocol.Envelope) {
	// R22: Parse and handle command, then send ack
	var cmd protocol.CommandPayload
	if err := json.Unmarshal(env.Payload, &cmd); err != nil {
		c.sendCommandReject(env, "invalid_command", "failed to parse command")
		return
	}

	// R22: Execute the command based on its type
	result, err := c.executeCommand(cmd)
	if err != nil {
		c.sendCommandReject(env, "command_failed", err.Error())
		return
	}

	// R22: Send positive ack with execution result
	c.sendCommandAck(&cmd, result)
}

// R22: executeCommand dispatches command execution
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

// R22: execStatusCommand returns runtime status information
func (c *MeshClient) execStatusCommand(cmd protocol.CommandPayload) (*CommandResult, error) {
	result := map[string]interface{}{
		"state":             string(c.state.Get()),
		"connectionGen":     c.connectionGen,
		"localSequence":     c.localSequence,
		"remoteSequence":    c.remoteSequence,
		"runtimeSessionId":  c.sessionID.String(),
		"deviceId":          c.conf.Identity.DeviceID.String(),
		"runtimeId":         c.conf.Identity.RuntimeID.String(),
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

// R22: sendCommandAck sends a positive command acknowledgment
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

	// R17: Increment sequence for outbound message
	c.localSequence++

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
		Sequence:             c.localSequence,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(mustMarshal(ack)),
		SentAt:               time.Now().UTC(),
		Payload:              mustMarshal(ack),
	}

	_ = c.writeEnvelope(env)
}

// R22: sendCommandReject sends a negative command acknowledgment
func (c *MeshClient) sendCommandReject(env *protocol.Envelope, code, reason string) {
	// R17: Increment sequence for outbound message
	c.localSequence++

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
		Sequence:             c.localSequence,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	_ = c.writeEnvelope(resp)
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

// R18: closeSocketWithGen performs generation-aware close on connection teardown
func (c *MeshClient) closeSocketWithGen(code int, reason string, gen int64) {
	c.connectionGen = gen
	c.closeSocketGen(code, reason, gen)
}

// R18: closeSocketGen sends close frame with generation context
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

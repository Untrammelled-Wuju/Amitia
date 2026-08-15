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
}

func NewMeshClient(conf MeshClientConfig) *MeshClient {
	return &MeshClient{
		conf:  conf,
		dialer: &websocket.Dialer{
			HandshakeTimeout: meshprotocol.HelloTimeoutSeconds * time.Second,
			TLSClientConfig:  &tls.Config{},
			Proxy:            http.ProxyFromEnvironment,
		},
		state:  NewConnectionManager(),
		stopCh: make(chan struct{}),
		backoff: NewBackoff(),
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

	defer c.closeSocket(websocket.CloseGoingAway, "")

	c.setState(StateHandshaking)
	if err := c.sendHello(); err != nil {
		return fmt.Errorf("hello: %w", err)
	}

	c.setState(StateReady)
	c.backoff.Reset()

	return c.readLoop()
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
		Sequence:             0,
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
		return
	}

	if !ack.Accepted {
		return
	}

	if ack.ResumeMode == protocol.ResumeModeFull {
		c.conf.Cursor = &SessionCursor{}
		c.setState(StateDegraded)
		return
	}

	if c.conf.Cursor != nil && ack.SessionID != "" {
		c.conf.Cursor.RuntimeSessionID = ack.SessionID
		c.conf.Cursor.ConnectionGeneration = env.ConnectionGeneration
	}
}

func (c *MeshClient) handlePing(env *protocol.Envelope) {
	var ping protocol.PingPayload
	if err := json.Unmarshal(env.Payload, &ping); err != nil {
		return
	}

	pong := protocol.PongPayload{Time: ping.Time}
	payloadBytes, _ := json.Marshal(pong)

	pongEnv := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypePong,
		MessageID:            env.MessageID,
		UserID:               c.conf.UserID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		RuntimeSessionID:     env.RuntimeSessionID,
		ConnectionGeneration: env.ConnectionGeneration,
		Sequence:             0,
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

	switch errPayload.Code {
	case "mesh.credential_revoked", "mesh.credential_expired":
		c.setState(StateRevoked)
	case "mesh.session_superseded":
		c.closeSocket(websocket.CloseNormalClosure, "superseded")
	case "mesh.cursor_reset_required":
		c.conf.Cursor = &SessionCursor{}
		c.setState(StateDegraded)
	}
}

func (c *MeshClient) sendPing() error {
	ping := protocol.PingPayload{Time: time.Now().UTC()}
	payloadBytes, _ := json.Marshal(ping)

	env := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypePing,
		MessageID:            uuid.New().String(),
		UserID:               c.conf.UserID,
		DeviceID:             c.conf.Identity.DeviceID,
		RuntimeID:            c.conf.Identity.RuntimeID,
		ConnectionGeneration: 1,
		Sequence:             0,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	return c.writeEnvelope(env)
}

func (c *MeshClient) sendUnsupportedCommand(env *protocol.Envelope) {
	errPayload := protocol.ErrorPayload{
		Code:    "mesh.unsupported_command",
		Message: "commands not supported in G21",
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
		RuntimeSessionID:     env.RuntimeSessionID,
		ConnectionGeneration: env.ConnectionGeneration,
		Sequence:             0,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	_ = c.writeEnvelope(resp)
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

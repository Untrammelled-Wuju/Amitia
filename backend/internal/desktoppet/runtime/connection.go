// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/log"
)

type SessionState string

const (
	SessionStateRegistering SessionState = "registering"
	SessionStateSyncing     SessionState = "syncing"
	SessionStateReady       SessionState = "ready"
	SessionStateDegraded    SessionState = "degraded"
	SessionStateClosing     SessionState = "closing"
	SessionStateClosed      SessionState = "closed"
)

type CapabilitySet map[string]bool

func (c CapabilitySet) Has(cap string) bool {
	return c[cap]
}

func (c CapabilitySet) ToList() []string {
	list := make([]string, 0, len(c))
	for k := range c {
		list = append(list, k)
	}
	return list
}

func (c CapabilitySet) Clone() CapabilitySet {
	clone := make(CapabilitySet, len(c))
	for k, v := range c {
		clone[k] = v
	}
	return clone
}

type outboundFrame struct {
	msg      contracts.RuntimeMessage
	priority int
}

const (
	closeHeartbeatTimeout = 4001
	closeReadError        = 4002
	closeWriteError       = 4003
	closeRegisterFailed   = 4004
)

var globalMessageSeq atomic.Uint64

type Connection struct {
	sessionID      string
	runtimeID      string
	deviceID       string
	userID         string
	ticketDeviceID  string
	ticketRuntimeID string
	conn           *websocket.Conn
	sendCh         chan outboundFrame
	done           chan struct{}
	closeOnce      sync.Once
	seq            atomic.Uint64
	lastBeatNS     atomic.Int64
	state          atomic.Value
	capabilities   CapabilitySet
	config         *DesktopPetRuntimeConfig
	onResult       func(msg *contracts.RuntimeMessage, payload *contracts.ResultPayload)
	onEvent        func(msg *contracts.RuntimeMessage, payload *contracts.EventPayload)
	onHeartbeat    func(runtimeID, sessionID string, payload *contracts.HeartbeatPayload)
	onClose        func(sessionID, runtimeID string, code int, reason string)
	onRegister     func(conn *Connection, payload *contracts.RegisterPayload) (*contracts.WelcomePayload, error)
}

func NewConnection(wsConn *websocket.Conn, config *DesktopPetRuntimeConfig) *Connection {
	c := &Connection{
		conn:         wsConn,
		sendCh:       make(chan outboundFrame, config.SendQueueSize),
		done:         make(chan struct{}),
		capabilities: make(CapabilitySet),
		config:       config,
	}
	c.lastBeatNS.Store(time.Now().UnixNano())
	c.state.Store(SessionStateRegistering)
	return c
}

func (c *Connection) Start(ctx context.Context) {
	go c.readerLoop()
	go c.writerLoop()
	go c.supervisorLoop(ctx)
}

func (c *Connection) readerLoop() {
	defer c.Close(closeReadError, "reader loop exited")
	c.conn.SetReadLimit(int64(c.config.MaxMessageBytes))
	_ = c.conn.SetReadDeadline(time.Now().Add(c.config.HeartbeatTimeout()))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.config.HeartbeatTimeout()))
		return nil
	})
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			c.Close(closeReadError, err.Error())
			return
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(c.config.HeartbeatTimeout()))
		var msg contracts.RuntimeMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Logger.Warnf("runtime conn: decode message failed sessionID=%s runtimeID=%s err=%v", c.sessionID, c.runtimeID, err)
			continue
		}
		c.routeMessage(&msg)
	}
}

func (c *Connection) routeMessage(msg *contracts.RuntimeMessage) {
	if msg.RuntimeID != "" && msg.RuntimeID != c.runtimeID {
		log.Logger.Warnf("runtime conn: runtimeID mismatch sessionID=%s connRuntimeID=%s msgRuntimeID=%s", c.sessionID, c.runtimeID, msg.RuntimeID)
		c.Close(4005, "runtimeID mismatch")
		return
	}
	if msg.SessionID != "" && c.sessionID != "" && msg.SessionID != c.sessionID {
		log.Logger.Warnf("runtime conn: sessionID mismatch connSessionID=%s msgSessionID=%s", c.sessionID, msg.SessionID)
		c.Close(4006, "sessionID mismatch")
		return
	}
	switch msg.Kind {
	case contracts.KindResult:
		var payload contracts.ResultPayload
		if len(msg.Payload) > 0 {
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				log.Logger.Warnf("runtime conn: decode result payload failed sessionID=%s err=%v", c.sessionID, err)
				return
			}
		}
		if c.onResult != nil {
			c.onResult(msg, &payload)
		}
	case contracts.KindEvent:
		var payload contracts.EventPayload
		if len(msg.Payload) > 0 {
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				log.Logger.Warnf("runtime conn: decode event payload failed sessionID=%s err=%v", c.sessionID, err)
				return
			}
		}
		if c.onEvent != nil {
			c.onEvent(msg, &payload)
		}
	case contracts.KindControl:
		c.routeControl(msg)
	default:
		log.Logger.Warnf("runtime conn: unknown message kind sessionID=%s kind=%s", c.sessionID, msg.Kind)
	}
}

func (c *Connection) routeControl(msg *contracts.RuntimeMessage) {
	switch msg.Name {
	case contracts.MsgRuntimeHeartbeat:
		var payload contracts.HeartbeatPayload
		if len(msg.Payload) > 0 {
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				log.Logger.Warnf("runtime conn: decode heartbeat payload failed sessionID=%s err=%v", c.sessionID, err)
				return
			}
		}
		c.lastBeatNS.Store(time.Now().UnixNano())
		if c.onHeartbeat != nil {
			c.onHeartbeat(c.runtimeID, c.sessionID, &payload)
		}
	case contracts.MsgRuntimeRegister:
		var payload contracts.RegisterPayload
		if len(msg.Payload) > 0 {
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				log.Logger.Warnf("runtime conn: decode register payload failed err=%v", err)
				c.Close(closeRegisterFailed, "invalid register payload")
				return
			}
		}
		if c.onRegister != nil {
			welcome, err := c.onRegister(c, &payload)
			if err != nil {
				log.Logger.Warnf("runtime conn: register rejected err=%v", err)
				c.Close(closeRegisterFailed, err.Error())
				return
			}
			welcomeMsg, berr := buildMessage(contracts.KindControl, contracts.MsgRuntimeWelcome, c.runtimeID, c.sessionID, welcome)
			if berr != nil {
				log.Logger.Errorf("runtime conn: build welcome failed err=%v", berr)
				c.Close(closeRegisterFailed, "build welcome failed")
				return
			}
			if serr := c.Send(welcomeMsg); serr != nil {
				log.Logger.Warnf("runtime conn: send welcome failed err=%v", serr)
				c.Close(closeRegisterFailed, "send welcome failed")
				return
			}
		}
	default:
		log.Logger.Infof("runtime conn: unhandled control message sessionID=%s name=%s", c.sessionID, msg.Name)
	}
}

func (c *Connection) writerLoop() {
	for {
		select {
		case <-c.done:
			return
		case frame, ok := <-c.sendCh:
			if !ok {
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.config.HeartbeatInterval()))
			if err := c.conn.WriteJSON(frame.msg); err != nil {
				c.Close(closeWriteError, err.Error())
				return
			}
		}
	}
}

func (c *Connection) supervisorLoop(ctx context.Context) {
	ticker := time.NewTicker(c.config.HeartbeatInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			c.Close(websocket.CloseGoingAway, "context cancelled")
			return
		case <-c.done:
			return
		case <-ticker.C:
			last := time.Unix(0, c.lastBeatNS.Load())
			if time.Since(last) > c.config.HeartbeatTimeout() {
				log.Logger.Warnf("runtime conn: heartbeat timeout sessionID=%s runtimeID=%s", c.sessionID, c.runtimeID)
				c.Close(closeHeartbeatTimeout, "heartbeat timeout")
				return
			}
		}
	}
}

func (c *Connection) Send(msg contracts.RuntimeMessage) (err error) {
	select {
	case <-c.done:
		return ErrRuntimeDisconnected
	default:
	}
	defer func() {
		if r := recover(); r != nil {
			err = ErrRuntimeDisconnected
		}
	}()
	select {
	case c.sendCh <- outboundFrame{msg: msg, priority: 0}:
		return nil
	default:
		return ErrRuntimeBusy
	}
}

func (c *Connection) SendBlocking(msg contracts.RuntimeMessage, timeout time.Duration) (err error) {
	select {
	case <-c.done:
		return ErrRuntimeDisconnected
	default:
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	defer func() {
		if r := recover(); r != nil {
			err = ErrRuntimeDisconnected
		}
	}()
	select {
	case c.sendCh <- outboundFrame{msg: msg, priority: 0}:
		return nil
	case <-timer.C:
		return ErrRuntimeBusy
	case <-c.done:
		return ErrRuntimeDisconnected
	}
}

func (c *Connection) Close(code int, reason string) {
	c.closeOnce.Do(func() {
		c.SetState(SessionStateClosing)
		close(c.done)
		close(c.sendCh)
		_ = c.conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(code, reason),
			time.Now().Add(5*time.Second),
		)
		_ = c.conn.Close()
		c.SetState(SessionStateClosed)
		if c.onClose != nil {
			c.onClose(c.sessionID, c.runtimeID, code, reason)
		}
	})
}

func (c *Connection) IsClosed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func (c *Connection) SessionID() string {
	return c.sessionID
}

func (c *Connection) RuntimeID() string {
	return c.runtimeID
}

func (c *Connection) UserID() string {
	return c.userID
}

func (c *Connection) State() SessionState {
	v := c.state.Load()
	if v == nil {
		return SessionStateRegistering
	}
	return v.(SessionState)
}

func (c *Connection) SetState(s SessionState) {
	c.state.Store(s)
}

func (c *Connection) Capabilities() CapabilitySet {
	return c.capabilities
}

func (c *Connection) SetCapabilities(caps []string) {
	c.capabilities = make(CapabilitySet, len(caps))
	for _, cap := range caps {
		c.capabilities[cap] = true
	}
}

func (c *Connection) HasCapability(cap string) bool {
	return c.capabilities.Has(cap)
}

func (c *Connection) LastHeartbeat() time.Time {
	return time.Unix(0, c.lastBeatNS.Load())
}

func (c *Connection) UpdateHeartbeat() {
	c.lastBeatNS.Store(time.Now().UnixNano())
}

func negotiateProtocol(clientMin, clientMax string) (string, error) {
	if compareVersions(clientMin, contracts.ProtocolMax) > 0 {
		return "", NewRuntimeError(ErrCodeRuntimeProtocolIncompatible, "client protocol min "+clientMin+" exceeds server max "+contracts.ProtocolMax, ErrRuntimeProtocolIncompatible)
	}
	if compareVersions(clientMax, contracts.ProtocolMin) < 0 {
		return "", NewRuntimeError(ErrCodeRuntimeProtocolIncompatible, "client protocol max "+clientMax+" below server min "+contracts.ProtocolMin, ErrRuntimeProtocolIncompatible)
	}
	selected := contracts.ProtocolMax
	if compareVersions(clientMax, contracts.ProtocolMax) < 0 {
		selected = clientMax
	}
	return selected, nil
}

func buildMessage(kind contracts.MessageKind, name string, runtimeID, sessionID string, payload interface{}) (contracts.RuntimeMessage, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return contracts.RuntimeMessage{}, err
	}
	return contracts.RuntimeMessage{
		SchemaVersion:   contracts.SchemaVersion,
		ProtocolVersion: contracts.ProtocolMax,
		Kind:            kind,
		Name:            name,
		MessageID:       nextSequence(),
		RuntimeID:       runtimeID,
		SessionID:       sessionID,
		Sequence:        globalMessageSeq.Add(1),
		SentAt:          time.Now(),
		Payload:         raw,
	}, nil
}

func nextSequence() string {
	return uuid.NewString()
}

func compareVersions(a, b string) int {
	av := parseVersionParts(a)
	bv := parseVersionParts(b)
	n := len(av)
	if len(bv) < n {
		n = len(bv)
	}
	for i := 0; i < n; i++ {
		if av[i] != bv[i] {
			return av[i] - bv[i]
		}
	}
	return len(av) - len(bv)
}

func parseVersionParts(v string) []int {
	parts := strings.Split(v, ".")
	result := make([]int, len(parts))
	for i, p := range parts {
		n, _ := strconv.Atoi(strings.TrimSpace(p))
		result[i] = n
	}
	return result
}

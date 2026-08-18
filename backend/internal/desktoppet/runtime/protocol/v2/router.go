package v2

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/u-ai/backend/internal/desktoppet/readiness"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/middleware"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/log"
)

type BootstrapTicketConsumer func(
	ctx context.Context,
	rawTicket string,
	runtimeID runtimeidentity.RuntimeID,
	deviceID runtimeidentity.DeviceID,
) (
	userID runtimeidentity.UserID,
	err error,
)

func RegisterInternalRoutes(
	r *gin.Engine,
	facade *RuntimeFacade,
	safeMode *readiness.SafeModeController,
	consumeTicket BootstrapTicketConsumer,
) {
	if facade == nil {
		return
	}

	path := facade.Config().Path
	if path == "" {
		path = "/internal/desktop-pet/runtime/ws"
	}

	r.GET(path, internalOriginMiddleware(facade, safeMode), gin.WrapH(&v2WSHandler{facade: facade, consumeTicket: consumeTicket}))
}

func RegisterUserRoutes(apiGroup *gin.RouterGroup, facade *RuntimeFacade) {
	if facade == nil {
		return
	}

	runtimeGroup := apiGroup.Group("/desktop-pets/runtime")

	runtimeGroup.GET("/status", func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": gin.H{"code": "UNAUTHORIZED", "message": "认证失败"}})
			return
		}
		conns := facade.ListConnections(userID)
		views := make([]gin.H, 0, len(conns))
		for _, conn := range conns {
			if conn == nil {
				continue
			}
			views = append(views, gin.H{
				"runtimeId": conn.RuntimeID,
				"deviceId":  conn.DeviceID,
				"state":     conn.State,
				"sessionId": conn.SessionIDValue(),
				"lastSeq":   conn.LastInboundSequence(),
				"lastBeat":  conn.LastHeartbeat().Format(time.RFC3339),
			})
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": views})
	})

	runtimeGroup.GET("/status/:runtimeId", func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		runtimeID := c.Param("runtimeId")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": gin.H{"code": "UNAUTHORIZED", "message": "认证失败"}})
			return
		}
		conn := facade.GetConnection(userID, "", runtimeID)
		if conn == nil {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
				"runtimeId": runtimeID,
				"state":     "offline",
			}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"runtimeId": conn.RuntimeID,
			"deviceId":  conn.DeviceID,
			"state":     conn.State,
			"sessionId": conn.SessionIDValue(),
			"lastSeq":   conn.LastInboundSequence(),
			"lastBeat":  conn.LastHeartbeat().Format(time.RFC3339),
		}})
	})

	runtimeGroup.GET("/metrics", func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": gin.H{"code": "UNAUTHORIZED", "message": "认证失败"}})
			return
		}
		actor, err := middleware.GetActorFromContext(c)
		if err != nil || !actor.HasRole("admin") {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": gin.H{"code": "FORBIDDEN", "message": "需要管理员权限"}})
			return
		}
		facade.handler.mu.RLock()
		connCount := len(facade.handler.connections)
		facade.handler.mu.RUnlock()
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"facadeStarted": facade.IsStarted(),
			"connections":   connCount,
		}})
	})
}

func internalOriginMiddleware(facade *RuntimeFacade, safeMode *readiness.SafeModeController) gin.HandlerFunc {
	return func(c *gin.Context) {
		if facade == nil || facade.Config() == nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}

		if safeMode != nil {
			if active, reason, _ := safeMode.IsInSafeMode(); active {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
					"code":   503,
					"msg":    "desktop pet safe mode",
					"reason": reason,
				})
				return
			}
		}

		if !facade.IsStarted() {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"code": 503,
				"msg":  "runtime v2 not started",
			})
			return
		}

		cfg := facade.Config()
		if cfg.LoopbackOnly {
			host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
			if err != nil {
				host = c.Request.RemoteAddr
			}
			ip := net.ParseIP(host)
			if ip == nil || !ip.IsLoopback() {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}

		c.Next()
	}
}

type v2WSHandler struct {
	facade        *RuntimeFacade
	consumeTicket BootstrapTicketConsumer
	upgrader      websocket.Upgrader
}

func (h *v2WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.facade == nil || !h.facade.IsStarted() {
		http.Error(w, "runtime v2 not started", http.StatusServiceUnavailable)
		return
	}

	rawTicket := strings.TrimSpace(r.URL.Query().Get("ticket"))
	deviceID := runtimeidentity.ParseDeviceID(r.URL.Query().Get("deviceId"))
	runtimeID := runtimeidentity.ParseRuntimeID(r.URL.Query().Get("runtimeId"))

	if rawTicket == "" || deviceID == "" || runtimeID == "" {
		http.Error(w, "runtime bootstrap credentials required", http.StatusUnauthorized)
		return
	}

	if h.consumeTicket == nil {
		http.Error(w, "runtime ticket validator unavailable", http.StatusServiceUnavailable)
		return
	}

	userID, err := h.consumeTicket(r.Context(), rawTicket, runtimeID, deviceID)
	if err != nil || userID == "" {
		http.Error(w, "runtime bootstrap ticket rejected", http.StatusUnauthorized)
		return
	}

	wsConn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Warn("[v2-ws] upgrade failed: ", err)
		return
	}

	v2Handler := h.facade.Handler()

	conn, err := v2Handler.HandleConnect(userID, deviceID, runtimeID)
	if err != nil {
		wsConn.Close()
		return
	}

	ctx := &wsConnContext{
		wsConn:  wsConn,
		conn:    conn,
		handler: v2Handler,
		sendCh:  make(chan []byte, 64),
		doneCh:  make(chan struct{}),
	}

	go ctx.writeLoop()

	dispatchCtx, dispatchCancel := context.WithCancel(r.Context())
	defer dispatchCancel()

	dispatcher := NewConnectionCommandDispatcher(h.facade.Commands(), v2Handler)
	go dispatcher.Run(dispatchCtx, conn, ctx.SendEnvelope)

	ctx.readLoop()
}

type wsConnContext struct {
	wsConn  *websocket.Conn
	conn    *Connection
	handler *Handler
	sendCh  chan []byte
	doneCh  chan struct{}
	mu      sync.Mutex
	once    sync.Once
}

func (ctx *wsConnContext) closeDone() {
	ctx.once.Do(func() { close(ctx.doneCh) })
}

func (ctx *wsConnContext) SendEnvelope(env *Envelope, sentAt string) error {
	if env == nil {
		return nil
	}
	if ctx.conn != nil {
		_, generation := ctx.conn.SessionSnapshot()
		if generation > 0 {
			env.ConnectionGeneration = generation
		}
		if env.Sequence <= 0 {
			env.Sequence = ctx.conn.NextOutboundSequence()
		}
	}
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}

	select {
	case ctx.sendCh <- data:
		return nil
	case <-ctx.doneCh:
		return ErrConnectionClosed
	default:
		return errors.New("runtime send queue full")
	}
}

func (ctx *wsConnContext) readLoop() {
	defer func() {
		ctx.closeDone()
		ctx.handler.HandleDisconnect(ctx.conn)
		ctx.wsConn.Close()
	}()

	for {
		_, data, err := ctx.wsConn.ReadMessage()
		if err != nil {
			return
		}

		var env Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}

		if env.MessageType == MessageTypeHello {
			if err := env.ValidateBase(); err != nil {
				return
			}
		} else {
			if err := env.ValidateEstablishedSession(); err != nil {
				return
			}
		}

		if env.UserID != ctx.conn.UserID ||
			env.DeviceID != ctx.conn.DeviceID ||
			env.RuntimeID != ctx.conn.RuntimeID {
			return
		}

		sessionID, _ := ctx.conn.SessionSnapshot()
		if env.MessageType != MessageTypeHello && env.RuntimeSessionID != runtimeidentity.RuntimeSessionID(sessionID) {
			return
		}

		if !env.VerifyPayloadHash() {
			return
		}

		switch MessageType(env.MessageType) {
		case MessageTypeHello:
			var payload HelloPayload
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				return
			}
			ack, err := ctx.handler.HandleHello(ctx.conn, &payload)
			if err != nil || ack == nil {
				return
			}
			ackEnv, err := ctx.handler.CreateEnvelope(MessageTypeHelloAck, "hello_ack", ctx.conn.RuntimeID, runtimeidentity.ParseRuntimeSessionID(ctx.conn.SessionIDValue()), ack, ctx.conn.UserID, ctx.conn.DeviceID)
			if err != nil || ackEnv == nil {
				return
			}
			if err := ctx.SendEnvelope(ackEnv, time.Now().Format("2006-01-02 15:04:05")); err != nil {
				return
			}
		case MessageTypeCommandAck:
			var ackPayload CommandAckPayload
			if err := json.Unmarshal(env.Payload, &ackPayload); err != nil {
				return
			}
			if err := ctx.handler.HandleCommandAck(ctx.conn, &env, &ackPayload); err != nil {
				return
			}
		case MessageTypePing:
			pongEnv, err := ctx.handler.CreateEnvelope(MessageTypePong, "pong", ctx.conn.RuntimeID, runtimeidentity.ParseRuntimeSessionID(ctx.conn.SessionIDValue()), protocol.PongPayload{Time: time.Now()}, ctx.conn.UserID, ctx.conn.DeviceID)
			if err != nil || pongEnv == nil {
				return
			}
			if err := ctx.SendEnvelope(pongEnv, time.Now().Format("2006-01-02 15:04:05")); err != nil {
				return
			}
		default:
			if _, err := ctx.handler.HandleEvent(ctx.conn, &env); err != nil {
				return
			}
		}
	}
}

func (ctx *wsConnContext) writeLoop() {
	defer func() {
		ctx.closeDone()
		ctx.wsConn.Close()
	}()

	for {
		select {
		case data := <-ctx.sendCh:
			ctx.mu.Lock()
			err := ctx.wsConn.WriteMessage(websocket.TextMessage, data)
			ctx.mu.Unlock()
			if err != nil {
				return
			}
		case <-ctx.doneCh:
			return
		}
	}
}

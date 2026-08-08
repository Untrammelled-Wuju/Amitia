package v2

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/u-ai/backend/internal/desktoppet/readiness"
	"github.com/u-ai/backend/internal/middleware"
	"github.com/u-ai/backend/log"
)

func RegisterInternalRoutes(
	r *gin.Engine,
	facade *RuntimeFacade,
	safeMode *readiness.SafeModeController,
) {
	if facade == nil {
		return
	}

	path := facade.Config().Path
	if path == "" {
		path = "/internal/desktop-pet/runtime/ws"
	}

	r.GET(path, internalOriginMiddleware(facade, safeMode), gin.WrapH(&v2WSHandler{facade: facade}))
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
				"deviceId":   conn.DeviceID,
				"state":     conn.State,
				"sessionId": conn.SessionID,
				"lastSeq":   conn.LastSeq,
				"lastBeat":  conn.LastBeat.Format(time.RFC3339),
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
			"deviceId":   conn.DeviceID,
			"state":     conn.State,
			"sessionId": conn.SessionID,
			"lastSeq":   conn.LastSeq,
			"lastBeat":  conn.LastBeat.Format(time.RFC3339),
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
	facade   *RuntimeFacade
	upgrader websocket.Upgrader
}

func (h *v2WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.facade == nil || !h.facade.IsStarted() {
		http.Error(w, "runtime v2 not started", http.StatusServiceUnavailable)
		return
	}

	wsConn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Warn("[v2-ws] upgrade failed: ", err)
		return
	}

	v2Handler := h.facade.Handler()

	userID := r.URL.Query().Get("userId")
	deviceID := r.URL.Query().Get("deviceId")
	runtimeID := r.URL.Query().Get("runtimeId")

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

		switch MessageType(env.MessageType) {
		case MessageTypeHello:
			var payload HelloPayload
			if err := json.Unmarshal(env.Payload, &payload); err == nil {
				ack, err := ctx.handler.HandleHello(ctx.conn, &payload)
				if err == nil && ack != nil {
					ackEnv, _ := ctx.handler.CreateEnvelope(MessageTypeHelloAck, "hello_ack", ctx.conn.RuntimeID, ctx.conn.SessionID, ack, ctx.conn.UserID, ctx.conn.DeviceID)
					if ackEnv != nil {
						if ackData, err := json.Marshal(ackEnv); err == nil {
							select {
							case ctx.sendCh <- ackData:
							default:
							}
						}
					}
				}
			}
		case MessageTypeCommandAck:
			var ackPayload CommandAckPayload
			if err := json.Unmarshal(env.Payload, &ackPayload); err == nil {
				_ = ctx.handler.HandleCommandAck(ctx.conn, &env, &ackPayload)
			}
		case MessageTypePing:
			pongEnv, _ := ctx.handler.CreateEnvelope(MessageTypePong, "pong", ctx.conn.RuntimeID, ctx.conn.SessionID, map[string]interface{}{"time": time.Now()}, ctx.conn.UserID, ctx.conn.DeviceID)
			if pongEnv != nil {
				if pongData, err := json.Marshal(pongEnv); err == nil {
					select {
					case ctx.sendCh <- pongData:
					default:
					}
				}
			}
		default:
			_, _ = ctx.handler.HandleEvent(ctx.conn, env.MessageName, env.Payload)
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

package workbenchterminal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/u-ai/backend/internal/workbenchhttp"
)

const maxStreamLife = 4 * time.Minute
const streamHeartbeat = 20 * time.Second

type terminalStreamCommand struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
}

type terminalStreamFrame struct {
	Type     string   `json:"type"`
	Snapshot Snapshot `json:"snapshot"`
}

func (h *Handler) RegisterStreamRoute(group *gin.Engine) {
	group.GET("/api/workbench/terminal/stream", h.stream)
}

func (h *Handler) streamTicket(c *gin.Context) {
	c.Header("Cache-Control", "no-store, private")
	c.Header("X-Content-Type-Options", "nosniff")
	var request struct {
		ConversationID string `json:"conversationId"`
		Origin         string `json:"origin"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(c, ErrInvalid)
		return
	}
	owner, ok := terminalOwner(c, request.ConversationID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问终端"})
		return
	}
	if supplied := c.Request.Header.Get("Origin"); supplied != "" && supplied != request.Origin {
		c.JSON(http.StatusForbidden, gin.H{"error": "终端连接来源不匹配"})
		return
	}
	if err := h.validateTask(c.Request.Context(), owner, c.Param("id"), request.ConversationID); err != nil {
		h.respondError(c, err)
		return
	}
	ticket, err := h.tickets.issue(owner, c.Param("id"), request.Origin, request.ConversationID)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ticket": ticket, "expiresInSeconds": int(ticketLifetime.Seconds())})
}

func (h *Handler) stream(c *gin.Context) {
	workbenchhttp.ProtectResponse(c.Writer.Header())
	if len(c.Query("ticket")) != 48 || c.Request.Header.Get("Upgrade") != "websocket" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "终端连接凭据无效"})
		return
	}
	cursor, parseErr := parseTerminalStreamCursor(c.DefaultQuery("cursor", "0"))
	if parseErr != nil {
		h.respondError(c, ErrInvalid)
		return
	}
	grant, err := h.tickets.consume(c.Query("ticket"), c.Request.Header.Get("Origin"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": ErrTicket.Error()})
		return
	}
	if err = h.validateTask(c.Request.Context(), grant.owner, grant.sessionID, grant.conversationID); err != nil {
		h.respondError(c, err)
		return
	}
	upgrade := websocket.Upgrader{
		ReadBufferSize: 4096, WriteBufferSize: 8192,
		CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == grant.origin },
	}
	conn, err := upgrade.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(8 * 1024)
	_ = conn.SetReadDeadline(time.Now().Add(75 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(75 * time.Second)) })
	ctx, cancel := context.WithTimeout(c.Request.Context(), maxStreamLife)
	defer cancel()
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			_, body, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var command terminalStreamCommand
			if err := json.Unmarshal(body, &command); err != nil {
				return
			}
			if err := h.validateTask(ctx, grant.owner, grant.sessionID, grant.conversationID); err != nil {
				return
			}
			switch command.Type {
			case "input":
				if len(command.Data) > 6000 {
					return
				}
				data, err := base64.StdEncoding.DecodeString(command.Data)
				if err != nil || h.manager.Input(grant.owner, grant.sessionID, data) != nil {
					return
				}
			case "resize":
				if h.manager.Resize(grant.owner, grant.sessionID, command.Rows, command.Cols) != nil {
					return
				}
			default:
				return
			}
		}
	}()
	ticker := time.NewTicker(streamHeartbeat)
	defer ticker.Stop()
	for {
		if err := h.validateTask(ctx, grant.owner, grant.sessionID, grant.conversationID); err != nil {
			return
		}
		session, err := h.manager.get(grant.owner, grant.sessionID)
		if err != nil {
			return
		}
		snap, changed := session.snapshotAndListen(cursor)
		if cursor != snap.Cursor || !snap.Running {
			_ = conn.SetWriteDeadline(time.Now().Add(6 * time.Second))
			if err := conn.WriteJSON(terminalStreamFrame{Type: "snapshot", Snapshot: snap}); err != nil {
				return
			}
			cursor = snap.Cursor
			if len(snap.Data) >= base64.StdEncoding.EncodedLen(maxReadBytes) {
				continue
			}
			if !snap.Running {
				return
			}
		}
		select {
		case <-changed:
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(6 * time.Second))
			if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(6*time.Second)); err != nil {
				return
			}
		case <-readerDone:
			return
		case <-ctx.Done():
			return
		}
	}
}

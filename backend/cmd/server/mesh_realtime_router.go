package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
)

type ownedRealtimeTicket struct {
	Engine  *business.Engine
	Request business.Request
	Origin  string
	Expires time.Time
}

var ownedRealtimeTickets = struct {
	sync.Mutex
	Items map[[32]byte]ownedRealtimeTicket
}{Items: make(map[[32]byte]ownedRealtimeTicket)}

func ownedRealtimeOrigin(origin string) (string, bool) {
	if origin == "" {
		return "", true
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" {
		return "", false
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host), true
}

func registerMeshRealtimeRouter(mesh *gin.RouterGroup, services *AppServices, coreID string) {
	mesh.POST("/realtime/tickets", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(401)
			return
		}
		var request business.Request
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<20)
		if c.ShouldBindJSON(&request) != nil || request.ExpectedScope == nil {
			c.JSON(400, gin.H{"message": "通话缺少原角色与完整权限范围"})
			return
		}
		if _, err := uuid.Parse(request.RequestID); err != nil {
			c.JSON(400, gin.H{"message": "通话请求编号必须为 UUID"})
			return
		}
		origin, valid := ownedRealtimeOrigin(c.GetHeader("Origin"))
		if !valid {
			c.AbortWithStatus(403)
			return
		}
		request.SpaceID, request.DeviceID, request.CoreID = actor.SpaceID.String(), actor.DeviceID.String(), coreID
		request.Attachments, request.Message = nil, ""
		ctx, scope, finish, err := services.OwnedBusiness.RealtimeAuthority(c.Request.Context(), request)
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		defer finish()
		if err := coordination.ValidateCurrent(ctx); err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		ticket, err := issueOwnedRealtimeTicket(services, request, origin, scope)
		if err != nil {
			c.JSON(503, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": ticket})
	})
}

func issueOwnedRealtimeTicket(services *AppServices, request business.Request, origin string, scope coordination.ExecutionScope) (gin.H, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expires := time.Now().Add(time.Minute)
	ownedRealtimeTickets.Lock()
	defer ownedRealtimeTickets.Unlock()
	for key, value := range ownedRealtimeTickets.Items {
		if !value.Expires.After(time.Now()) {
			delete(ownedRealtimeTickets.Items, key)
		}
	}
	if len(ownedRealtimeTickets.Items) >= 4096 {
		return nil, errors.New("通话票据已达上限，请稍后重试")
	}
	ownedRealtimeTickets.Items[sha256.Sum256([]byte(token))] = ownedRealtimeTicket{Engine: services.OwnedBusiness, Request: request, Origin: origin, Expires: expires}
	return gin.H{"ticket": token, "expiresAt": expires, "wsPath": "/api/device-mesh/v1/business/realtime/session", "executionScope": scope}, nil
}

func registerMeshRealtimePublicRoutes(router *gin.Engine, services *AppServices) {
	router.GET("/api/device-mesh/v1/business/realtime/session", func(c *gin.Context) {
		key := sha256.Sum256([]byte(c.Query("ticket")))
		ownedRealtimeTickets.Lock()
		ticket, exists := ownedRealtimeTickets.Items[key]
		delete(ownedRealtimeTickets.Items, key)
		ownedRealtimeTickets.Unlock()
		origin, valid := ownedRealtimeOrigin(c.GetHeader("Origin"))
		if !exists || !ticket.Expires.After(time.Now()) || !valid || origin != ticket.Origin || services.OwnedBusiness == nil || ticket.Engine != services.OwnedBusiness {
			c.AbortWithStatus(401)
			return
		}
		ctx, scope, finish, err := services.OwnedBusiness.RealtimeAuthority(c.Request.Context(), ticket.Request)
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		defer finish()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
			value, ok := ownedRealtimeOrigin(r.Header.Get("Origin"))
			return ok && value == ticket.Origin
		}}
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetReadLimit(2 << 20)
		var writer sync.Mutex
		write := func(value any) error {
			writer.Lock()
			defer writer.Unlock()
			if err := coordination.ValidateCurrent(ctx); err != nil {
				return err
			}
			if err := coordination.ValidateRoleRevision(ctx, services.DeviceMesh, scope); err != nil {
				return err
			}
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			return conn.WriteJSON(value)
		}
		var state sync.Mutex
		var workerCancel context.CancelFunc
		busy := false
		defer func() {
			state.Lock()
			if workerCancel != nil {
				workerCancel()
			}
			state.Unlock()
		}()
		go func() {
			timer := time.NewTicker(500 * time.Millisecond)
			defer timer.Stop()
			for {
				select {
				case <-ctx.Done():
					conn.Close()
					return
				case <-timer.C:
					if coordination.ValidateCurrent(ctx) != nil || coordination.ValidateRoleRevision(ctx, services.DeviceMesh, scope) != nil {
						cancel()
						conn.Close()
						return
					}
				}
			}
		}()
		if write(gin.H{"type": "ready", "executionScope": scope, "conversationId": ticket.Request.ConversationID}) != nil {
			return
		}
		var pcm []byte
		var visual *business.Attachment
		turnID := ""
		for {
			kind, payload, err := conn.ReadMessage()
			if err != nil {
				return
			}
			state.Lock()
			active := busy
			state.Unlock()
			if kind == websocket.BinaryMessage {
				if active || turnID == "" || len(pcm)+len(payload) > (1<<20)-44 || len(payload)%2 != 0 {
					write(gin.H{"type": "error", "message": "语音轮次无效或超过单轮 32 秒限制"})
					return
				}
				pcm = append(pcm, payload...)
				continue
			}
			var input struct {
				Type       string                       `json:"type"`
				RequestID  string                       `json:"requestId"`
				Scope      *coordination.ExecutionScope `json:"expectedExecutionScope"`
				Attachment *business.Attachment         `json:"attachment"`
			}
			if json.Unmarshal(payload, &input) != nil {
				write(gin.H{"type": "error", "message": "实时消息格式无效"})
				return
			}
			switch input.Type {
			case "stop":
				return
			case "interrupt":
				state.Lock()
				if workerCancel != nil {
					workerCancel()
				}
				state.Unlock()
				pcm, visual, turnID = nil, nil, ""
			case "turn_start":
				if active || turnID != "" || input.Scope == nil {
					write(gin.H{"type": "error", "message": "上一轮尚未结束或缺少原权限范围"})
					continue
				}
				if _, err := uuid.Parse(input.RequestID); err != nil {
					write(gin.H{"type": "error", "message": "轮次编号无效"})
					continue
				}
				expected, original := *input.Scope, *ticket.Request.ExpectedScope
				expected.RequestID, expected.TurnID, expected.ExecutionID = original.RequestID, original.TurnID, original.ExecutionID
				if expected != original {
					write(gin.H{"type": "error", "message": "通话角色或服务权限已变化"})
					return
				}
				turnID, pcm, visual = input.RequestID, nil, nil
			case "visual":
				if turnID == "" || active || input.Attachment == nil || input.Attachment.Kind != "image" || business.ValidateAttachments([]business.Attachment{*input.Attachment}) != nil {
					write(gin.H{"type": "error", "message": "视觉帧无效，请重新发送当前轮次"})
					continue
				}
				visual = input.Attachment
			case "turn_end":
				if active || turnID == "" {
					write(gin.H{"type": "error", "message": "没有可提交的语音轮次"})
					continue
				}
				state.Lock()
				request := ticket.Request
				state.Unlock()
				request.RequestID = turnID
				turnPCM, turnVisual := pcm, visual
				pcm, visual, turnID = nil, nil, ""
				workerContext, stop := context.WithCancel(ctx)
				state.Lock()
				busy, workerCancel = true, stop
				state.Unlock()
				go func() {
					defer stop()
					defer func() {
						state.Lock()
						busy, workerCancel = false, nil
						state.Unlock()
						write(gin.H{"type": "turn_ready", "requestId": request.RequestID})
					}()
					result, err := services.OwnedBusiness.RealtimeTurn(workerContext, request, turnPCM, turnVisual, func(event business.Event) error {
						if err := workerContext.Err(); err != nil {
							return err
						}
						return write(gin.H{"type": "event", "requestId": request.RequestID, "data": event})
					})
					if err != nil {
						write(gin.H{"type": "error", "requestId": request.RequestID, "message": err.Error(), "saved": result.Saved})
						return
					}
					if !result.Saved || workerContext.Err() != nil {
						return
					}
					state.Lock()
					ticket.Request.ConversationID, ticket.Request.ConversationOrigin = result.ConversationID, result.ConversationOrigin
					state.Unlock()
					if write(gin.H{"type": "completed", "requestId": request.RequestID, "data": result}) != nil {
						return
					}
					request.ExpectedScope = &result.Scope
					speech, err := services.OwnedBusiness.Speech(workerContext, request, result.Text)
					if err != nil {
						write(gin.H{"type": "error", "requestId": request.RequestID, "message": err.Error(), "saved": true})
						return
					}
					if workerContext.Err() != nil {
						return
					}
					write(gin.H{"type": "audio", "requestId": request.RequestID, "data": speech})
				}()
			default:
				write(gin.H{"type": "error", "message": "不支持的实时事件"})
			}
		}
	})
}

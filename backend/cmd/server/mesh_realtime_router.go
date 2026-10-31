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
	voicewake "github.com/u-ai/backend/internal/devicemesh/wake"
	"github.com/u-ai/backend/internal/middleware/security"
	amitialog "log"
)

type ownedRealtimeTicket struct {
	Engine    *business.Engine
	Request   business.Request
	VoiceType string
	Resource  string
	DialogID  string
	Origin    string
	Expires   time.Time
}

type meshRealtimeTicketRequest struct {
	business.Request
	VoiceType string `json:"voiceType,omitempty"`
	Resource  string `json:"resourceId,omitempty"`
	DialogID  string `json:"dialogId,omitempty"`
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
		var input meshRealtimeTicketRequest
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<20)
		if c.ShouldBindJSON(&input) != nil || input.ExpectedScope == nil {
			c.JSON(400, gin.H{"message": "通话缺少原角色与完整权限范围"})
			return
		}
		input.VoiceType = strings.TrimSpace(input.VoiceType)
		input.Resource = strings.TrimSpace(input.Resource)
		input.DialogID = strings.TrimSpace(input.DialogID)
		if len(input.VoiceType) > 256 || len(input.Resource) > 256 || len(input.DialogID) > 256 {
			c.JSON(400, gin.H{"message": "通话音色、资源或旧对话标识超过长度限制"})
			return
		}
		request := input.Request
		if _, err := uuid.Parse(request.RequestID); err != nil {
			c.JSON(400, gin.H{"message": err.Error()})
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
		ticket, err := issueOwnedRealtimeTicket(services, request, input.VoiceType, input.Resource, input.DialogID, origin, scope)
		if err != nil {
			c.JSON(503, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": ticket})
	})
}

func issueOwnedRealtimeTicket(services *AppServices, request business.Request, voiceType, resource, dialogID, origin string, scope coordination.ExecutionScope) (gin.H, error) {
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
	ownedRealtimeTickets.Items[sha256.Sum256([]byte(token))] = ownedRealtimeTicket{Engine: services.OwnedBusiness, Request: request, VoiceType: voiceType, Resource: resource, DialogID: dialogID, Origin: origin, Expires: expires}
	return gin.H{"ticket": token, "expiresAt": expires, "wsPath": "/api/device-mesh/v1/business/realtime/session", "executionScope": scope, "voiceType": voiceType, "resourceId": resource, "dialogId": dialogID}, nil
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
		if write(gin.H{"type": "ready", "executionScope": scope, "conversationId": ticket.Request.ConversationID, "voiceType": ticket.VoiceType, "resourceId": ticket.Resource, "dialogId": ticket.DialogID, "capabilities": gin.H{"audioInput": true, "audioOutput": true, "cameraInput": true, "screenInput": true, "visualContext": true, "streamingASR": true, "streamingText": true, "turnBasedTTS": true, "wakeWord": true, "playbackProgress": true}}) != nil {
			return
		}
		var pcm []byte
		var visual *business.Attachment
		turnID := ""
		var wakeDetector voicewake.WakeDetector
		var wakeConfigID string
		var wakeSequence uint64
		wakeArmed := false
		defer func() {
			if wakeDetector != nil {
				if err := wakeDetector.Unload(); err != nil {
					amitialog.Printf("[original-error] %v", err)
				}
			}
		}()
		for {
			kind, payload, err := conn.ReadMessage()
			if err != nil {
				return
			}
			state.Lock()
			active := busy
			state.Unlock()
			if kind == websocket.BinaryMessage {
				if wakeArmed && wakeDetector != nil {
					wakeSequence++
					detected, detectErr := wakeDetector.Process(&voicewake.VoiceAudioFrame{SessionID: ticket.Request.RequestID, Sequence: wakeSequence, TimestampNS: time.Now().UnixNano(), SampleRate: voicewake.CanonicalSampleRate, Channels: voicewake.CanonicalChannels, Encoding: voicewake.CanonicalEncoding, PCM: payload})
					if detectErr != nil {
						write(gin.H{"type": "error", "message": detectErr.Error()})
						return
					}
					if detected.Detected {
						wakeArmed = false
						if write(gin.H{"type": "wake_detected", "data": gin.H{"configId": wakeConfigID, "phraseId": detected.PhraseID, "confidence": detected.Confidence, "detectedAt": time.Unix(0, detected.DetectedAtNS).UTC()}}) != nil {
							return
						}
					}
					continue
				}
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
				ConfigID   string                       `json:"configId"`
				Generation int64                        `json:"generationId"`
			}
			if json.Unmarshal(payload, &input) != nil {
				write(gin.H{"type": "error", "message": "实时消息格式无效"})
				return
			}
			switch input.Type {
			case "stop":
				return
			case "wake_arm":
				if wakeDetector != nil {
					if err := wakeDetector.Unload(); err != nil {
						amitialog.Printf("[original-error] %v", err)
					}
				}
				wakeDetector, wakeConfigID, err = loadMeshWakeDetector(ctx, services.DB, input.ConfigID)
				if err != nil {
					write(gin.H{"type": "wake_status", "armed": false, "message": err.Error()})
					continue
				}
				wakeArmed = true
				wakeSequence = 0
				write(gin.H{"type": "wake_status", "armed": true, "configId": wakeConfigID})
			case "wake_disarm":
				wakeArmed = false
				if wakeDetector != nil {
					wakeDetector.Reset()
				}
				write(gin.H{"type": "wake_status", "armed": false, "configId": wakeConfigID})
			case "playback_progress":
				write(gin.H{"type": "playback_status", "requestId": input.RequestID, "generationId": input.Generation, "acknowledged": true})
			case "tts_playback_ended":
				write(gin.H{"type": "tts_ended", "requestId": input.RequestID, "generationId": input.Generation})
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
				write(gin.H{"type": "vision_status", "requestId": turnID, "data": gin.H{"available": true, "kind": input.Attachment.Kind, "hash": input.Attachment.Hash}})
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
						switch event.Type {
						case "transcribed":
							return write(gin.H{"type": "asr_final", "requestId": request.RequestID, "data": gin.H{"eventId": "realtime-asr-" + event.Scope.TurnID, "transcript": event.Text, "conversationId": event.ConversationID, "turnId": event.Scope.TurnID, "executionId": event.Scope.ExecutionID}})
						case "delta":
							return write(gin.H{"type": "assistant_text", "requestId": request.RequestID, "data": gin.H{"text": event.Text, "delta": true}})
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
					speech, err := services.OwnedBusiness.Speech(workerContext, request, result.Text, business.SpeechOptions{VoiceType: ticket.VoiceType, ResourceID: ticket.Resource})
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

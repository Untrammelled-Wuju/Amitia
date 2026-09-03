// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type callRuntime struct {
	call     *RealtimeCallSession
	pipeline *VisualPipeline
}

type callRuntimeRegistry struct {
	mu    sync.RWMutex
	calls map[string]*callRuntime
}

var realtimeCallRegistry = &callRuntimeRegistry{calls: make(map[string]*callRuntime)}

var realtimeVisualAnalyzer VisualAnalyzer

func SetVisualAnalyzer(analyzer VisualAnalyzer) {
	realtimeVisualAnalyzer = analyzer
}

func (r *callRuntimeRegistry) Add(runtime *callRuntime) {
	if runtime == nil || runtime.call == nil {
		return
	}
	r.mu.Lock()
	r.calls[runtime.call.CallID] = runtime
	r.mu.Unlock()
}

func (r *callRuntimeRegistry) Get(callID string) (*callRuntime, bool) {
	r.mu.RLock()
	runtime, ok := r.calls[callID]
	r.mu.RUnlock()
	return runtime, ok
}

func (r *callRuntimeRegistry) Remove(callID string) {
	r.mu.Lock()
	runtime := r.calls[callID]
	delete(r.calls, callID)
	r.mu.Unlock()
	if runtime != nil {
		runtime.call.Close()
		if runtime.pipeline != nil {
			runtime.pipeline.Close()
		}
	}
}

func HandleVisualSession(c *gin.Context) {
	callID := c.Query("callId")
	ticket := c.Query("ticket")
	runtime, ok := realtimeCallRegistry.Get(callID)
	if !ok || runtime.call == nil || !runtime.call.VerifyVisualTicket(ticket) {
		c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "invalid or expired realtime visual session"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(4 * 1024 * 1024)

	_ = conn.WriteJSON(gin.H{
		"event": "visual.connected",
		"data": gin.H{
			"callId":        callID,
			"maxFrameBytes": maxVisualFrameBytes,
			"maxFps":        runtime.call.Capabilities.MaxVisualFPS,
			"latestWins":    true,
		},
	})

	for {
		var envelope struct {
			Event string              `json:"event"`
			Data  VisualFrameEnvelope `json:"data"`
		}
		if err := conn.ReadJSON(&envelope); err != nil {
			return
		}
		switch envelope.Event {
		case "visual.frame":
			frame, err := ParseVisualFrame(callID, envelope.Data)
			if err != nil {
				_ = conn.WriteJSON(gin.H{"event": "visual.rejected", "data": err.Error()})
				continue
			}
			accepted := runtime.pipeline != nil && runtime.pipeline.Submit(frame)
			_ = conn.WriteJSON(gin.H{
				"event": "visual.accepted",
				"data": gin.H{
					"source":   frame.Source,
					"sequence": frame.Sequence,
					"accepted": accepted,
				},
			})
		case "stop":
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "visual session stopped"))
			return
		}
	}
}

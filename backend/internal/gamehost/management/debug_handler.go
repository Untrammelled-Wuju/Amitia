package management

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/gamehost"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type DebugToolScope struct {
	UserID         string
	CharacterID    string
	ConversationID string
	Channel        string
	SessionID      string
	RequestID      string
	ToolCallID     string
}

type DebugToolInvokeFunc func(context.Context, string, json.RawMessage, DebugToolScope) (any, bool)

type DebugHandler struct {
	container  *gamehost.GameHostContainer
	invokeTool DebugToolInvokeFunc
}

func NewDebugHandler(container *gamehost.GameHostContainer, invokers ...DebugToolInvokeFunc) *DebugHandler {
	h := &DebugHandler{container: container}
	if len(invokers) > 0 {
		h.invokeTool = invokers[0]
	}
	return h
}

type ResidueReport struct {
	TargetRuntimeID  string `json:"targetRuntimeId,omitempty"`
	PluginCount      int    `json:"pluginCount"`
	RuntimeCount     int    `json:"runtimeCount"`
	ConnectionCount  int    `json:"connectionCount"`
	HandshakeCount   int    `json:"handshakeCount"`
	PendingRPCCount  int    `json:"pendingRpcCount"`
	ChannelCount     int    `json:"channelCount"`
	StreamCount      int    `json:"streamCount"`
	BinaryCount      int    `json:"binaryCount"`
	SecretLeaseCount int    `json:"secretLeaseCount"`
	ProcessCount     int    `json:"processCount"`
	ControlSinkCount int    `json:"controlSinkCount"`
	HostAPIInflight  int    `json:"hostApiInflight"`
	LifecycleIntent  string `json:"lifecycleIntent,omitempty"`
	EmergencyLatched bool   `json:"emergencyLatched"`
}

func (h *DebugHandler) GetResidueReport(c *gin.Context) {
	if h.container == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "container unavailable"})
		return
	}
	runtimeID := domain.RuntimeInstanceID(c.Query("runtimeId"))
	if runtimeID != "" {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": h.runtimeResidue(runtimeID)})
		return
	}

	report := ResidueReport{}
	if h.container.PluginRegistry != nil {
		report.PluginCount = h.container.PluginRegistry.Count()
	}
	if h.container.RuntimeManager != nil {
		report.RuntimeCount = len(h.container.RuntimeManager.ListRuntimes())
	}
	if h.container.ConnectionRegistry != nil {
		report.ConnectionCount = h.container.ConnectionRegistry.ActiveCount()
	}
	if h.container.HandshakeManager != nil {
		report.HandshakeCount = h.container.HandshakeManager.Count()
	}
	if h.container.RPCLifecycle != nil {
		report.PendingRPCCount = h.container.RPCLifecycle.Registry().Count()
	}
	if h.container.ChannelRegistry != nil {
		report.ChannelCount = h.container.ChannelRegistry.Count()
	}
	if h.container.StreamManager != nil {
		report.StreamCount = h.container.StreamManager.Count()
	}
	if h.container.BinaryObjectRegistry != nil {
		report.BinaryCount = h.container.BinaryObjectRegistry.CountActive()
	}
	if h.container.SecretLeaseAdapter != nil {
		report.SecretLeaseCount = h.container.SecretLeaseAdapter.Count()
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": report})
}

func (h *DebugHandler) runtimeResidue(runtimeID domain.RuntimeInstanceID) ResidueReport {
	report := ResidueReport{TargetRuntimeID: string(runtimeID)}
	if h.container.RuntimeManager != nil {
		if _, err := h.container.RuntimeManager.GetRuntime(runtimeID); err == nil {
			report.RuntimeCount = 1
		}
		report.LifecycleIntent, _ = h.container.RuntimeManager.GetLifecycleIntent(runtimeID)
		report.EmergencyLatched = h.container.RuntimeManager.IsEmergencyLatched(runtimeID)
	}
	if h.container.ConnectionRegistry != nil {
		connections := h.container.ConnectionRegistry.ListByRuntime(runtimeID)
		for _, conn := range connections {
			if conn != nil && conn.IsActive() {
				report.ConnectionCount++
			}
			if conn != nil && h.container.HandshakeManager != nil {
				if _, ok := h.container.HandshakeManager.GetState(string(conn.ID)); ok {
					report.HandshakeCount++
				}
			}
		}
	}
	if h.container.RPCLifecycle != nil {
		report.PendingRPCCount = h.container.RPCLifecycle.Registry().CountByRuntime(runtimeID)
	}
	if h.container.ChannelRegistry != nil {
		report.ChannelCount = h.container.ChannelRegistry.CountByRuntime(runtimeID)
	}
	if h.container.StreamManager != nil {
		report.StreamCount = h.container.StreamManager.CountByRuntime(runtimeID)
	}
	if h.container.BinaryObjectRegistry != nil {
		if items, err := h.container.BinaryObjectRegistry.ListByRuntime(runtimeID); err == nil {
			report.BinaryCount = len(items)
		}
	}
	if h.container.SecretLeaseAdapter != nil {
		report.SecretLeaseCount = len(h.container.SecretLeaseAdapter.ActiveRuntimeLeases(string(runtimeID)))
	}
	if h.container.ControlSinkRegistry != nil {
		report.ControlSinkCount = len(h.container.ControlSinkRegistry.ListByRuntime(runtimeID))
	}
	if h.container.HostAPIInvocationTracker != nil {
		report.HostAPIInflight = h.container.HostAPIInvocationTracker.CountRuntimeHostAPIWork(runtimeID)
	}
	report.ProcessCount = h.container.CountRuntimeProcesses(runtimeID)
	return report
}

type debugToolInvokeRequest struct {
	ToolID         string          `json:"toolId"`
	Input          json.RawMessage `json:"input"`
	UserID         string          `json:"userId,omitempty"`
	CharacterID    string          `json:"characterId,omitempty"`
	ConversationID string          `json:"conversationId,omitempty"`
	Channel        string          `json:"channel,omitempty"`
	SessionID      string          `json:"sessionId,omitempty"`
	RequestID      string          `json:"requestId,omitempty"`
	ToolCallID     string          `json:"toolCallId,omitempty"`
}

// InvokeTool is developer-only and exists to exercise the exact canonical
// ToolFacade -> ExecutionPipeline -> GameHost -> external process path in E2E
// tests. It is protected by RequireGameHostDeveloperAccess and is never exposed
// as a normal user-facing Game Center RPC shortcut.
func (h *DebugHandler) InvokeTool(c *gin.Context) {
	if h == nil || h.invokeTool == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "canonical tool invoker unavailable"})
		return
	}
	var req debugToolInvokeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid tool invocation payload"})
		return
	}
	req.ToolID = strings.TrimSpace(req.ToolID)
	if req.ToolID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "toolId required"})
		return
	}
	if len(req.Input) == 0 {
		req.Input = json.RawMessage(`{}`)
	}
	result, ok := h.invokeTool(c.Request.Context(), req.ToolID, req.Input, DebugToolScope{
		UserID: strings.TrimSpace(req.UserID), CharacterID: strings.TrimSpace(req.CharacterID),
		ConversationID: strings.TrimSpace(req.ConversationID), Channel: strings.TrimSpace(req.Channel),
		SessionID: strings.TrimSpace(req.SessionID), RequestID: strings.TrimSpace(req.RequestID), ToolCallID: strings.TrimSpace(req.ToolCallID),
	})
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "tool not found or unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

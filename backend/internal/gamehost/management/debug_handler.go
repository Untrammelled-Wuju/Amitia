package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/gamehost"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type DebugHandler struct{ container *gamehost.GameHostContainer }

func NewDebugHandler(container *gamehost.GameHostContainer) *DebugHandler {
	return &DebugHandler{container: container}
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

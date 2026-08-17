package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/gamehost"
)

type DebugHandler struct {
	container *gamehost.GameHostContainer
}

func NewDebugHandler(container *gamehost.GameHostContainer) *DebugHandler {
	return &DebugHandler{container: container}
}

type ResidueReport struct {
	PluginCount      int `json:"pluginCount"`
	RuntimeCount     int `json:"runtimeCount"`
	ConnectionCount  int `json:"connectionCount"`
	HandshakeCount   int `json:"handshakeCount"`
	PendingRPCCount  int `json:"pendingRpcCount"`
	ChannelCount     int `json:"channelCount"`
	StreamCount      int `json:"streamCount"`
	BinaryCount      int `json:"binaryCount"`
	SecretLeaseCount int `json:"secretLeaseCount"`
}

func (h *DebugHandler) GetResidueReport(c *gin.Context) {
	if h.container == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "container unavailable"})
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

package management

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	ghrpc "github.com/u-ai/backend/internal/gamehost/rpc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type RPCHandler struct {
	invoker RPCInvoker
}

func NewRPCHandler(invoker RPCInvoker) *RPCHandler {
	return &RPCHandler{invoker: invoker}
}

type RPCInvokeRequest struct {
	Method  string          `json:"method" binding:"required"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Timeout int64           `json:"timeoutMs,omitempty"`
}

type RPCInvokeResponse struct {
	Code    int             `json:"code"`
	Msg     string          `json:"msg"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func (h *RPCHandler) InvokeRPC(c *gin.Context) {
	if h.invoker == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "rpc invoker unavailable"})
		return
	}

	runtimeID := strings.TrimSpace(c.Param("runtimeId"))
	if runtimeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "runtimeId required"})
		return
	}

	serviceID := strings.TrimSpace(c.Param("serviceId"))
	if serviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "serviceId required"})
		return
	}

	var req RPCInvokeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid request body: " + err.Error()})
		return
	}

	req.Method = strings.TrimSpace(req.Method)
	if req.Method == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "method required"})
		return
	}
	if err := protocol.ValidateMethod(req.Method); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid RPC method: " + err.Error()})
		return
	}
	if namespace, _, err := ghrpc.ParseMethod(req.Method); err != nil || ghrpc.IsReservedNamespace(namespace) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "reserved host RPC namespaces are not available through the management debug endpoint"})
		return
	}
	if req.Timeout < 0 || req.Timeout > 120000 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "timeoutMs must be between 0 and 120000"})
		return
	}

	timeout := 30 * time.Second
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Millisecond
	}

	resp, err := h.invoker.SendCustomRPC(c.Request.Context(), runtimeID, serviceID, req.Method, req.Payload, timeout)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, RPCInvokeResponse{
		Code:    200,
		Msg:     "ok",
		Payload: resp.Payload,
	})
}

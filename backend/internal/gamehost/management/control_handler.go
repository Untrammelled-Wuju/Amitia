package management

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	ErrStaleEpoch       = errors.New("stale epoch")
	ErrPermissionDenied = errors.New("permission denied")
)

type TakeoverResult struct {
	Success  bool
	NewEpoch uint64
}

type ReleaseResult struct {
	Success  bool
	NewEpoch uint64
}

type TakeoverFunc func(ctx context.Context, runtimeID string) (TakeoverResult, error)
type ReleaseFunc func(ctx context.Context, runtimeID string, targetMode string, expectedEpoch uint64) (ReleaseResult, error)
type EmergencyStopFunc func(ctx context.Context, runtimeID string) error

type ControlHandler struct {
	takeoverFn      TakeoverFunc
	releaseFn       ReleaseFunc
	emergencyStopFn EmergencyStopFunc
}

func NewControlHandler(takeoverFn TakeoverFunc, releaseFn ReleaseFunc, emergencyStopFn EmergencyStopFunc) *ControlHandler {
	return &ControlHandler{
		takeoverFn:      takeoverFn,
		releaseFn:       releaseFn,
		emergencyStopFn: emergencyStopFn,
	}
}

type ReleaseRequest struct {
	TargetMode    string `json:"targetMode"`
	ExpectedEpoch uint64 `json:"expectedEpoch"`
}

type ControlMutationResult struct {
	Success  bool   `json:"success"`
	NewEpoch uint64 `json:"newEpoch,omitempty"`
	Mode     string `json:"mode,omitempty"`
}

func (h *ControlHandler) Takeover(c *gin.Context) {
	if h.takeoverFn == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "takeover service unavailable"})
		return
	}

	runtimeID := strings.TrimSpace(c.Param("runtimeId"))
	if runtimeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "runtimeId required"})
		return
	}

	result, err := h.takeoverFn(c.Request.Context(), runtimeID)
	if err != nil {
		if errors.Is(err, ErrStaleEpoch) {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "stale epoch, please refresh"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": ControlMutationResult{Success: result.Success, NewEpoch: result.NewEpoch, Mode: "user"}})
}

func (h *ControlHandler) Release(c *gin.Context) {
	if h.releaseFn == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "release service unavailable"})
		return
	}

	runtimeID := strings.TrimSpace(c.Param("runtimeId"))
	if runtimeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "runtimeId required"})
		return
	}

	var req ReleaseRequest
	if err := c.ShouldBindJSON(&req); err == nil {
		// body optional
	}

	targetMode := req.TargetMode
	if targetMode == "" {
		targetMode = "observe"
	}

	result, err := h.releaseFn(c.Request.Context(), runtimeID, targetMode, req.ExpectedEpoch)
	if err != nil {
		if errors.Is(err, ErrStaleEpoch) {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "stale epoch, please refresh"})
			return
		}
		if errors.Is(err, ErrPermissionDenied) {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": ControlMutationResult{Success: result.Success, NewEpoch: result.NewEpoch, Mode: targetMode}})
}

func (h *ControlHandler) EmergencyStop(c *gin.Context) {
	if h.emergencyStopFn == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "emergency stop service unavailable"})
		return
	}

	runtimeID := strings.TrimSpace(c.Param("runtimeId"))
	if runtimeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "runtimeId required"})
		return
	}

	err := h.emergencyStopFn(c.Request.Context(), runtimeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": ControlMutationResult{Success: true, Mode: "suspended"}})
}

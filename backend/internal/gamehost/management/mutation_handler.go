package management

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type MutationHandler struct {
	packageSvc *PackageMutationService
	runtimeSvc *RuntimeMutationService
}

func NewMutationHandler(packageSvc *PackageMutationService, runtimeSvc *RuntimeMutationService) *MutationHandler {
	return &MutationHandler{
		packageSvc: packageSvc,
		runtimeSvc: runtimeSvc,
	}
}

func (h *MutationHandler) Install(c *gin.Context) {
	if h.packageSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "package mutation service unavailable"})
		return
	}

	var req PackageInstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid request body"})
		return
	}

	result, err := h.packageSvc.Install(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		if errors.Is(err, ErrNotGamePlugin) {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *MutationHandler) Update(c *gin.Context) {
	if h.packageSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "package mutation service unavailable"})
		return
	}

	var req PackageUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid request body"})
		return
	}

	result, err := h.packageSvc.Update(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		if errors.Is(err, ErrNotGamePlugin) {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *MutationHandler) Enable(c *gin.Context) {
	if h.packageSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "package mutation service unavailable"})
		return
	}

	extensionID := strings.TrimSpace(c.Param("extensionId"))
	if extensionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "extensionId required"})
		return
	}

	result, err := h.packageSvc.Enable(c.Request.Context(), extensionID)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		if errors.Is(err, ErrNotGamePlugin) {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *MutationHandler) Disable(c *gin.Context) {
	if h.packageSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "package mutation service unavailable"})
		return
	}

	extensionID := strings.TrimSpace(c.Param("extensionId"))
	if extensionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "extensionId required"})
		return
	}

	result, err := h.packageSvc.Disable(c.Request.Context(), extensionID)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		if errors.Is(err, ErrNotGamePlugin) {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *MutationHandler) Uninstall(c *gin.Context) {
	if h.packageSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "package mutation service unavailable"})
		return
	}

	extensionID := strings.TrimSpace(c.Param("extensionId"))
	if extensionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "extensionId required"})
		return
	}

	result, err := h.packageSvc.Uninstall(c.Request.Context(), extensionID)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		if errors.Is(err, ErrNotGamePlugin) {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *MutationHandler) StartRuntime(c *gin.Context) {
	if h.runtimeSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "runtime mutation service unavailable"})
		return
	}

	runtimeID := strings.TrimSpace(c.Param("runtimeId"))
	if runtimeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "runtimeId required"})
		return
	}

	result, err := h.runtimeSvc.Start(c.Request.Context(), runtimeID)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		if errors.Is(err, ErrRuntimeNotGameCenter) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
			return
		}
		if errors.Is(err, ErrRuntimeExecutorUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": err.Error()})
			return
		}
		if isRuntimeExecutorError(err, "runtime not in startable state") {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "runtime not in startable state"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *MutationHandler) StopRuntime(c *gin.Context) {
	if h.runtimeSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "runtime mutation service unavailable"})
		return
	}

	runtimeID := strings.TrimSpace(c.Param("runtimeId"))
	if runtimeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "runtimeId required"})
		return
	}

	result, err := h.runtimeSvc.Stop(c.Request.Context(), runtimeID)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		if errors.Is(err, ErrRuntimeNotGameCenter) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
			return
		}
		if errors.Is(err, ErrRuntimeExecutorUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *MutationHandler) RestartRuntime(c *gin.Context) {
	if h.runtimeSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "runtime mutation service unavailable"})
		return
	}

	runtimeID := strings.TrimSpace(c.Param("runtimeId"))
	if runtimeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "runtimeId required"})
		return
	}

	result, err := h.runtimeSvc.Restart(c.Request.Context(), runtimeID)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		if errors.Is(err, ErrRuntimeNotGameCenter) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
			return
		}
		if errors.Is(err, ErrRuntimeExecutorUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": err.Error()})
			return
		}
		if isRuntimeExecutorError(err, "runtime not in startable state") {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "runtime not in startable state"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func isRuntimeExecutorError(err error, marker string) bool {
	if err == nil {
		return false
	}
	return len(err.Error()) >= len(marker) && containsSubstring(err.Error(), marker)
}

func containsSubstring(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

package management

import (
"context"
"errors"
"net/http"
"strings"
"time"

	"github.com/gin-gonic/gin"
)

type MutationHandler struct {
	packageSvc *PackageMutationService
	runtimeSvc *RuntimeMutationService
}

func extensionIDFromMutationRequest(c *gin.Context) string {
	extensionID := strings.TrimSpace(c.Param("extensionId"))
	if extensionID != "" {
		return extensionID
	}
	var request struct {
		ExtensionID string `json:"extensionId"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		return ""
	}
	return strings.TrimSpace(request.ExtensionID)
}

func NewMutationHandler(packageSvc *PackageMutationService, runtimeSvc *RuntimeMutationService) *MutationHandler {
	return &MutationHandler{
		packageSvc: packageSvc,
		runtimeSvc: runtimeSvc,
	}
}

func writePackageLifecycleRetired(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"code": 410,
		"msg":  ErrPackageLifecycleRequired.Error(),
		"replacement": gin.H{
			"upload":           "/api/extensions/packages/artifacts",
			"confirm":          "/api/extensions/packages/previews/:sessionId/confirm",
			"install":          "/api/extensions/packages/operations/install",
			"update":           "/api/extensions/packages/operations/update",
			"uninstallPreview": "/api/extensions/kernel/extensions/uninstall/preview",
			"uninstallConfirm": "/api/extensions/kernel/extensions/uninstall/confirm",
			"uninstall":        "/api/extensions/kernel/extensions/uninstall",
		},
	})
}

func (h *MutationHandler) Install(c *gin.Context) {
	writePackageLifecycleRetired(c)
}

func (h *MutationHandler) Update(c *gin.Context) {
	writePackageLifecycleRetired(c)
}

func (h *MutationHandler) Enable(c *gin.Context) {
	if h.packageSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "package mutation service unavailable"})
		return
	}

	extensionID := extensionIDFromMutationRequest(c)
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

	extensionID := extensionIDFromMutationRequest(c)
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
	writePackageLifecycleRetired(c)
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

	startCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 10*time.Minute)
	defer cancel()
	result, err := h.runtimeSvc.Start(startCtx, runtimeID)
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

	restartCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 10*time.Minute)
	defer cancel()
	result, err := h.runtimeSvc.Restart(restartCtx, runtimeID)
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

package desktop

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type ExtensionUpdateMeta struct {
	ExtensionID        string    `json:"extensionId"`
	Version            string    `json:"version"`
	ManifestVersion    int       `json:"manifestVersion"`
	PackageURL         string    `json:"packageUrl"`
	PackageSHA256      string    `json:"packageSha256"`
	PackageSize        int64     `json:"packageSize"`
	PublisherID        string    `json:"publisherId"`
	PublisherKeyID     string    `json:"publisherKeyId,omitempty"`
	Signature          string    `json:"signature,omitempty"`
	MinimumHostVersion string    `json:"minimumHostVersion,omitempty"`
	MaximumHostVersion string    `json:"maximumHostVersion,omitempty"`
	SupportedPlatforms []string  `json:"supportedPlatforms,omitempty"`
	SupportedArch      []string  `json:"supportedArch,omitempty"`
	PublishedAt        time.Time `json:"publishedAt"`
	ReleaseChannel     string    `json:"releaseChannel,omitempty"`
}

type UpdateOperationInfo struct {
	OperationID string    `json:"operationId"`
	ExtensionID string    `json:"extensionId"`
	Status      string    `json:"status"`
	Type        string    `json:"type"`
	Version     string    `json:"version"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Error       string    `json:"error,omitempty"`
}

type UpdateOperationStepInfo struct {
	StepID    string    `json:"stepId"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"startedAt,omitempty"`
	EndedAt   time.Time `json:"endedAt,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type UpdateManagerInterface interface {
	CheckForUpdates(ctx context.Context, extensionID string) ([]ExtensionUpdateMeta, error)
	DownloadUpdate(ctx context.Context, extensionID, version string) (string, error)
	InstallUpdate(ctx context.Context, operationID string) error
	CancelUpdate(ctx context.Context, operationID string) error
	RetryUpdate(ctx context.Context, operationID string) error
	RollbackUpdate(ctx context.Context, operationID string) error
	GetOperation(ctx context.Context, operationID string) (*UpdateOperationInfo, error)
	GetOperationSteps(ctx context.Context, operationID string) ([]UpdateOperationStepInfo, error)
}

type UpdateAPI struct {
	manager UpdateManagerInterface
}

func NewUpdateAPI(manager UpdateManagerInterface) *UpdateAPI {
	return &UpdateAPI{manager: manager}
}

func (api *UpdateAPI) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/extensions/:extensionId/updates", api.ListUpdates)
	group.POST("/extensions/:extensionId/updates/check", api.CheckUpdates)
	group.POST("/extensions/:extensionId/updates/download", api.DownloadUpdate)
	group.POST("/extensions/:extensionId/updates/install", api.InstallUpdate)
	group.POST("/extensions/:extensionId/updates/cancel", api.CancelUpdate)
	group.POST("/extensions/:extensionId/updates/retry", api.RetryUpdate)
	group.POST("/extensions/:extensionId/updates/rollback", api.RollbackUpdate)
	group.GET("/extensions/updates/operations/:operationId", api.GetOperation)
	group.GET("/extensions/updates/operations/:operationId/steps", api.GetOperationSteps)
}

func (api *UpdateAPI) ListUpdates(c *gin.Context) {
	if api.manager == nil {
		apiError(c, http.StatusServiceUnavailable, "update manager unavailable")
		return
	}
	extID := c.Param("extensionId")
	updates, err := api.manager.CheckForUpdates(c.Request.Context(), extID)
	if err != nil {
		apiError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": updates, "total": len(updates)})
}

func (api *UpdateAPI) CheckUpdates(c *gin.Context) {
	if api.manager == nil {
		apiError(c, http.StatusServiceUnavailable, "update manager unavailable")
		return
	}
	extID := c.Param("extensionId")
	updates, err := api.manager.CheckForUpdates(c.Request.Context(), extID)
	if err != nil {
		apiError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"extensionId": extID, "items": updates, "total": len(updates)})
}

func (api *UpdateAPI) DownloadUpdate(c *gin.Context) {
	if api.manager == nil {
		apiError(c, http.StatusServiceUnavailable, "update manager unavailable")
		return
	}
	extID := c.Param("extensionId")
	var req struct {
		Version string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	if req.Version == "" {
		apiError(c, http.StatusBadRequest, "version is required")
		return
	}
	operationID, err := api.manager.DownloadUpdate(c.Request.Context(), extID, req.Version)
	if err != nil {
		apiError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"extensionId": extID, "operationId": operationID, "version": req.Version})
}

func (api *UpdateAPI) InstallUpdate(c *gin.Context) {
	if api.manager == nil {
		apiError(c, http.StatusServiceUnavailable, "update manager unavailable")
		return
	}
	extID := c.Param("extensionId")
	var req struct {
		OperationID string `json:"operationId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	if req.OperationID == "" {
		apiError(c, http.StatusBadRequest, "operationId is required")
		return
	}
	if err := api.manager.InstallUpdate(c.Request.Context(), req.OperationID); err != nil {
		apiError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"extensionId": extID, "operationId": req.OperationID, "status": "installed"})
}

func (api *UpdateAPI) CancelUpdate(c *gin.Context) {
	if api.manager == nil {
		apiError(c, http.StatusServiceUnavailable, "update manager unavailable")
		return
	}
	extID := c.Param("extensionId")
	var req struct {
		OperationID string `json:"operationId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	if req.OperationID == "" {
		apiError(c, http.StatusBadRequest, "operationId is required")
		return
	}
	if err := api.manager.CancelUpdate(c.Request.Context(), req.OperationID); err != nil {
		apiError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"extensionId": extID, "operationId": req.OperationID, "status": "cancelled"})
}

func (api *UpdateAPI) RetryUpdate(c *gin.Context) {
	if api.manager == nil {
		apiError(c, http.StatusServiceUnavailable, "update manager unavailable")
		return
	}
	extID := c.Param("extensionId")
	var req struct {
		OperationID string `json:"operationId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	if req.OperationID == "" {
		apiError(c, http.StatusBadRequest, "operationId is required")
		return
	}
	if err := api.manager.RetryUpdate(c.Request.Context(), req.OperationID); err != nil {
		apiError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"extensionId": extID, "operationId": req.OperationID, "status": "retrying"})
}

func (api *UpdateAPI) RollbackUpdate(c *gin.Context) {
	if api.manager == nil {
		apiError(c, http.StatusServiceUnavailable, "update manager unavailable")
		return
	}
	extID := c.Param("extensionId")
	var req struct {
		OperationID string `json:"operationId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiError(c, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	if req.OperationID == "" {
		apiError(c, http.StatusBadRequest, "operationId is required")
		return
	}
	if err := api.manager.RollbackUpdate(c.Request.Context(), req.OperationID); err != nil {
		apiError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"extensionId": extID, "operationId": req.OperationID, "status": "rolled_back"})
}

func (api *UpdateAPI) GetOperation(c *gin.Context) {
	if api.manager == nil {
		apiError(c, http.StatusServiceUnavailable, "update manager unavailable")
		return
	}
	operationID := c.Param("operationId")
	op, err := api.manager.GetOperation(c.Request.Context(), operationID)
	if err != nil {
		apiError(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, op)
}

func (api *UpdateAPI) GetOperationSteps(c *gin.Context) {
	if api.manager == nil {
		apiError(c, http.StatusServiceUnavailable, "update manager unavailable")
		return
	}
	operationID := c.Param("operationId")
	steps, err := api.manager.GetOperationSteps(c.Request.Context(), operationID)
	if err != nil {
		apiError(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"operationId": operationID, "items": steps, "total": len(steps)})
}

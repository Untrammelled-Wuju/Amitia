package extension

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/package_distribution"
	middlewaresecurity "github.com/u-ai/backend/internal/middleware/security"
)

func registerPackageDistributionRoutes(group *gin.RouterGroup, runtime *Runtime) {
	group.POST("/updates/check", func(c *gin.Context) { checkPackageDistributionUpdate(c, runtime) })
	group.POST("/updates/downloads", func(c *gin.Context) { startPackageDistributionDownload(c, runtime) })
	group.GET("/updates/downloads/:downloadId", func(c *gin.Context) { getPackageDistributionDownload(c, runtime) })
	group.POST("/updates/downloads/:downloadId/cancel", func(c *gin.Context) { cancelPackageDistributionDownload(c, runtime) })
	group.POST("/updates/downloads/:downloadId/retry", func(c *gin.Context) { retryPackageDistributionDownload(c, runtime) })
	group.POST("/updates/previews", func(c *gin.Context) { previewPackageDistributionDownload(c, runtime) })

	group.GET("/versions", func(c *gin.Context) { listPackageVersions(c, runtime) })
	group.GET("/versions/compare", func(c *gin.Context) { comparePackageVersions(c, runtime) })
	group.POST("/versions/rollback/preview", func(c *gin.Context) { previewPackageRollback(c, runtime) })
	group.POST("/versions/rollback/confirm", func(c *gin.Context) { confirmPackageRollback(c, runtime) })
	group.POST("/versions/rollback", func(c *gin.Context) { executePackageRollback(c, runtime) })
}

func packageDistributionService(runtime *Runtime) *package_distribution.Service {
	if runtime == nil || runtime.Kernel == nil {
		return nil
	}
	container := runtime.Kernel.Container()
	if container == nil {
		return nil
	}
	return container.PackageDistribution
}

func checkPackageDistributionUpdate(c *gin.Context, runtime *Runtime) {
	service := packageDistributionService(runtime)
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "package distribution unavailable", "code": "PACKAGE_DISTRIBUTION_UNAVAILABLE"})
		return
	}
	var body struct {
		ExtensionID string `json:"extensionId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.ExtensionID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	currentVersion, err := installedExtensionVersion(c.Request.Context(), runtime, body.ExtensionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "PACKAGE_NOT_INSTALLED"})
		return
	}
	update, err := service.Check(c.Request.Context(), body.ExtensionID, currentVersion)
	if err != nil {
		writePackageDistributionError(c, err)
		return
	}
	if update == nil {
		c.JSON(http.StatusOK, gin.H{"available": false, "extensionId": body.ExtensionID, "currentVersion": currentVersion, "items": []any{}, "total": 0})
		return
	}
	c.JSON(http.StatusOK, gin.H{"available": true, "extensionId": body.ExtensionID, "currentVersion": currentVersion, "update": update, "items": []package_distribution.ExtensionUpdateMetadata{*update}, "total": 1})
}

func startPackageDistributionDownload(c *gin.Context, runtime *Runtime) {
	service := packageDistributionService(runtime)
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "package distribution unavailable", "code": "PACKAGE_DISTRIBUTION_UNAVAILABLE"})
		return
	}
	var body struct {
		ExtensionID string `json:"extensionId"`
		Version     string `json:"version"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if strings.TrimSpace(body.ExtensionID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "extensionId required"})
		return
	}
	currentVersion, err := installedExtensionVersion(c.Request.Context(), runtime, body.ExtensionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "PACKAGE_NOT_INSTALLED"})
		return
	}
	state, err := service.StartDownload(c.Request.Context(), body.ExtensionID, currentVersion, strings.TrimSpace(body.Version))
	if err != nil {
		writePackageDistributionError(c, err)
		return
	}
	c.Header("Location", fmt.Sprintf("/api/extensions/updates/downloads/%s", state.DownloadID))
	c.JSON(http.StatusAccepted, state)
}

func getPackageDistributionDownload(c *gin.Context, runtime *Runtime) {
	service := packageDistributionService(runtime)
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "package distribution unavailable", "code": "PACKAGE_DISTRIBUTION_UNAVAILABLE"})
		return
	}
	state, metadata, err := service.GetDownload(c.Param("downloadId"))
	if err != nil {
		writePackageDistributionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"download": state, "update": metadata})
}

func cancelPackageDistributionDownload(c *gin.Context, runtime *Runtime) {
	service := packageDistributionService(runtime)
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "package distribution unavailable", "code": "PACKAGE_DISTRIBUTION_UNAVAILABLE"})
		return
	}
	if err := service.CancelDownload(c.Param("downloadId")); err != nil {
		writePackageDistributionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"downloadId": c.Param("downloadId"), "status": "cancelled"})
}

func retryPackageDistributionDownload(c *gin.Context, runtime *Runtime) {
	service := packageDistributionService(runtime)
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "package distribution unavailable", "code": "PACKAGE_DISTRIBUTION_UNAVAILABLE"})
		return
	}
	state, err := service.RetryDownload(c.Param("downloadId"))
	if err != nil {
		writePackageDistributionError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, state)
}

func previewPackageDistributionDownload(c *gin.Context, runtime *Runtime) {
	service := packageDistributionService(runtime)
	if service == nil || runtime == nil || runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "package distribution unavailable", "code": "PACKAGE_DISTRIBUTION_UNAVAILABLE"})
		return
	}
	var body struct {
		ExtensionID        string `json:"extensionId"`
		DownloadID         string `json:"downloadId"`
		ScopeType          string `json:"scopeType"`
		ScopeID            string `json:"scopeId"`
		AllowUnsignedDev   bool   `json:"allowUnsignedDev"`
		DeveloperSessionID string `json:"developerSessionId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(body.ExtensionID) == "" || strings.TrimSpace(body.DownloadID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "extensionId and downloadId required"})
		return
	}
	if body.ScopeType == "" {
		body.ScopeType = "global"
	}
	archivePath, metadata, err := service.CompletedDownload(body.DownloadID)
	if err != nil {
		writePackageDistributionError(c, err)
		return
	}
	file, err := os.Open(archivePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": "PACKAGE_DOWNLOAD_UNAVAILABLE"})
		return
	}
	defer file.Close()
	actor := middlewaresecurity.GetActor(c)
	allowUnsignedLocal := actor != nil && actor.IsLocalTrusted
	preview, err := runtime.Kernel.PreviewPackage(c.Request.Context(), kernelruntime.PackagePreviewRequest{
		SpaceID:            kernelAPIUser(c),
		ScopeType:          body.ScopeType,
		ScopeID:            body.ScopeID,
		FileName:           "extension-update.amitiax",
		AllowUnsignedDev:   body.AllowUnsignedDev || allowUnsignedLocal,
		AllowUnsignedLocal: allowUnsignedLocal,
		DeveloperSessionID: body.DeveloperSessionID,
	}, file)
	if err != nil {
		status, code, message := kernelruntime.PackageErrorResponse(err)
		c.JSON(status, gin.H{"error": message, "code": code})
		return
	}
	if preview.ExtensionID != body.ExtensionID || preview.Version != metadata.Version {
		c.JSON(http.StatusConflict, gin.H{"error": "downloaded package identity does not match registry update", "code": "PACKAGE_DISTRIBUTION_IDENTITY_MISMATCH"})
		return
	}
	presentation, err := kernelReadImportSession(c.Request.Context(), runtime, preview.SessionID, kernelAPIUser(c), body.ScopeType, body.ScopeID)
	if err != nil {
		status, code, message := kernelruntime.PackageErrorResponse(err)
		c.JSON(status, gin.H{"error": message, "code": code})
		return
	}
	service.CleanupDownload(body.DownloadID)
	c.JSON(http.StatusCreated, gin.H{
		"artifactId":  preview.ArtifactID,
		"archiveHash": preview.ArchiveHash,
		"downloadId":  body.DownloadID,
		"preview":     presentation,
		"update":      metadata,
	})
}

func installedExtensionVersion(ctx context.Context, runtime *Runtime, extensionID string) (string, error) {
	if runtime == nil || runtime.Kernel == nil {
		return "", errors.New("extension kernel unavailable")
	}
	container := runtime.Kernel.Container()
	if container == nil || container.InstallationRepository == nil {
		return "", errors.New("extension installation repository unavailable")
	}
	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return "", err
	}
	return installation.InstalledVersion.String(), nil
}

func listPackageVersions(c *gin.Context, runtime *Runtime) {
	container := packageKernelContainer(runtime)
	if container == nil || container.PackageRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "extension kernel unavailable"})
		return
	}
	extensionID := strings.TrimSpace(c.Query("extensionId"))
	if extensionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "extensionId required"})
		return
	}
	records, err := container.PackageRepository.ListPackageVersions(c.Request.Context(), extensionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]gin.H, 0, len(records))
	for _, record := range records {
		items = append(items, gin.H{
			"version":              record.Version,
			"artifactId":           record.ArtifactID,
			"packageHash":          record.ArchiveHash,
			"archiveHash":          record.ArchiveHash,
			"manifestHash":         record.ManifestHash,
			"contentTreeHash":      record.ContentTreeHash,
			"installedAt":          record.InstalledAt,
			"operationId":          record.InstallOperationID,
			"active":               record.IsActive,
			"versionState":         record.VersionState,
			"generationId":         record.GenerationID,
			"installedPath":        record.InstalledPath,
			"installedTreeHash":    record.InstalledTreeHash,
			"retainedUntil":        record.RetainedUntil,
			"uninstalledAt":        record.UninstalledAt,
			"uninstallOperationId": record.UninstallOperationID,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

func comparePackageVersions(c *gin.Context, runtime *Runtime) {
	container := packageKernelContainer(runtime)
	if container == nil || container.PackageRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "extension kernel unavailable"})
		return
	}
	fromVersion := strings.TrimSpace(c.Query("from"))
	toVersion := strings.TrimSpace(c.Query("to"))
	extensionID := strings.TrimSpace(c.Query("extensionId"))
	if extensionID == "" || fromVersion == "" || toVersion == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "extensionId, from and to versions are required"})
		return
	}
	comparison, err := container.PackageRepository.ComparePackageVersions(c.Request.Context(), extensionID, fromVersion, toVersion)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, comparison)
}

func previewPackageRollback(c *gin.Context, runtime *Runtime) {
	container := packageKernelContainer(runtime)
	if container == nil || container.PackageRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "extension kernel unavailable"})
		return
	}
	var body struct {
		ExtensionID   string `json:"extensionId"`
		TargetVersion string `json:"targetVersion"`
		ScopeType     string `json:"scopeType"`
		ScopeID       string `json:"scopeId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.ExtensionID) == "" || strings.TrimSpace(body.TargetVersion) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	scopeType := strings.TrimSpace(body.ScopeType)
	if scopeType == "" {
		scopeType = "global"
	}
	preview, err := runtime.Kernel.PreviewPackageRollback(c.Request.Context(), body.ExtensionID, body.TargetVersion, kernelAPIUser(c), scopeType, body.ScopeID)
	if err != nil {
		status, code, message := kernelruntime.PackageErrorResponse(err)
		c.JSON(status, gin.H{"error": message, "code": code})
		return
	}
	c.JSON(http.StatusOK, preview)
}

func confirmPackageRollback(c *gin.Context, runtime *Runtime) {
	if runtime == nil || runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "extension kernel unavailable"})
		return
	}
	var body struct {
		ExtensionID      string          `json:"extensionId"`
		TargetVersion    string          `json:"targetVersion"`
		ScopeType        string          `json:"scopeType"`
		ScopeID          string          `json:"scopeId"`
		PreviewSessionID string          `json:"previewSessionId"`
		Confirmations    map[string]bool `json:"confirmations"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(body.ExtensionID) == "" || strings.TrimSpace(body.TargetVersion) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "extensionId and targetVersion required"})
		return
	}
	if body.ScopeType == "" {
		body.ScopeType = "global"
	}
	confirmation, err := runtime.Kernel.ConfirmPackageRollback(c.Request.Context(), kernelruntime.PackageRollbackConfirmationRequest{
		ExtensionID:      body.ExtensionID,
		TargetVersion:    body.TargetVersion,
		SpaceID:          kernelAPIUser(c),
		ScopeType:        body.ScopeType,
		ScopeID:          body.ScopeID,
		PreviewSessionID: body.PreviewSessionID,
		Confirmations:    body.Confirmations,
	})
	if err != nil {
		status, code, message := kernelruntime.PackageErrorResponse(err)
		c.JSON(status, gin.H{"error": message, "code": code})
		return
	}
	c.JSON(http.StatusOK, confirmation)
}

func executePackageRollback(c *gin.Context, runtime *Runtime) {
	if runtime == nil || runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "extension kernel unavailable"})
		return
	}
	var body struct {
		ExtensionID       string `json:"extensionId"`
		TargetVersion     string `json:"targetVersion"`
		ScopeType         string `json:"scopeType"`
		ScopeID           string `json:"scopeId"`
		ConfirmationToken string `json:"confirmationToken"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(body.ExtensionID) == "" || strings.TrimSpace(body.TargetVersion) == "" || strings.TrimSpace(body.ConfirmationToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "extensionId, targetVersion and confirmationToken required"})
		return
	}
	if body.ScopeType == "" {
		body.ScopeType = "global"
	}
	result, err := runtime.Kernel.ExecutePackageRollback(c.Request.Context(), body.ExtensionID, body.TargetVersion, kernelAPIUser(c), body.ScopeType, body.ScopeID, body.ConfirmationToken)
	if err != nil {
		status, code, message := kernelruntime.PackageErrorResponse(err)
		c.JSON(status, gin.H{"error": message, "code": code})
		return
	}
	c.JSON(http.StatusOK, result)
}

func packageKernelContainer(runtime *Runtime) *kernelruntime.Container {
	if runtime == nil || runtime.Kernel == nil {
		return nil
	}
	return runtime.Kernel.Container()
}

func writePackageDistributionError(c *gin.Context, err error) {
	if errors.Is(err, package_distribution.ErrRegistryNotConfigured) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "code": "EXTENSION_UPDATE_REGISTRY_NOT_CONFIGURED"})
		return
	}
	if errors.Is(err, package_distribution.ErrDownloadNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "PACKAGE_DOWNLOAD_NOT_FOUND"})
		return
	}
	switch package_distribution.ErrorCodeOf(err) {
	case package_distribution.ErrorCodeNoUpdate:
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "NO_UPDATE"})
	case package_distribution.ErrorCodeIndexInvalid, package_distribution.ErrorCodeIndexSignatureInvalid:
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "code": "EXTENSION_UPDATE_INDEX_INVALID"})
	case package_distribution.ErrorCodePackageHashMismatch:
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "code": "PACKAGE_HASH_MISMATCH"})
	case package_distribution.ErrorCodeCompatibilityFailed, package_distribution.ErrorCodePublisherMismatch:
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "EXTENSION_UPDATE_INCOMPATIBLE"})
	case package_distribution.ErrorCodeNetwork:
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "code": "EXTENSION_UPDATE_REGISTRY_UNAVAILABLE"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": "PACKAGE_DISTRIBUTION_FAILED"})
	}
}

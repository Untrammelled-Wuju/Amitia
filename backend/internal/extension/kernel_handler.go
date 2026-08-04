package extension

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
)

const packageAPIReplacement = "/api/extensions/packages/artifacts"

func registerExtensionPackageRoutes(group *gin.RouterGroup, runtime *Runtime) {
	group.GET("/packages/status", func(c *gin.Context) {
		if runtime == nil || runtime.Kernel == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"ready": false})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ready": true, "root": runtime.Kernel.Root(), "installed": len(runtime.Kernel.List())})
	})
	group.GET("/packages", func(c *gin.Context) {
		if runtime == nil || runtime.Kernel == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "extension package service unavailable"})
			return
		}
		c.JSON(http.StatusOK, runtime.Kernel.List())
	})
	group.POST("/packages/artifacts", func(c *gin.Context) { createPackageArtifactPreview(c, runtime) })
	group.POST("/packages/previews", func(c *gin.Context) { createPackageArtifactPreview(c, runtime) })
	group.POST("/packages/previews/:sessionId/confirm", func(c *gin.Context) { confirmPackagePreview(c, runtime) })
	group.POST("/packages/operations/install", func(c *gin.Context) { executePackageInstallOperation(c, runtime) })
	group.POST("/packages/operations/update", func(c *gin.Context) { executePackageUpdateOperation(c, runtime) })
	group.GET("/packages/operations/:operationId", func(c *gin.Context) {
		if runtime == nil || runtime.Kernel == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "extension package service unavailable"})
			return
		}
		operation, err := kernelReadPackageOperation(c.Request.Context(), runtime, kernelAPIUser(c), c.Param("operationId"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, operation)
	})
}

func retiredPackageInstallEndpoint(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{"error_code": "PACKAGE_INSTALL_ENDPOINT_RETIRED", "replacement_endpoint": packageAPIReplacement})
}

func retiredPackagePreviewEndpoint(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{"error_code": "PACKAGE_PREVIEW_ENDPOINT_RETIRED", "replacement_endpoint": packageAPIReplacement})
}

func retiredLegacyPackageEndpoint(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{"error_code": "LEGACY_PACKAGE_ENDPOINT_RETIRED", "replacement_endpoint": packageAPIReplacement})
}

func createPackageArtifactPreview(c *gin.Context, runtime *Runtime) {
	if runtime == nil || runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "extension package service unavailable"})
		return
	}
	maxBody := package_security.DefaultArchivePolicy().MaxArchiveBytes + (1 << 20)
	if c.Request.ContentLength > maxBody {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "package upload exceeds limit"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBody)
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": err.Error()})
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		file, header, err = c.Request.FormFile("package")
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "package file required"})
		return
	}
	defer file.Close()
	request := kernelruntime.PackagePreviewRequest{UserID: kernelAPIUser(c), ScopeType: c.Request.FormValue("scopeType"), ScopeID: c.Request.FormValue("scopeId"), FileName: header.Filename,
		AllowUnsignedDev: strings.EqualFold(c.Request.FormValue("allowUnsignedDev"), "true"), DeveloperSessionID: c.Request.FormValue("developerSessionId")}
	if request.ScopeType == "" {
		request.ScopeType = "global"
	}
	preview, err := runtime.Kernel.PreviewPackage(c.Request.Context(), request, file)
	if err != nil {
		status, code, msg := kernelruntime.PackageErrorResponse(err)
		c.JSON(status, gin.H{"error": msg, "code": code})
		return
	}
	presentation, err := kernelReadImportSession(c.Request.Context(), runtime, preview.SessionID, request.UserID, request.ScopeType, request.ScopeID)
	if err != nil {
		status, code, msg := kernelruntime.PackageErrorResponse(err)
		c.JSON(status, gin.H{"error": msg, "code": code})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"artifactId": preview.ArtifactID, "archiveHash": preview.ArchiveHash, "preview": presentation})
}

func confirmPackagePreview(c *gin.Context, runtime *Runtime) {
	if runtime == nil || runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "extension package service unavailable"})
		return
	}
	var body struct {
		ScopeType     string          `json:"scopeType"`
		ScopeID       string          `json:"scopeId"`
		Confirmations map[string]bool `json:"confirmations"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmation request invalid"})
		return
	}
	if body.ScopeType == "" {
		body.ScopeType = "global"
	}
	confirmation, err := runtime.Kernel.ConfirmPackagePreview(c.Request.Context(), kernelruntime.PackagePreviewConfirmationRequest{SessionID: c.Param("sessionId"), UserID: kernelAPIUser(c), ScopeType: body.ScopeType, ScopeID: body.ScopeID, Confirmations: body.Confirmations})
	if err != nil {
		status, code, msg := kernelruntime.PackageErrorResponse(err)
		c.JSON(status, gin.H{"error": msg, "code": code})
		return
	}
	c.JSON(http.StatusOK, confirmation)
}

func executePackageInstallOperation(c *gin.Context, runtime *Runtime) {
	if runtime == nil || runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "extension package service unavailable"})
		return
	}
	var request kernelruntime.PackageInstallRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "install operation request invalid"})
		return
	}
	request.UserID = kernelAPIUser(c)
	if request.ScopeType == "" {
		request.ScopeType = "global"
	}
	if request.IdempotencyKey == "" {
		request.IdempotencyKey = c.GetHeader("X-Idempotency-Key")
	}
	result, err := runtime.Kernel.ExecutePackageInstall(c.Request.Context(), request)
	if err != nil {
		status, code, msg := kernelruntime.PackageErrorResponse(err)
		c.JSON(status, gin.H{"error": msg, "code": code})
		return
	}
	c.Header("Location", fmt.Sprintf("/api/extensions/packages/operations/%s", result.OperationID))
	c.JSON(http.StatusAccepted, result)
}

func executePackageUpdateOperation(c *gin.Context, runtime *Runtime) {
	if runtime == nil || runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "extension package service unavailable"})
		return
	}
	var request kernelruntime.PackageInstallRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "update operation request invalid"})
		return
	}
	request.UserID = kernelAPIUser(c)
	if request.ScopeType == "" {
		request.ScopeType = "global"
	}
	if request.IdempotencyKey == "" {
		request.IdempotencyKey = c.GetHeader("X-Idempotency-Key")
	}
	result, err := runtime.Kernel.ExecutePackageUpdate(c.Request.Context(), request)
	if err != nil {
		status, code, msg := kernelruntime.PackageErrorResponse(err)
		c.JSON(status, gin.H{"error": msg, "code": code})
		return
	}
	c.Header("Location", fmt.Sprintf("/api/extensions/packages/operations/%s", result.OperationID))
	c.JSON(http.StatusAccepted, result)
}

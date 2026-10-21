package extension

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
)

func (api *KernelAPI) registerAgentSkillRoutes(group *gin.RouterGroup) {
	routes := group.Group("/kernel/agent-skills")
	routes.POST("/import/preview", api.previewAgentSkillImport)
	routes.POST("/import/install", api.installAgentSkillImport)
	routes.GET("", api.listAgentSkills)
	routes.GET("/metrics", api.agentSkillMetrics)
	routes.GET("/:id", api.getAgentSkill)
	routes.POST("/:id/enable", api.enableAgentSkill)
	routes.POST("/:id/disable", api.disableAgentSkill)
	routes.DELETE("/:id", api.removeAgentSkill)
	routes.GET("/:id/compatibility", api.agentSkillCompatibility)
	routes.GET("/:id/resources", api.agentSkillResources)
	routes.GET("/:id/resources/content", api.agentSkillResourceContent)
	routes.GET("/:id/assets/content", api.agentSkillAssetContent)
	routes.GET("/:id/activations", api.agentSkillActivations)
}

func (api *KernelAPI) agentSkillRuntime() (*kernel.AgentSkillRuntime, string, error) {
	if api.runtime == nil || api.runtime.Kernel == nil || api.runtime.Kernel.Container() == nil {
		return nil, "", fmt.Errorf("agent skill service unavailable")
	}
	container := api.runtime.Kernel.Container()
	if container.AgentSkillRuntime == nil {
		return nil, "", fmt.Errorf("agent skill runtime unavailable")
	}
	return container.AgentSkillRuntime, container.ExtRoot, nil
}

func (api *KernelAPI) listAgentSkills(c *gin.Context) {
	service, _, err := api.agentSkillRuntime()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	items, total, err := service.Installed(c.Request.Context(), kernelAPIUser(c), c.Query("characterId"), c.Query("conversationId"), c.Query("query"), c.Query("status"), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (api *KernelAPI) getAgentSkill(c *gin.Context) {
	service, _, err := api.agentSkillRuntime()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	definition, err := service.GetInstalled(c.Request.Context(), kernelAPIUser(c), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	activations, _ := service.ListHistory(c.Request.Context(), kernelAPIUser(c), definition.ExtensionID, 20)
	c.JSON(http.StatusOK, gin.H{"definition": definition, "compatibilityReport": definition.CompatibilityReport, "activations": activations})
}

func (api *KernelAPI) previewAgentSkillImport(c *gin.Context) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent skill service unavailable"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 55<<20)
	if err := c.Request.ParseMultipartForm(55 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent skill import request is invalid"})
		return
	}
	files, rootName, err := readAgentSkillUpload(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	built, err := kernel.BuildLocalAgentSkillPackage(c.Request.Context(), kernelAPIUser(c), rootName, files)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	preview, err := api.runtime.Kernel.PreviewPackage(c.Request.Context(), kernel.PackagePreviewRequest{
		SpaceID: kernelAPIUser(c), ScopeType: "global", FileName: built.FileName,
		AllowUnsignedLocal: true,
	}, bytes.NewReader(built.Archive))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"previewId": preview.SessionID, "definition": built.Definition, "compatibilityReport": built.Definition.CompatibilityReport,
		"files": built.Definition.Resources, "expiresAt": preview.ExpiresAt, "packagePreview": preview,
		"requiredConfirmations": preview.RequiredConfirmations, "riskFlags": preview.RiskFlags,
	})
}

func (api *KernelAPI) installAgentSkillImport(c *gin.Context) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent skill service unavailable"})
		return
	}
	var body struct {
		PreviewID     string          `json:"previewId"`
		Enable        bool            `json:"enable"`
		Confirmations map[string]bool `json:"confirmations"`
		ScopeType     string          `json:"scopeType"`
		ScopeID       string          `json:"scopeId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.PreviewID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "previewId is required"})
		return
	}
	if body.ScopeType == "" {
		body.ScopeType = "global"
	}
	spaceID := kernelAPIUser(c)
	session, err := api.runtime.Kernel.Container().PackageRepository.GetPreview(c.Request.Context(), body.PreviewID, spaceID, body.ScopeType, body.ScopeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent skill preview is unavailable"})
		return
	}
	var required []string
	if err := json.Unmarshal([]byte(session.RequiredConfirmationsJSON), &required); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "agent skill preview is corrupt"})
		return
	}
	confirmations := map[string]bool{}
	for _, item := range required {
		confirmations[item] = body.Confirmations[item]
	}
	confirmation, err := api.runtime.Kernel.ConfirmPackagePreview(c.Request.Context(), kernel.PackagePreviewConfirmationRequest{
		SessionID: body.PreviewID, SpaceID: spaceID, ScopeType: body.ScopeType, ScopeID: body.ScopeID, Confirmations: confirmations,
	})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	result, err := api.runtime.Kernel.ExecutePackageInstall(c.Request.Context(), kernel.PackageInstallRequest{
		SessionID: body.PreviewID, SpaceID: spaceID, ScopeType: body.ScopeType, ScopeID: body.ScopeID,
		ConfirmationToken: confirmation.ConfirmationToken, IdempotencyKey: "agent-skill-import-" + body.PreviewID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if body.Enable {
		if err := api.runtime.Kernel.Enable(c.Request.Context(), result.ExtensionID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	service, _, err := api.agentSkillRuntime()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	definition, err := service.GetInstalled(c.Request.Context(), spaceID, result.ExtensionID)
	if err != nil {
		c.JSON(http.StatusCreated, gin.H{"extensionId": result.ExtensionID, "operationId": result.OperationID})
		return
	}
	c.JSON(http.StatusCreated, definition)
}

func (api *KernelAPI) enableAgentSkill(c *gin.Context) {
	api.setAgentSkillEnabled(c, true)
}

func (api *KernelAPI) disableAgentSkill(c *gin.Context) {
	api.setAgentSkillEnabled(c, false)
}

func (api *KernelAPI) setAgentSkillEnabled(c *gin.Context, enabled bool) {
	service, _, err := api.agentSkillRuntime()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	if err := service.SetEnabled(c.Request.Context(), kernelAPIUser(c), c.Param("id"), enabled); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"enabled": enabled})
}

func (api *KernelAPI) removeAgentSkill(c *gin.Context) {
	service, _, err := api.agentSkillRuntime()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	spaceID := kernelAPIUser(c)
	definition, err := service.GetInstalled(c.Request.Context(), spaceID, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err := api.runtime.Kernel.Disable(c.Request.Context(), definition.ExtensionID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	scopeType := string(definition.Scope)
	if scopeType == "" {
		scopeType = "global"
	}
	preview, err := api.runtime.Kernel.PreviewPackageUninstall(c.Request.Context(), definition.ExtensionID, spaceID, scopeType, definition.ScopeID)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	confirmations := map[string]bool{}
	for _, item := range kernel.RequiredUninstallConfirmations(preview) {
		confirmations[item] = true
	}
	confirmation, err := api.runtime.Kernel.ConfirmPackageUninstall(c.Request.Context(), kernel.ConfirmPackageUninstallRequest{
		ExtensionID: definition.ExtensionID, SpaceID: spaceID, ScopeType: scopeType, ScopeID: definition.ScopeID, Confirmations: confirmations,
	})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if _, err := api.runtime.Kernel.ExecutePackageUninstall(c.Request.Context(), kernel.ExecutePackageUninstallRequest{
		ExtensionID: definition.ExtensionID, SpaceID: spaceID, ScopeType: scopeType, ScopeID: definition.ScopeID,
		ConfirmationToken: confirmation.Token, IdempotencyKey: "agent-skill-remove-" + preview.PreviewHash,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (api *KernelAPI) agentSkillCompatibility(c *gin.Context) {
	service, _, err := api.agentSkillRuntime()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	definition, err := service.GetInstalled(c.Request.Context(), kernelAPIUser(c), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, definition.CompatibilityReport)
}

func (api *KernelAPI) agentSkillResources(c *gin.Context) {
	service, _, err := api.agentSkillRuntime()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	definition, err := service.GetInstalled(c.Request.Context(), kernelAPIUser(c), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	kind := c.Query("kind")
	items := make([]agent_skill.SkillResourceDescriptor, 0)
	for _, item := range definition.Resources {
		if kind == "" || string(item.Kind) == kind {
			items = append(items, item)
		}
	}
	c.JSON(http.StatusOK, items)
}

func (api *KernelAPI) agentSkillResourceContent(c *gin.Context) {
	api.serveAgentSkillContent(c, false)
}

func (api *KernelAPI) agentSkillAssetContent(c *gin.Context) {
	api.serveAgentSkillContent(c, true)
}

func (api *KernelAPI) serveAgentSkillContent(c *gin.Context, assetOnly bool) {
	service, _, err := api.agentSkillRuntime()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	resource, content, err := service.ReadInstalledResource(c.Request.Context(), kernelAPIUser(c), c.Param("id"), c.Query("path"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if assetOnly && resource.Kind != agent_skill.KindAsset {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource is not an asset"})
		return
	}
	if !assetOnly && !resource.TextReadable {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource is not readable as text"})
		return
	}
	if strings.Contains(strings.ToLower(resource.MIMEType), "svg") && unsafeAgentSkillSVG(content) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unsafe SVG is blocked"})
		return
	}
	mimeType := resource.MIMEType
	if mimeType == "" {
		mimeType = mime.TypeByExtension(path.Ext(resource.Path))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Header("Content-Disposition", "attachment; filename=\""+strings.ReplaceAll(path.Base(resource.Path), "\"", "")+"\"")
	c.Data(http.StatusOK, mimeType, content)
}

func (api *KernelAPI) agentSkillActivations(c *gin.Context) {
	service, _, err := api.agentSkillRuntime()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	items, err := service.ListHistory(c.Request.Context(), kernelAPIUser(c), c.Param("id"), 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (api *KernelAPI) agentSkillMetrics(c *gin.Context) {
	service, _, err := api.agentSkillRuntime()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	items, total, err := service.Installed(c.Request.Context(), kernelAPIUser(c), "", "", "", "", 1, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	enabled := 0
	for _, item := range items {
		if item.Enabled {
			enabled++
		}
	}
	c.JSON(http.StatusOK, gin.H{"installed": total, "enabled": enabled})
}

func readAgentSkillUpload(c *gin.Context) (map[string][]byte, string, error) {
	if c.PostForm("source") == "directory" {
		headers := c.Request.MultipartForm.File["files"]
		var paths []string
		if err := json.Unmarshal([]byte(c.PostForm("paths")), &paths); err != nil || len(paths) != len(headers) {
			return nil, "", fmt.Errorf("directory paths are invalid")
		}
		files := map[string][]byte{}
		for index, header := range headers {
			file, err := header.Open()
			if err != nil {
				return nil, "", err
			}
			content, readErr := io.ReadAll(io.LimitReader(file, 20<<20))
			file.Close()
			if readErr != nil {
				return nil, "", readErr
			}
			files[paths[index]] = content
		}
		return files, c.PostForm("rootName"), nil
	}
	header, err := c.FormFile("file")
	if err != nil {
		return nil, "", fmt.Errorf("ZIP file is required")
	}
	file, err := header.Open()
	if err != nil {
		return nil, "", err
	}
	raw, err := io.ReadAll(io.LimitReader(file, 55<<20))
	file.Close()
	if err != nil {
		return nil, "", err
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, "", fmt.Errorf("invalid ZIP archive")
	}
	files := map[string][]byte{}
	var total int64
	for _, entry := range reader.File {
		clean := path.Clean(strings.ReplaceAll(entry.Name, "\\", "/"))
		if clean == "." || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
			return nil, "", fmt.Errorf("archive path is invalid")
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return nil, "", fmt.Errorf("symbolic links are not allowed")
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		if entry.UncompressedSize64 > 20<<20 {
			return nil, "", fmt.Errorf("archive entry exceeds size limit")
		}
		total += int64(entry.UncompressedSize64)
		if total > 50<<20 || len(files) >= 500 {
			return nil, "", fmt.Errorf("archive exceeds size limit")
		}
		rc, err := entry.Open()
		if err != nil {
			return nil, "", err
		}
		content, readErr := io.ReadAll(io.LimitReader(rc, 20<<20))
		rc.Close()
		if readErr != nil {
			return nil, "", readErr
		}
		files[clean] = content
	}
	return files, "", nil
}

func unsafeAgentSkillSVG(content []byte) bool {
	lower := strings.ToLower(string(content))
	for _, needle := range []string{"<script", "javascript:", "data:", "onload=", "onerror=", "http://", "https://", "xlink:href"} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

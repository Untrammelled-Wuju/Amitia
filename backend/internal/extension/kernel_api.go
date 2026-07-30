package extension

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
)

type KernelAPI struct {
	runtime *Runtime
}

func NewKernelAPI(runtime *Runtime) *KernelAPI {
	return &KernelAPI{runtime: runtime}
}

func (api *KernelAPI) RegisterRoutes(group *gin.RouterGroup) {
	kernel := group.Group("/kernel")
	kernel.GET("/extensions", api.listExtensions)
	kernel.GET("/extension", api.getExtension)
	kernel.POST("/extensions/preview", api.previewInstall)
	kernel.POST("/extensions/install", api.install)
	kernel.POST("/extensions/enable", api.enable)
	kernel.POST("/extensions/disable", api.disable)
	kernel.POST("/extensions/uninstall", api.uninstall)
	kernel.POST("/extensions/resume-uninstall", api.resumeUninstall)
	kernel.POST("/extensions/pause", api.pause)
	kernel.POST("/extensions/rollback", api.rollback)
	kernel.GET("/status", api.status)
}

func (api *KernelAPI) status(c *gin.Context) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ready": false})
		return
	}
	container := api.runtime.Kernel.Container()
	if container == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ready": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ready": true,
		"root":  api.runtime.Kernel.Root(),
		"count": len(api.runtime.Kernel.List()),
		"time":  time.Now().UTC().Format(time.RFC3339),
	})
}

func (api *KernelAPI) listExtensions(c *gin.Context) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kernel unavailable"})
		return
	}
	container := api.runtime.Kernel.Container()
	if container == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "container unavailable"})
		return
	}
	ctx := c.Request.Context()
	insts, err := container.InstallationRepository.ListInstallations(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	type extensionItem struct {
		ExtensionID    string    `json:"extensionId"`
		Version        string    `json:"version"`
		InstallationID string    `json:"installationId"`
		State          string    `json:"state"`
		Enablement     string    `json:"enablement"`
		InstalledAt    time.Time `json:"installedAt"`
		UpdatedAt      time.Time `json:"updatedAt"`
		Generation     int64     `json:"generation"`
	}
	items := make([]extensionItem, 0, len(insts))
	for _, inst := range insts {
		items = append(items, extensionItem{
			ExtensionID:    string(inst.ExtensionID),
			Version:        inst.InstalledVersion.String(),
			InstallationID: inst.InstallationID,
			State:          string(inst.InstallationState),
			Enablement:     string(inst.EnablementState),
			InstalledAt:    inst.InstalledAt,
			UpdatedAt:      inst.UpdatedAt,
			Generation:     inst.Generation,
		})
	}
	c.JSON(http.StatusOK, gin.H{"extensions": items, "total": len(items)})
}

func (api *KernelAPI) getExtension(c *gin.Context) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kernel unavailable"})
		return
	}
	container := api.runtime.Kernel.Container()
	if container == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "container unavailable"})
		return
	}
	extID := c.Query("id")
	if extID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id query parameter required"})
		return
	}
	ctx := c.Request.Context()

	inst, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "extension not found"})
		return
	}

	modules, _ := container.ModuleRepository.ListModules(ctx, domain.ExtensionID(extID))
	contribs, _ := container.ContributionRepository.ListContributions(ctx, domain.ExtensionID(extID))

	type moduleInfo struct {
		ID                string `json:"id"`
		Type              string `json:"type"`
		Runtime           string `json:"runtime"`
		EntryPoint        string `json:"entryPoint"`
		ContributionCount int    `json:"contributionCount"`
	}
	type contribInfo struct {
		ID       string `json:"id"`
		Kind     string `json:"kind"`
		ModuleID string `json:"moduleId"`
		Name     string `json:"name"`
	}

	moduleList := make([]moduleInfo, 0, len(modules))
	for _, mod := range modules {
		runtimeType := ""
		entryPoint := ""
		if mod.Runtime != nil {
			runtimeType = string(mod.Runtime.Type)
			entryPoint = mod.Runtime.EntryPoint
		}
		count := 0
		for _, contrib := range contribs {
			if contrib.ModuleID == mod.ID {
				count++
			}
		}
		moduleList = append(moduleList, moduleInfo{
			ID:                string(mod.ID),
			Type:              string(mod.Type),
			Runtime:           runtimeType,
			EntryPoint:        entryPoint,
			ContributionCount: count,
		})
	}

	contribList := make([]contribInfo, 0, len(contribs))
	for _, contrib := range contribs {
		contribList = append(contribList, contribInfo{
			ID:       string(contrib.ID),
			Kind:     string(contrib.Kind),
			ModuleID: string(contrib.ModuleID),
			Name:     contrib.Name.Default,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"extensionId":    string(inst.ExtensionID),
		"version":        inst.InstalledVersion.String(),
		"installationId": inst.InstallationID,
		"state":          string(inst.InstallationState),
		"enablement":     string(inst.EnablementState),
		"installedAt":    inst.InstalledAt,
		"updatedAt":      inst.UpdatedAt,
		"generation":     inst.Generation,
		"modules":        moduleList,
		"contributions":  contribList,
	})
}

func (api *KernelAPI) previewInstall(c *gin.Context) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kernel unavailable"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, package_security.DefaultArchivePolicy().MaxArchiveBytes+(1<<20))
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": err.Error()})
		return
	}
	file, header, err := c.Request.FormFile("package")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer file.Close()
	preview, err := api.runtime.Kernel.PreviewPackage(c.Request.Context(), kernelruntime.PackagePreviewRequest{UserID: kernelAPIUser(c), ScopeType: kernelAPIScopeType(c), ScopeID: c.Request.FormValue("scopeId"), FileName: header.Filename}, file)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, preview)
}

func (api *KernelAPI) install(c *gin.Context) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kernel unavailable"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, package_security.DefaultArchivePolicy().MaxArchiveBytes+(1<<20))
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": err.Error()})
		return
	}
	file, header, err := c.Request.FormFile("package")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer file.Close()
	userID := kernelAPIUser(c)
	scopeType := kernelAPIScopeType(c)
	scopeID := c.Request.FormValue("scopeId")
	preview, err := api.runtime.Kernel.PreviewPackage(c.Request.Context(), kernelruntime.PackagePreviewRequest{UserID: userID, ScopeType: scopeType, ScopeID: scopeID, FileName: header.Filename}, file)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if len(preview.RequiredConfirmations) > 0 {
		c.Header("Deprecation", "true")
		c.JSON(http.StatusConflict, gin.H{"sessionId": preview.SessionID, "requiredConfirmations": preview.RequiredConfirmations, "preview": preview})
		return
	}
	result, err := api.runtime.Kernel.ExecutePackageInstall(c.Request.Context(), kernelruntime.PackageInstallRequest{SessionID: preview.SessionID, UserID: userID, ScopeType: scopeType, ScopeID: scopeID, Confirmations: map[string]bool{}})
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, result)
}

func kernelAPIUser(c *gin.Context) string {
	if value, exists := c.Get("authenticated_user_id"); exists {
		return fmt.Sprint(value)
	}
	return "kernel-api"
}

func kernelAPIScopeType(c *gin.Context) string {
	if value := c.Request.FormValue("scopeType"); value != "" {
		return value
	}
	return "global"
}

func (api *KernelAPI) enable(c *gin.Context) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kernel unavailable"})
		return
	}
	extID := c.Query("id")
	if extID == "" {
		var body struct {
			ID string `json:"id"`
		}
		if err := c.ShouldBindJSON(&body); err == nil && body.ID != "" {
			extID = body.ID
		}
	}
	if extID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	if err := api.runtime.Kernel.Enable(c.Request.Context(), extID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"extensionId": extID, "enablement": "enabled"})
}

func (api *KernelAPI) disable(c *gin.Context) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kernel unavailable"})
		return
	}
	extID := c.Query("id")
	if extID == "" {
		var body struct {
			ID string `json:"id"`
		}
		if err := c.ShouldBindJSON(&body); err == nil && body.ID != "" {
			extID = body.ID
		}
	}
	if extID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	if err := api.runtime.Kernel.Disable(c.Request.Context(), extID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"extensionId": extID, "enablement": "disabled"})
}

func (api *KernelAPI) uninstall(c *gin.Context) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kernel unavailable"})
		return
	}
	extID := c.Query("id")
	scopeType := c.Query("scopeType")
	scopeID := c.Query("scopeId")
	if extID == "" {
		var body struct {
			ID        string `json:"id"`
			ScopeType string `json:"scopeType"`
			ScopeID   string `json:"scopeId"`
		}
		if err := c.ShouldBindJSON(&body); err == nil && body.ID != "" {
			extID = body.ID
			scopeType = body.ScopeType
			scopeID = body.ScopeID
		}
	}
	if extID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	if scopeType == "" {
		scopeType = "global"
	}
	op, err := api.runtime.Kernel.ExecutePackageUninstall(c.Request.Context(), extID, kernelAPIUser(c), scopeType, scopeID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, op)
}

func (api *KernelAPI) resumeUninstall(c *gin.Context) {
	c.Header("Deprecation", "true")
	api.uninstall(c)
}

func (api *KernelAPI) pause(c *gin.Context) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kernel unavailable"})
		return
	}
	extID := c.Query("id")
	if extID == "" {
		var body struct {
			ID string `json:"id"`
		}
		if err := c.ShouldBindJSON(&body); err == nil && body.ID != "" {
			extID = body.ID
		}
	}
	if extID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	if err := api.runtime.Kernel.Disable(c.Request.Context(), extID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"extensionId": extID, "state": "paused"})
}

func (api *KernelAPI) rollback(c *gin.Context) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kernel unavailable"})
		return
	}
	extID := c.Query("id")
	version := c.Query("version")
	scopeType := c.Query("scopeType")
	scopeID := c.Query("scopeId")
	if extID == "" {
		var body struct {
			ID        string `json:"id"`
			Version   string `json:"version"`
			ScopeType string `json:"scopeType"`
			ScopeID   string `json:"scopeId"`
		}
		if err := c.ShouldBindJSON(&body); err == nil && body.ID != "" {
			extID = body.ID
			version = body.Version
			scopeType = body.ScopeType
			scopeID = body.ScopeID
		}
	}
	if extID == "" || version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id and version required"})
		return
	}
	if scopeType == "" {
		scopeType = "global"
	}
	result, err := api.runtime.Kernel.ExecutePackageRollback(c.Request.Context(), extID, version, kernelAPIUser(c), scopeType, scopeID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

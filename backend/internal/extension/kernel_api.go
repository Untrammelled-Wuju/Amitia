package extension

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
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
	kernel.GET("/extensions/:id", api.getExtension)
	kernel.POST("/extensions/preview", api.previewInstall)
	kernel.POST("/extensions/install", api.install)
	kernel.POST("/extensions/:id/enable", api.enable)
	kernel.POST("/extensions/:id/disable", api.disable)
	kernel.DELETE("/extensions/:id", api.uninstall)
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
	extID := c.Param("id")
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
	tempPath, cleanup, err := saveUploadToTemp(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer cleanup()

	preview, err := api.runtime.Kernel.PreviewInstall(c.Request.Context(), tempPath)
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
	tempPath, cleanup, err := saveUploadToTemp(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer cleanup()

	result, err := api.runtime.Kernel.ExecuteInstall(c.Request.Context(), tempPath)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (api *KernelAPI) enable(c *gin.Context) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kernel unavailable"})
		return
	}
	extID := c.Param("id")
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
	extID := c.Param("id")
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
	extID := c.Param("id")
	if err := api.runtime.Kernel.Uninstall(c.Request.Context(), extID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"extensionId": extID, "uninstalled": true})
}

func saveUploadToTemp(c *gin.Context) (string, func(), error) {
	file, err := c.FormFile("package")
	if err != nil {
		return "", nil, err
	}
	temp, err := os.CreateTemp("", "amitiax-*.amitiax")
	if err != nil {
		return "", nil, err
	}
	path := temp.Name()
	temp.Close()
	if err := c.SaveUploadedFile(file, path); err != nil {
		os.Remove(path)
		return "", nil, err
	}
	cleanup := func() { os.Remove(path) }
	return path, cleanup, nil
}

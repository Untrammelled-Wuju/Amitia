package extension

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
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
	kernel.GET("/extension", api.getExtension)
	kernel.POST("/extensions/preview", api.previewInstall)
	kernel.POST("/extensions/install", api.install)
	kernel.POST("/extensions/enable", api.enable)
	kernel.POST("/extensions/disable", api.disable)
	kernel.POST("/extensions/uninstall/preview", api.previewUninstall)
	kernel.POST("/extensions/uninstall/confirm", api.confirmUninstall)
	kernel.POST("/extensions/uninstall", api.uninstall)
	kernel.POST("/extensions/resume-uninstall", api.resumeUninstall)
	kernel.POST("/extensions/pause", api.pause)
	kernel.POST("/extensions/rollback", api.rollback)
	kernel.GET("/status", api.status)
}

type publicUninstallPreviewRequest struct {
	ExtensionID string `json:"extensionId" binding:"required"`
	ScopeType   string `json:"scopeType"`
	ScopeID     string `json:"scopeId"`
}

type publicUninstallPreviewResponse struct {
	ExtensionID             string   `json:"extensionId"`
	CurrentVersion          string   `json:"currentVersion"`
	Enabled                 bool     `json:"enabled"`
	Dependents              []string `json:"dependents"`
	Installable             bool     `json:"uninstallable"`
	ArtifactPolicy          string   `json:"artifactPolicy"`
	PreviewHash             string   `json:"previewHash"`
	SecurityPolicyHash      string   `json:"securityPolicyHash"`
	SnapshotRequirementHash string   `json:"snapshotRequirementHash"`
	RequiredConfirmations   []string `json:"requiredConfirmations"`
}

type publicUninstallConfirmRequest struct {
	ExtensionID   string          `json:"extensionId" binding:"required"`
	ScopeType     string          `json:"scopeType"`
	ScopeID       string          `json:"scopeId"`
	Confirmations map[string]bool `json:"confirmations"`
}

type publicUninstallConfirmResponse struct {
	ConfirmationToken string    `json:"confirmationToken"`
	ExpiresAt         time.Time `json:"expiresAt"`
}

type publicUninstallRequest struct {
	ExtensionID       string `json:"extensionId" binding:"required"`
	ScopeType         string `json:"scopeType"`
	ScopeID           string `json:"scopeId"`
	ConfirmationToken string `json:"confirmationToken" binding:"required"`
	IdempotencyKey    string `json:"idempotencyKey"`
}

type publicResumeUninstallRequest struct {
	ConfirmationToken string `json:"confirmationToken" binding:"required"`
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
		Name           string    `json:"name"`
		ExtensionID    string    `json:"extensionId"`
		Version        string    `json:"version"`
		InstallationID string    `json:"installationId"`
		State          string    `json:"state"`
		Enablement     string    `json:"enablement"`
		SystemManaged  bool      `json:"systemManaged"`
		InstalledAt    time.Time `json:"installedAt"`
		UpdatedAt      time.Time `json:"updatedAt"`
		Generation     int64     `json:"generation"`
	}
	defsByExt := map[domain.ExtensionID][]domain.ExtensionDefinition{}
	if defs, err := container.DefinitionRepository.ListExtensions(ctx); err == nil {
		for _, def := range defs {
			defsByExt[def.ID] = append(defsByExt[def.ID], def)
		}
	}
	resolveName := func(inst domain.ExtensionInstallation) string {
		defs := defsByExt[inst.ExtensionID]
		fallback := ""
		for _, def := range defs {
			if fallback == "" {
				fallback = def.Name.Default
			}
			if def.Version == inst.InstalledVersion {
				return def.Name.Default
			}
		}
		return fallback
	}
	resolveSystemManaged := func(inst domain.ExtensionInstallation) bool {
		for _, def := range defsByExt[inst.ExtensionID] {
			if value, ok := def.Metadata["system.managed"].(bool); ok && value {
				return true
			}
		}
		return strings.HasPrefix(string(inst.ExtensionID), "com.amitia.builtin.")
	}
	items := make([]extensionItem, 0, len(insts))
	for _, inst := range insts {
		items = append(items, extensionItem{
			Name:           resolveName(inst),
			ExtensionID:    string(inst.ExtensionID),
			Version:        inst.InstalledVersion.String(),
			InstallationID: inst.InstallationID,
			State:          installationStateForView(inst),
			Enablement:     string(inst.EnablementState),
			SystemManaged:  resolveSystemManaged(inst),
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
	name := ""
	if def, err := container.DefinitionRepository.GetExtension(ctx, inst.ExtensionID, inst.InstalledVersion); err == nil {
		name = def.Name.Default
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
		"name":           name,
		"extensionId":    string(inst.ExtensionID),
		"version":        inst.InstalledVersion.String(),
		"installationId": inst.InstallationID,
		"state":          installationStateForView(inst),
		"enablement":     string(inst.EnablementState),
		"systemManaged":  isSystemManagedExtension(ctx, container, string(inst.ExtensionID)),
		"installedAt":    inst.InstalledAt,
		"updatedAt":      inst.UpdatedAt,
		"generation":     inst.Generation,
		"modules":        moduleList,
		"contributions":  contribList,
	})
}

func installationStateForView(inst domain.ExtensionInstallation) string {
	if inst.InstallationState == "" {
		return string(domain.InstallationStateInstalled)
	}
	return string(inst.InstallationState)
}

func isSystemManagedExtension(ctx context.Context, container *kernelruntime.Container, extensionID string) bool {
	if strings.HasPrefix(extensionID, "com.amitia.builtin.") {
		return true
	}
	if container == nil || container.DefinitionRepository == nil || container.InstallationRepository == nil {
		return false
	}
	inst, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return false
	}
	def, err := container.DefinitionRepository.GetExtension(ctx, domain.ExtensionID(extensionID), inst.InstalledVersion)
	if err != nil {
		return false
	}
	value, _ := def.Metadata["system.managed"].(bool)
	return value
}

func (api *KernelAPI) previewInstall(c *gin.Context) {
	retiredPackagePreviewEndpoint(c)
}

func (api *KernelAPI) install(c *gin.Context) {
	retiredPackageInstallEndpoint(c)
}

func kernelAPIUser(c *gin.Context) string {
	if value, exists := c.Get(authenticatedUserKey); exists {
		return fmt.Sprint(value)
	}
	return ""
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

func (api *KernelAPI) previewUninstall(c *gin.Context) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kernel unavailable"})
		return
	}
	var req publicUninstallPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}
	scopeType := req.ScopeType
	if scopeType == "" {
		scopeType = "global"
	}
	ctx := c.Request.Context()
	if isSystemManagedExtension(ctx, api.runtime.Kernel.Container(), req.ExtensionID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "系统插件不可卸载", "code": "SYSTEM_EXTENSION_UNINSTALL_FORBIDDEN"})
		return
	}
	kr := api.runtime.Kernel
	preview, err := kr.PreviewPackageUninstall(ctx, req.ExtensionID, kernelAPIUser(c), scopeType, req.ScopeID)
	if err != nil {
		status, code, msg := kernelruntime.PackageErrorResponse(err)
		c.JSON(status, gin.H{"error": msg, "code": code})
		return
	}
	resp := publicUninstallPreviewResponse{
		ExtensionID:             preview.ExtensionID,
		CurrentVersion:          preview.CurrentVersion,
		Enabled:                 preview.Enabled,
		Dependents:              preview.Dependents,
		Installable:             preview.Installable,
		ArtifactPolicy:          string(preview.ArtifactPolicy),
		PreviewHash:             preview.PreviewHash,
		SecurityPolicyHash:      preview.SecurityPolicyHash,
		SnapshotRequirementHash: preview.SnapshotRequirementHash,
		RequiredConfirmations:   kernelruntime.RequiredUninstallConfirmations(preview),
	}
	c.JSON(http.StatusOK, resp)
}

func (api *KernelAPI) confirmUninstall(c *gin.Context) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kernel unavailable"})
		return
	}
	var req publicUninstallConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}
	scopeType := req.ScopeType
	if scopeType == "" {
		scopeType = "global"
	}
	ctx := c.Request.Context()
	if isSystemManagedExtension(ctx, api.runtime.Kernel.Container(), req.ExtensionID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "系统插件不可卸载", "code": "SYSTEM_EXTENSION_UNINSTALL_FORBIDDEN"})
		return
	}
	kr := api.runtime.Kernel
	result, err := kr.ConfirmPackageUninstall(ctx, kernelruntime.ConfirmPackageUninstallRequest{
		ExtensionID:   req.ExtensionID,
		UserID:        kernelAPIUser(c),
		ScopeType:     scopeType,
		ScopeID:       req.ScopeID,
		Confirmations: req.Confirmations,
	})
	if err != nil {
		status, code, msg := kernelruntime.PackageErrorResponse(err)
		c.JSON(status, gin.H{"error": msg, "code": code})
		return
	}
	c.JSON(http.StatusOK, publicUninstallConfirmResponse{ConfirmationToken: result.Token, ExpiresAt: result.ExpiresAt})
}

func (api *KernelAPI) uninstall(c *gin.Context) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kernel unavailable"})
		return
	}
	var req publicUninstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}
	scopeType := req.ScopeType
	if scopeType == "" {
		scopeType = "global"
	}
	if isSystemManagedExtension(c.Request.Context(), api.runtime.Kernel.Container(), req.ExtensionID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "系统插件不可卸载", "code": "SYSTEM_EXTENSION_UNINSTALL_FORBIDDEN"})
		return
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(c.GetHeader("X-Idempotency-Key"))
	}
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	}
	op, err := api.runtime.Kernel.ExecutePackageUninstall(c.Request.Context(), kernelruntime.ExecutePackageUninstallRequest{ExtensionID: req.ExtensionID, UserID: kernelAPIUser(c), ScopeType: scopeType, ScopeID: req.ScopeID, ConfirmationToken: req.ConfirmationToken, IdempotencyKey: idempotencyKey})
	if err != nil {
		status, code, msg := kernelruntime.PackageErrorResponse(err)
		c.JSON(status, gin.H{"error": msg, "code": code})
		return
	}
	c.JSON(http.StatusOK, op)
}

func (api *KernelAPI) resumeUninstall(c *gin.Context) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kernel unavailable"})
		return
	}
	var req publicResumeUninstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}
	claims, err := api.runtime.Kernel.VerifyUninstallConfirmation(req.ConfirmationToken)
	if err != nil {
		status, code, msg := kernelruntime.PackageErrorResponse(err)
		c.JSON(status, gin.H{"error": msg, "code": code})
		return
	}
	op, err := api.runtime.Kernel.ExecutePackageUninstall(c.Request.Context(), kernelruntime.ExecutePackageUninstallRequest{ExtensionID: claims.ExtensionID, UserID: claims.UserID, ScopeType: claims.ScopeType, ScopeID: claims.ScopeID, ConfirmationToken: req.ConfirmationToken})
	if err != nil {
		status, code, msg := kernelruntime.PackageErrorResponse(err)
		c.JSON(status, gin.H{"error": msg, "code": code})
		return
	}
	c.JSON(http.StatusOK, op)
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
	var body struct {
		ID                string `json:"id"`
		Version           string `json:"version"`
		ScopeType         string `json:"scopeType"`
		ScopeID           string `json:"scopeId"`
		ConfirmationToken string `json:"confirmationToken"`
	}
	_ = c.ShouldBindJSON(&body)
	extID := c.Query("id")
	version := c.Query("version")
	scopeType := c.Query("scopeType")
	scopeID := c.Query("scopeId")
	confirmationToken := body.ConfirmationToken
	if extID == "" && body.ID != "" {
		extID = body.ID
		version = body.Version
		scopeType = body.ScopeType
		scopeID = body.ScopeID
		confirmationToken = body.ConfirmationToken
	}
	if extID == "" || version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id and version required"})
		return
	}
	if scopeType == "" {
		scopeType = "global"
	}
	result, err := api.runtime.Kernel.ExecutePackageRollback(c.Request.Context(), extID, version, kernelAPIUser(c), scopeType, scopeID, confirmationToken)
	if err != nil {
		status, code, msg := kernelruntime.PackageErrorResponse(err)
		c.JSON(status, gin.H{"error": msg, "code": code})
		return
	}
	c.JSON(http.StatusOK, result)
}

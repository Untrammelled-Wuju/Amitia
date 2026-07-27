package extension

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/dev_mode"
)

type DevModeAPI struct {
	runtime     *Runtime
	watcher     *dev_mode.FileWatcher
	watchMu     sync.Mutex
	watchEvents map[dev_mode.WorkspaceID]chan dev_mode.FileChangeEvent
}

func NewDevModeAPI(runtime *Runtime) *DevModeAPI {
	return &DevModeAPI{
		runtime:     runtime,
		watcher:     dev_mode.NewFileWatcher(500 * time.Millisecond),
		watchEvents: make(map[dev_mode.WorkspaceID]chan dev_mode.FileChangeEvent),
	}
}

func (api *DevModeAPI) RegisterRoutes(group *gin.RouterGroup) {
	dm := group.Group("/dev-mode")
	dm.POST("/workspaces", api.registerWorkspace)
	dm.GET("/workspaces", api.listWorkspaces)
	dm.GET("/workspaces/:id", api.getWorkspace)
	dm.POST("/workspaces/:id/trust", api.grantTrust)
	dm.DELETE("/workspaces/:id/trust", api.revokeTrust)
	dm.POST("/workspaces/:id/build", api.buildWorkspace)
	dm.POST("/workspaces/:id/reload", api.reloadWorkspace)
	dm.POST("/workspaces/:id/watch/start", api.startWatch)
	dm.POST("/workspaces/:id/watch/stop", api.stopWatch)
	dm.DELETE("/workspaces/:id", api.removeWorkspace)
	dm.POST("/workspaces/:id/sessions", api.openSession)
	dm.DELETE("/workspaces/:id/sessions", api.closeSession)
	dm.GET("/workspaces/:id/revisions", api.listRevisions)
}

func (api *DevModeAPI) resolve(c *gin.Context) (*kernel.Container, bool) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "kernel unavailable"})
		return nil, false
	}
	container := api.runtime.Kernel.Container()
	if container == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "container unavailable"})
		return nil, false
	}
	if container.DevModeRegistry == nil || container.DevModePipeline == nil || container.DevModeReloader == nil || container.DevModeSessions == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "dev mode not initialized"})
		return nil, false
	}
	return container, true
}

func generateWorkspaceID() dev_mode.WorkspaceID {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return dev_mode.WorkspaceID("ws-" + hex.EncodeToString(b))
}

type registerWorkspaceRequest struct {
	ExtensionID   string `json:"extensionId"`
	Path          string `json:"path"`
	ManifestPath  string `json:"manifestPath"`
	WatchEnabled  bool   `json:"watchEnabled"`
	AutoReload    bool   `json:"autoReload"`
}

func (api *DevModeAPI) registerWorkspace(c *gin.Context) {
	container, ok := api.resolve(c)
	if !ok {
		return
	}
	var req registerWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ExtensionID == "" || req.Path == "" || req.ManifestPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "extensionId, path and manifestPath are required"})
		return
	}
	ctx := c.Request.Context()
	ws, err := container.DevModeRegistry.Register(ctx, dev_mode.RegisterWorkspaceInput{
		WorkspaceID:  generateWorkspaceID(),
		ExtensionID:  dev_mode.ExtensionID(req.ExtensionID),
		PathReference: req.Path,
		ManifestPath: req.ManifestPath,
		WatchEnabled: req.WatchEnabled,
		AutoReload:   req.AutoReload,
	})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if ws.AutoReload {
		container.DevModeReloader.Enable(ws.WorkspaceID)
	}
	c.JSON(http.StatusCreated, serializeWorkspace(ws))
}

func (api *DevModeAPI) listWorkspaces(c *gin.Context) {
	container, ok := api.resolve(c)
	if !ok {
		return
	}
	list := container.DevModeRegistry.List()
	items := make([]gin.H, 0, len(list))
	for _, ws := range list {
		items = append(items, serializeWorkspace(ws))
	}
	c.JSON(http.StatusOK, gin.H{"workspaces": items, "total": len(items)})
}

func (api *DevModeAPI) getWorkspace(c *gin.Context) {
	container, ok := api.resolve(c)
	if !ok {
		return
	}
	id := dev_mode.WorkspaceID(c.Param("id"))
	ws, err := container.DevModeRegistry.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	result := serializeWorkspace(ws)
	if currentRev, found := container.DevModePipeline.CurrentRevision(id); found {
		result["currentRevision"] = serializeRevision(currentRev)
	}
	c.JSON(http.StatusOK, result)
}

func (api *DevModeAPI) grantTrust(c *gin.Context) {
	container, ok := api.resolve(c)
	if !ok {
		return
	}
	id := dev_mode.WorkspaceID(c.Param("id"))
	if err := container.DevModeRegistry.GrantDevTrust(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	ws, _ := container.DevModeRegistry.Get(id)
	c.JSON(http.StatusOK, gin.H{"workspaceId": id, "devTrust": true, "workspace": serializeWorkspace(ws)})
}

func (api *DevModeAPI) revokeTrust(c *gin.Context) {
	container, ok := api.resolve(c)
	if !ok {
		return
	}
	id := dev_mode.WorkspaceID(c.Param("id"))
	if err := container.DevModeRegistry.RevokeDevTrust(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	ws, _ := container.DevModeRegistry.Get(id)
	c.JSON(http.StatusOK, gin.H{"workspaceId": id, "devTrust": false, "workspace": serializeWorkspace(ws)})
}

type buildRequest struct {
	SourceMap *bool  `json:"sourceMap"`
	OutDir    string `json:"outDir"`
}

func (api *DevModeAPI) buildWorkspace(c *gin.Context) {
	container, ok := api.resolve(c)
	if !ok {
		return
	}
	id := dev_mode.WorkspaceID(c.Param("id"))
	var req buildRequest
	_ = c.ShouldBindJSON(&req)
	opts := dev_mode.BuildOptions{
		SourceMap:        true,
		Deterministic:    true,
		IncludeResources: true,
		OutDir:           req.OutDir,
	}
	if req.SourceMap != nil {
		opts.SourceMap = *req.SourceMap
	}
	ctx := c.Request.Context()
	rev, err := container.DevModePipeline.Build(ctx, id, opts)
	if err != nil {
		status := http.StatusInternalServerError
		if rev != nil {
			c.JSON(status, gin.H{"error": err.Error(), "revision": serializeRevision(*rev)})
			return
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"revision": serializeRevision(*rev)})
}

type reloadRequest struct {
	Reason string `json:"reason"`
}

func (api *DevModeAPI) reloadWorkspace(c *gin.Context) {
	container, ok := api.resolve(c)
	if !ok {
		return
	}
	id := dev_mode.WorkspaceID(c.Param("id"))
	var req reloadRequest
	_ = c.ShouldBindJSON(&req)
	if req.Reason == "" {
		req.Reason = "manual"
	}
	container.DevModeReloader.Enable(id)
	ctx := c.Request.Context()
	ev, err := container.DevModeReloader.Reload(ctx, id, req.Reason, nil)
	if err != nil {
		status := http.StatusInternalServerError
		if ev != nil {
			c.JSON(status, gin.H{"error": err.Error(), "event": serializeReloadEvent(*ev)})
			return
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"event": serializeReloadEvent(*ev)})
}

func (api *DevModeAPI) startWatch(c *gin.Context) {
	container, ok := api.resolve(c)
	if !ok {
		return
	}
	id := dev_mode.WorkspaceID(c.Param("id"))
	ws, err := container.DevModeRegistry.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	api.watchMu.Lock()
	defer api.watchMu.Unlock()
	if api.watcher.IsRunning(id) {
		c.JSON(http.StatusConflict, gin.H{"error": "watcher already running"})
		return
	}
	events := make(chan dev_mode.FileChangeEvent, 64)
	api.watchEvents[id] = events
	ctx := c.Request.Context()
	if err := api.watcher.Start(ctx, id, ws.PathReference, events); err != nil {
		delete(api.watchEvents, id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	go func(ch chan dev_mode.FileChangeEvent) {
		for range ch {
		}
	}(events)
	c.JSON(http.StatusOK, gin.H{"workspaceId": id, "watching": true})
}

func (api *DevModeAPI) stopWatch(c *gin.Context) {
	id := dev_mode.WorkspaceID(c.Param("id"))
	api.watchMu.Lock()
	ch, hasCh := api.watchEvents[id]
	api.watchMu.Unlock()
	if err := api.watcher.Stop(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if hasCh {
		api.watchMu.Lock()
		delete(api.watchEvents, id)
		api.watchMu.Unlock()
		close(ch)
	}
	c.JSON(http.StatusOK, gin.H{"workspaceId": id, "watching": false})
}

func (api *DevModeAPI) removeWorkspace(c *gin.Context) {
	container, ok := api.resolve(c)
	if !ok {
		return
	}
	id := dev_mode.WorkspaceID(c.Param("id"))
	api.watchMu.Lock()
	if ch, exists := api.watchEvents[id]; exists {
		_ = api.watcher.Stop(id)
		delete(api.watchEvents, id)
		close(ch)
	}
	api.watchMu.Unlock()
	container.DevModeReloader.Disable(id)
	if err := container.DevModeRegistry.Remove(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspaceId": id, "removed": true})
}

type openSessionRequest struct {
	DeviceID  string   `json:"deviceId"`
	UserAgent string   `json:"userAgent"`
	Scopes    []string `json:"scopes"`
}

func (api *DevModeAPI) openSession(c *gin.Context) {
	container, ok := api.resolve(c)
	if !ok {
		return
	}
	id := dev_mode.WorkspaceID(c.Param("id"))
	var req openSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()
	sess, err := container.DevModeSessions.Open(ctx, id, req.DeviceID, req.UserAgent, req.Scopes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, serializeSession(sess))
}

func (api *DevModeAPI) closeSession(c *gin.Context) {
	container, ok := api.resolve(c)
	if !ok {
		return
	}
	id := dev_mode.WorkspaceID(c.Param("id"))
	sess, err := container.DevModeSessions.GetByWorkspace(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err := container.DevModeSessions.Revoke(sess.SessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspaceId": id, "sessionId": sess.SessionID, "revoked": true})
}

func (api *DevModeAPI) listRevisions(c *gin.Context) {
	container, ok := api.resolve(c)
	if !ok {
		return
	}
	id := dev_mode.WorkspaceID(c.Param("id"))
	history := container.DevModePipeline.History(id)
	items := make([]gin.H, 0, len(history))
	for _, rev := range history {
		items = append(items, serializeRevision(rev))
	}
	c.JSON(http.StatusOK, gin.H{"revisions": items, "total": len(items)})
}

func serializeWorkspace(ws *dev_mode.DevelopmentWorkspace) gin.H {
	return gin.H{
		"workspaceId":     string(ws.WorkspaceID),
		"extensionId":     string(ws.ExtensionID),
		"path":            ws.PathReference,
		"manifestPath":    ws.ManifestPath,
		"status":          string(ws.Status),
		"watchEnabled":    ws.WatchEnabled,
		"autoReload":      ws.AutoReload,
		"devTrust":        ws.DevTrust,
		"currentRevision": string(ws.CurrentRevision),
		"createdAt":       ws.CreatedAt,
		"updatedAt":       ws.UpdatedAt,
		"lastReloadAt":    ws.LastReloadAt,
		"failureCount":    ws.FailureCount,
		"lastError":       ws.LastError,
	}
}

func serializeRevision(rev dev_mode.Revision) gin.H {
	errs := make([]gin.H, 0, len(rev.Errors))
	for _, e := range rev.Errors {
		errs = append(errs, gin.H{
			"file":    e.File,
			"line":    e.Line,
			"column":  e.Column,
			"message": e.Message,
			"code":    e.Code,
		})
	}
	return gin.H{
		"revisionId":      string(rev.RevisionID),
		"workspaceId":     string(rev.WorkspaceID),
		"manifestHash":    rev.ManifestHash,
		"sourceHash":      rev.SourceHash,
		"builtAt":         rev.BuiltAt,
		"buildDurationMs": rev.BuildDurationMs,
		"artifactPath":    rev.ArtifactPath,
		"errors":          errs,
		"warnings":        rev.Warnings,
		"status":          string(rev.Status),
		"errorCount":      len(rev.Errors),
	}
}

func serializeReloadEvent(ev dev_mode.ReloadEvent) gin.H {
	return gin.H{
		"workspaceId": string(ev.WorkspaceID),
		"revisionId":  string(ev.RevisionID),
		"stage":       string(ev.Stage),
		"reason":      ev.Reason,
		"startedAt":   ev.StartedAt,
		"completedAt": ev.CompletedAt,
		"success":     ev.Success,
		"error":       ev.Error,
	}
}

func serializeSession(sess *dev_mode.DeveloperSession) gin.H {
	return gin.H{
		"sessionId":   sess.SessionID,
		"workspaceId": string(sess.WorkspaceID),
		"deviceId":    sess.DeviceID,
		"userAgent":   sess.UserAgent,
		"startedAt":   sess.StartedAt,
		"expiresAt":   sess.ExpiresAt,
		"revoked":     sess.Revoked,
		"scopes":      sess.Scopes,
	}
}

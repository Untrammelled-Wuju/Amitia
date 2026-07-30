package extension

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	watchCancel map[dev_mode.WorkspaceID]context.CancelFunc
	sseMu       sync.RWMutex
	sseClients  map[dev_mode.WorkspaceID]map[string]chan dev_mode.ReloadEvent
}

func NewDevModeAPI(runtime *Runtime) *DevModeAPI {
	api := &DevModeAPI{
		runtime:     runtime,
		watcher:     dev_mode.NewFileWatcher(500 * time.Millisecond),
		watchEvents: make(map[dev_mode.WorkspaceID]chan dev_mode.FileChangeEvent),
		watchCancel: make(map[dev_mode.WorkspaceID]context.CancelFunc),
		sseClients:  make(map[dev_mode.WorkspaceID]map[string]chan dev_mode.ReloadEvent),
	}
	if runtime != nil && runtime.Kernel != nil {
		container := runtime.Kernel.Container()
		if container != nil && container.DevModeReloader != nil {
			container.DevModeReloader.SetFileWatcher(api.watcher)
		}
	}
	return api
}

func (api *DevModeAPI) Stop() {
	api.watchMu.Lock()
	for id := range api.watchEvents {
		if cancel, ok := api.watchCancel[id]; ok {
			cancel()
		}
		_ = api.watcher.Stop(id)
		if ch, exists := api.watchEvents[id]; exists {
			close(ch)
		}
		delete(api.watchEvents, id)
		delete(api.watchCancel, id)
	}
	api.watchMu.Unlock()

	api.sseMu.Lock()
	for id, clients := range api.sseClients {
		for clientID, ch := range clients {
			close(ch)
			delete(clients, clientID)
		}
		delete(api.sseClients, id)
	}
	api.sseMu.Unlock()
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
	dm.GET("/workspaces/:id/events", api.streamEvents)
}

func (api *DevModeAPI) resolve(c *gin.Context) (*kernel.Container, bool) {
	if !extensionDevelopmentModeEnabled() {
		c.JSON(http.StatusForbidden, gin.H{"error": "developer mode is disabled"})
		return nil, false
	}
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

func extensionDevelopmentModeEnabled() bool {
	enabled, err := strconv.ParseBool(strings.TrimSpace(os.Getenv("AMITIA_EXTENSION_DEV_MODE")))
	return err == nil && enabled
}

func generateWorkspaceID() dev_mode.WorkspaceID {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return dev_mode.WorkspaceID("ws-" + hex.EncodeToString(b))
}

type registerWorkspaceRequest struct {
	ExtensionID  string `json:"extensionId"`
	Path         string `json:"path"`
	ManifestPath string `json:"manifestPath"`
	WatchEnabled bool   `json:"watchEnabled"`
	AutoReload   bool   `json:"autoReload"`
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
		WorkspaceID:   generateWorkspaceID(),
		ExtensionID:   dev_mode.ExtensionID(req.ExtensionID),
		OwnerUserID:   kernelAPIUser(c),
		PathReference: req.Path,
		ManifestPath:  req.ManifestPath,
		WatchEnabled:  req.WatchEnabled,
		AutoReload:    req.AutoReload,
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
	workspace, err := container.DevModeRegistry.Get(id)
	if err != nil || workspace.OwnerUserID != kernelAPIUser(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "workspace ownership mismatch"})
		return
	}
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
	workspace, err := container.DevModeRegistry.Get(id)
	if err != nil || workspace.OwnerUserID != kernelAPIUser(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "workspace ownership mismatch"})
		return
	}
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

	watchCtx, watchCancel := context.WithCancel(context.Background())
	api.watchCancel[id] = watchCancel

	if err := api.watcher.Start(watchCtx, id, ws.PathReference, events); err != nil {
		delete(api.watchEvents, id)
		delete(api.watchCancel, id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	go api.processFileChanges(watchCtx, container, id, events)

	c.JSON(http.StatusOK, gin.H{"workspaceId": id, "watching": true, "autoReload": ws.AutoReload})
}

func (api *DevModeAPI) processFileChanges(ctx context.Context, container *kernel.Container, id dev_mode.WorkspaceID, events <-chan dev_mode.FileChangeEvent) {
	const debounceInterval = 800 * time.Millisecond
	var debounceTimer *time.Timer
	var pendingChanges int

	flush := func() {
		pendingChanges = 0
		if debounceTimer != nil {
			debounceTimer.Stop()
			debounceTimer = nil
		}
		api.triggerAutoRebuild(ctx, container, id)
	}

	for {
		select {
		case <-ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return
		case _, ok := <-events:
			if !ok {
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				return
			}
			pendingChanges++
			if debounceTimer == nil {
				debounceTimer = time.AfterFunc(debounceInterval, flush)
			} else {
				debounceTimer.Reset(debounceInterval)
			}
		}
	}
}

func (api *DevModeAPI) triggerAutoRebuild(ctx context.Context, container *kernel.Container, id dev_mode.WorkspaceID) {
	ws, err := container.DevModeRegistry.Get(id)
	if err != nil {
		return
	}
	if ws.Status == dev_mode.WorkspaceStatusBuilding || ws.Status == dev_mode.WorkspaceStatusReloading {
		return
	}

	buildCtx, buildCancel := context.WithTimeout(ctx, 120*time.Second)
	defer buildCancel()

	rev, buildErr := container.DevModePipeline.Build(buildCtx, id, dev_mode.BuildOptions{
		SourceMap:        true,
		Deterministic:    true,
		IncludeResources: true,
	})
	if buildErr != nil {
		api.broadcastSSE(id, dev_mode.ReloadEvent{
			WorkspaceID: id,
			Stage:       dev_mode.ReloadStageRebuild,
			Reason:      "auto-rebuild failed",
			StartedAt:   time.Now().UTC(),
			CompletedAt: time.Now().UTC(),
			Success:     false,
			Error:       buildErr.Error(),
		})
		return
	}
	if rev == nil {
		return
	}

	if !ws.AutoReload {
		api.broadcastSSE(id, dev_mode.ReloadEvent{
			WorkspaceID: id,
			RevisionID:  rev.RevisionID,
			Stage:       dev_mode.ReloadStageRebuild,
			Reason:      "auto-rebuild succeeded (auto-reload disabled)",
			StartedAt:   rev.BuiltAt,
			CompletedAt: time.Now().UTC(),
			Success:     true,
		})
		return
	}

	if !container.DevModeReloader.IsEnabled(id) {
		container.DevModeReloader.Enable(id)
	}

	reloadCtx, reloadCancel := context.WithTimeout(ctx, 120*time.Second)
	defer reloadCancel()

	ev, reloadErr := container.DevModeReloader.Reload(reloadCtx, id, "auto-reload after file change", nil)
	if ev != nil {
		api.broadcastSSE(id, *ev)
	}
	if reloadErr != nil && ev == nil {
		api.broadcastSSE(id, dev_mode.ReloadEvent{
			WorkspaceID: id,
			Stage:       dev_mode.ReloadStageLoad,
			Reason:      "auto-reload failed",
			StartedAt:   time.Now().UTC(),
			CompletedAt: time.Now().UTC(),
			Success:     false,
			Error:       reloadErr.Error(),
		})
	}
}

func (api *DevModeAPI) broadcastSSE(id dev_mode.WorkspaceID, ev dev_mode.ReloadEvent) {
	api.sseMu.RLock()
	clients, exists := api.sseClients[id]
	api.sseMu.RUnlock()
	if !exists || len(clients) == 0 {
		return
	}
	api.sseMu.RLock()
	snapshot := make([]chan dev_mode.ReloadEvent, 0, len(clients))
	for _, ch := range clients {
		snapshot = append(snapshot, ch)
	}
	api.sseMu.RUnlock()

	for _, ch := range snapshot {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (api *DevModeAPI) stopWatch(c *gin.Context) {
	id := dev_mode.WorkspaceID(c.Param("id"))
	api.watchMu.Lock()
	if cancel, ok := api.watchCancel[id]; ok {
		cancel()
		delete(api.watchCancel, id)
	}
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
	ctx := c.Request.Context()

	api.watchMu.Lock()
	if cancel, exists := api.watchCancel[id]; exists {
		cancel()
		delete(api.watchCancel, id)
	}
	if ch, exists := api.watchEvents[id]; exists {
		delete(api.watchEvents, id)
		close(ch)
	}
	api.watchMu.Unlock()

	if err := container.DevModeReloader.CloseDevMode(ctx, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "workspaceId": id, "removed": true})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspaceId": id, "removed": true})
}

type openSessionRequest struct {
	DeviceID  string `json:"deviceId"`
	UserAgent string `json:"userAgent"`
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
	workspace, err := container.DevModeRegistry.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	userID := kernelAPIUser(c)
	if workspace.OwnerUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "workspace ownership mismatch"})
		return
	}
	if !workspace.DevTrust {
		c.JSON(http.StatusForbidden, gin.H{"error": "developer trust is required"})
		return
	}
	sess, err := container.DevModeSessions.Open(ctx, id, workspace.ExtensionID, userID, req.DeviceID, req.UserAgent, kernel.CurrentPackagePolicyVersion(), workspace.DevTrust, workspace.DevTrustVersion)
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
	workspace, workspaceErr := container.DevModeRegistry.Get(id)
	if workspaceErr != nil || workspace.OwnerUserID != kernelAPIUser(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "workspace ownership mismatch"})
		return
	}
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

func (api *DevModeAPI) streamEvents(c *gin.Context) {
	id := dev_mode.WorkspaceID(c.Param("id"))

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	clientID := fmt.Sprintf("client-%d", time.Now().UnixNano())
	eventCh := make(chan dev_mode.ReloadEvent, 32)

	api.sseMu.Lock()
	if api.sseClients[id] == nil {
		api.sseClients[id] = make(map[string]chan dev_mode.ReloadEvent)
	}
	api.sseClients[id][clientID] = eventCh
	api.sseMu.Unlock()

	defer func() {
		api.sseMu.Lock()
		if clients, exists := api.sseClients[id]; exists {
			if ch, exists := clients[clientID]; exists {
				close(ch)
				delete(clients, clientID)
			}
			if len(clients) == 0 {
				delete(api.sseClients, id)
			}
		}
		api.sseMu.Unlock()
	}()

	_, _ = fmt.Fprintf(c.Writer, "event: connected\ndata: {\"workspaceId\":\"%s\",\"clientId\":\"%s\"}\n\n", id, clientID)
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	notify := c.Request.Context().Done()

	for {
		select {
		case <-notify:
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprintf(c.Writer, ": heartbeat\n\n")
			flusher.Flush()
		case ev, ok := <-eventCh:
			if !ok {
				return
			}
			data, _ := json.Marshal(serializeReloadEvent(ev))
			_, _ = fmt.Fprintf(c.Writer, "event: reload\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
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

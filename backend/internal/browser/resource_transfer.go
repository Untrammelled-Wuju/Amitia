package browser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type productionResourceTransfer struct {
	tabs     TabResolver
	dom      DOMBackend
	input    InputBackend
	elements *elementStore
	policy   *InteractionPolicy
	tabMgr   *productionTabManager

	downloadPolicy   DownloadPolicy
	screenshotPolicy ScreenshotPolicy
	uploadPolicy     UploadPolicy

	mu        sync.RWMutex
	downloads map[BrowserDownloadID]*downloadRecord
	guidMap   map[string]BrowserDownloadID
	subscribed bool
}

func NewProductionResourceTransfer(
	tabs TabResolver,
	dom DOMBackend,
	elements *elementStore,
	policy *InteractionPolicy,
	tabMgr *productionTabManager,
) BrowserResourceTransfer {
	return &productionResourceTransfer{
		tabs:             tabs,
		dom:              dom,
		elements:         elements,
		policy:           policy,
		tabMgr:           tabMgr,
		downloadPolicy:   DefaultDownloadPolicy(),
		screenshotPolicy: DefaultScreenshotPolicy(),
		uploadPolicy:     DefaultUploadPolicy(),
		downloads:        make(map[BrowserDownloadID]*downloadRecord),
		guidMap:          make(map[string]BrowserDownloadID),
	}
}

func (r *productionResourceTransfer) ensureDownloadSubscription() {
	r.mu.Lock()
	if r.subscribed {
		r.mu.Unlock()
		return
	}
	r.subscribed = true
	r.mu.Unlock()

	client := r.dom.GetClient()
	if client == nil {
		return
	}

	client.SubscribeEvent("Browser.downloadWillBegin", func(params json.RawMessage) {
		var evt struct {
			FrameID           string `json:"frameId"`
			GUID              string `json:"guid"`
			URL               string `json:"url"`
			SuggestedFilename string `json:"suggestedFilename"`
		}
		if err := json.Unmarshal(params, &evt); err != nil || evt.GUID == "" {
			return
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		for id, rec := range r.downloads {
			if rec.state == DownloadStatePending {
				if rec.suggestedFilename == "" || rec.suggestedFilename == evt.SuggestedFilename {
					r.guidMap[evt.GUID] = id
					rec.state = DownloadStateInProgress
					rec.guid = evt.GUID
					if rec.suggestedFilename == "" && evt.SuggestedFilename != "" {
						rec.suggestedFilename = evt.SuggestedFilename
					}
					break
				}
			}
		}
	})

	client.SubscribeEvent("Browser.downloadProgress", func(params json.RawMessage) {
		var evt struct {
			GUID         string  `json:"guid"`
			TotalBytes   float64 `json:"totalBytes"`
			ReceivedBytes float64 `json:"receivedBytes"`
			State        string  `json:"state"`
		}
		if err := json.Unmarshal(params, &evt); err != nil || evt.GUID == "" {
			return
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		downloadID, ok := r.guidMap[evt.GUID]
		if !ok {
			return
		}
		rec, ok := r.downloads[downloadID]
		if !ok {
			return
		}
		switch evt.State {
		case "inProgress":
			rec.state = DownloadStateInProgress
			rec.totalBytes = int64(evt.TotalBytes)
			rec.receivedBytes = int64(evt.ReceivedBytes)
		case "completed":
			rec.state = DownloadStateCompleted
			rec.receivedBytes = int64(evt.ReceivedBytes)
			rec.totalBytes = int64(evt.TotalBytes)
			now := time.Now()
			rec.completedAt = &now
			rec.stagedPath = filepath.Join(r.downloadPolicy.StagingRootPath, rec.suggestedFilename)
			delete(r.guidMap, evt.GUID)
		case "canceled":
			rec.state = DownloadStateCancelled
			delete(r.guidMap, evt.GUID)
		}
	})
}

func (r *productionResourceTransfer) configureDownloadBehavior(ctx context.Context, targetID TargetID) error {
	client := r.dom.GetClient()
	if client == nil {
		return fmt.Errorf("CDP client not available")
	}
	sessionID := r.dom.GetSession(targetID)
	if sessionID == "" {
		return fmt.Errorf("no session for target")
	}
	params := map[string]interface{}{
		"behavior":     r.downloadPolicy.Behavior,
		"downloadPath": r.downloadPolicy.StagingRootPath,
	}
	return client.Call(ctx, "Page.setDownloadBehavior", sessionID, params, nil)
}

func (r *productionResourceTransfer) Download(ctx context.Context, request BrowserDownloadRequest) (*BrowserDownloadResult, *BrowserError) {
	if err := ctx.Err(); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	if request.SessionID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "session ID is required",
		}
	}

	if request.TabID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "tab ID is required",
		}
	}

	if request.ResourceURI == "" {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "resource URI is required",
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	resolved, err := r.tabs.ResolveTab(ctx, request.SessionID, request.TabID)
	if err != nil {
		return nil, err
	}

	r.ensureDownloadSubscription()
	if r.downloadPolicy.StagingRootPath != "" {
		if err := r.configureDownloadBehavior(ctx, resolved.TargetID); err != nil {
			return nil, &BrowserError{
				Code:    ErrCodeDownloadFailed,
				Message: "failed to configure download behavior",
				Cause:   err,
			}
		}
	}

	if request.TriggerElement != nil && request.TriggerElement.StableID != "" {
		record, ok := r.elements.get(request.TriggerElement.StableID)
		if !ok {
			return nil, &BrowserError{
				Code:    ErrCodeElementNotFound,
				Message: "trigger element ref not found",
			}
		}

		if record.sessionID != request.SessionID || record.tabID != request.TabID {
			return nil, &BrowserError{
				Code:    ErrCodeStaleElement,
				Message: "trigger element does not belong to this session/tab",
			}
		}

		if record.runtimeGeneration != resolved.RuntimeGeneration {
			r.elements.remove(request.TriggerElement.StableID)
			return nil, &BrowserError{
				Code:    ErrCodeStaleElement,
				Message: "trigger element belongs to stale runtime generation",
			}
		}

		if err := r.dom.EnableDOM(ctx, resolved.TargetID); err != nil {
			return nil, &BrowserError{
				Code:    ErrCodeDownloadFailed,
				Message: "failed to enable DOM",
				Cause:   err,
			}
		}

		if err := r.dom.ScrollIntoView(ctx, resolved.TargetID, record.backendNodeID); err != nil {
			return nil, &BrowserError{
				Code:    ErrCodeDownloadFailed,
				Message: "failed to scroll trigger element into view",
				Cause:   err,
			}
		}

		if err := r.input.DispatchMouseMove(ctx, resolved.TargetID, 0, 0); err != nil {
			return nil, &BrowserError{
				Code:    ErrCodeDownloadFailed,
				Message: "failed to move mouse for download trigger",
				Cause:   err,
			}
		}
	}

	var filename string
	if request.Filename != "" {
		filename = SanitizeFilename(request.Filename)
	}

	downloadID := BrowserDownloadID("bd_" + uuid.New().String())
	rec := &downloadRecord{
		id:                downloadID,
		sessionID:         request.SessionID,
		tabID:             request.TabID,
		runtimeGeneration: resolved.RuntimeGeneration,
		state:             DownloadStatePending,
		startedAt:         time.Now(),
		suggestedFilename: filename,
	}
	r.downloads[downloadID] = rec

	resultFilename := filename
	if resultFilename == "" {
		resultFilename = rec.suggestedFilename
	}

	return &BrowserDownloadResult{
		ResourceURI: request.ResourceURI,
		Filename:    resultFilename,
		DownloadID:  string(downloadID),
	}, nil
}

func (r *productionResourceTransfer) Upload(ctx context.Context, request BrowserUploadRequest) (*BrowserUploadResult, *BrowserError) {
	if err := ctx.Err(); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	if request.SessionID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "session ID is required",
		}
	}

	if request.TabID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "tab ID is required",
		}
	}

	if request.ResourceURI == "" {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "resource URI is required",
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	resolved, err := r.tabs.ResolveTab(ctx, request.SessionID, request.TabID)
	if err != nil {
		return nil, err
	}

	var targetElement *BrowserElementRef
	if request.Element != nil {
		targetElement = request.Element
	} else if request.TargetInput != "" {
		return nil, &BrowserError{
			Code:    ErrCodeUploadTargetNotFileInput,
			Message: "legacy TargetInput not supported, use Element",
		}
	} else {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "target element is required",
		}
	}

	record, ok := r.elements.get(targetElement.StableID)
	if !ok {
		return nil, &BrowserError{
			Code:    ErrCodeElementNotFound,
			Message: "upload target element ref not found",
		}
	}

	if record.sessionID != request.SessionID || record.tabID != request.TabID {
		return nil, &BrowserError{
			Code:    ErrCodeStaleElement,
			Message: "upload target element does not belong to this session/tab",
		}
	}

	if record.runtimeGeneration != resolved.RuntimeGeneration {
		r.elements.remove(targetElement.StableID)
		return nil, &BrowserError{
			Code:    ErrCodeStaleElement,
			Message: "upload target element belongs to stale runtime generation",
		}
	}

	if !record.isFileInput {
		return nil, &BrowserError{
			Code:    ErrCodeUploadTargetNotFileInput,
			Message: "upload target must be input[type=file]",
		}
	}

	if record.disabled {
		return nil, &BrowserError{
			Code:    ErrCodeUploadTargetNotFileInput,
			Message: "upload target is disabled",
		}
	}

	if err := r.dom.EnableDOM(ctx, resolved.TargetID); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeUploadFailed,
			Message: "failed to enable DOM",
			Cause:   err,
		}
	}

	sourcePath := request.ResourceURI
	if strings.HasPrefix(sourcePath, "file://") {
		sourcePath = strings.TrimPrefix(sourcePath, "file://")
	}

	if err := r.dom.SetFileInputFiles(ctx, resolved.TargetID, record.backendNodeID, []string{sourcePath}); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeUploadFailed,
			Message: "failed to set file input files",
			Cause:   err,
		}
	}

	return &BrowserUploadResult{
		ResourceURI:    request.ResourceURI,
		Success:        true,
		TargetStableID: record.stableID,
		FileSet:        true,
		Verified:       true,
	}, nil
}

func (r *productionResourceTransfer) Screenshot(ctx context.Context, request BrowserScreenshotRequest) (*BrowserScreenshotResult, *BrowserError) {
	if err := ctx.Err(); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	if request.SessionID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "session ID is required",
		}
	}

	if request.TabID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "tab ID is required",
		}
	}

	format := request.Format
	if format == "" {
		format = r.screenshotPolicy.DefaultFormat
	}

	if !IsValidScreenshotFormat(format) {
		return nil, &BrowserError{
			Code:    ErrCodeScreenshotInvalidFormat,
			Message: "unsupported screenshot format: " + format,
		}
	}

	if !IsValidScreenshotQuality(format, request.Quality) {
		return nil, &BrowserError{
			Code:    ErrCodeScreenshotInvalidQuality,
			Message: "invalid screenshot quality for format " + format,
		}
	}

	if request.FullPage && !r.screenshotPolicy.AllowFullPage {
		return nil, &BrowserError{
			Code:    ErrCodeScreenshotFailed,
			Message: "full page screenshot is not allowed",
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	resolved, err := r.tabs.ResolveTab(ctx, request.SessionID, request.TabID)
	if err != nil {
		return nil, err
	}

	if err := r.dom.EnableDOM(ctx, resolved.TargetID); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeScreenshotFailed,
			Message: "failed to enable DOM",
			Cause:   err,
		}
	}

	client := r.dom.GetClient()
	if client == nil {
		return nil, &BrowserError{
			Code:    ErrCodeScreenshotFailed,
			Message: "CDP client not available",
		}
	}

	sessionID := r.dom.GetSession(resolved.TargetID)
	if sessionID == "" {
		if engine := r.tabMgr.engine(); engine != nil {
			pc := engine.Pages().(*chromiumPageController)
			sessionID = pc.ensureSession(ctx, client, resolved.TargetID)
		}
	}
	if sessionID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeScreenshotFailed,
			Message: "failed to get CDP session",
		}
	}

	params := map[string]interface{}{
		"format": format,
	}
	if format != ScreenshotFormatPNG {
		if request.Quality > 0 {
			params["quality"] = request.Quality
		} else {
			params["quality"] = 80
		}
	}
	if request.FullPage {
		params["captureBeyondViewport"] = true
	}

	var result struct {
		Data string `json:"data"`
	}
	if err := client.Call(ctx, "Page.captureScreenshot", sessionID, params, &result); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeScreenshotFailed,
			Message: "Page.captureScreenshot failed",
			Cause:   err,
		}
	}

	if result.Data == "" {
		return nil, &BrowserError{
			Code:    ErrCodeScreenshotFailed,
			Message: "empty screenshot data",
		}
	}

	imgData, decodeErr := base64.StdEncoding.DecodeString(result.Data)
	if decodeErr != nil {
		return nil, &BrowserError{
			Code:    ErrCodeScreenshotFailed,
			Message: "failed to decode screenshot data",
			Cause:   decodeErr,
		}
	}

	var docGen uint64
	if r.tabMgr != nil {
		if rec, ok := r.tabMgr.store.get(request.TabID); ok {
			docGen = rec.documentGeneration
		}
	}

	stagedPath := ""
	if r.screenshotPolicy.StagingRootPath != "" {
		targetDir := filepath.Join(r.screenshotPolicy.StagingRootPath, "screenshots")
		if err := os.MkdirAll(targetDir, 0700); err == nil {
			stagedPath = filepath.Join(targetDir, fmt.Sprintf("screenshot_%s.%s", uuid.New().String()[:8], format))
			if err := os.WriteFile(stagedPath, imgData, 0600); err == nil {
				stagedPath = "file://" + stagedPath
			} else {
				stagedPath = ""
			}
		}
	}

	return &BrowserScreenshotResult{
		ResourceURI:        stagedPath,
		Format:             format,
		SizeBytes:          int64(len(imgData)),
		RuntimeGeneration:  resolved.RuntimeGeneration,
		DocumentGeneration: docGen,
	}, nil
}

func (r *productionResourceTransfer) invalidateDownloadsForGeneration(generation uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, rec := range r.downloads {
		if rec.runtimeGeneration == generation {
			rec.state = DownloadStateFailed
			delete(r.downloads, id)
		}
	}
}

func (r *productionResourceTransfer) invalidateAllDownloads() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, rec := range r.downloads {
		rec.state = DownloadStateCancelled
		delete(r.downloads, id)
	}
}

func (r *productionResourceTransfer) ClaimDownload(downloadID BrowserDownloadID) (*downloadRecord, *BrowserError) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec, ok := r.downloads[downloadID]
	if !ok {
		return nil, &BrowserError{
			Code:    ErrCodeDownloadNotStarted,
			Message: "download not found",
		}
	}

	if rec.claimed {
		return nil, &BrowserError{
			Code:    ErrCodeDownloadAmbiguous,
			Message: "download already claimed",
		}
	}

	if rec.state != DownloadStateCompleted {
		return nil, &BrowserError{
			Code:    ErrCodeDownloadOutcomeUnknown,
			Message: "download not in completed state: " + rec.state,
		}
	}

	rec.claimed = true
	return rec, nil
}

func (r *productionResourceTransfer) FindPendingDownload(sessionID BrowserSessionID, tabID BrowserTabID) (*downloadRecord, *BrowserError) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var found *downloadRecord
	count := 0
	for _, rec := range r.downloads {
		if rec.sessionID == sessionID && rec.tabID == tabID && !rec.claimed && (rec.state == DownloadStatePending || rec.state == DownloadStateInProgress || rec.state == DownloadStateCompleted) {
			found = rec
			count++
		}
	}

	if count == 0 {
		return nil, &BrowserError{
			Code:    ErrCodeDownloadNotStarted,
			Message: "no pending download found",
		}
	}

	if count > 1 {
		return nil, &BrowserError{
			Code:    ErrCodeDownloadAmbiguous,
			Message: "multiple pending downloads found",
		}
	}

	return found, nil
}

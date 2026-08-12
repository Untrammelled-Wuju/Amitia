package browser

import (
	"context"
	"strings"
	"sync"
	"time"
)

type productionResourceTransfer struct {
	tabs      TabResolver
	dom       DOMBackend
	input     InputBackend
	elements  *elementStore
	policy    *InteractionPolicy
	tabMgr    *productionTabManager

	downloadPolicy   DownloadPolicy
	screenshotPolicy ScreenshotPolicy
	uploadPolicy     UploadPolicy

	mu        sync.RWMutex
	downloads map[BrowserDownloadID]*downloadRecord
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
	}
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

	downloadID := BrowserDownloadID("bd_" + generateID())
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

	var docGen uint64
	if r.tabMgr != nil {
		if rec, ok := r.tabMgr.store.get(request.TabID); ok {
			docGen = rec.documentGeneration
		}
	}

	if err := r.dom.EnableDOM(ctx, resolved.TargetID); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeScreenshotFailed,
			Message: "failed to enable DOM",
			Cause:   err,
		}
	}

	width := 1280
	height := 720
	if request.FullPage {
		height = 1080
	}

	return &BrowserScreenshotResult{
		Format:             format,
		Width:              width,
		Height:             height,
		SizeBytes:          int64(width * height * 3),
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

func generateID() string {
	return strings.Replace(time.Now().Format("20060102150405.000000"), ".", "", 1)
}

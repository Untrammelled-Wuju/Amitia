package media

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/iosnative/staging"
	"github.com/u-ai/backend/internal/nativebridge"
)

type MediaHandler struct {
	bridge   nativebridge.Bridge
	importer *staging.StagingImporter
}

func NewMediaHandler(bridge nativebridge.Bridge, importer *staging.StagingImporter) *MediaHandler {
	return &MediaHandler{bridge: bridge, importer: importer}
}

func (h *MediaHandler) Execute(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationPhotosPick:
		return h.handlePhotosPick(ctx, request)
	case OperationPhotosStatus:
		return h.handlePhotosStatus(ctx, request)
	case OperationPhotosList:
		return h.handlePhotosList(ctx, request)
	case OperationPhotosGet:
		return h.handlePhotosGet(ctx, request)
	case OperationPhotosExport:
		return h.handlePhotosExport(ctx, request)
	case OperationPhotosSave:
		return h.handlePhotosSave(ctx, request)
	case OperationPhotosDelete:
		return h.handlePhotosDelete(ctx, request)
	case OperationPhotosManageLimited:
		return h.handlePhotosManageLimited(ctx, request)
	case OperationCameraStatus:
		return h.handleCameraStatus(ctx, request)
	case OperationCameraDevices:
		return h.handleCameraDevices(ctx, request)
	case OperationCameraCapturePhoto:
		return h.handleCameraCapturePhoto(ctx, request)
	case OperationCameraRecordVideo:
		return h.handleCameraRecordVideo(ctx, request)
	case OperationAudioStatus:
		return h.handleAudioStatus(ctx, request)
	case OperationAudioRecord:
		return h.handleAudioRecord(ctx, request)
	case OperationStagingImport:
		return h.handleStagingImport(ctx, request)
	default:
		return NewMediaError(request, nativebridge.ErrOperationNotSupported, fmt.Sprintf("unsupported operation: %s", request.Operation))
	}
}

func (h *MediaHandler) bridgeCall(ctx context.Context, request nativebridge.Request, operation string, payload map[string]any) nativebridge.Response {
	if h.bridge == nil {
		return NewMediaError(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}
	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       operation,
			Payload:         payload,
		})
		if err != nil {
			done <- NewMediaError(request, ErrOutcomeUnknown, err.Error())
			return
		}
		done <- resp
	}()
	select {
	case <-ctx.Done():
		return NewMediaError(request, ErrTimeout, operation+" cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *MediaHandler) handleStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationStatus, map[string]any{})
}

func (h *MediaHandler) handlePhotosPick(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	payload := map[string]any{}

	if kinds, ok := request.Payload["kinds"].([]any); ok {
		validKinds := make([]string, 0, len(kinds))
		for _, k := range kinds {
			if s, ok := k.(string); ok && IsValidPickerKind(s) {
				validKinds = append(validKinds, s)
			}
		}
		if len(validKinds) == 0 {
			validKinds = []string{"image"}
		}
		payload["kinds"] = validKinds
	} else {
		payload["kinds"] = []string{"image"}
	}

	if selectionLimit, ok := request.Payload["selectionLimit"].(float64); ok {
		payload["selectionLimit"] = ClampSelectionLimit(int(selectionLimit))
	} else {
		payload["selectionLimit"] = DefaultSelectionLimit
	}

	if ordered, ok := request.Payload["ordered"].(bool); ok {
		payload["ordered"] = ordered
	}

	if preferredEncoding, ok := request.Payload["preferredEncoding"].(string); ok && preferredEncoding != "" {
		payload["preferredEncoding"] = preferredEncoding
	}

	if maxTotalBytes, ok := request.Payload["maxTotalBytes"].(float64); ok {
		payload["maxTotalBytes"] = ClampMaxTotalBytes(int64(maxTotalBytes))
	} else {
		payload["maxTotalBytes"] = DefaultMaxTotalBytes
	}

	return h.bridgeCall(ctx, request, OperationPhotosPick, payload)
}

func (h *MediaHandler) handlePhotosStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationPhotosStatus, map[string]any{})
}

func (h *MediaHandler) handlePhotosList(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	payload := map[string]any{}

	if mediaType, ok := request.Payload["mediaType"].(string); ok && mediaType != "" {
		payload["mediaType"] = mediaType
	}

	if createdAfter, ok := request.Payload["createdAfter"].(string); ok && createdAfter != "" {
		payload["createdAfter"] = createdAfter
	}

	if createdBefore, ok := request.Payload["createdBefore"].(string); ok && createdBefore != "" {
		payload["createdBefore"] = createdBefore
	}

	if favorite, ok := request.Payload["favorite"].(bool); ok {
		payload["favorite"] = favorite
	}

	if limit, ok := request.Payload["limit"].(float64); ok {
		payload["limit"] = ClampPhotoListLimit(int(limit))
	} else {
		payload["limit"] = DefaultPhotoListLimit
	}

	if cursor, ok := request.Payload["cursor"].(string); ok && cursor != "" {
		payload["cursor"] = cursor
	}

	if sort, ok := request.Payload["sort"].(string); ok && sort != "" {
		payload["sort"] = sort
	}

	return h.bridgeCall(ctx, request, OperationPhotosList, payload)
}

func (h *MediaHandler) handlePhotosGet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	assetRef, ok := request.Payload["assetRef"].(string)
	if !ok || assetRef == "" {
		return NewMediaError(request, ErrPhotoAssetNotFound, "missing required field: assetRef")
	}
	return h.bridgeCall(ctx, request, OperationPhotosGet, map[string]any{
		"assetRef": assetRef,
	})
}

func (h *MediaHandler) handlePhotosExport(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	assetRef, ok := request.Payload["assetRef"].(string)
	if !ok || assetRef == "" {
		return NewMediaError(request, ErrPhotoAssetNotFound, "missing required field: assetRef")
	}

	payload := map[string]any{
		"assetRef": assetRef,
	}

	if representation, ok := request.Payload["representation"].(string); ok && IsValidRepresentation(representation) {
		payload["representation"] = representation
	} else {
		payload["representation"] = "current"
	}

	if networkAccess, ok := request.Payload["networkAccess"].(bool); ok {
		payload["networkAccess"] = networkAccess
	}

	if maxBytes, ok := request.Payload["maxBytes"].(float64); ok {
		payload["maxBytes"] = ClampMaxTotalBytes(int64(maxBytes))
	} else {
		payload["maxBytes"] = MaxExportBytes
	}

	return h.bridgeCall(ctx, request, OperationPhotosExport, payload)
}

func (h *MediaHandler) handlePhotosSave(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	resourceURI, ok := request.Payload["resourceUri"].(string)
	if !ok || resourceURI == "" {
		return NewMediaError(request, ErrResourceURIInvalid, "missing required field: resourceUri")
	}

	payload := map[string]any{
		"resourceUri": resourceURI,
	}

	if mediaType, ok := request.Payload["mediaType"].(string); ok && mediaType != "" {
		payload["mediaType"] = mediaType
	}

	if albumRef, ok := request.Payload["albumRef"].(string); ok && albumRef != "" {
		payload["albumRef"] = albumRef
	}

	if preserveMetadata, ok := request.Payload["preserveMetadata"].(bool); ok {
		payload["preserveMetadata"] = preserveMetadata
	}

	return h.bridgeCall(ctx, request, OperationPhotosSave, payload)
}

func (h *MediaHandler) handlePhotosDelete(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	assetRefsRaw, ok := request.Payload["assetRefs"].([]any)
	if !ok || len(assetRefsRaw) == 0 {
		return NewMediaError(request, ErrPhotoAssetNotFound, "missing required field: assetRefs")
	}

	assetRefs := make([]string, 0, len(assetRefsRaw))
	for _, ref := range assetRefsRaw {
		if s, ok := ref.(string); ok && s != "" {
			assetRefs = append(assetRefs, s)
		}
	}

	if len(assetRefs) == 0 {
		return NewMediaError(request, ErrPhotoAssetNotFound, "no valid assetRefs provided")
	}

	return h.bridgeCall(ctx, request, OperationPhotosDelete, map[string]any{
		"assetRefs": assetRefs,
	})
}

func (h *MediaHandler) handlePhotosManageLimited(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationPhotosManageLimited, map[string]any{})
}

func (h *MediaHandler) handleCameraStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationCameraStatus, map[string]any{})
}

func (h *MediaHandler) handleCameraDevices(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationCameraDevices, map[string]any{})
}

func (h *MediaHandler) handleCameraCapturePhoto(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	payload := map[string]any{}

	if deviceRef, ok := request.Payload["deviceRef"].(string); ok && deviceRef != "" {
		payload["deviceRef"] = deviceRef
	}

	if quality, ok := request.Payload["quality"].(string); ok && IsValidQuality(quality) {
		payload["quality"] = quality
	} else {
		payload["quality"] = "high"
	}

	if flash, ok := request.Payload["flash"].(string); ok && IsValidFlashMode(flash) {
		payload["flash"] = flash
	} else {
		payload["flash"] = "auto"
	}

	if format, ok := request.Payload["format"].(string); ok && IsValidFormat(format) {
		payload["format"] = format
	} else {
		payload["format"] = "jpeg"
	}

	if mirrorFrontCamera, ok := request.Payload["mirrorFrontCamera"].(bool); ok {
		payload["mirrorFrontCamera"] = mirrorFrontCamera
	}

	if saveToPhotos, ok := request.Payload["saveToPhotos"].(bool); ok {
		payload["saveToPhotos"] = saveToPhotos
	}

	return h.bridgeCall(ctx, request, OperationCameraCapturePhoto, payload)
}

func (h *MediaHandler) handleCameraRecordVideo(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	maxDurationMs, ok := request.Payload["maxDurationMs"].(float64)
	if !ok || maxDurationMs <= 0 {
		return NewMediaError(request, ErrInvalidRequest, "missing required field: maxDurationMs")
	}

	payload := map[string]any{
		"maxDurationMs": ClampVideoDuration(int64(maxDurationMs)),
	}

	if deviceRef, ok := request.Payload["deviceRef"].(string); ok && deviceRef != "" {
		payload["deviceRef"] = deviceRef
	}

	if includeAudio, ok := request.Payload["includeAudio"].(bool); ok {
		payload["includeAudio"] = includeAudio
	}

	if quality, ok := request.Payload["quality"].(string); ok && IsValidQuality(quality) {
		payload["quality"] = quality
	} else {
		payload["quality"] = "high"
	}

	if torch, ok := request.Payload["torch"].(string); ok && IsValidTorchMode(torch) {
		payload["torch"] = torch
	}

	if saveToPhotos, ok := request.Payload["saveToPhotos"].(bool); ok {
		payload["saveToPhotos"] = saveToPhotos
	}

	return h.bridgeCall(ctx, request, OperationCameraRecordVideo, payload)
}

func (h *MediaHandler) handleAudioStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationAudioStatus, map[string]any{})
}

func (h *MediaHandler) handleAudioRecord(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	maxDurationMs, ok := request.Payload["maxDurationMs"].(float64)
	if !ok || maxDurationMs <= 0 {
		return NewMediaError(request, ErrInvalidRequest, "missing required field: maxDurationMs")
	}

	payload := map[string]any{
		"maxDurationMs": ClampAudioDuration(int64(maxDurationMs)),
	}

	if format, ok := request.Payload["format"].(string); ok && IsValidAudioFormat(format) {
		payload["format"] = format
	} else {
		payload["format"] = "m4a"
	}

	if sampleRate, ok := request.Payload["sampleRate"].(float64); ok && sampleRate > 0 {
		payload["sampleRate"] = int(sampleRate)
	} else {
		payload["sampleRate"] = DefaultAudioSampleRate
	}

	if channels, ok := request.Payload["channels"].(float64); ok && channels > 0 {
		payload["channels"] = int(channels)
	} else {
		payload["channels"] = DefaultAudioChannels
	}

	if quality, ok := request.Payload["quality"].(string); ok && IsValidQuality(quality) {
		payload["quality"] = quality
	} else {
		payload["quality"] = "high"
	}

	return h.bridgeCall(ctx, request, OperationAudioRecord, payload)
}

func (h *MediaHandler) handleStagingImport(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.importer == nil {
		return NewMediaError(request, ErrStagingImporterUnavailable, "staging importer is not configured")
	}

	nativeStagingID, ok := request.Payload["nativeStagingId"].(string)
	if !ok || nativeStagingID == "" {
		return NewMediaError(request, ErrInvalidRequest, "missing required field: nativeStagingId")
	}

	getPayloadString := func(m map[string]any, key string) string {
		if v, ok := m[key].(string); ok {
			return v
		}
		return ""
	}

	req := staging.StagingImportRequest{
		NativeStagingID: nativeStagingID,
		TaskRunID:       getPayloadString(request.Payload, "taskRunId"),
		MimeType:        getPayloadString(request.Payload, "mimeType"),
		Filename:        getPayloadString(request.Payload, "filename"),
		Source:          getPayloadString(request.Payload, "source"),
	}

	if maxReadBytes, ok := request.Payload["maxReadBytes"].(float64); ok && maxReadBytes > 0 {
		req.MaxReadBytes = int64(maxReadBytes)
	}

	bridge := &stagingMediaBridge{handler: h, ctx: ctx}
	result, err := h.importer.ImportWithBridge(req, bridge)
	if err != nil {
		return NewMediaError(request, ErrStagingImportFailed, err.Error())
	}

	return nativebridge.Response{
		ProtocolVersion: request.ProtocolVersion,
		RequestId:       request.RequestId,
		Status:          "ok",
		Result: map[string]any{
			"resourceUri": result.ResourceURI,
			"size":        result.Size,
			"mimeType":    result.MimeType,
			"filename":    result.Filename,
			"importedAt":  result.ImportedAt,
			"checksum":    result.Checksum,
		},
	}
}

type stagingMediaBridge struct {
	handler *MediaHandler
	ctx     context.Context
}

func (b *stagingMediaBridge) call(operation string, payload map[string]any) nativebridge.Response {
	if b.handler.bridge == nil {
		return nativebridge.Response{Status: "error", Error: &nativebridge.Error{Code: "BRIDGE_UNAVAILABLE", Message: "ios native bridge is not available"}}
	}
	resp, err := b.handler.bridge.Execute(b.ctx, nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "staging-" + operation,
		Platform:        "ios",
		Operation:       operation,
		Payload:         payload,
	})
	if err != nil {
		return nativebridge.Response{Status: "error", Error: &nativebridge.Error{Code: "BRIDGE_ERROR", Message: err.Error()}}
	}
	return resp
}

func (b *stagingMediaBridge) Stat(id string) (int64, string, string, error) {
	resp := b.call("native.resource.stat", map[string]any{
		"nativeStagingId": id,
	})
	if resp.Status != "ok" {
		return 0, "", "", fmt.Errorf("stat failed: %s", resp.Error)
	}
	size, _ := resp.Result["size"].(float64)
	mimeType, _ := resp.Result["mimeType"].(string)
	filename, _ := resp.Result["filename"].(string)
	return int64(size), mimeType, filename, nil
}

func (b *stagingMediaBridge) ReadChunk(id string, offset, length int64) ([]byte, error) {
	resp := b.call("native.resource.read_chunk", map[string]any{
		"nativeStagingId": id,
		"offset":          offset,
		"length":          length,
	})
	if resp.Status != "ok" {
		return nil, fmt.Errorf("read_chunk failed: %s", resp.Error)
	}
	content, _ := resp.Result["contentBase64"].(string)
	return []byte(content), nil
}

func (b *stagingMediaBridge) Release(id string) error {
	resp := b.call("native.resource.release", map[string]any{
		"nativeStagingId": id,
	})
	if resp.Status != "ok" {
		return fmt.Errorf("release failed: %s", resp.Error)
	}
	return nil
}

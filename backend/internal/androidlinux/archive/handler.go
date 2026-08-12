//go:build linux && !android

package archive

import (
	"context"
	"fmt"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Handle(ctx context.Context, operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case OperationDetect:
		return h.handleDetect(ctx, payload)
	case OperationList:
		return h.handleList(ctx, payload)
	case OperationExtract:
		return h.handleExtract(ctx, payload)
	case OperationCreate:
		return h.handleCreate(ctx, payload)
	case OperationVerify:
		return h.handleVerify(ctx, payload)
	default:
		return nil, fmt.Errorf("unknown archive operation: %s", operation)
	}
}

func (h *Handler) handleDetect(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := DetectRequest{
		Path: getStringKey(payload, "path"),
	}

	result, err := h.service.Detect(ctx, req)
	if err != nil {
		return nil, err
	}

	return detectResultToMap(result), nil
}

func (h *Handler) handleList(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := ListRequest{
		Path:               getStringKey(payload, "path"),
		Limit:              getIntKey(payload, "limit", 0),
		Offset:             getIntKey(payload, "offset", 0),
		IncludeDirectories: getBoolKey(payload, "includeDirectories", false),
	}

	entries, total, err := h.service.List(ctx, req)
	if err != nil {
		return nil, err
	}

	list := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		list = append(list, entryToMap(e))
	}

	return map[string]any{
		"path":       req.Path,
		"entries":    list,
		"count":      len(list),
		"totalCount": total,
		"limit":      req.Limit,
		"offset":     req.Offset,
	}, nil
}

func (h *Handler) handleExtract(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := ExtractRequest{
		Path:           getStringKey(payload, "path"),
		Target:         getStringKey(payload, "target"),
		Overwrite:      getBoolKey(payload, "overwrite", false),
		StripOptions:   getIntKey(payload, "stripComponents", 0),
		Include:        getStringSliceKey(payload, "include"),
		Exclude:        getStringSliceKey(payload, "exclude"),
		AllowSymlinks:  getBoolKey(payload, "allowSymlinks", false),
	}

	if me, ok := payload["maxEntries"].(float64); ok {
		maxE := int(me)
		req.MaxEntries = &maxE
	}
	if mb, ok := payload["maxBytes"].(float64); ok {
		maxB := int64(mb)
		req.MaxBytes = &maxB
	}

	entryCount, totalBytes, err := h.service.Extract(ctx, req)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"path":       req.Path,
		"target":     req.Target,
		"entryCount": entryCount,
		"totalBytes": totalBytes,
	}, nil
}

func (h *Handler) handleCreate(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := CreateRequest{
		Sources:         getStringSliceKey(payload, "sources"),
		Target:          getStringKey(payload, "target"),
		Format:          Format(getStringKey(payload, "format")),
		BasePath:        getStringKey(payload, "basePath"),
		IncludeHidden:   getBoolKey(payload, "includeHidden", false),
		FollowSymlinks:  getBoolKey(payload, "followSymlinks", false),
		Overwrite:       getBoolKey(payload, "overwrite", false),
	}

	if cl, ok := payload["compressionLevel"].(float64); ok {
		level := int(cl)
		req.CompressionLevel = &level
	}

	entryCount, totalBytes, err := h.service.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"target":     req.Target,
		"format":     string(req.Format),
		"entryCount": entryCount,
		"totalBytes": totalBytes,
	}, nil
}

func (h *Handler) handleVerify(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := DetectRequest{
		Path: getStringKey(payload, "path"),
	}

	result, err := h.service.Verify(ctx, req)
	if err != nil {
		return nil, err
	}

	return verifyResultToMap(result), nil
}

func detectResultToMap(r DetectResult) map[string]any {
	m := map[string]any{
		"path":    r.Path,
		"format":  string(r.Format),
		"archive": r.Archive,
		"sizeBytes": r.SizeBytes,
	}
	if r.MIMEType != "" {
		m["mimeType"] = r.MIMEType
	}
	if r.Compressed {
		m["compressed"] = true
	}
	if r.EntryCount != nil {
		m["entryCount"] = *r.EntryCount
	}
	if r.Encrypted {
		m["encrypted"] = true
	}
	return m
}

func entryToMap(e Entry) map[string]any {
	m := map[string]any{
		"name":      e.Name,
		"path":      e.Path,
		"type":      e.Type,
		"sizeBytes": e.SizeBytes,
	}
	if e.Mode != "" {
		m["mode"] = e.Mode
	}
	if e.ModifiedAt != nil {
		m["modifiedAt"] = e.ModifiedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if e.LinkTarget != "" {
		m["linkTarget"] = e.LinkTarget
	}
	if e.Encrypted {
		m["encrypted"] = true
	}
	return m
}

func verifyResultToMap(r *VerifyResult) map[string]any {
	m := map[string]any{
		"valid":                   r.Valid,
		"format":                  string(r.Format),
		"entryCount":              r.EntryCount,
		"totalUncompressedBytes":  r.TotalUncompressedBytes,
		"unsafeEntries":           r.UnsafeEntries,
		"corruptEntries":          r.CorruptEntries,
		"encryptedEntries":        r.EncryptedEntries,
	}
	if len(r.Warnings) > 0 {
		m["warnings"] = r.Warnings
	}
	return m
}

func getStringKey(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getIntKey(m map[string]any, key string, defaultVal int) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return defaultVal
}

func getBoolKey(m map[string]any, key string, defaultVal bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return defaultVal
}

func getStringSliceKey(m map[string]any, key string) []string {
	if v, ok := m[key].([]any); ok {
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	if v, ok := m[key].([]string); ok {
		return v
	}
	return nil
}

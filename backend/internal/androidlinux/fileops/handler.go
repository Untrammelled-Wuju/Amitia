//go:build linux && !android

package fileops

import (
	"encoding/base64"
	"fmt"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Handle(operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case OpFileStat:
		return h.handleStat(payload)
	case OpFileList:
		return h.handleList(payload)
	case OpFileRead:
		return h.handleRead(payload)
	case OpFileWrite:
		return h.handleWrite(payload)
	case OpFileAppend:
		return h.handleAppend(payload)
	case OpFileMkdir:
		return h.handleMkdir(payload)
	case OpFileTouch:
		return h.handleTouch(payload)
	case OpFileCopy:
		return h.handleCopy(payload)
	case OpFileMove:
		return h.handleMove(payload)
	case OpFileDelete:
		return h.handleDelete(payload)
	case OpFileSearch:
		return h.handleSearch(payload)
	case OpFileChmod:
		return h.handleChmod(payload)
	case OpFileReadlink:
		return h.handleReadlink(payload)
	case OpFileSymlink:
		return h.handleSymlink(payload)
	default:
		return nil, fmt.Errorf("unknown file operation: %s", operation)
	}
}

const (
	OpFileStat     = "file.stat"
	OpFileList     = "file.list"
	OpFileRead     = "file.read"
	OpFileWrite    = "file.write"
	OpFileAppend   = "file.append"
	OpFileMkdir    = "file.mkdir"
	OpFileTouch    = "file.touch"
	OpFileCopy     = "file.copy"
	OpFileMove     = "file.move"
	OpFileDelete   = "file.delete"
	OpFileSearch   = "file.search"
	OpFileChmod    = "file.chmod"
	OpFileReadlink = "file.readlink"
	OpFileSymlink  = "file.symlink"
)

func (h *Handler) handleStat(payload map[string]any) (map[string]any, error) {
	path := getStringField(payload, "path")
	if path == "" {
		return nil, ErrPathDenied("", "path is required")
	}

	result, err := h.service.Stat(path)
	if err != nil {
		return nil, err
	}

	return statResultToMap(result), nil
}

func (h *Handler) handleList(payload map[string]any) (map[string]any, error) {
	path := getStringField(payload, "path")
	if path == "" {
		return nil, ErrPathDenied("", "path is required")
	}

	opts := ListOptions{
		Limit:          getIntField(payload, "limit", 0),
		IncludeHidden:  getBoolField(payload, "includeHidden", false),
		FollowSymlinks: getBoolField(payload, "followSymlinks", false),
	}

	results, err := h.service.List(path, opts)
	if err != nil {
		return nil, err
	}

	list := make([]map[string]any, 0, len(results))
	for _, r := range results {
		list = append(list, statResultToMap(r))
	}

	return map[string]any{
		"path":    path,
		"entries": list,
		"count":   len(list),
	}, nil
}

func (h *Handler) handleRead(payload map[string]any) (map[string]any, error) {
	path := getStringField(payload, "path")
	if path == "" {
		return nil, ErrPathDenied("", "path is required")
	}

	opts := ReadOptions{
		Offset:   getInt64Field(payload, "offset", 0),
		MaxBytes: getInt64Field(payload, "maxBytes", 0),
	}

	result, err := h.service.Read(path, opts)
	if err != nil {
		return nil, err
	}

	var content any
	if result.IsBinary {
		content = map[string]any{
			"encoding": "base64",
			"data":     base64.StdEncoding.EncodeToString(result.Content),
		}
	} else {
		content = string(result.Content)
	}

	return map[string]any{
		"path":      result.Path,
		"offset":    result.Offset,
		"bytesRead": result.BytesRead,
		"content":   content,
		"eof":       result.EOF,
		"isBinary":  result.IsBinary,
	}, nil
}

func (h *Handler) handleWrite(payload map[string]any) (map[string]any, error) {
	path := getStringField(payload, "path")
	if path == "" {
		return nil, ErrPathDenied("", "path is required")
	}

	data, err := decodeWriteContent(payload)
	if err != nil {
		return nil, err
	}

	opts := WriteOptions{
		Overwrite:     getBoolField(payload, "overwrite", true),
		CreateParents: getBoolField(payload, "createParents", false),
		Mode:          uint32(getIntField(payload, "mode", 0)),
	}

	result, err := h.service.Write(path, data, opts)
	if err != nil {
		return nil, err
	}

	return statResultToMap(result), nil
}

func (h *Handler) handleAppend(payload map[string]any) (map[string]any, error) {
	path := getStringField(payload, "path")
	if path == "" {
		return nil, ErrPathDenied("", "path is required")
	}

	data, err := decodeWriteContent(payload)
	if err != nil {
		return nil, err
	}

	result, err := h.service.Append(path, data)
	if err != nil {
		return nil, err
	}

	return statResultToMap(result), nil
}

func (h *Handler) handleMkdir(payload map[string]any) (map[string]any, error) {
	path := getStringField(payload, "path")
	if path == "" {
		return nil, ErrPathDenied("", "path is required")
	}

	opts := MkdirOptions{
		Recursive: getBoolField(payload, "recursive", false),
		Mode:      uint32(getIntField(payload, "mode", 0)),
	}

	result, err := h.service.Mkdir(path, opts)
	if err != nil {
		return nil, err
	}

	return statResultToMap(result), nil
}

func (h *Handler) handleTouch(payload map[string]any) (map[string]any, error) {
	path := getStringField(payload, "path")
	if path == "" {
		return nil, ErrPathDenied("", "path is required")
	}

	result, err := h.service.Touch(path)
	if err != nil {
		return nil, err
	}

	return statResultToMap(result), nil
}

func (h *Handler) handleCopy(payload map[string]any) (map[string]any, error) {
	source := getStringField(payload, "source")
	if source == "" {
		return nil, ErrPathDenied("", "source is required")
	}

	destination := getStringField(payload, "destination")
	if destination == "" {
		return nil, ErrPathDenied("", "destination is required")
	}

	opts := CopyOptions{
		Recursive: getBoolField(payload, "recursive", false),
		Overwrite: getBoolField(payload, "overwrite", true),
	}

	result, err := h.service.Copy(source, destination, opts)
	if err != nil {
		return nil, err
	}

	return statResultToMap(result), nil
}

func (h *Handler) handleMove(payload map[string]any) (map[string]any, error) {
	source := getStringField(payload, "source")
	if source == "" {
		return nil, ErrPathDenied("", "source is required")
	}

	destination := getStringField(payload, "destination")
	if destination == "" {
		return nil, ErrPathDenied("", "destination is required")
	}

	opts := MoveOptions{
		Overwrite: getBoolField(payload, "overwrite", true),
	}

	result, err := h.service.Move(source, destination, opts)
	if err != nil {
		return nil, err
	}

	return statResultToMap(result), nil
}

func (h *Handler) handleDelete(payload map[string]any) (map[string]any, error) {
	path := getStringField(payload, "path")
	if path == "" {
		return nil, ErrPathDenied("", "path is required")
	}

	opts := DeleteOptions{
		Recursive: getBoolField(payload, "recursive", false),
	}

	err := h.service.Delete(path, opts)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"path":    path,
		"deleted": true,
	}, nil
}

func (h *Handler) handleSearch(payload map[string]any) (map[string]any, error) {
	root := getStringField(payload, "root")
	if root == "" {
		root = getStringField(payload, "path")
	}
	if root == "" {
		return nil, ErrPathDenied("", "root is required")
	}

	opts := SearchOptions{
		Query:          getStringField(payload, "query"),
		MaxDepth:       getIntField(payload, "maxDepth", 0),
		Limit:          getIntField(payload, "limit", 0),
		IncludeHidden:  getBoolField(payload, "includeHidden", false),
		FollowSymlinks: getBoolField(payload, "followSymlinks", false),
	}

	results, err := h.service.Search(root, opts)
	if err != nil {
		return nil, err
	}

	list := make([]map[string]any, 0, len(results))
	for _, r := range results {
		list = append(list, statResultToMap(r))
	}

	return map[string]any{
		"root":    root,
		"query":   opts.Query,
		"results": list,
		"count":   len(list),
	}, nil
}

func (h *Handler) handleChmod(payload map[string]any) (map[string]any, error) {
	path := getStringField(payload, "path")
	if path == "" {
		return nil, ErrPathDenied("", "path is required")
	}

	mode := uint32(getIntField(payload, "mode", 0))

	result, err := h.service.Chmod(path, mode)
	if err != nil {
		return nil, err
	}

	return statResultToMap(result), nil
}

func (h *Handler) handleReadlink(payload map[string]any) (map[string]any, error) {
	path := getStringField(payload, "path")
	if path == "" {
		return nil, ErrPathDenied("", "path is required")
	}

	target, err := h.service.Readlink(path)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"path":   path,
		"target": target,
	}, nil
}

func (h *Handler) handleSymlink(payload map[string]any) (map[string]any, error) {
	target := getStringField(payload, "target")
	if target == "" {
		return nil, ErrPathDenied("", "target is required")
	}

	linkPath := getStringField(payload, "linkPath")
	if linkPath == "" {
		return nil, ErrPathDenied("", "linkPath is required")
	}

	result, err := h.service.Symlink(target, linkPath)
	if err != nil {
		return nil, err
	}

	return statResultToMap(result), nil
}

func statResultToMap(r StatResult) map[string]any {
	m := map[string]any{
		"path":    r.Path,
		"name":    r.Name,
		"type":    r.Type,
		"size":    r.Size,
		"mode":    fmt.Sprintf("%04o", r.Mode),
		"modTime": r.ModTime.Format("2006-01-02T15:04:05Z07:00"),
		"isDir":   r.IsDir,
	}
	if r.IsSymlink {
		m["isSymlink"] = true
		m["linkTarget"] = r.LinkTarget
	}
	return m
}

func decodeWriteContent(payload map[string]any) ([]byte, error) {
	if data, ok := payload["data"].(string); ok {
		if encoding, ok := payload["encoding"].(string); ok && encoding == "base64" {
			decoded, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				return nil, ErrIOFailed("invalid base64 data: " + err.Error())
			}
			return decoded, nil
		}
		return []byte(data), nil
	}
	return nil, ErrIOFailed("data is required")
}

func getStringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getIntField(m map[string]any, key string, defaultVal int) int {
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

func getInt64Field(m map[string]any, key string, defaultVal int64) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	}
	return defaultVal
}

func getBoolField(m map[string]any, key string, defaultVal bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return defaultVal
}


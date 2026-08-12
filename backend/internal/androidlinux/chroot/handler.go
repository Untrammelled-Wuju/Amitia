//go:build linux && !android

package chroot

import (
	"context"
	"fmt"
)

type Handler struct {
	service   *Service
	workspace string
}

func NewHandler(service *Service, workspace string) *Handler {
	return &Handler{
		service:   service,
		workspace: workspace,
	}
}

func (h *Handler) Handle(ctx context.Context, operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case OpChrootStatus:
		return h.handleStatus(ctx)
	case OpChrootInspect:
		return h.handleInspect(ctx, payload)
	case OpChrootExec:
		return h.handleExec(ctx, payload)
	default:
		return nil, fmt.Errorf("unknown chroot operation: %s", operation)
	}
}

func (h *Handler) handleStatus(ctx context.Context) (map[string]any, error) {
	status := h.service.Status(ctx, h.workspace)
	return map[string]any{
		"enabled":               status.Enabled,
		"defaultRootfsPath":     status.DefaultRootFSP,
		"knownRootfsPaths":      status.KnownFSPs,
		"maxFsBytes":            status.MaxFSBytes,
		"maxEnvironments":       status.MaxEnvironments,
		"availableEnvironments": status.AvailableEnvironments,
		"execBackends":          status.ExecBackends,
	}, nil
}

func (h *Handler) handleInspect(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := ChrootInspectRequest{
		RootFSPath: getStringKey(payload, "rootfsPath"),
	}

	result, err := h.service.Inspect(ctx, req)
	if err != nil {
		return nil, err
	}

	m := map[string]any{
		"rootfsPath": result.RootFSPath,
		"exists":     result.Exists,
		"valid":      result.Valid,
		"totalBytes": result.TotalBytes,
		"fileCount":  result.FileCount,
		"hasBinSh":   result.HasBinSH,
		"hasBinBash": result.HasBinBash,
	}
	if result.Error != "" {
		m["error"] = result.Error
	}
	return m, nil
}

func (h *Handler) handleExec(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := ChrootExecRequest{
		RootFSPath:  getStringKey(payload, "rootfsPath"),
		Command:     getStringKey(payload, "command"),
		Args:        getStringSliceKey(payload, "args"),
		Environment: getStringMapKey(payload, "environment"),
		Stdin:       getStringKey(payload, "stdin"),
		WorkingDir:  getStringKey(payload, "workingDir"),
		User:        getStringKey(payload, "user"),
	}

	if timeoutMs, ok := payload["timeoutMs"].(float64); ok {
		req.TimeoutMs = int64(timeoutMs)
	}
	if maxOutput, ok := payload["maxOutputBytes"].(float64); ok {
		req.MaxOutputBytes = int64(maxOutput)
	}

	result, err := h.service.Exec(ctx, req)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"rootfsPath":      result.RootFSPath,
		"exitCode":        result.ExitCode,
		"stdout":          result.Stdout,
		"stderr":          result.Stderr,
		"stdoutTruncated": result.StdoutTruncated,
		"stderrTruncated": result.StderrTruncated,
		"stdoutBytes":     result.StdoutBytes,
		"stderrBytes":     result.StderrBytes,
		"durationMs":      result.DurationMs,
		"environment":     result.Environment,
	}, nil
}

func getStringKey(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
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

func getStringMapKey(m map[string]any, key string) map[string]string {
	result := map[string]string{}
	if v, ok := m[key].(map[string]any); ok {
		for k, val := range v {
			if s, ok := val.(string); ok {
				result[k] = s
			}
		}
	}
	return result
}

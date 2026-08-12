//go:build linux && !android

package ssh

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
	case OpSSHStatus:
		return h.handleStatus(ctx)
	case OpSSHExec:
		return h.handleExec(ctx, payload)
	case OpSSHHostKeyScan:
		return h.handleHostKeyScan(ctx, payload)
	default:
		return nil, fmt.Errorf("unknown ssh operation: %s", operation)
	}
}

func (h *Handler) handleStatus(ctx context.Context) (map[string]any, error) {
	status := h.service.Status(ctx)
	return map[string]any{
		"enabled":         status.Enabled,
		"defaultUser":     status.DefaultUser,
		"knownHostsCount": status.KnownHostsCount,
		"maxSessions":     status.MaxSessions,
		"activeSessions":  status.ActiveSessions,
	}, nil
}

func (h *Handler) handleExec(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := SSHExecRequest{
		Host:        getStringKey(payload, "host"),
		Port:        getIntKey(payload, "port", 0),
		User:        getStringKey(payload, "user"),
		Command:     getStringKey(payload, "command"),
		Stdin:       getStringKey(payload, "stdin"),
		Environment: getStringMapKey(payload, "environment"),
		WorkingDir:  getStringKey(payload, "workingDir"),
		HostKey:     getStringKey(payload, "hostKey"),
		PrivateKey:  getStringKey(payload, "privateKey"),
		Password:    getStringKey(payload, "password"),
		HostKeyPolicy: getStringKey(payload, "hostKeyPolicy"),
	}

	if agentAuth, ok := payload["agentAuth"].(bool); ok {
		req.AgentAuth = agentAuth
	}
	if agentSocket, ok := payload["agentSocket"].(string); ok {
		req.AgentSocket = agentSocket
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
		"exitCode":        result.ExitCode,
		"stdout":          result.Stdout,
		"stderr":          result.Stderr,
		"stdoutTruncated": result.StdoutTruncated,
		"stderrTruncated": result.StderrTruncated,
		"stdoutBytes":     result.StdoutBytes,
		"stderrBytes":     result.StderrBytes,
		"durationMs":      result.DurationMs,
	}, nil
}

func (h *Handler) handleHostKeyScan(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := HostKeyScanRequest{
		Host: getStringKey(payload, "host"),
		Port: getIntKey(payload, "port", 0),
	}

	if timeoutMs, ok := payload["timeoutMs"].(float64); ok {
		req.TimeoutMs = int64(timeoutMs)
	}

	result, err := h.service.ScanHostKey(ctx, req)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"host":         result.Host,
		"port":         result.Port,
		"algorithms":   result.Algorithms,
		"rawKeys":      result.RawKeys,
		"fingerprints": result.Fingerprints,
	}, nil
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

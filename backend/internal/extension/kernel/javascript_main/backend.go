package javascript_main

import (
	"context"
	"encoding/json"
	goruntime "runtime"

	"github.com/u-ai/backend/internal/extension/kernel/runtime"
)

type JavaScriptRuntimeCapabilities struct {
	Backend          string   `json:"backend"`
	NodeVersion      string   `json:"nodeVersion"`
	SupportedFormats []string `json:"supportedFormats"`
	MaxMemoryMB      int      `json:"maxMemoryMB"`
	MaxConcurrent    int      `json:"maxConcurrent"`
	NetworkDisabled  bool     `json:"networkDisabled"`
	Platform         string   `json:"platform"`
	Architecture     string   `json:"architecture"`
	HasSourceMap     bool     `json:"hasSourceMap"`
	HasTypeScript    bool     `json:"hasTypeScript"`
}

type JavaScriptRuntimeSpec struct {
	InstanceID           string
	ExtensionID          string
	ModuleID             string
	DefinitionHash       string
	Generation           int
	Entry                string
	ModuleFormat         string
	HostAPIVersion       string
	ResourceLimits       runtime.ResourceLimits
	SessionToken         string
	AllowedContributions []string
	NetworkDisabled      bool
	Env                  []string
}

type JavaScriptRuntimeInstance interface {
	InstanceID() string
	ExtensionID() string
	ModuleID() string
	Generation() int
	State() HostState
	Health() HealthReport
	Invoke(ctx context.Context, contributionID string, input []byte, deadline int64) (json.RawMessage, error)
	Start(ctx context.Context) error
	Stop(ctx context.Context, reason string) error
}

type JavaScriptRuntimeBackend interface {
	Capabilities() JavaScriptRuntimeCapabilities
	Start(ctx context.Context, spec JavaScriptRuntimeSpec) (JavaScriptRuntimeInstance, error)
}

func DefaultCapabilities() JavaScriptRuntimeCapabilities {
	return JavaScriptRuntimeCapabilities{
		Backend:          "node-process",
		SupportedFormats: []string{".mjs", ".cjs", ".js"},
		MaxMemoryMB:      512,
		MaxConcurrent:    4,
		NetworkDisabled:  true,
		Platform:         goruntime.GOOS,
		Architecture:     goruntime.GOARCH,
		HasSourceMap:     true,
		HasTypeScript:    true,
	}
}

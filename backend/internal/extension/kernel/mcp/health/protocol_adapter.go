package health

import (
	"context"
	"net/http"
	"time"
)

type DiscoverResult struct {
	Era               ProtocolEra
	SupportedVersions []string
	Capabilities      map[string]any
	ServerInfo        MCPServerInfo
	TTLMs             int64
	CacheScope        string
}

type CallResult struct {
	Response    map[string]any
	Error       string
	ProtocolVer string
}

type SubscribeResult struct {
	SubscriptionID string
	Error          string
}

type HealthProbeResult struct {
	Reachable       bool
	LatencyMS       int64
	ProtocolVersion string
	ServerInfo      MCPServerInfo
	Error           string
	ErrorDetail     string
}

type MCPProtocolAdapter interface {
	Era() ProtocolEra
	Discover(ctx context.Context, endpoint string, headers map[string]string) (DiscoverResult, error)
	Call(ctx context.Context, endpoint, method string, params map[string]any, headers map[string]string) (CallResult, error)
	Subscribe(ctx context.Context, endpoint string, params map[string]any, headers map[string]string) (SubscribeResult, error)
	Close() error
	HealthProbe(ctx context.Context, endpoint string, headers map[string]string) (HealthProbeResult, error)
}

type ModernHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type LegacyHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Modern2026Adapter struct {
	client  ModernHTTPClient
	timeout time.Duration
}

func NewModern2026Adapter(client ModernHTTPClient, timeout time.Duration) *Modern2026Adapter {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &Modern2026Adapter{client: client, timeout: timeout}
}

func (a *Modern2026Adapter) Era() ProtocolEra {
	return MCPProtocolEraModern
}

func (a *Modern2026Adapter) Discover(ctx context.Context, endpoint string, headers map[string]string) (DiscoverResult, error) {
	return DiscoverResult{Era: MCPProtocolEraModern}, nil
}

func (a *Modern2026Adapter) Call(ctx context.Context, endpoint, method string, params map[string]any, headers map[string]string) (CallResult, error) {
	return CallResult{}, nil
}

func (a *Modern2026Adapter) Subscribe(ctx context.Context, endpoint string, params map[string]any, headers map[string]string) (SubscribeResult, error) {
	return SubscribeResult{}, nil
}

func (a *Modern2026Adapter) Close() error {
	return nil
}

func (a *Modern2026Adapter) HealthProbe(ctx context.Context, endpoint string, headers map[string]string) (HealthProbeResult, error) {
	return HealthProbeResult{}, nil
}

type Legacy2025Adapter struct {
	client  LegacyHTTPClient
	timeout time.Duration
}

func NewLegacy2025Adapter(client LegacyHTTPClient, timeout time.Duration) *Legacy2025Adapter {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &Legacy2025Adapter{client: client, timeout: timeout}
}

func (a *Legacy2025Adapter) Era() ProtocolEra {
	return MCPProtocolEraLegacy
}

func (a *Legacy2025Adapter) Discover(ctx context.Context, endpoint string, headers map[string]string) (DiscoverResult, error) {
	return DiscoverResult{Era: MCPProtocolEraLegacy}, nil
}

func (a *Legacy2025Adapter) Call(ctx context.Context, endpoint, method string, params map[string]any, headers map[string]string) (CallResult, error) {
	return CallResult{}, nil
}

func (a *Legacy2025Adapter) Subscribe(ctx context.Context, endpoint string, params map[string]any, headers map[string]string) (SubscribeResult, error) {
	return SubscribeResult{}, nil
}

func (a *Legacy2025Adapter) Close() error {
	return nil
}

func (a *Legacy2025Adapter) HealthProbe(ctx context.Context, endpoint string, headers map[string]string) (HealthProbeResult, error) {
	return HealthProbeResult{}, nil
}

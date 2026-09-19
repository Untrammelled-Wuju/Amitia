package delivery

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	basechannel "github.com/u-ai/backend/internal/channel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/serviceauth"
)

const channelProviderCapability capability.CapabilityID = "channel.provider"

type ChannelProviderDefinitionSource interface {
	ListByCapability(capabilityID capability.CapabilityID) []*capability.CapabilityProviderDefinition
}

type PluginChannelProviderRegistry struct {
	source            ChannelProviderDefinitionSource
	extensionIsActive func(extensionID string) bool
}

func NewPluginChannelProviderRegistry(source ChannelProviderDefinitionSource) *PluginChannelProviderRegistry {
	return &PluginChannelProviderRegistry{source: source}
}

func (r *PluginChannelProviderRegistry) SetExtensionActiveChecker(checker func(extensionID string) bool) {
	if r == nil {
		return
	}
	r.extensionIsActive = checker
}

func (r *PluginChannelProviderRegistry) Provider(channelID string) (*basechannel.HTTPProvider, error) {
	def, err := r.definition(channelID)
	if err != nil {
		return nil, err
	}
	transport := metadataMap(def.Metadata, "transport")
	baseURL, err := resolveTransportBaseURL(transport)
	if err != nil {
		return nil, fmt.Errorf("channel provider %s: %w", channelID, err)
	}
	token, err := serviceauth.Token(def.ExtensionID, def.ModuleID)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{
		"Authorization":         "Bearer " + token,
		"X-Amitia-Extension-ID": def.ExtensionID,
		"X-Amitia-Module-ID":    def.ModuleID,
	}
	return basechannel.NewHTTPProvider(basechannel.HTTPProviderOptions{
		BaseURL: baseURL,
		Definition: basechannel.Definition{
			ID:          basechannel.ID(channelID),
			Name:        channelID,
			Description: "plugin channel provider",
			Version:     "1.0.0",
			PublisherID: def.ExtensionID,
		},
		HealthPath:        stringValue(transport["healthPath"]),
		SendPath:          stringValue(transport["sendPath"]),
		ImagePath:         stringValue(transport["imagePath"]),
		VoicePath:         stringValue(transport["voicePath"]),
		StatusPath:        stringValue(transport["statusPath"]),
		ConnectPath:       stringValue(transport["connectPath"]),
		DisconnectPath:    stringValue(transport["disconnectPath"]),
		ConfigPath:        stringValue(transport["configPath"]),
		MessagesPath:      stringValue(transport["messagesPath"]),
		DefaultHeaders:    headers,
		UseFallbackImage:  boolValue(transport["preferFallbackImage"]),
		IdempotencyHeader: true,
	}), nil
}

func resolveTransportBaseURL(transport map[string]any) (string, error) {
	transportType := strings.ToLower(stringValue(transport["type"]))
	if transportType == "" {
		transportType = "http"
	}
	if transportType != "http" {
		return "", fmt.Errorf("unsupported transport type %s", transportType)
	}
	if baseURL := strings.TrimRight(stringValue(transport["baseUrl"]), "/"); baseURL != "" {
		if err := validateLoopbackBaseURL(baseURL); err != nil {
			return "", err
		}
		return baseURL, nil
	}
	port := intValue(transport["defaultPort"])
	if envName := stringValue(transport["portEnv"]); envName != "" {
		raw := strings.TrimSpace(os.Getenv(envName))
		if raw == "" {
			return "", fmt.Errorf("transport port environment variable %s is not set", envName)
		}
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 65535 {
			return "", fmt.Errorf("transport port environment variable %s is invalid", envName)
		}
		port = parsed
	}
	if port <= 0 || port > 65535 {
		return "", fmt.Errorf("transport defaultPort is required")
	}
	host := stringValue(transport["host"])
	if host == "" {
		host = "127.0.0.1"
	}
	if !isLoopbackHost(host) {
		return "", fmt.Errorf("transport host must be loopback")
	}
	if strings.Contains(host, ":") {
		host = "[" + strings.Trim(host, "[]") + "]"
	}
	return fmt.Sprintf("http://%s:%d", host, port), nil
}

func validateLoopbackBaseURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid transport baseUrl")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("transport baseUrl must use http or https")
	}
	if !isLoopbackHost(parsed.Hostname()) {
		return fmt.Errorf("transport baseUrl must use a loopback host")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(host)
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func (r *PluginChannelProviderRegistry) Has(channelID string) bool {
	_, err := r.definition(channelID)
	return err == nil
}

func (r *PluginChannelProviderRegistry) Channels() []string {
	if r == nil || r.source == nil {
		return nil
	}
	definitions := r.source.ListByCapability(channelProviderCapability)
	result := make([]string, 0, len(definitions))
	seen := make(map[string]struct{}, len(definitions))
	for _, def := range definitions {
		if !r.definitionActive(def) {
			continue
		}
		id := channelIDFromDefinition(def)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

func (r *PluginChannelProviderRegistry) StatusData(ctx context.Context, channelID string) (map[string]any, error) {
	provider, err := r.Provider(channelID)
	if err != nil {
		return nil, err
	}
	return provider.StatusData(ctx)
}

func (r *PluginChannelProviderRegistry) Connect(ctx context.Context, channelID string, config map[string]any) (map[string]any, error) {
	provider, err := r.Provider(channelID)
	if err != nil {
		return nil, err
	}
	return provider.ConnectResult(ctx, config)
}

func (r *PluginChannelProviderRegistry) Disconnect(ctx context.Context, channelID string) error {
	provider, err := r.Provider(channelID)
	if err != nil {
		return err
	}
	return provider.Disconnect(ctx)
}

func (r *PluginChannelProviderRegistry) Messages(ctx context.Context, channelID, conversationID string, limit, offset int) (map[string]any, error) {
	provider, err := r.Provider(channelID)
	if err != nil {
		return nil, err
	}
	return provider.Messages(ctx, map[string]any{
		"channelId":      channelID,
		"conversationId": conversationID,
		"limit":          limit,
		"offset":         offset,
	})
}

func (r *PluginChannelProviderRegistry) definition(channelID string) (*capability.CapabilityProviderDefinition, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return nil, fmt.Errorf("channelId is required")
	}
	if r == nil || r.source == nil {
		return nil, fmt.Errorf("channel provider registry unavailable")
	}
	for _, def := range r.source.ListByCapability(channelProviderCapability) {
		if def == nil {
			continue
		}
		if !r.definitionActive(def) {
			continue
		}
		if channelIDFromDefinition(def) == channelID {
			return def, nil
		}
	}
	return nil, fmt.Errorf("channel provider not found: %s", channelID)
}

func (r *PluginChannelProviderRegistry) definitionActive(def *capability.CapabilityProviderDefinition) bool {
	if def == nil {
		return false
	}
	if r == nil || r.extensionIsActive == nil {
		return true
	}
	extensionID := strings.TrimSpace(def.ExtensionID)
	return extensionID == "" || r.extensionIsActive(extensionID)
}

func channelIDFromDefinition(def *capability.CapabilityProviderDefinition) string {
	if def == nil {
		return ""
	}
	if value := stringValue(def.Metadata["channelId"]); value != "" {
		return value
	}
	if value := stringValue(def.Metadata["id"]); value != "" {
		return value
	}
	labels := metadataMap(def.Metadata, "labels")
	return stringValue(labels["channelId"])
}

func metadataMap(metadata map[string]any, key string) map[string]any {
	if metadata == nil {
		return nil
	}
	value, ok := metadata[key].(map[string]any)
	if !ok {
		return nil
	}
	return value
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func boolValue(value any) bool {
	result, _ := value.(bool)
	return result
}

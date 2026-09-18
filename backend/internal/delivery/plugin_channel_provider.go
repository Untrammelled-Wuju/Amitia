package delivery

import (
	"context"
	"fmt"
	"sort"
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
	source ChannelProviderDefinitionSource
}

func NewPluginChannelProviderRegistry(source ChannelProviderDefinitionSource) *PluginChannelProviderRegistry {
	return &PluginChannelProviderRegistry{source: source}
}

func (r *PluginChannelProviderRegistry) Provider(channelID string) (*basechannel.HTTPProvider, error) {
	def, err := r.definition(channelID)
	if err != nil {
		return nil, err
	}
	sidecar := metadataMap(def.Metadata, "sidecar")
	port := intValue(sidecar["defaultPort"])
	if port <= 0 {
		return nil, fmt.Errorf("channel provider %s has no sidecar defaultPort", channelID)
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
		BaseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		Definition: basechannel.Definition{
			ID:          basechannel.ID(channelID),
			Name:        channelID,
			Description: "plugin channel provider",
			Version:     "1.0.0",
			PublisherID: def.ExtensionID,
		},
		HealthPath:        stringValue(sidecar["healthPath"]),
		SendPath:          stringValue(sidecar["sendPath"]),
		ImagePath:         stringValue(sidecar["imagePath"]),
		VoicePath:         stringValue(sidecar["voicePath"]),
		StatusPath:        stringValue(sidecar["statusPath"]),
		ConnectPath:       stringValue(sidecar["connectPath"]),
		DisconnectPath:    stringValue(sidecar["disconnectPath"]),
		ConfigPath:        stringValue(sidecar["configPath"]),
		MessagesPath:      stringValue(sidecar["messagesPath"]),
		DefaultHeaders:    headers,
		UseFallbackImage:  boolValue(sidecar["preferFallbackImage"]),
		IdempotencyHeader: true,
	}), nil
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
		if channelIDFromDefinition(def) == channelID {
			return def, nil
		}
	}
	return nil, fmt.Errorf("channel provider not found: %s", channelID)
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

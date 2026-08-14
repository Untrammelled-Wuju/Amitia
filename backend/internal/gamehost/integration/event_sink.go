package integration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/gamehost/contracts"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/notification"
)

type KernelEventNotificationSink struct {
	eventService *event.Service
	plugins      contracts.PluginRegistry
}

func NewKernelEventNotificationSink(eventService *event.Service, plugins contracts.PluginRegistry) *KernelEventNotificationSink {
	return &KernelEventNotificationSink{
		eventService: eventService,
		plugins:      plugins,
	}
}

func (s *KernelEventNotificationSink) Publish(ctx context.Context, n notification.Notification) error {
	if s.eventService == nil {
		return nil
	}

	extensionID, err := s.resolveExtensionID(ctx, n.PluginID)
	if err != nil {
		return fmt.Errorf("resolve extension id for plugin %q: %w", n.PluginID, err)
	}

	metadata := map[string]string{
		"pluginId":  string(n.PluginID),
		"runtimeId": string(n.RuntimeID),
		"serviceId": string(n.ServiceID),
		"method":    n.Method,
	}
	metadataJSON, _ := json.Marshal(metadata)

	opts := event.PublishOptions{
		ProducerExtensionID: extensionID,
		TraceID:             n.ID,
		Metadata:            metadataJSON,
	}

	_, err = s.eventService.Publish(ctx, "gamehost.notification", 1, n.Payload, opts)
	if err != nil {
		return fmt.Errorf("publish gamehost notification event: %w", err)
	}
	return nil
}

func (s *KernelEventNotificationSink) resolveExtensionID(ctx context.Context, pluginID domain.PluginID) (string, error) {
	if s.plugins == nil {
		return "", fmt.Errorf("plugin registry not available")
	}
	descriptor, err := s.plugins.Get(ctx, pluginID)
	if err != nil {
		return "", fmt.Errorf("plugin %q not found: %w", pluginID, err)
	}
	return descriptor.ExtensionID, nil
}

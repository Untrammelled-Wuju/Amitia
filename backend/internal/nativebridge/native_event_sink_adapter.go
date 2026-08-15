package nativebridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/u-ai/backend/internal/extension/kernel/event"
)

type nativeEventSinkAdapter struct {
	service *event.Service
}

func NewNativeEventSinkAdapter(service *event.Service) NativeEventSink {
	return &nativeEventSinkAdapter{service: service}
}

func (a *nativeEventSinkAdapter) PublishNativeEvent(ctx context.Context, platform string, generation uint64, payload json.RawMessage) error {
	if a.service == nil {
		return fmt.Errorf("native event sink: event service not available")
	}

	wrapper := map[string]any{
		"platform":   platform,
		"generation": generation,
		"payload":    json.RawMessage(payload),
	}

	wrapperBytes, err := json.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("native event sink: marshal wrapper: %w", err)
	}

	_, err = a.service.Publish(ctx, "device.native.event", 1, wrapperBytes, event.PublishOptions{
		ProducerID:   "nativebridge:" + platform,
		ProducerType: event.EventProducerTypeSystem,
	})
	if err != nil {
		log.Printf("native event sink: publish failed for platform %s: %v", platform, err)
		return fmt.Errorf("native event sink: publish: %w", err)
	}
	return nil
}

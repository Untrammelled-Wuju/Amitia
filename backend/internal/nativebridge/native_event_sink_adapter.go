package nativebridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/u-ai/backend/internal/extension/kernel/event"
)

type NativeEventSinkRouter interface {
	ResumeBackgroundTask(ctx context.Context, taskRunID string) error
	SignalBackgroundExpiration(ctx context.Context, taskRunID string) error
}

type nativeEventSinkAdapter struct {
	service  *event.Service
	router   NativeEventSinkRouter
}

func NewNativeEventSinkAdapter(service *event.Service) NativeEventSink {
	return &nativeEventSinkAdapter{service: service}
}

func NewNativeEventSinkAdapterWithRouter(service *event.Service, router NativeEventSinkRouter) NativeEventSink {
	return &nativeEventSinkAdapter{service: service, router: router}
}

func (a *nativeEventSinkAdapter) PublishNativeEvent(ctx context.Context, platform string, generation uint64, payload json.RawMessage) error {
	if a.service == nil {
		return fmt.Errorf("native event sink: event service not available")
	}

	var eventMeta struct {
		Domain string `json:"domain"`
		Event  string `json:"event"`
		Data   struct {
			TaskRunID string `json:"taskRunId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &eventMeta); err == nil {
		if eventMeta.Domain == "background" && eventMeta.Data.TaskRunID != "" && a.router != nil {
			switch eventMeta.Event {
			case "execution_window_started":
				if err := a.router.ResumeBackgroundTask(ctx, eventMeta.Data.TaskRunID); err != nil {
					log.Printf("native event sink: resume background task %s failed: %v", eventMeta.Data.TaskRunID, err)
				}
			case "execution_window_expired":
				if err := a.router.SignalBackgroundExpiration(ctx, eventMeta.Data.TaskRunID); err != nil {
					log.Printf("native event sink: signal expiration for %s failed: %v", eventMeta.Data.TaskRunID, err)
				}
			}
		}
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

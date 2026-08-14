package integration

import (
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/notification"
)

type ControlAuditAdapter struct {
	eventService *event.Service
	pluginReg    notification.PluginRegistry
}

func NewControlAuditAdapter(eventService *event.Service, pluginReg notification.PluginRegistry) *ControlAuditAdapter {
	return &ControlAuditAdapter{
		eventService: eventService,
		pluginReg:    pluginReg,
	}
}

func (a *ControlAuditAdapter) RecordTransition(event control.AuthorityAuditEvent) {
	if a.eventService == nil {
		return
	}

	extensionID := a.resolveExtensionID(event.PluginID)

	metadata := map[string]string{
		"runtimeId": string(event.RuntimeID),
		"pluginId":  string(event.PluginID),
		"previousMode": string(event.PreviousMode),
		"newMode": string(event.NewMode),
		"previousEpoch": fmt.Sprintf("%d", event.PreviousEpoch),
		"newEpoch": fmt.Sprintf("%d", event.NewEpoch),
		"actor": string(event.Actor),
		"reason": string(event.Reason),
		"result": string(event.Result),
	}
	if event.Error != "" {
		metadata["error"] = event.Error
	}

	metadataJSON, _ := json.Marshal(metadata)

	payload := map[string]interface{}{
		"type":      "control_authority_audit",
		"timestamp": event.Timestamp,
		"metadata":  metadata,
	}
	payloadJSON, _ := json.Marshal(payload)

	opts := eventPublishOptions{
		producerExtensionID: extensionID,
		metadata:            metadataJSON,
	}

	_, _ = a.eventService.Publish(nil, "gamehost.control.audit", 1, payloadJSON, eventPublishOptions{
		producerExtensionID: opts.producerExtensionID,
		metadata:            opts.metadata,
	})
}

type eventPublishOptions struct {
	producerExtensionID string
	metadata            []byte
}

func (a *ControlAuditAdapter) resolveExtensionID(pluginID string) string {
	if a.pluginReg == nil {
		return ""
	}
	descriptor, err := a.pluginReg.GetByPluginID(pluginID)
	if err != nil {
		return ""
	}
	if descriptor == nil {
		return ""
	}
	return descriptor.ExtensionID
}

var _ control.AuthorityAuditSink = (*ControlAuditAdapter)(nil)

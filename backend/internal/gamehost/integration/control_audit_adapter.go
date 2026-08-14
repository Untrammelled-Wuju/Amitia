package integration

import (
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/contracts"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ControlAuditAdapter struct {
	eventService *event.Service
	pluginReg    contracts.PluginRegistry
}

func NewControlAuditAdapter(eventService *event.Service, pluginReg contracts.PluginRegistry) *ControlAuditAdapter {
	return &ControlAuditAdapter{
		eventService: eventService,
		pluginReg:    pluginReg,
	}
}

func (a *ControlAuditAdapter) RecordTransition(evt control.AuthorityAuditEvent) {
	if a.eventService == nil {
		return
	}

	extensionID := a.resolveExtensionID(evt.PluginID)

	metadata := map[string]string{
		"runtimeId":     string(evt.RuntimeID),
		"pluginId":      string(evt.PluginID),
		"previousMode":  string(evt.PreviousMode),
		"newMode":       string(evt.NewMode),
		"previousEpoch": fmt.Sprintf("%d", evt.PreviousEpoch),
		"newEpoch":      fmt.Sprintf("%d", evt.NewEpoch),
		"actor":         string(evt.Actor),
		"reason":        string(evt.Reason),
		"result":        string(evt.Result),
	}
	if evt.Error != "" {
		metadata["error"] = evt.Error
	}

	metadataJSON, _ := json.Marshal(metadata)

	payload := map[string]interface{}{
		"type":      "control_authority_audit",
		"timestamp": evt.Timestamp,
		"metadata":  metadata,
	}
	payloadJSON, _ := json.Marshal(payload)

	opts := event.PublishOptions{
		ProducerExtensionID: extensionID,
		Metadata:            metadataJSON,
	}

	_, _ = a.eventService.Publish(nil, "gamehost.control.audit", 1, payloadJSON, opts)
}

func (a *ControlAuditAdapter) resolveExtensionID(pluginID domain.PluginID) string {
	if a.pluginReg == nil {
		return ""
	}
	descriptor, err := a.pluginReg.Get(nil, pluginID)
	if err != nil {
		return ""
	}
	return descriptor.ExtensionID
}

var _ control.AuthorityAuditSink = (*ControlAuditAdapter)(nil)

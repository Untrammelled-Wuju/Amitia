package integration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/gamehost/contracts"
	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ControlMetricsAdapter struct {
	eventService *event.Service
	pluginReg    contracts.PluginRegistry
}

func NewControlMetricsAdapter(eventService *event.Service, pluginReg contracts.PluginRegistry) *ControlMetricsAdapter {
	return &ControlMetricsAdapter{eventService: eventService, pluginReg: pluginReg}
}

func (a *ControlMetricsAdapter) RecordOutputDecision(runtimeID domain.RuntimeInstanceID, kind control.ControlOutputKind, reason control.OutputDecisionReason, allowed bool) {
	if a.eventService == nil {
		return
	}
	metadata := map[string]string{
		"runtimeId": string(runtimeID),
		"kind":      string(kind),
		"reason":    string(reason),
		"allowed":   fmt.Sprintf("%t", allowed),
	}
	metadataJSON, _ := json.Marshal(metadata)
	payloadJSON, _ := json.Marshal(map[string]interface{}{
		"type":      "control_output_metric",
		"runtimeId": string(runtimeID),
		"kind":      kind,
		"reason":    reason,
		"allowed":   allowed,
	})
	_, _ = a.eventService.Publish(context.Background(), "gamehost.control.metrics", 1, payloadJSON, event.PublishOptions{Metadata: metadataJSON})
}

var _ control.MetricsSink = (*ControlMetricsAdapter)(nil)

package event

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type LifecycleEventEmitter struct {
	emitter HostEventEmitter
}

func NewLifecycleEventEmitter(emitter HostEventEmitter) *LifecycleEventEmitter {
	return &LifecycleEventEmitter{emitter: emitter}
}

type ExtensionLifecyclePayload struct {
	ExtensionID   string `json:"extensionId"`
	Version       string `json:"version"`
	OperationID   string `json:"operationId"`
	Timestamp     string `json:"timestamp"`
	Reason        string `json:"reason,omitempty"`
	PreviousState string `json:"previousState,omitempty"`
	NewState      string `json:"newState,omitempty"`
	ModuleID      string `json:"moduleId,omitempty"`
}

func (l *LifecycleEventEmitter) EmitExtensionInstalled(ctx context.Context, extensionID, version, operationID string) error {
	payload := ExtensionLifecyclePayload{
		ExtensionID: extensionID,
		Version:     version,
		OperationID: operationID,
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		NewState:    "installed",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("event: marshal lifecycle payload: %w", err)
	}
	_, err = l.emitter.Emit(ctx, "extension.installed", 1, data, PublishOptions{
		ProducerID:    "host",
		ProducerType:  EventProducerTypeSystem,
		OperationID:   operationID,
		AggregateType: "extension",
		AggregateID:   extensionID,
	})
	return err
}

func (l *LifecycleEventEmitter) EmitExtensionEnabled(ctx context.Context, extensionID, version, operationID string) error {
	payload := ExtensionLifecyclePayload{
		ExtensionID: extensionID,
		Version:     version,
		OperationID: operationID,
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		NewState:    "enabled",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("event: marshal lifecycle payload: %w", err)
	}
	_, err = l.emitter.Emit(ctx, "extension.enabled", 1, data, PublishOptions{
		ProducerID:    "host",
		ProducerType:  EventProducerTypeSystem,
		OperationID:   operationID,
		AggregateType: "extension",
		AggregateID:   extensionID,
	})
	return err
}

func (l *LifecycleEventEmitter) EmitExtensionDisabled(ctx context.Context, extensionID, version, operationID, reason string) error {
	payload := ExtensionLifecyclePayload{
		ExtensionID: extensionID,
		Version:     version,
		OperationID: operationID,
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		NewState:    "disabled",
		Reason:      reason,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("event: marshal lifecycle payload: %w", err)
	}
	_, err = l.emitter.Emit(ctx, "extension.disabled", 1, data, PublishOptions{
		ProducerID:    "host",
		ProducerType:  EventProducerTypeSystem,
		OperationID:   operationID,
		AggregateType: "extension",
		AggregateID:   extensionID,
	})
	return err
}

func (l *LifecycleEventEmitter) EmitExtensionUninstalled(ctx context.Context, extensionID, version, operationID, reason string) error {
	payload := ExtensionLifecyclePayload{
		ExtensionID: extensionID,
		Version:     version,
		OperationID: operationID,
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		NewState:    "uninstalled",
		Reason:      reason,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("event: marshal lifecycle payload: %w", err)
	}
	_, err = l.emitter.Emit(ctx, "extension.uninstalled", 1, data, PublishOptions{
		ProducerID:    "host",
		ProducerType:  EventProducerTypeSystem,
		OperationID:   operationID,
		AggregateType: "extension",
		AggregateID:   extensionID,
	})
	return err
}

func (l *LifecycleEventEmitter) EmitExtensionUpgraded(ctx context.Context, extensionID, oldVersion, newVersion, operationID string) error {
	payload := ExtensionLifecyclePayload{
		ExtensionID:   extensionID,
		Version:       newVersion,
		OperationID:   operationID,
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		PreviousState: oldVersion,
		NewState:      "upgraded",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("event: marshal lifecycle payload: %w", err)
	}
	_, err = l.emitter.Emit(ctx, "extension.upgraded", 1, data, PublishOptions{
		ProducerID:    "host",
		ProducerType:  EventProducerTypeSystem,
		OperationID:   operationID,
		AggregateType: "extension",
		AggregateID:   extensionID,
	})
	return err
}

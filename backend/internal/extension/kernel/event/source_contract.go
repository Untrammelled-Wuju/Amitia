package event

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func (b *RuntimeBridge) InstalledEventContracts(ctx context.Context, extensionID string, generation int64) ([]EventTypeDefinition, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if generation < 1 || b.extensionGenerations[extensionID] != generation {
		return nil, fmt.Errorf("设备插件事件契约的安装代次无效")
	}
	var contracts []EventTypeDefinition
	for _, id := range b.extensionTypes[extensionID] {
		definition, err := b.service.GetEventType(ctx, id, 1)
		if err != nil {
			continue
		}
		encoded, err := json.Marshal(definition)
		if err != nil || len(encoded) > 32<<10 {
			return nil, fmt.Errorf("设备插件事件契约超过上限")
		}
		var copy EventTypeDefinition
		if err := json.Unmarshal(encoded, &copy); err != nil {
			return nil, err
		}
		copy.DefinitionHash = copy.Hash()
		if _, err := compileSourceEventSchema(copy.PayloadSchema); err != nil {
			return nil, err
		}
		contracts = append(contracts, copy)
		if len(contracts) > 64 {
			return nil, fmt.Errorf("设备插件事件契约数量超过上限")
		}
	}
	return contracts, ctx.Err()
}

type SourceEventProvenance struct {
	ScopeSnapshotID       string `json:"scopeSnapshotId"`
	SourceDeviceID        string `json:"sourceDeviceId"`
	TaskRunID             string `json:"taskRunId"`
	TaskGeneration        int64  `json:"taskGeneration"`
	AttemptID             string `json:"attemptId"`
	RequestID             string `json:"requestId"`
	InstalledGeneration   int64  `json:"installedGeneration"`
	DefinitionFingerprint string `json:"definitionFingerprint"`
	EntryHash             string `json:"entryHash"`
	BundleHash            string `json:"bundleHash"`
	SchemaHash            string `json:"schemaHash"`
	PayloadHash           string `json:"payloadHash"`
}

func (b *RuntimeBridge) PublishSourceContract(ctx context.Context, definition EventTypeDefinition, extensionID string, payload json.RawMessage, opts PublishOptions, provenance ...SourceEventProvenance) (PublishResult, error) {
	if !definition.EventTypeID.IsExtensionNamespace(extensionID) || definition.EventTypeID.IsReservedNamespace() || definition.Version != 1 || definition.DefinitionHash != definition.Hash() {
		return PublishResult{}, fmt.Errorf("原设备事件契约无效")
	}
	compiled, err := compileSourceEventSchema(definition.PayloadSchema)
	if err != nil {
		return PublishResult{}, err
	}
	if compiled != nil {
		var document any
		if json.Unmarshal(payload, &document) != nil || compiled.Validate(document) != nil {
			return PublishResult{}, fmt.Errorf("原设备事件内容不符合固定事件契约")
		}
	}
	registry := NewEventSchemaRegistry()
	if err := registry.RegisterEventType(ctx, definition); err != nil {
		return PublishResult{}, err
	}
	opts.ProducerID, opts.ProducerType, opts.ProducerExtensionID = extensionID, EventProducerTypeExtension, extensionID
	if len(provenance) > 1 {
		return PublishResult{}, fmt.Errorf("原设备事件宿主来源数量无效")
	}
	if len(provenance) == 1 {
		value := provenance[0]
		hash := sha256.Sum256(payload)
		if value.SourceDeviceID == "" || value.TaskRunID != opts.AggregateID || value.ScopeSnapshotID != opts.ScopeSnapshotID || value.InstalledGeneration != opts.ProducerGeneration || value.RequestID != opts.TraceID || value.SchemaHash != definition.Hash() || value.PayloadHash != hex.EncodeToString(hash[:]) || value.TaskGeneration < 1 || value.AttemptID == "" {
			return PublishResult{}, fmt.Errorf("宿主来源审计与原设备事件确认不一致")
		}
		opts.hostProvenance, err = json.Marshal(value)
		if err != nil {
			return PublishResult{}, err
		}
	}
	publisher := NewEventPublisher(registry, b.service.outboxRepo, b.service.db, b.service.loopGuard, b.service.config.MaxDepth)
	return publisher.Publish(ctx, definition.EventTypeID, definition.Version, payload, opts)
}

func (b *RuntimeBridge) PublishInstalledSourceEvent(ctx context.Context, extensionID string, generation int64, typeID EventTypeID, payload json.RawMessage, opts PublishOptions) (PublishResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if generation < 1 || b.extensionGenerations[extensionID] != generation || typeID.IsReservedNamespace() || !typeID.IsExtensionNamespace(extensionID) {
		return PublishResult{}, fmt.Errorf("设备事件与当前插件安装代次或命名空间不一致")
	}
	registered := false
	for _, id := range b.extensionTypes[extensionID] {
		registered = registered || id == typeID
	}
	if !registered {
		return PublishResult{}, fmt.Errorf("当前设备插件未声明此事件类型")
	}
	definition, err := b.service.GetEventType(ctx, typeID, 1)
	if err != nil {
		return PublishResult{}, err
	}
	compiled, err := compileSourceEventSchema(definition.PayloadSchema)
	if err != nil {
		return PublishResult{}, err
	}
	if compiled != nil {
		var document any
		if json.Unmarshal(payload, &document) != nil || compiled.Validate(document) != nil {
			return PublishResult{}, fmt.Errorf("设备事件内容不符合当前安装代次的固定事件契约")
		}
	}
	opts.ProducerGeneration = generation
	return b.PublishFromRuntime(ctx, extensionID, typeID, 1, payload, opts)
}

func compileSourceEventSchema(raw json.RawMessage) (*jsonschema.Schema, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var document any
	if len(raw) > 32<<10 || json.Unmarshal(raw, &document) != nil {
		return nil, fmt.Errorf("原设备事件结构声明无效或超过上限")
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.UseLoader(jsonschema.SchemeURLLoader{})
	const resource = "https://amitia.invalid/source-event.schema.json"
	if err := compiler.AddResource(resource, document); err != nil {
		return nil, err
	}
	compiled, err := compiler.Compile(resource)
	if err != nil {
		return nil, fmt.Errorf("原设备事件结构声明不能安全解析：%w", err)
	}
	return compiled, nil
}

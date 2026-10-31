package business

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	amitialog "log"
)

type MemoryConsumerEntry struct {
	coordination.Resource
	Scope coordination.ExecutionScope `json:"executionScope"`
}

type MemoryConsumerPort interface {
	Read(context.Context, string, string, bool) ([]MemoryConsumerEntry, error)
	Change(context.Context, MemoryConsumerEntry, map[string]json.RawMessage, bool, string) (coordination.Acknowledgement, error)
}

type MemoryConsumerReader func(context.Context, string, int) ([]MemoryConsumerEntry, error)

type MemoryDeletionPort interface {
	DeleteEntries(context.Context, []MemoryConsumerEntry, string) (int64, error)
}

type MemoryRelationPort interface {
	Link(context.Context, string, string, string, string, float64, string) (coordination.Acknowledgement, error)
	Links(context.Context, string, string, int) ([]coordination.Resource, error)
}

func MemoryContent(entry MemoryConsumerEntry) (map[string]json.RawMessage, map[string]json.RawMessage, error) {
	var document map[string]json.RawMessage
	if json.Unmarshal(entry.Body, &document) != nil || document == nil {
		return nil, nil, errors.New("记忆资源内容无效")
	}
	var content map[string]json.RawMessage
	if json.Unmarshal(document["content"], &content) != nil || content == nil {
		return nil, nil, errors.New("记忆资源正文无效")
	}
	return document, content, nil
}

func MemoryText(document map[string]json.RawMessage, key string) string {
	var value string
	if err := json.Unmarshal(document[key], &value); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	return value
}

func MemoryValue(content map[string]json.RawMessage) string {
	for _, key := range []string{"value", "text", "attributeValue", "content", "summary"} {
		if value := MemoryText(content, key); value != "" {
			return value
		}
	}
	facts := map[string]json.RawMessage{}
	for key, value := range content {
		facts[key] = value
	}
	for _, key := range []string{"metadata", "executionScope", "conversationId", "source", "createdAt", "updatedAt", "createdAtUtc", "updatedAtUtc", "importance", "confidence", "memoryType", "memorySubtype", "category", "attributeName", "retentionLevel", "memoryStrength", "pinned", "reinforceCount", "strengthUpdatedAt", "lastReinforcedAt", "decayState", "occurredAtUtc", "endedAtUtc", "validFromUtc", "validToUtc", "localDate", "timezone", "temporalPrecision", "allowContextUse"} {
		delete(facts, key)
	}
	if len(facts) > 0 {
		return string(MemoryJSON(facts))
	}
	return ""
}

func MemoryActive(entry MemoryConsumerEntry) bool {
	return !entry.Deleted && coordination.ResourceUsable(entry.Body, time.Now())
}

func MemoryJSON(value any) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}

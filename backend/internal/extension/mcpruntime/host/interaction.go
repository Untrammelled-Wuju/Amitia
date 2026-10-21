package host

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type SamplingExecutor interface {
	GenerateMCPSampling(context.Context, json.RawMessage) (any, error)
}

type PendingInteraction struct {
	ID        string          `json:"id"`
	ServerID  string          `json:"serverId"`
	Kind      string          `json:"kind"`
	Request   json.RawMessage `json:"request"`
	CreatedAt string          `json:"createdAt"`
	ExpiresAt string          `json:"expiresAt"`
}

type InteractionDecision struct {
	Action  string         `json:"action"`
	Content map[string]any `json:"content"`
}

type pendingEntry struct {
	value    PendingInteraction
	decision chan InteractionDecision
	resolved bool
}

type Broker struct {
	executor SamplingExecutor
	mu       sync.Mutex
	pending  map[string]*pendingEntry
}

func NewBroker(executor SamplingExecutor) *Broker {
	return &Broker{executor: executor, pending: map[string]*pendingEntry{}}
}

func (b *Broker) CreateMessage(ctx context.Context, serverID string, request json.RawMessage) (any, error) {
	decision, err := b.await(ctx, serverID, "sampling", request, 60*time.Second)
	if err != nil {
		return nil, err
	}
	if decision.Action != "accept" {
		return nil, fmt.Errorf("sampling declined by user")
	}
	if b.executor == nil {
		return nil, fmt.Errorf("sampling model router unavailable")
	}
	result, err := b.executor.GenerateMCPSampling(ctx, request)
	if err != nil {
		return nil, err
	}
	resultPayload, err := json.Marshal(result)
	if err != nil || len(resultPayload) > 1<<20 {
		return nil, fmt.Errorf("sampling result invalid")
	}
	resultDecision, err := b.await(ctx, serverID, "sampling_result", resultPayload, 60*time.Second)
	if err != nil {
		return nil, err
	}
	if resultDecision.Action != "accept" {
		return nil, fmt.Errorf("sampling result rejected by user")
	}
	return result, nil
}

func (b *Broker) Elicit(ctx context.Context, serverID string, request json.RawMessage) (any, error) {
	decision, err := b.await(ctx, serverID, "elicitation", request, 5*time.Minute)
	if err != nil {
		return map[string]any{"action": "cancel"}, err
	}
	switch decision.Action {
	case "accept":
		return map[string]any{"action": "accept", "content": decision.Content}, nil
	case "decline":
		return map[string]any{"action": "decline"}, nil
	default:
		return map[string]any{"action": "cancel"}, nil
	}
}

func (b *Broker) await(ctx context.Context, serverID, kind string, request json.RawMessage, maximum time.Duration) (InteractionDecision, error) {
	if len(request) > 1<<20 || !json.Valid(request) {
		return InteractionDecision{}, fmt.Errorf("invalid interaction request")
	}
	deadline := time.Now().Add(maximum)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	entry := &pendingEntry{value: PendingInteraction{ID: uuid.NewString(), ServerID: serverID, Kind: kind, Request: append(json.RawMessage(nil), request...), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), ExpiresAt: deadline.UTC().Format(time.RFC3339Nano)}, decision: make(chan InteractionDecision, 1)}
	b.mu.Lock()
	b.pending[entry.value.ID] = entry
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.pending, entry.value.ID)
		b.mu.Unlock()
	}()
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case decision := <-entry.decision:
		return decision, nil
	case <-ctx.Done():
		return InteractionDecision{}, ctx.Err()
	case <-timer.C:
		return InteractionDecision{}, context.DeadlineExceeded
	}
}

func (b *Broker) List() []PendingInteraction {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	items := make([]PendingInteraction, 0, len(b.pending))
	for id, entry := range b.pending {
		expires, err := time.Parse(time.RFC3339Nano, entry.value.ExpiresAt)
		if err != nil || !expires.After(now) {
			delete(b.pending, id)
			continue
		}
		item := entry.value
		item.Request = append(json.RawMessage(nil), item.Request...)
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return strings.Compare(items[i].CreatedAt, items[j].CreatedAt) < 0 })
	return items
}

func (b *Broker) Resolve(id string, decision InteractionDecision) error {
	decision.Action = strings.ToLower(strings.TrimSpace(decision.Action))
	if decision.Action != "accept" && decision.Action != "decline" && decision.Action != "cancel" {
		return fmt.Errorf("MCP_INTERACTION_INVALID: action")
	}
	b.mu.Lock()
	entry, ok := b.pending[id]
	if ok && decision.Action == "accept" && entry.value.Kind == "elicitation" {
		if err := validateElicitationDecision(entry.value.Request, decision.Content); err != nil {
			b.mu.Unlock()
			return err
		}
	}
	if ok && entry.resolved {
		b.mu.Unlock()
		return fmt.Errorf("MCP_INTERACTION_ALREADY_RESOLVED")
	}
	if ok {
		entry.resolved = true
	}
	b.mu.Unlock()
	if !ok {
		return fmt.Errorf("MCP_INTERACTION_NOT_FOUND")
	}
	select {
	case entry.decision <- decision:
		return nil
	default:
		b.mu.Lock()
		entry.resolved = false
		b.mu.Unlock()
		return fmt.Errorf("MCP_INTERACTION_ALREADY_RESOLVED")
	}
}

func validateElicitationDecision(requestRaw json.RawMessage, content map[string]any) error {
	encoded, err := json.Marshal(content)
	if err != nil || len(encoded) > 64<<10 {
		return fmt.Errorf("MCP_INTERACTION_INVALID: content")
	}
	var request map[string]any
	if json.Unmarshal(requestRaw, &request) != nil {
		return fmt.Errorf("MCP_INTERACTION_INVALID: request")
	}
	if request["mode"] == "url" {
		if len(content) != 0 {
			return fmt.Errorf("MCP_INTERACTION_INVALID: URL content")
		}
		return nil
	}
	schema, _ := request["requestedSchema"].(map[string]any)
	if schema == nil {
		schema, _ = request["schema"].(map[string]any)
	}
	properties, _ := schema["properties"].(map[string]any)
	required := map[string]bool{}
	if values, ok := schema["required"].([]any); ok {
		for _, value := range values {
			if name, valid := value.(string); valid {
				required[name] = true
			}
		}
	}
	for name := range required {
		if value, exists := content[name]; !exists || value == nil || value == "" {
			return fmt.Errorf("MCP_INTERACTION_INVALID: required field")
		}
	}
	for name, value := range content {
		rawField, exists := properties[name]
		if !exists {
			return fmt.Errorf("MCP_INTERACTION_INVALID: unknown field")
		}
		field, _ := rawField.(map[string]any)
		fieldType, _ := field["type"].(string)
		switch fieldType {
		case "string":
			text, valid := value.(string)
			if !valid || len(text) > 10000 {
				return fmt.Errorf("MCP_INTERACTION_INVALID: string field")
			}
		case "number":
			if _, valid := value.(float64); !valid {
				return fmt.Errorf("MCP_INTERACTION_INVALID: number field")
			}
		case "integer":
			number, valid := value.(float64)
			if !valid || number != float64(int64(number)) {
				return fmt.Errorf("MCP_INTERACTION_INVALID: integer field")
			}
		case "boolean":
			if _, valid := value.(bool); !valid {
				return fmt.Errorf("MCP_INTERACTION_INVALID: boolean field")
			}
		default:
			return fmt.Errorf("MCP_INTERACTION_INVALID: field type")
		}
	}
	return nil
}

package hook

import (
	"context"
	"encoding/json"
	"fmt"
)

type HostRevalidator interface {
	Revalidate(ctx context.Context, point HookPointDefinition, phase HookPhase, original, mutated json.RawMessage) error
}

type NopHostRevalidator struct{}

func (NopHostRevalidator) Revalidate(_ context.Context, _ HookPointDefinition, _ HookPhase, _, _ json.RawMessage) error {
	return nil
}

type SchemaRevalidator struct {
	validators map[string]PayloadSchemaValidator
}

type PayloadSchemaValidator interface {
	Validate(payload json.RawMessage) error
}

func NewSchemaRevalidator() *SchemaRevalidator {
	return &SchemaRevalidator{
		validators: make(map[string]PayloadSchemaValidator),
	}
}

func (r *SchemaRevalidator) Register(pointID string, v PayloadSchemaValidator) {
	r.validators[pointID] = v
}

func (r *SchemaRevalidator) Revalidate(_ context.Context, point HookPointDefinition, _ HookPhase, _, mutated json.RawMessage) error {
	v, ok := r.validators[point.HookPointID]
	if !ok {
		if len(point.InputSchema) > 0 {
			return validateAgainstJSONSchema(point.InputSchema, mutated)
		}
		return nil
	}
	return v.Validate(mutated)
}

type ImmutableFieldRevalidator struct {
	immutablePaths map[string][]string
}

func NewImmutableFieldRevalidator() *ImmutableFieldRevalidator {
	return &ImmutableFieldRevalidator{
		immutablePaths: make(map[string][]string),
	}
}

func (r *ImmutableFieldRevalidator) Register(pointID string, paths []string) {
	r.immutablePaths[pointID] = paths
}

func (r *ImmutableFieldRevalidator) Revalidate(_ context.Context, point HookPointDefinition, _ HookPhase, original, mutated json.RawMessage) error {
	paths := r.immutablePaths[point.HookPointID]
	if len(paths) == 0 {
		paths = defaultImmutablePaths(point)
	}
	return verifyImmutablePaths(original, mutated, paths)
}

func defaultImmutablePaths(point HookPointDefinition) []string {
	pointImmutableMap := map[string][]string{
		"tool.before_execute/1":  {"/toolId", "/runtime", "/permission", "/scope", "/idempotencyKey", "/approvalMode"},
		"tool.after_execute/1":   {"/toolId", "/result/sideEffects", "/result/rawOutput"},
		"model.before_request/1": {"/request/model", "/request/apiKey", "/request/baseUrl", "/systemPrompt", "/securityPrompt"},
		"model.after_response/1": {"/response/content", "/response/rawContent", "/response/usage"},
	}
	if paths, ok := pointImmutableMap[point.HookPointID]; ok {
		return paths
	}
	return nil
}

func verifyImmutablePaths(original, mutated json.RawMessage, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	var origMap, mutMap map[string]any
	if err := json.Unmarshal(original, &origMap); err != nil {
		return nil
	}
	if err := json.Unmarshal(mutated, &mutMap); err != nil {
		return nil
	}
	for _, path := range paths {
		origVal, origOk := traverseJSONPath(origMap, path)
		mutVal, mutOk := traverseJSONPath(mutMap, path)
		if origOk != mutOk {
			return fmt.Errorf("immutable field %s presence changed", path)
		}
		if origOk && !jsonEqual(origVal, mutVal) {
			return fmt.Errorf("immutable field %s was modified by hook", path)
		}
	}
	return nil
}

func traverseJSONPath(obj map[string]any, path string) (any, bool) {
	if path == "" || path == "/" {
		return obj, true
	}
	segments := splitJSONPath(path)
	current := any(obj)
	for _, seg := range segments {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		val, exists := m[seg]
		if !exists {
			return nil, false
		}
		current = val
	}
	return current, true
}

func splitJSONPath(path string) []string {
	if path == "" {
		return nil
	}
	if path[0] == '/' {
		path = path[1:]
	}
	if path == "" {
		return nil
	}
	var segments []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if i > start {
				segments = append(segments, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		segments = append(segments, path[start:])
	}
	return segments
}

func jsonEqual(a, b any) bool {
	aj, erra := json.Marshal(a)
	bj, errb := json.Marshal(b)
	if erra != nil || errb != nil {
		return false
	}
	return string(aj) == string(bj)
}

func validateAgainstJSONSchema(schema, payload json.RawMessage) error {
	if len(payload) == 0 {
		return nil
	}
	var parsed map[string]any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return fmt.Errorf("payload is not valid JSON object: %w", err)
	}
	return nil
}

type CompositeRevalidator struct {
	validators []HostRevalidator
}

func NewCompositeRevalidator(validators ...HostRevalidator) *CompositeRevalidator {
	return &CompositeRevalidator{validators: validators}
}

func (r *CompositeRevalidator) Revalidate(ctx context.Context, point HookPointDefinition, phase HookPhase, original, mutated json.RawMessage) error {
	for _, v := range r.validators {
		if err := v.Revalidate(ctx, point, phase, original, mutated); err != nil {
			return err
		}
	}
	return nil
}

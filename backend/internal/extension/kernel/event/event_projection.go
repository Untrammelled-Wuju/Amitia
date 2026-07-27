package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type EventProjectionRequest struct {
	IncludeFields []string `json:"include_fields,omitempty"`
	ExcludeFields []string `json:"exclude_fields,omitempty"`
	RequirePermissions []string `json:"require_permissions,omitempty"`
}

type ProjectionResult struct {
	Payload   json.RawMessage
	Hash      string
	OmittedFields []string
	MaskedFields  []string
}

type PayloadProjector struct {
	rules          []EventProjectionRule
	sensitiveRules []SensitiveFieldRule
}

func NewPayloadProjector(def EventTypeDefinition) *PayloadProjector {
	return &PayloadProjector{
		rules:          def.ProjectionRules,
		sensitiveRules: def.SensitiveFields,
	}
}

func (p *PayloadProjector) Project(payload json.RawMessage, req EventProjectionRequest, grantedPermissions map[string]bool) (ProjectionResult, error) {
	if len(payload) == 0 {
		return ProjectionResult{Payload: json.RawMessage("{}")}, nil
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return ProjectionResult{}, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	projected := make(map[string]any)
	included := make(map[string]bool)
	if len(req.IncludeFields) > 0 {
		for _, f := range req.IncludeFields {
			included[f] = true
		}
	}
	for k, v := range raw {
		if len(included) > 0 && !included[k] {
			continue
		}
		if isExcluded(k, req.ExcludeFields) {
			continue
		}
		rule := p.findSensitiveRule(k)
		if rule != nil {
			switch rule.DefaultAction {
			case SensitiveOmit:
				continue
			case SensitiveMask:
				projected[k] = maskValue(v)
				continue
			case SensitiveHash:
				projected[k] = hashValue(v)
				continue
			case SensitiveSummary:
				projected[k] = summarizeValue(v)
				continue
			case SensitiveAllowWithPermission:
				allowed := false
				for _, req := range rule.RequiredPermission {
					if grantedPermissions[req.Permission] {
						allowed = true
						break
					}
				}
				if !allowed {
					projected[k] = maskValue(v)
					continue
				}
			}
		}
		projected[k] = v
	}
	for _, rule := range p.rules {
		if rule.SourcePath != "" && rule.TargetPath != "" {
			if rule.RequiredPermission != "" && !grantedPermissions[rule.RequiredPermission] {
				continue
			}
			if v, ok := lookupPath(raw, rule.SourcePath); ok {
			setPath(projected, rule.TargetPath, v)
		}
	}
	}
	out, err := json.Marshal(projected)
	if err != nil {
		return ProjectionResult{}, fmt.Errorf("%w: marshal: %v", ErrInvalidProjection, err)
	}
	return ProjectionResult{
		Payload: out,
		Hash:    computePayloadHash(out),
	}, nil
}

func (p *PayloadProjector) findSensitiveRule(field string) *SensitiveFieldRule {
	for i := range p.sensitiveRules {
		if p.sensitiveRules[i].Path == field || matchPath(p.sensitiveRules[i].Path, field) {
			return &p.sensitiveRules[i]
		}
	}
	return nil
}

func isExcluded(field string, excluded []string) bool {
	for _, e := range excluded {
		if e == field {
			return true
		}
	}
	return false
}

func maskValue(v any) any {
	s := fmt.Sprintf("%v", v)
	if len(s) <= 2 {
		return strings.Repeat("*", len(s))
	}
	return s[:1] + strings.Repeat("*", len(s)-2) + s[len(s)-1:]
}

func hashValue(v any) any {
	b, _ := json.Marshal(v)
	h := computePayloadHash(b)
	if len(h) > 8 {
		return h[:8]
	}
	return h
}

func summarizeValue(v any) any {
	s := fmt.Sprintf("%v", v)
	if len(s) <= 32 {
		return s
	}
	return s[:32] + "..."
}

func lookupPath(raw map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var cur any = raw
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := m[p]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

func setPath(target map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	if len(parts) == 1 {
		target[parts[0]] = value
		return
	}
	cur := target
	for i, p := range parts {
		if i == len(parts)-1 {
			cur[p] = value
			return
		}
		if _, ok := cur[p]; !ok {
			cur[p] = make(map[string]any)
		}
		if next, ok := cur[p].(map[string]any); ok {
			cur = next
		} else {
			nm := make(map[string]any)
			cur[p] = nm
			cur = nm
		}
	}
}

func matchPath(pattern, field string) bool {
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, ".")
		fParts := strings.Split(field, ".")
		if len(parts) != len(fParts) {
			return false
		}
		for i := range parts {
			if parts[i] != "*" && parts[i] != fParts[i] {
				return false
			}
		}
		return true
	}
	return pattern == field
}

func DefaultMessageCreatedProjection() EventProjectionRequest {
	return EventProjectionRequest{
		IncludeFields: []string{
			"messageId",
			"conversationId",
			"characterId",
			"direction",
			"messageType",
			"createdAt",
			"hasText",
			"attachmentTypes",
		},
	}
}

func ValidateProjectionRequest(req EventProjectionRequest, def EventTypeDefinition) error {
	if len(req.IncludeFields) == 0 && len(req.ExcludeFields) == 0 {
		return nil
	}
	for _, f := range req.IncludeFields {
		if !isFieldAllowedBySchema(f, def) {
			return fmt.Errorf("%w: field %s not in schema", ErrInvalidProjection, f)
		}
	}
	for _, f := range req.ExcludeFields {
		if !isFieldAllowedBySchema(f, def) {
			return fmt.Errorf("%w: field %s not in schema", ErrInvalidProjection, f)
		}
	}
	return nil
}

func isFieldAllowedBySchema(field string, def EventTypeDefinition) bool {
	if len(def.PayloadSchema) == 0 {
		return true
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(def.PayloadSchema, &schema); err != nil {
		return true
	}
	if schema.Properties == nil {
		return true
	}
	_, ok := schema.Properties[field]
	return ok
}

var _ = errors.New

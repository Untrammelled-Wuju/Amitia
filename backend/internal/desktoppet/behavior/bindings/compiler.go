package bindings

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
)

type astNode struct {
	Op       string            `json:"op"`
	Key      string            `json:"key,omitempty"`
	Value    string            `json:"value,omitempty"`
	Values   []string          `json:"values,omitempty"`
	Min      *float64          `json:"min,omitempty"`
	Max      *float64          `json:"max,omitempty"`
	Children []json.RawMessage `json:"children,omitempty"`
	Child    json.RawMessage   `json:"child,omitempty"`
}

func Compile(conditionsJSON json.RawMessage) (ConditionNode, error) {
	return compileWithFields(conditionsJSON, nil)
}

func CompileWithFields(conditionsJSON json.RawMessage, allowedFields map[string]string) (ConditionNode, error) {
	return compileWithFields(conditionsJSON, allowedFields)
}

func compileWithFields(conditionsJSON json.RawMessage, allowedFields map[string]string) (ConditionNode, error) {
	if len(conditionsJSON) == 0 {
		return nil, nil
	}
	trimmed := strings.TrimSpace(string(conditionsJSON))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var arr []json.RawMessage
		if err := json.Unmarshal(conditionsJSON, &arr); err != nil {
			return nil, fmt.Errorf("invalid conditions array: %w", err)
		}
		if len(arr) == 0 {
			return nil, nil
		}
		children := make([]ConditionNode, 0, len(arr))
		for _, raw := range arr {
			node, err := compileNode(raw, 1, allowedFields)
			if err != nil {
				return nil, err
			}
			children = append(children, node)
		}
		return &AndNode{Children: children}, nil
	}
	return compileNode(conditionsJSON, 1, allowedFields)
}

func compileNode(raw json.RawMessage, depth int, allowedFields map[string]string) (ConditionNode, error) {
	if depth > behavior.MaxBindingASTDepth {
		return nil, fmt.Errorf("AST depth %d exceeds maximum of %d", depth, behavior.MaxBindingASTDepth)
	}
	var node astNode
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("invalid AST node: %w", err)
	}
	if node.Op == "" {
		return nil, fmt.Errorf("AST node missing required 'op' field")
	}
	if allowedFields != nil && node.Key != "" {
		if !isFieldAllowed(node.Key, allowedFields) {
			return nil, fmt.Errorf("field %q is not in the allowed fields schema", node.Key)
		}
	}
	switch node.Op {
	case "eq":
		if node.Key == "" {
			return nil, fmt.Errorf("eq node requires a 'key' field")
		}
		return &EqNode{Key: node.Key, Value: node.Value}, nil
	case "in":
		if node.Key == "" {
			return nil, fmt.Errorf("in node requires a 'key' field")
		}
		return &InNode{Key: node.Key, Values: node.Values}, nil
	case "range":
		if node.Key == "" {
			return nil, fmt.Errorf("range node requires a 'key' field")
		}
		minVal := 0.0
		maxVal := 0.0
		if node.Min != nil {
			minVal = *node.Min
		}
		if node.Max != nil {
			maxVal = *node.Max
		}
		return &RangeNode{Key: node.Key, Min: minVal, Max: maxVal}, nil
	case "exists":
		if node.Key == "" {
			return nil, fmt.Errorf("exists node requires a 'key' field")
		}
		return &ExistsNode{Key: node.Key}, nil
	case "and":
		if len(node.Children) == 0 {
			return nil, fmt.Errorf("and node requires at least one child")
		}
		children := make([]ConditionNode, 0, len(node.Children))
		for i, childRaw := range node.Children {
			if isSelfReference(raw, childRaw) {
				return nil, fmt.Errorf("circular reference detected: and node child %d self-references parent", i)
			}
			child, err := compileNode(childRaw, depth+1, allowedFields)
			if err != nil {
				return nil, err
			}
			children = append(children, child)
		}
		return &AndNode{Children: children}, nil
	case "or":
		if len(node.Children) == 0 {
			return nil, fmt.Errorf("or node requires at least one child")
		}
		children := make([]ConditionNode, 0, len(node.Children))
		for i, childRaw := range node.Children {
			if isSelfReference(raw, childRaw) {
				return nil, fmt.Errorf("circular reference detected: or node child %d self-references parent", i)
			}
			child, err := compileNode(childRaw, depth+1, allowedFields)
			if err != nil {
				return nil, err
			}
			children = append(children, child)
		}
		return &OrNode{Children: children}, nil
	case "not":
		if len(node.Child) == 0 {
			return nil, fmt.Errorf("not node requires a child")
		}
		if isSelfReference(raw, node.Child) {
			return nil, fmt.Errorf("circular reference detected: not node child self-references parent")
		}
		child, err := compileNode(node.Child, depth+1, allowedFields)
		if err != nil {
			return nil, err
		}
		return &NotNode{Child: child}, nil
	default:
		return nil, fmt.Errorf("unknown operator: %s", node.Op)
	}
}

func isSelfReference(parent, child json.RawMessage) bool {
	if len(parent) == 0 || len(child) == 0 {
		return false
	}
	pTrimmed := strings.TrimSpace(string(parent))
	cTrimmed := strings.TrimSpace(string(child))
	return pTrimmed == cTrimmed
}

var envelopeFieldWhitelist = map[string]bool{
	"origin":         true,
	"eventType":      true,
	"userId":         true,
	"characterId":    true,
	"eventId":        true,
	"installationId": true,
	"petInstanceId":  true,
	"conversationId": true,
	"interactionId":  true,
	"sessionId":      true,
	"schemaVersion":  true,
	"priorityHint":   true,
	"correlationId":  true,
	"causationId":    true,
	"sequence":       true,
	"dedupKey":       true,
}

func isFieldAllowed(key string, allowedFields map[string]string) bool {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) == 1 {
		if envelopeFieldWhitelist[parts[0]] {
			return true
		}
		_, ok := allowedFields[parts[0]]
		return ok
	}
	switch parts[0] {
	case "payload":
		_, ok := allowedFields[parts[1]]
		return ok
	case "event":
		return envelopeFieldWhitelist[parts[1]]
	default:
		return false
	}
}

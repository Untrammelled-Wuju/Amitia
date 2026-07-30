package bindings

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type ConditionNode interface {
	Eval(ctx EvalContext) bool
}

type EvalContext struct {
	Event   map[string]interface{}
	Origin  string
	Payload map[string]interface{}
}

type EqNode struct {
	Key   string
	Value string
}

func (n *EqNode) Eval(ctx EvalContext) bool {
	return resolveString(ctx, n.Key) == n.Value
}

type InNode struct {
	Key    string
	Values []string
}

func (n *InNode) Eval(ctx EvalContext) bool {
	s := resolveString(ctx, n.Key)
	for _, v := range n.Values {
		if s == v {
			return true
		}
	}
	return false
}

type RangeNode struct {
	Key string
	Min float64
	Max float64
}

func (n *RangeNode) Eval(ctx EvalContext) bool {
	f, ok := resolveFloat(ctx, n.Key)
	if !ok {
		return false
	}
	return f >= n.Min && f <= n.Max
}

type ExistsNode struct {
	Key string
}

func (n *ExistsNode) Eval(ctx EvalContext) bool {
	return fieldExists(ctx, n.Key)
}

type AndNode struct {
	Children []ConditionNode
}

func (n *AndNode) Eval(ctx EvalContext) bool {
	for _, c := range n.Children {
		if c == nil {
			continue
		}
		if !c.Eval(ctx) {
			return false
		}
	}
	return true
}

type OrNode struct {
	Children []ConditionNode
}

func (n *OrNode) Eval(ctx EvalContext) bool {
	for _, c := range n.Children {
		if c == nil {
			continue
		}
		if c.Eval(ctx) {
			return true
		}
	}
	return false
}

type NotNode struct {
	Child ConditionNode
}

func (n *NotNode) Eval(ctx EvalContext) bool {
	if n.Child == nil {
		return false
	}
	return !n.Child.Eval(ctx)
}

func resolveValue(ctx EvalContext, key string) (interface{}, bool) {
	if key == "origin" {
		return ctx.Origin, true
	}
	parts := strings.SplitN(key, ".", 2)
	if len(parts) == 1 {
		if ctx.Payload != nil {
			v, ok := ctx.Payload[parts[0]]
			return v, ok
		}
		return nil, false
	}
	switch parts[0] {
	case "payload":
		if ctx.Payload != nil {
			v, ok := ctx.Payload[parts[1]]
			return v, ok
		}
		return nil, false
	case "event":
		if ctx.Event != nil {
			v, ok := ctx.Event[parts[1]]
			return v, ok
		}
		return nil, false
	default:
		return nil, false
	}
}

func resolveString(ctx EvalContext, key string) string {
	v, ok := resolveValue(ctx, key)
	if !ok || v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case bool:
		return strconv.FormatBool(val)
	case json.Number:
		return val.String()
	default:
		return fmt.Sprintf("%v", val)
	}
}

func resolveFloat(ctx EvalContext, key string) (float64, bool) {
	v, ok := resolveValue(ctx, key)
	if !ok || v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	case bool:
		if val {
			return 1, true
		}
		return 0, true
	case json.Number:
		f, err := val.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func fieldExists(ctx EvalContext, key string) bool {
	_, ok := resolveValue(ctx, key)
	return ok
}

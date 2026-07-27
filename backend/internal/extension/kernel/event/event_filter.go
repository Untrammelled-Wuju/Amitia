package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type FilterOperator string

const (
	FilterOpEquals       FilterOperator = "equals"
	FilterOpNotEquals    FilterOperator = "not_equals"
	FilterOpIn           FilterOperator = "in"
	FilterOpExists       FilterOperator = "exists"
	FilterOpPrefix       FilterOperator = "prefix"
	FilterOpSuffix       FilterOperator = "suffix"
	FilterOpContains     FilterOperator = "contains"
	FilterOpNumericGT    FilterOperator = "numeric_gt"
	FilterOpNumericGTE   FilterOperator = "numeric_gte"
	FilterOpNumericLT    FilterOperator = "numeric_lt"
	FilterOpNumericLTE   FilterOperator = "numeric_lte"
	FilterOpAnd          FilterOperator = "and"
	FilterOpOr           FilterOperator = "or"
	FilterOpNot          FilterOperator = "not"
)

var allowedOperators = map[FilterOperator]bool{
	FilterOpEquals:     true,
	FilterOpNotEquals:  true,
	FilterOpIn:         true,
	FilterOpExists:     true,
	FilterOpPrefix:     true,
	FilterOpSuffix:     true,
	FilterOpContains:   true,
	FilterOpNumericGT:  true,
	FilterOpNumericGTE: true,
	FilterOpNumericLT:  true,
	FilterOpNumericLTE: true,
	FilterOpAnd:        true,
	FilterOpOr:         true,
	FilterOpNot:        true,
}

type FilterNode struct {
	Operator  FilterOperator     `json:"op"`
	Field     string             `json:"field,omitempty"`
	Value     json.RawMessage    `json:"value,omitempty"`
	Children  []FilterNode       `json:"children,omitempty"`
}

type EventFilterDefinition struct {
	Root     FilterNode `json:"root"`
	Hash     string     `json:"hash"`
}

type CompiledFilter struct {
	def       EventFilterDefinition
	root      filterMatcher
	allowedFields map[string]bool
}

type filterMatcher interface {
	match(fields map[string]any) bool
}

type equalsMatcher struct {
	field string
	value any
}

func (m equalsMatcher) match(fields map[string]any) bool {
	v, ok := fields[m.field]
	if !ok {
		return false
	}
	return fmt.Sprintf("%v", v) == fmt.Sprintf("%v", m.value)
}

type notEqualsMatcher struct {
	field string
	value any
}

func (m notEqualsMatcher) match(fields map[string]any) bool {
	v, ok := fields[m.field]
	if !ok {
		return true
	}
	return fmt.Sprintf("%v", v) != fmt.Sprintf("%v", m.value)
}

type inMatcher struct {
	field   string
	values  []any
}

func (m inMatcher) match(fields map[string]any) bool {
	v, ok := fields[m.field]
	if !ok {
		return false
	}
	s := fmt.Sprintf("%v", v)
	for _, val := range m.values {
		if fmt.Sprintf("%v", val) == s {
			return true
		}
	}
	return false
}

type existsMatcher struct {
	field string
}

func (m existsMatcher) match(fields map[string]any) bool {
	_, ok := fields[m.field]
	return ok
}

type prefixMatcher struct {
	field string
	value string
}

func (m prefixMatcher) match(fields map[string]any) bool {
	v, ok := fields[m.field]
	if !ok {
		return false
	}
	return strings.HasPrefix(fmt.Sprintf("%v", v), m.value)
}

type suffixMatcher struct {
	field string
	value string
}

func (m suffixMatcher) match(fields map[string]any) bool {
	v, ok := fields[m.field]
	if !ok {
		return false
	}
	return strings.HasSuffix(fmt.Sprintf("%v", v), m.value)
}

type containsMatcher struct {
	field string
	value string
}

func (m containsMatcher) match(fields map[string]any) bool {
	v, ok := fields[m.field]
	if !ok {
		return false
	}
	return strings.Contains(fmt.Sprintf("%v", v), m.value)
}

type numericMatcher struct {
	field   string
	value   float64
	compare func(a, b float64) bool
}

func (m numericMatcher) match(fields map[string]any) bool {
	v, ok := fields[m.field]
	if !ok {
		return false
	}
	f, ok := toFloat64(v)
	if !ok {
		return false
	}
	return m.compare(f, m.value)
}

type andMatcher struct {
	children []filterMatcher
}

func (m andMatcher) match(fields map[string]any) bool {
	for _, c := range m.children {
		if !c.match(fields) {
			return false
		}
	}
	return true
}

type orMatcher struct {
	children []filterMatcher
}

func (m orMatcher) match(fields map[string]any) bool {
	for _, c := range m.children {
		if c.match(fields) {
			return true
		}
	}
	return false
}

type notMatcher struct {
	child filterMatcher
}

func (m notMatcher) match(fields map[string]any) bool {
	return !m.child.match(fields)
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func CompileFilter(def EventFilterDefinition, allowedFields []string) (*CompiledFilter, error) {
	if len(allowedFields) == 0 {
		return nil, errors.New("event: allowed fields required")
	}
	allowed := make(map[string]bool, len(allowedFields))
	for _, f := range allowedFields {
		allowed[f] = true
	}
	root, err := compileNode(def.Root, allowed)
	if err != nil {
		return nil, err
	}
	return &CompiledFilter{def: def, root: root, allowedFields: allowed}, nil
}

func compileNode(node FilterNode, allowed map[string]bool) (filterMatcher, error) {
	if !allowedOperators[node.Operator] {
		return nil, fmt.Errorf("%w: unknown operator %s", ErrInvalidFilter, node.Operator)
	}
	switch node.Operator {
	case FilterOpAnd:
		if len(node.Children) == 0 {
			return nil, fmt.Errorf("%w: and requires children", ErrInvalidFilter)
		}
		children := make([]filterMatcher, 0, len(node.Children))
		for _, c := range node.Children {
			m, err := compileNode(c, allowed)
			if err != nil {
				return nil, err
			}
			children = append(children, m)
		}
		return andMatcher{children: children}, nil
	case FilterOpOr:
		if len(node.Children) == 0 {
			return nil, fmt.Errorf("%w: or requires children", ErrInvalidFilter)
		}
		children := make([]filterMatcher, 0, len(node.Children))
		for _, c := range node.Children {
			m, err := compileNode(c, allowed)
			if err != nil {
				return nil, err
			}
			children = append(children, m)
		}
		return orMatcher{children: children}, nil
	case FilterOpNot:
		if len(node.Children) != 1 {
			return nil, fmt.Errorf("%w: not requires exactly one child", ErrInvalidFilter)
		}
		child, err := compileNode(node.Children[0], allowed)
		if err != nil {
			return nil, err
		}
		return notMatcher{child: child}, nil
	case FilterOpExists:
		if node.Field == "" {
			return nil, fmt.Errorf("%w: exists requires field", ErrInvalidFilter)
		}
		if !allowed[node.Field] {
			return nil, fmt.Errorf("%w: field %s not allowed", ErrInvalidFilter, node.Field)
		}
		return existsMatcher{field: node.Field}, nil
	case FilterOpEquals, FilterOpNotEquals:
		if node.Field == "" {
			return nil, fmt.Errorf("%w: %s requires field", ErrInvalidFilter, node.Operator)
		}
		if !allowed[node.Field] {
			return nil, fmt.Errorf("%w: field %s not allowed", ErrInvalidFilter, node.Field)
		}
		var val any
		if len(node.Value) > 0 {
			if err := json.Unmarshal(node.Value, &val); err != nil {
				return nil, fmt.Errorf("%w: invalid value: %v", ErrInvalidFilter, err)
			}
		}
		if node.Operator == FilterOpEquals {
			return equalsMatcher{field: node.Field, value: val}, nil
		}
		return notEqualsMatcher{field: node.Field, value: val}, nil
	case FilterOpIn:
		if node.Field == "" {
			return nil, fmt.Errorf("%w: in requires field", ErrInvalidFilter)
		}
		if !allowed[node.Field] {
			return nil, fmt.Errorf("%w: field %s not allowed", ErrInvalidFilter, node.Field)
		}
		var values []any
		if len(node.Value) > 0 {
			if err := json.Unmarshal(node.Value, &values); err != nil {
				return nil, fmt.Errorf("%w: invalid in values: %v", ErrInvalidFilter, err)
			}
		}
		return inMatcher{field: node.Field, values: values}, nil
	case FilterOpPrefix, FilterOpSuffix, FilterOpContains:
		if node.Field == "" {
			return nil, fmt.Errorf("%w: %s requires field", ErrInvalidFilter, node.Operator)
		}
		if !allowed[node.Field] {
			return nil, fmt.Errorf("%w: field %s not allowed", ErrInvalidFilter, node.Field)
		}
		var s string
		if len(node.Value) > 0 {
			if err := json.Unmarshal(node.Value, &s); err != nil {
				return nil, fmt.Errorf("%w: invalid string value: %v", ErrInvalidFilter, err)
			}
		}
		switch node.Operator {
		case FilterOpPrefix:
			return prefixMatcher{field: node.Field, value: s}, nil
		case FilterOpSuffix:
			return suffixMatcher{field: node.Field, value: s}, nil
		default:
			return containsMatcher{field: node.Field, value: s}, nil
		}
	case FilterOpNumericGT, FilterOpNumericGTE, FilterOpNumericLT, FilterOpNumericLTE:
		if node.Field == "" {
			return nil, fmt.Errorf("%w: %s requires field", ErrInvalidFilter, node.Operator)
		}
		if !allowed[node.Field] {
			return nil, fmt.Errorf("%w: field %s not allowed", ErrInvalidFilter, node.Field)
		}
		var f float64
		if len(node.Value) > 0 {
			if err := json.Unmarshal(node.Value, &f); err != nil {
				return nil, fmt.Errorf("%w: invalid numeric value: %v", ErrInvalidFilter, err)
			}
		}
		var cmp func(a, b float64) bool
		switch node.Operator {
		case FilterOpNumericGT:
			cmp = func(a, b float64) bool { return a > b }
		case FilterOpNumericGTE:
			cmp = func(a, b float64) bool { return a >= b }
		case FilterOpNumericLT:
			cmp = func(a, b float64) bool { return a < b }
		default:
			cmp = func(a, b float64) bool { return a <= b }
		}
		return numericMatcher{field: node.Field, value: f, compare: cmp}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported operator %s", ErrInvalidFilter, node.Operator)
	}
}

func (c *CompiledFilter) Match(fields map[string]any) bool {
	if c == nil || c.root == nil {
		return true
	}
	return c.root.match(fields)
}

func (c *CompiledFilter) Definition() EventFilterDefinition {
	return c.def
}

func (c *CompiledFilter) AllowedFields() map[string]bool {
	return c.allowedFields
}

func (c *CompiledFilter) AllowedFieldsList() []string {
	if c == nil || c.allowedFields == nil {
		return nil
	}
	result := make([]string, 0, len(c.allowedFields))
	for f := range c.allowedFields {
		result = append(result, f)
	}
	return result
}

func ExtractFilterFields(payload json.RawMessage, allowedFields []string) map[string]any {
	if len(payload) == 0 {
		return map[string]any{}
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return map[string]any{}
	}
	result := make(map[string]any, len(allowedFields))
	for _, f := range allowedFields {
		if v, ok := raw[f]; ok {
			result[f] = v
		}
	}
	return result
}

package workflow

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type RefSource string

const (
	RefSourceInput     RefSource = "input"
	RefSourceConfig    RefSource = "config"
	RefSourceRuntime   RefSource = "runtime"
	RefSourceNodeOutput RefSource = "node_output"
	RefSourceLiteral   RefSource = "literal"
)

type WorkflowValueRef struct {
	Source RefSource `json:"source"`
	NodeID string    `json:"nodeId,omitempty"`
	Path   []string  `json:"path,omitempty"`
}

func ParseWorkflowValueRef(s string) (*WorkflowValueRef, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty value ref")
	}

	knownPrefixes := []string{"input.", "config.", "runtime.", "steps.", "node.", "literal:"}
	for _, prefix := range knownPrefixes {
		if strings.HasPrefix(s, prefix) {
			rest := s[len(prefix):]
			switch prefix {
			case "input.", "config.", "runtime.":
				src := RefSource(prefix[:len(prefix)-1])
				path := splitRefPath(rest)
				if len(path) == 0 {
					return nil, fmt.Errorf("value ref has no path: %s", s)
				}
				return &WorkflowValueRef{Source: src, Path: path}, nil
			case "steps.", "node.":
				parts := splitRefPath(rest)
				if len(parts) < 2 {
					return nil, fmt.Errorf("node output ref must include node id and path: %s", s)
				}
				return &WorkflowValueRef{Source: RefSourceNodeOutput, NodeID: parts[0], Path: parts[1:]}, nil
			case "literal:":
				return &WorkflowValueRef{Source: RefSourceLiteral, Path: []string{rest}}, nil
			}
		}
	}
	return nil, fmt.Errorf("unknown value ref format: %s", s)
}

func splitRefPath(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ".")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (r *WorkflowValueRef) String() string {
	switch r.Source {
	case RefSourceInput, RefSourceConfig, RefSourceRuntime:
		return string(r.Source) + "." + strings.Join(r.Path, ".")
	case RefSourceNodeOutput:
		return "steps." + r.NodeID + "." + strings.Join(r.Path, ".")
	case RefSourceLiteral:
		return "literal:" + strings.Join(r.Path, ".")
	default:
		return string(r.Source)
	}
}

type ExpressionOp string

const (
	OpEq         ExpressionOp = "eq"
	OpNe         ExpressionOp = "ne"
	OpGt         ExpressionOp = "gt"
	OpGte        ExpressionOp = "gte"
	OpLt         ExpressionOp = "lt"
	OpLte        ExpressionOp = "lte"
	OpAnd        ExpressionOp = "and"
	OpOr         ExpressionOp = "or"
	OpNot        ExpressionOp = "not"
	OpExists     ExpressionOp = "exists"
	OpIsNull     ExpressionOp = "is_null"
	OpIn         ExpressionOp = "in"
	OpNotIn      ExpressionOp = "not_in"
	OpContains   ExpressionOp = "contains"
	OpStartsWith ExpressionOp = "starts_with"
	OpEndsWith   ExpressionOp = "ends_with"
	OpMatches    ExpressionOp = "matches"
)

const (
	MaxExpressionDepth = 16
	MaxExpressionNodes = 256
	MaxExpressionBytes = 64 * 1024
)

type WorkflowExpression struct {
	Op    ExpressionOp    `json:"op"`
	Left  *WorkflowExpression `json:"left,omitempty"`
	Right *WorkflowExpression `json:"right,omitempty"`
	Args  []*WorkflowExpression `json:"args,omitempty"`
	Ref   *WorkflowValueRef `json:"ref,omitempty"`
	Value any               `json:"value,omitempty"`
}

func CompileExpression(raw json.RawMessage) (*WorkflowExpression, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) > MaxExpressionBytes {
		return nil, fmt.Errorf("expression exceeds max size %d", MaxExpressionBytes)
	}
	var expr WorkflowExpression
	if err := json.Unmarshal(raw, &expr); err != nil {
		return nil, fmt.Errorf("parse expression: %w", err)
	}
	if err := validateExpression(&expr, 0); err != nil {
		return nil, err
	}
	return &expr, nil
}

func validateExpression(expr *WorkflowExpression, depth int) error {
	if expr == nil {
		return nil
	}
	if depth > MaxExpressionDepth {
		return fmt.Errorf("expression depth exceeds %d", MaxExpressionDepth)
	}
	if expr.Op == "" {
		if expr.Ref != nil {
			return nil
		}
		if expr.Value != nil {
			return nil
		}
		if expr.Left != nil {
			return validateExpression(expr.Left, depth+1)
		}
		return nil
	}
	switch expr.Op {
	case OpAnd, OpOr:
		if len(expr.Args) == 0 {
			return fmt.Errorf("expression op %s requires args", expr.Op)
		}
		for _, arg := range expr.Args {
			if err := validateExpression(arg, depth+1); err != nil {
				return err
			}
		}
	case OpNot:
		if expr.Right == nil {
			return fmt.Errorf("expression op not requires right operand")
		}
		return validateExpression(expr.Right, depth+1)
	case OpEq, OpNe, OpGt, OpGte, OpLt, OpLte,
		OpIn, OpNotIn, OpContains, OpStartsWith, OpEndsWith, OpMatches:
		if expr.Left == nil || expr.Right == nil {
			return fmt.Errorf("expression op %s requires left and right operands", expr.Op)
		}
		if err := validateExpression(expr.Left, depth+1); err != nil {
			return err
		}
		return validateExpression(expr.Right, depth+1)
	case OpExists, OpIsNull:
		if expr.Ref == nil {
			return fmt.Errorf("expression op %s requires ref", expr.Op)
		}
	default:
		return fmt.Errorf("unknown expression op: %s", expr.Op)
	}
	return nil
}

type ExpressionEvalConfig struct {
	Input   map[string]any
	Config  map[string]any
	Runtime map[string]any
	Outputs map[string]json.RawMessage
	Now     time.Time
}

func EvaluateExpression(expr *WorkflowExpression, cfg ExpressionEvalConfig) (bool, error) {
	if expr == nil {
		return true, nil
	}
	return evalNode(expr, cfg, 0)
}

func evalNode(expr *WorkflowExpression, cfg ExpressionEvalConfig, depth int) (bool, error) {
	if depth > MaxExpressionDepth {
		return false, fmt.Errorf("expression evaluation depth exceeded")
	}
	switch expr.Op {
	case OpAnd:
		for _, arg := range expr.Args {
			v, err := evalNode(arg, cfg, depth+1)
			if err != nil {
				return false, err
			}
			if !v {
				return false, nil
			}
		}
		return true, nil
	case OpOr:
		for _, arg := range expr.Args {
			v, err := evalNode(arg, cfg, depth+1)
			if err != nil {
				return false, err
			}
			if v {
				return true, nil
			}
		}
		return false, nil
	case OpNot:
		v, err := evalNode(expr.Right, cfg, depth+1)
		if err != nil {
			return false, err
		}
		return !v, nil
	case OpExists:
		v, err := resolveRef(expr.Ref, cfg)
		if err != nil {
			return false, err
		}
		return v != nil, nil
	case OpIsNull:
		v, err := resolveRef(expr.Ref, cfg)
		if err != nil {
			return false, err
		}
		return v == nil, nil
	case OpEq, OpNe, OpGt, OpGte, OpLt, OpLte,
		OpIn, OpNotIn, OpContains, OpStartsWith, OpEndsWith, OpMatches:
		left, err := evalOperand(expr.Left, cfg, depth+1)
		if err != nil {
			return false, err
		}
		right, err := evalOperand(expr.Right, cfg, depth+1)
		if err != nil {
			return false, err
		}
		return compareValues(expr.Op, left, right)
	default:
		return false, fmt.Errorf("unknown op: %s", expr.Op)
	}
}

func evalOperand(expr *WorkflowExpression, cfg ExpressionEvalConfig, depth int) (any, error) {
	if expr.Ref != nil {
		return resolveRef(expr.Ref, cfg)
	}
	if expr.Value != nil {
		return expr.Value, nil
	}
	if expr.Op != "" {
		return evalNode(expr, cfg, depth+1)
	}
	return nil, nil
}

func resolveRef(ref *WorkflowValueRef, cfg ExpressionEvalConfig) (any, error) {
	if ref == nil {
		return nil, nil
	}
	var root any
	switch ref.Source {
	case RefSourceInput:
		root = cfg.Input
	case RefSourceConfig:
		root = cfg.Config
	case RefSourceRuntime:
		root = cfg.Runtime
	case RefSourceNodeOutput:
		if cfg.Outputs == nil {
			return nil, nil
		}
		raw, ok := cfg.Outputs[ref.NodeID]
		if !ok || len(raw) == 0 {
			return nil, nil
		}
		var parsed any
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, fmt.Errorf("parse node output: %w", err)
		}
		root = parsed
	case RefSourceLiteral:
		if len(ref.Path) > 0 {
			return ref.Path[0], nil
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown ref source: %s", ref.Source)
	}
	if root == nil {
		return nil, nil
	}
	return traversePath(root, ref.Path), nil
}

func traversePath(v any, path []string) any {
	cur := v
	for _, key := range path {
		switch m := cur.(type) {
		case map[string]any:
			cur = m[key]
		case map[string]json.RawMessage:
			cur = m[key]
		default:
			return nil
		}
		if cur == nil {
			return nil
		}
	}
	return cur
}

func compareValues(op ExpressionOp, left, right any) (bool, error) {
	switch op {
	case OpEq:
		return fmt.Sprintf("%v", left) == fmt.Sprintf("%v", right), nil
	case OpNe:
		return fmt.Sprintf("%v", left) != fmt.Sprintf("%v", right), nil
	case OpGt, OpGte, OpLt, OpLte:
		lf, rf, err := toFloatPair(left, right)
		if err != nil {
			return false, err
		}
		switch op {
		case OpGt:
			return lf > rf, nil
		case OpGte:
			return lf >= rf, nil
		case OpLt:
			return lf < rf, nil
		case OpLte:
			return lf <= rf, nil
		}
	case OpIn:
		return collectionContains(right, left)
	case OpNotIn:
		inside, err := collectionContains(right, left)
		if err != nil {
			return false, err
		}
		return !inside, nil
	case OpContains:
		return stringContains(left, right)
	case OpStartsWith:
		return stringStartsWith(left, right)
	case OpEndsWith:
		return stringEndsWith(left, right)
	case OpMatches:
		return stringMatches(left, right)
	}
	return false, nil
}

func toFloatPair(left, right any) (float64, float64, error) {
	var lf, rf float64
	switch v := left.(type) {
	case float64:
		lf = v
	case int:
		lf = float64(v)
	case int64:
		lf = float64(v)
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, 0, fmt.Errorf("convert left operand: %w", err)
		}
		lf = f
	case string:
		var err error
		lf, err = json.Number(v).Float64()
		if err != nil {
			return 0, 0, fmt.Errorf("convert left operand: %w", err)
		}
	default:
		return 0, 0, fmt.Errorf("cannot compare non-numeric left operand %T", left)
	}
	switch v := right.(type) {
	case float64:
		rf = v
	case int:
		rf = float64(v)
	case int64:
		rf = float64(v)
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, 0, fmt.Errorf("convert right operand: %w", err)
		}
		rf = f
	case string:
		var err error
		rf, err = json.Number(v).Float64()
		if err != nil {
			return 0, 0, fmt.Errorf("convert right operand: %w", err)
		}
	default:
		return 0, 0, fmt.Errorf("cannot compare non-numeric right operand %T", right)
	}
	return lf, rf, nil
}

func collectionContains(collection, item any) (bool, error) {
	switch v := collection.(type) {
	case []any:
		for _, elem := range v {
			if fmt.Sprintf("%v", elem) == fmt.Sprintf("%v", item) {
				return true, nil
			}
		}
		return false, nil
	case []string:
		for _, elem := range v {
			if elem == fmt.Sprintf("%v", item) {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("in/not_in requires collection")
	}
}

func stringContains(haystack, needle any) (bool, error) {
	hs := fmt.Sprintf("%v", haystack)
	n := fmt.Sprintf("%v", needle)
	return strings.Contains(hs, n), nil
}

func stringStartsWith(s, prefix any) (bool, error) {
	return strings.HasPrefix(fmt.Sprintf("%v", s), fmt.Sprintf("%v", prefix)), nil
}

func stringEndsWith(s, suffix any) (bool, error) {
	return strings.HasSuffix(fmt.Sprintf("%v", s), fmt.Sprintf("%v", suffix)), nil
}

func stringMatches(s, pattern any) (bool, error) {
	p := fmt.Sprintf("%v", pattern)
	if len(p) > 4096 {
		return false, fmt.Errorf("regex pattern too long")
	}
	re, err := regexp.Compile(p)
	if err != nil {
		return false, fmt.Errorf("invalid regex: %w", err)
	}
	return re.MatchString(fmt.Sprintf("%v", s)), nil
}

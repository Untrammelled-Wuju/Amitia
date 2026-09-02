package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultBuiltinDataMaxItems = 10000

type PassthroughHandler struct{}

func (PassthroughHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return input, nil
}

// ConditionHandler keeps the historical passthrough behavior unless an
// explicit condition operation is configured in runtime metadata (or the
// dedicated conditionOp input field). Business payloads are allowed to contain
// an unrelated "op" key without silently changing old workflow semantics.
type ConditionHandler struct{}

func (ConditionHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var payload any
	if err := decodeJSONAny(input, &payload); err != nil {
		return nil, fmt.Errorf("condition input: %w", err)
	}
	object, ok := payload.(map[string]any)
	if !ok {
		return input, nil
	}
	op := normalizeBuiltinOp(builtinFirstString(node.Runtime.Metadata["op"], object["conditionOp"]))
	if op == "" {
		return input, nil
	}
	result, err := evaluateLogicObject(object, op)
	if err != nil {
		return nil, fmt.Errorf("condition: %w", err)
	}
	out := cloneAnyMap(object)
	out["result"] = result
	return marshalBuiltinOutput(out)
}

// LogicHandler implements pure boolean/comparison nodes. It intentionally does
// not execute arbitrary expressions or code: all supported operations are
// bounded and deterministic, which keeps Dry Run and recovery semantics exact.
type LogicHandler struct{}

func (LogicHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var object map[string]any
	if err := json.Unmarshal(input, &object); err != nil {
		return nil, fmt.Errorf("logic input: %w", err)
	}
	op := normalizeBuiltinOp(builtinFirstString(object["op"], node.Runtime.Metadata["op"]))
	if op == "" {
		return nil, fmt.Errorf("logic op is required")
	}
	result, err := evaluateLogicObject(object, op)
	if err != nil {
		return nil, fmt.Errorf("logic: %w", err)
	}
	return marshalBuiltinOutput(map[string]any{"result": result})
}

// ExtractHandler provides JSONPath-like extraction for common workflow data
// shaping. Supported paths include "a.b", "items[0].name" and wildcard
// segments such as "items[*].id". It is deliberately read-only.
type ExtractHandler struct{}

func (ExtractHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var object map[string]any
	if err := json.Unmarshal(input, &object); err != nil {
		return nil, fmt.Errorf("extract input: %w", err)
	}
	source := builtinSource(object)
	if source == nil {
		// When no explicit source is configured, treat non-control fields as the
		// source object. This keeps simple drag-and-map workflows ergonomic.
		source = stripBuiltinControlFields(object, "path", "paths", "aliases", "required", "default", "unwrap", "mode")
	}
	required := boolValue(object["required"])
	unwrap := true
	if value, exists := object["unwrap"]; exists {
		unwrap = boolValue(value)
	}
	path := strings.TrimSpace(builtinFirstString(object["path"], node.Runtime.Metadata["path"]))
	paths := stringSliceAny(object["paths"])
	if path != "" && len(paths) == 0 {
		value, found, err := extractBuiltinPath(source, path, defaultBuiltinDataMaxItems)
		if err != nil {
			return nil, err
		}
		if !found {
			if fallback, exists := object["default"]; exists {
				value = fallback
				found = true
			}
		}
		if !found && required {
			return nil, fmt.Errorf("extract path %q not found", path)
		}
		if !found {
			value = nil
		}
		if unwrap {
			return marshalBuiltinOutput(value)
		}
		return marshalBuiltinOutput(map[string]any{"value": value, "found": found, "path": path})
	}
	if len(paths) == 0 {
		if fields := stringSliceAny(object["fields"]); len(fields) > 0 {
			paths = fields
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("extract requires path or paths")
	}
	aliases := stringMapAny(object["aliases"])
	result := make(map[string]any, len(paths))
	for _, itemPath := range paths {
		value, found, err := extractBuiltinPath(source, itemPath, defaultBuiltinDataMaxItems)
		if err != nil {
			return nil, err
		}
		if !found {
			if required {
				return nil, fmt.Errorf("extract path %q not found", itemPath)
			}
			continue
		}
		key := aliases[itemPath]
		if strings.TrimSpace(key) == "" {
			key = builtinPathResultKey(itemPath)
		}
		result[key] = value
	}
	return marshalBuiltinOutput(result)
}

// TransformHandler is the workflow-v2 data transformation runtime. It contains
// the operations that previously only existed in the legacy Workshop executor,
// plus a few common low-risk data operations used by Android automation flows.
type TransformHandler struct{}

func (TransformHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var object map[string]any
	if err := json.Unmarshal(input, &object); err != nil {
		return nil, fmt.Errorf("transform input: %w", err)
	}
	op := normalizeBuiltinOp(builtinFirstString(object["op"], node.Runtime.Metadata["op"]))
	if op == "" {
		// Compatibility with the original kernel handler: metadata.field means
		// select one top-level field and emit it directly.
		field := strings.TrimSpace(builtinFirstString(node.Runtime.Metadata["field"]))
		if field == "" {
			return input, nil
		}
		value, found, err := extractBuiltinPath(object, field, defaultBuiltinDataMaxItems)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("transform field %s not found", field)
		}
		return marshalBuiltinOutput(value)
	}

	source := builtinSource(object)
	if source == nil && op != "coalesce" {
		source = object
	}
	result, err := executeBuiltinTransform(op, source, object)
	if err != nil {
		return nil, fmt.Errorf("transform %s: %w", op, err)
	}
	return marshalBuiltinOutput(result)
}

func executeBuiltinTransform(op string, source any, cfg map[string]any) (any, error) {
	fields := stringSliceAny(cfg["fields"])
	switch op {
	case "pick":
		object, ok := asAnyMap(source)
		if !ok {
			return nil, fmt.Errorf("pick requires object source")
		}
		result := map[string]any{}
		for _, field := range fields {
			if value, exists := object[field]; exists {
				result[field] = value
			}
		}
		return result, nil
	case "omit":
		object, ok := asAnyMap(source)
		if !ok {
			return nil, fmt.Errorf("omit requires object source")
		}
		result := cloneAnyMap(object)
		for _, field := range fields {
			delete(result, field)
		}
		return result, nil
	case "rename":
		object, ok := asAnyMap(source)
		if !ok {
			return nil, fmt.Errorf("rename requires object source")
		}
		result := cloneAnyMap(object)
		for from, target := range stringMapAny(cfg["mapping"]) {
			if value, exists := result[from]; exists {
				result[target] = value
				delete(result, from)
			}
		}
		return result, nil
	case "set":
		object, ok := asAnyMap(source)
		if !ok {
			return nil, fmt.Errorf("set requires object source")
		}
		result := cloneAnyMap(object)
		values, _ := asAnyMap(cfg["values"])
		for key, value := range values {
			result[key] = value
		}
		return result, nil
	case "merge":
		object, ok := asAnyMap(source)
		if !ok {
			return nil, fmt.Errorf("merge requires object source")
		}
		result := cloneAnyMap(object)
		other, ok := asAnyMap(cfg["with"])
		if !ok {
			return nil, fmt.Errorf("merge requires object 'with'")
		}
		for key, value := range other {
			result[key] = value
		}
		return result, nil
	case "flatten":
		array, ok := asAnySlice(source)
		if !ok {
			return nil, fmt.Errorf("flatten requires array source")
		}
		result := make([]any, 0, len(array))
		for _, item := range array {
			if nested, ok := asAnySlice(item); ok {
				result = append(result, nested...)
			} else {
				result = append(result, item)
			}
			if len(result) > defaultBuiltinDataMaxItems {
				return nil, fmt.Errorf("array result exceeds %d items", defaultBuiltinDataMaxItems)
			}
		}
		return result, nil
	case "array_map":
		array, ok := asAnySlice(source)
		if !ok {
			return nil, fmt.Errorf("array_map requires array source")
		}
		if len(array) > defaultBuiltinDataMaxItems {
			return nil, fmt.Errorf("array exceeds %d items", defaultBuiltinDataMaxItems)
		}
		path := strings.TrimSpace(builtinFirstString(cfg["path"], cfg["field"]))
		if path == "" && len(fields) == 0 {
			return nil, fmt.Errorf("array_map requires path/field or fields")
		}
		result := make([]any, 0, len(array))
		for _, item := range array {
			if path != "" {
				value, _, err := extractBuiltinPath(item, path, defaultBuiltinDataMaxItems)
				if err != nil {
					return nil, err
				}
				result = append(result, value)
				continue
			}
			itemObject, ok := asAnyMap(item)
			if !ok {
				return nil, fmt.Errorf("array_map fields mode requires object elements")
			}
			mapped := map[string]any{}
			for _, field := range fields {
				if value, exists := itemObject[field]; exists {
					mapped[field] = value
				}
			}
			result = append(result, mapped)
		}
		return result, nil
	case "array_filter":
		array, ok := asAnySlice(source)
		if !ok {
			return nil, fmt.Errorf("array_filter requires array source")
		}
		if len(array) > defaultBuiltinDataMaxItems {
			return nil, fmt.Errorf("array exceeds %d items", defaultBuiltinDataMaxItems)
		}
		path := strings.TrimSpace(builtinFirstString(cfg["path"], cfg["field"]))
		operator := normalizeBuiltinOp(builtinFirstString(cfg["operator"], cfg["compare"]))
		if path == "" || operator == "" {
			return nil, fmt.Errorf("array_filter requires field/path and operator")
		}
		expected := cfg["expected"]
		result := make([]any, 0, len(array))
		for _, item := range array {
			actual, _, err := extractBuiltinPath(item, path, defaultBuiltinDataMaxItems)
			if err != nil {
				return nil, err
			}
			matched, err := compareBuiltin(operator, actual, expected)
			if err != nil {
				return nil, err
			}
			if matched {
				result = append(result, item)
			}
		}
		return result, nil
	case "array_take":
		array, ok := asAnySlice(source)
		if !ok {
			return nil, fmt.Errorf("array_take requires array source")
		}
		count := int(numberValue(cfg["count"], 0))
		if count < 0 {
			count = 0
		}
		if count > len(array) {
			count = len(array)
		}
		return append([]any(nil), array[:count]...), nil
	case "array_sort":
		array, ok := asAnySlice(source)
		if !ok {
			return nil, fmt.Errorf("array_sort requires array source")
		}
		if len(array) > defaultBuiltinDataMaxItems {
			return nil, fmt.Errorf("array exceeds %d items", defaultBuiltinDataMaxItems)
		}
		path := strings.TrimSpace(builtinFirstString(cfg["path"], cfg["field"]))
		if path == "" {
			return nil, fmt.Errorf("array_sort requires field/path")
		}
		descending := strings.EqualFold(builtinFirstString(cfg["direction"]), "desc") || boolValue(cfg["descending"])
		result := append([]any(nil), array...)
		sort.SliceStable(result, func(i, j int) bool {
			left, _, _ := extractBuiltinPath(result[i], path, defaultBuiltinDataMaxItems)
			right, _, _ := extractBuiltinPath(result[j], path, defaultBuiltinDataMaxItems)
			cmp := compareOrderBuiltin(left, right)
			if descending {
				return cmp > 0
			}
			return cmp < 0
		})
		return result, nil
	case "to_string":
		if text, ok := source.(string); ok {
			return text, nil
		}
		if source == nil {
			return "", nil
		}
		if raw, err := json.Marshal(source); err == nil && (isCompositeBuiltin(source)) {
			return string(raw), nil
		}
		return fmt.Sprint(source), nil
	case "to_number":
		number, ok := floatBuiltin(source)
		if !ok {
			return nil, fmt.Errorf("value cannot be converted to number")
		}
		return number, nil
	case "to_boolean":
		return truthyBuiltin(source), nil
	case "json_parse":
		text, ok := source.(string)
		if !ok {
			return nil, fmt.Errorf("json_parse requires string source")
		}
		var value any
		decoder := json.NewDecoder(strings.NewReader(text))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		return normalizeJSONNumbers(value), nil
	case "json_stringify":
		pretty := boolValue(cfg["pretty"])
		var raw []byte
		var err error
		if pretty {
			raw, err = json.MarshalIndent(source, "", "  ")
		} else {
			raw, err = json.Marshal(source)
		}
		if err != nil {
			return nil, err
		}
		return string(raw), nil
	case "unique":
		array, ok := asAnySlice(source)
		if !ok {
			return nil, fmt.Errorf("unique requires array source")
		}
		path := strings.TrimSpace(builtinFirstString(cfg["path"], cfg["field"]))
		seen := map[string]struct{}{}
		result := make([]any, 0, len(array))
		for _, item := range array {
			keyValue := item
			if path != "" {
				keyValue, _, _ = extractBuiltinPath(item, path, defaultBuiltinDataMaxItems)
			}
			key := stableBuiltinKey(keyValue)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, item)
		}
		return result, nil
	case "join":
		array, ok := asAnySlice(source)
		if !ok {
			return nil, fmt.Errorf("join requires array source")
		}
		separator := builtinFirstString(cfg["separator"], ",")
		parts := make([]string, len(array))
		for i, item := range array {
			parts[i] = fmt.Sprint(item)
		}
		return strings.Join(parts, separator), nil
	case "split":
		text, ok := source.(string)
		if !ok {
			return nil, fmt.Errorf("split requires string source")
		}
		separator := builtinFirstString(cfg["separator"], ",")
		if separator == "" {
			return nil, fmt.Errorf("split separator must not be empty")
		}
		parts := strings.Split(text, separator)
		if len(parts) > defaultBuiltinDataMaxItems {
			return nil, fmt.Errorf("split result exceeds %d items", defaultBuiltinDataMaxItems)
		}
		out := make([]any, len(parts))
		for i := range parts {
			out[i] = parts[i]
		}
		return out, nil
	case "length", "count":
		switch value := source.(type) {
		case string:
			return len([]rune(value)), nil
		case []any:
			return len(value), nil
		case map[string]any:
			return len(value), nil
		default:
			return 0, nil
		}
	case "coalesce":
		values, _ := asAnySlice(cfg["values"])
		if len(values) == 0 {
			values = []any{cfg["value"], cfg["source"], cfg["fallback"]}
		}
		for _, value := range values {
			if !emptyBuiltin(value) {
				return value, nil
			}
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported operation")
	}
}

func evaluateLogicObject(object map[string]any, op string) (bool, error) {
	return evaluateLogicObjectDepth(object, op, 0)
}

func evaluateLogicObjectDepth(object map[string]any, op string, depth int) (bool, error) {
	if depth > 32 {
		return false, fmt.Errorf("logic nesting exceeds 32 levels")
	}
	op = normalizeBuiltinOp(op)
	evaluateArg := func(value any) (bool, error) {
		if nested, ok := asAnyMap(value); ok {
			nestedOp := normalizeBuiltinOp(builtinFirstString(nested["op"]))
			if nestedOp != "" {
				return evaluateLogicObjectDepth(nested, nestedOp, depth+1)
			}
		}
		return truthyBuiltin(value), nil
	}
	switch op {
	case "and", "all":
		args, _ := asAnySlice(object["args"])
		if len(args) == 0 {
			return false, fmt.Errorf("%s requires args", op)
		}
		for _, arg := range args {
			matched, err := evaluateArg(arg)
			if err != nil {
				return false, err
			}
			if !matched {
				return false, nil
			}
		}
		return true, nil
	case "or", "any":
		args, _ := asAnySlice(object["args"])
		if len(args) == 0 {
			return false, fmt.Errorf("%s requires args", op)
		}
		for _, arg := range args {
			matched, err := evaluateArg(arg)
			if err != nil {
				return false, err
			}
			if matched {
				return true, nil
			}
		}
		return false, nil
	case "not":
		returnValue, err := evaluateArg(firstAny(object["value"], object["right"]))
		return !returnValue, err
	case "xor":
		args, _ := asAnySlice(object["args"])
		if len(args) == 0 {
			return false, fmt.Errorf("xor requires args")
		}
		count := 0
		for _, arg := range args {
			matched, err := evaluateArg(arg)
			if err != nil {
				return false, err
			}
			if matched {
				count++
			}
		}
		return count == 1, nil
	case "exists":
		_, exists := object["value"]
		return exists && object["value"] != nil, nil
	case "empty":
		return emptyBuiltin(object["value"]), nil
	case "not_empty":
		return !emptyBuiltin(object["value"]), nil
	case "truthy":
		return truthyBuiltin(object["value"]), nil
	case "falsy":
		return !truthyBuiltin(object["value"]), nil
	default:
		return compareBuiltin(op, object["left"], object["right"])
	}
}

func compareBuiltin(op string, left, right any) (bool, error) {
	op = normalizeBuiltinOp(op)
	switch op {
	case "eq", "equals":
		return reflect.DeepEqual(normalizeComparableBuiltin(left), normalizeComparableBuiltin(right)), nil
	case "ne", "neq", "not_equals":
		return !reflect.DeepEqual(normalizeComparableBuiltin(left), normalizeComparableBuiltin(right)), nil
	case "gt", "gte", "lt", "lte":
		lf, lok := floatBuiltin(left)
		rf, rok := floatBuiltin(right)
		if !lok || !rok {
			return false, fmt.Errorf("%s requires numeric operands", op)
		}
		switch op {
		case "gt":
			return lf > rf, nil
		case "gte":
			return lf >= rf, nil
		case "lt":
			return lf < rf, nil
		default:
			return lf <= rf, nil
		}
	case "contains", "not_contains":
		matched := false
		switch value := left.(type) {
		case string:
			matched = strings.Contains(value, fmt.Sprint(right))
		case []any:
			for _, item := range value {
				if reflect.DeepEqual(normalizeComparableBuiltin(item), normalizeComparableBuiltin(right)) {
					matched = true
					break
				}
			}
		case map[string]any:
			_, matched = value[fmt.Sprint(right)]
		default:
			matched = strings.Contains(fmt.Sprint(left), fmt.Sprint(right))
		}
		if op == "not_contains" {
			return !matched, nil
		}
		return matched, nil
	case "starts_with":
		return strings.HasPrefix(fmt.Sprint(left), fmt.Sprint(right)), nil
	case "ends_with":
		return strings.HasSuffix(fmt.Sprint(left), fmt.Sprint(right)), nil
	case "in", "not_in":
		array, ok := asAnySlice(right)
		if !ok {
			return false, fmt.Errorf("%s requires array right operand", op)
		}
		matched := false
		for _, item := range array {
			if reflect.DeepEqual(normalizeComparableBuiltin(left), normalizeComparableBuiltin(item)) {
				matched = true
				break
			}
		}
		if op == "not_in" {
			return !matched, nil
		}
		return matched, nil
	case "matches", "regex":
		pattern := fmt.Sprint(right)
		if len(pattern) > 2048 {
			return false, fmt.Errorf("regex pattern is too long")
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false, err
		}
		return re.MatchString(fmt.Sprint(left)), nil
	default:
		return false, fmt.Errorf("unsupported operator %q", op)
	}
}

func builtinSource(object map[string]any) any {
	if value, exists := object["value"]; exists {
		return value
	}
	if value, exists := object["source"]; exists {
		return value
	}
	return nil
}

func extractBuiltinPath(source any, path string, maxItems int) (any, bool, error) {
	path = strings.TrimSpace(path)
	if path == "" || path == "$" {
		return source, true, nil
	}
	segments, err := parseBuiltinPath(path)
	if err != nil {
		return nil, false, err
	}
	values := []any{source}
	wildcard := false
	for _, segment := range segments {
		next := make([]any, 0)
		for _, current := range values {
			switch segment.kind {
			case builtinPathKey:
				object, ok := asAnyMap(current)
				if !ok {
					continue
				}
				if value, exists := object[segment.key]; exists {
					next = append(next, value)
				}
			case builtinPathIndex:
				array, ok := asAnySlice(current)
				if !ok || segment.index < 0 || segment.index >= len(array) {
					continue
				}
				next = append(next, array[segment.index])
			case builtinPathWildcard:
				wildcard = true
				if array, ok := asAnySlice(current); ok {
					next = append(next, array...)
				} else if object, ok := asAnyMap(current); ok {
					keys := make([]string, 0, len(object))
					for key := range object {
						keys = append(keys, key)
					}
					sort.Strings(keys)
					for _, key := range keys {
						next = append(next, object[key])
					}
				}
			}
			if len(next) > maxItems {
				return nil, false, fmt.Errorf("path expansion exceeds %d items", maxItems)
			}
		}
		values = next
		if len(values) == 0 {
			return nil, false, nil
		}
	}
	if wildcard || len(values) > 1 {
		return values, true, nil
	}
	return values[0], true, nil
}

type builtinPathSegmentKind int

const (
	builtinPathKey builtinPathSegmentKind = iota
	builtinPathIndex
	builtinPathWildcard
)

type builtinPathSegment struct {
	kind  builtinPathSegmentKind
	key   string
	index int
}

func parseBuiltinPath(path string) ([]builtinPathSegment, error) {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "$")
	path = strings.TrimPrefix(path, ".")
	segments := make([]builtinPathSegment, 0, 8)
	var token strings.Builder
	flush := func() {
		if token.Len() == 0 {
			return
		}
		segments = append(segments, builtinPathSegment{kind: builtinPathKey, key: token.String()})
		token.Reset()
	}
	for i := 0; i < len(path); {
		switch path[i] {
		case '.':
			flush()
			i++
		case '[':
			flush()
			end := strings.IndexByte(path[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("invalid path %q: missing ]", path)
			}
			end += i
			content := strings.TrimSpace(path[i+1 : end])
			content = strings.Trim(content, "\"'")
			if content == "*" {
				segments = append(segments, builtinPathSegment{kind: builtinPathWildcard})
			} else if index, err := strconv.Atoi(content); err == nil {
				segments = append(segments, builtinPathSegment{kind: builtinPathIndex, index: index})
			} else if content != "" {
				segments = append(segments, builtinPathSegment{kind: builtinPathKey, key: content})
			} else {
				return nil, fmt.Errorf("invalid empty path segment")
			}
			i = end + 1
		default:
			token.WriteByte(path[i])
			i++
		}
	}
	flush()
	return segments, nil
}

func builtinPathResultKey(path string) string {
	segments, err := parseBuiltinPath(path)
	if err == nil {
		for i := len(segments) - 1; i >= 0; i-- {
			if segments[i].kind == builtinPathKey && segments[i].key != "" {
				return segments[i].key
			}
		}
	}
	return strings.Trim(path, " $.")
}

func stripBuiltinControlFields(source map[string]any, fields ...string) map[string]any {
	result := cloneAnyMap(source)
	for _, field := range fields {
		delete(result, field)
	}
	return result
}

func cloneAnyMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func asAnyMap(value any) (map[string]any, bool) {
	object, ok := value.(map[string]any)
	return object, ok
}

func asAnySlice(value any) ([]any, bool) {
	array, ok := value.([]any)
	return array, ok
}

func stringSliceAny(value any) []string {
	switch typed := value.(type) {
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				result = append(result, text)
			}
		}
		return result
	case []string:
		return append([]string(nil), typed...)
	case string:
		parts := strings.Split(typed, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			if text := strings.TrimSpace(part); text != "" {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

func stringMapAny(value any) map[string]string {
	result := map[string]string{}
	object, ok := asAnyMap(value)
	if !ok {
		return result
	}
	for key, item := range object {
		text := strings.TrimSpace(fmt.Sprint(item))
		if text != "" {
			result[key] = text
		}
	}
	return result
}

func builtinFirstString(values ...any) string {
	for _, value := range values {
		if value == nil {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" {
			return text
		}
	}
	return ""
}

func firstAny(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func normalizeBuiltinOp(op string) string {
	op = strings.ToLower(strings.TrimSpace(op))
	op = strings.ReplaceAll(op, "-", "_")
	op = strings.ReplaceAll(op, " ", "_")
	return op
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, _ := strconv.ParseBool(strings.TrimSpace(typed))
		return parsed
	case float64:
		return typed != 0
	default:
		return false
	}
}

func numberValue(value any, fallback float64) float64 {
	if parsed, ok := floatBuiltin(value); ok {
		return parsed
	}
	return fallback
}

func floatBuiltin(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, !math.IsNaN(typed) && !math.IsInf(typed, 0)
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func truthyBuiltin(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case string:
		text := strings.TrimSpace(strings.ToLower(typed))
		if text == "" || text == "false" || text == "0" || text == "null" || text == "nil" {
			return false
		}
		return true
	case float64:
		return typed != 0
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return !reflect.ValueOf(value).IsZero()
	}
}

func emptyBuiltin(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func isCompositeBuiltin(value any) bool {
	switch value.(type) {
	case map[string]any, []any:
		return true
	default:
		return false
	}
}

func compareOrderBuiltin(left, right any) int {
	if lf, lok := floatBuiltin(left); lok {
		if rf, rok := floatBuiltin(right); rok {
			switch {
			case lf < rf:
				return -1
			case lf > rf:
				return 1
			default:
				return 0
			}
		}
	}
	return strings.Compare(fmt.Sprint(left), fmt.Sprint(right))
}

func normalizeComparableBuiltin(value any) any {
	if number, ok := floatBuiltin(value); ok {
		return number
	}
	return value
}

func stableBuiltinKey(value any) string {
	if raw, err := json.Marshal(value); err == nil {
		return string(raw)
	}
	return fmt.Sprintf("%T:%v", value, value)
}

func normalizeJSONNumbers(value any) any {
	switch typed := value.(type) {
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return integer
		}
		if number, err := typed.Float64(); err == nil {
			return number
		}
		return typed.String()
	case []any:
		for i := range typed {
			typed[i] = normalizeJSONNumbers(typed[i])
		}
		return typed
	case map[string]any:
		for key, item := range typed {
			typed[key] = normalizeJSONNumbers(item)
		}
		return typed
	default:
		return value
	}
}

func decodeJSONAny(raw json.RawMessage, target *any) error {
	if len(raw) == 0 {
		*target = map[string]any{}
		return nil
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	*target = normalizeJSONNumbers(*target)
	return nil
}

func marshalBuiltinOutput(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

type workflowPauseSignalKey struct{}

type WorkflowPauseError struct {
	Remaining time.Duration
}

func (e *WorkflowPauseError) Error() string { return "workflow paused" }

func withWorkflowPauseSignal(ctx context.Context, signal <-chan struct{}) context.Context {
	if signal == nil {
		return ctx
	}
	return context.WithValue(ctx, workflowPauseSignalKey{}, signal)
}

func workflowPauseSignalFromContext(ctx context.Context) <-chan struct{} {
	if ctx == nil {
		return nil
	}
	signal, _ := ctx.Value(workflowPauseSignalKey{}).(<-chan struct{})
	return signal
}

type waitPauseProgress struct {
	RemainingMS int64 `json:"remainingMs"`
}

type WaitHandler struct{}

func (WaitHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	durationMS := int64(0)
	if raw, ok := node.Runtime.Metadata["durationMs"].(float64); ok {
		durationMS = int64(raw)
	}
	if durationMS == 0 {
		var payload struct {
			DurationMS int64 `json:"durationMs"`
		}
		_ = json.Unmarshal(input, &payload)
		durationMS = payload.DurationMS
	}
	if durationMS < 0 {
		return nil, fmt.Errorf("wait duration must not be negative")
	}
	duration := time.Duration(durationMS) * time.Millisecond
	deadline := time.Now().Add(duration)
	timer := time.NewTimer(duration)
	defer timer.Stop()
	pauseSignal := workflowPauseSignalFromContext(ctx)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-pauseSignal:
		remaining := time.Until(deadline)
		if remaining < 0 {
			remaining = 0
		}
		progress, _ := json.Marshal(waitPauseProgress{RemainingMS: remaining.Milliseconds()})
		return progress, &WorkflowPauseError{Remaining: remaining}
	case <-timer.C:
		return input, nil
	}
}

type NestedWorkflowHandler struct {
	Executor *WorkflowExecutor
}

func (h NestedWorkflowHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	if h.Executor == nil {
		return nil, fmt.Errorf("nested workflow executor not configured")
	}
	execution, ok := ExecutionContextFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("nested workflow context missing")
	}
	targetID := strings.TrimSpace(node.TargetID)
	if targetID == "" {
		targetID = strings.TrimSpace(node.Runtime.RuntimeID)
	}
	if targetID == "" {
		return nil, fmt.Errorf("nested workflow target missing")
	}

	target := node.ExecutionTarget.Normalized(WorkflowExecutionLocal)
	if node.ExecutionTarget.Placement == WorkflowExecutionDevice {
		runner := h.Executor.RemoteWorkflowRunner()
		if runner == nil {
			err := fmt.Errorf("remote workflow runner not configured")
			if target.OfflinePolicy == WorkflowOfflineWait {
				return nil, &WorkflowDeviceUnavailableError{DeviceID: target.DeviceID, Cause: err}
			}
			return nil, err
		}
		// Execute has already appended the currently running workflow frame to
		// CallStack. Forward that complete distributed stack unchanged so the
		// target executor can append its own frame and reject Cloud -> Device ->
		// Cloud (or cross-device) recursion consistently.
		execution.Depth++
		execution.InvocationID = fmt.Sprintf("%s/%s", execution.InvocationID, node.ID)
		execution.IdempotencyKey = fmt.Sprintf("%s/%s", execution.IdempotencyKey, node.ID)
		output, err := runner.RunRemoteWorkflow(ctx, RemoteWorkflowRequest{
			WorkflowID: targetID,
			Input:      input,
			Target:     target,
			Context:    execution,
		})
		if err != nil && target.OfflinePolicy == WorkflowOfflineWait {
			var unavailable *WorkflowDeviceUnavailableError
			if errors.As(err, &unavailable) {
				return nil, unavailable
			}
		}
		return output, err
	}
	if node.ExecutionTarget.Placement == WorkflowExecutionAuto {
		return nil, fmt.Errorf("nested workflow auto placement requires an explicit device workflow selection")
	}

	definition, exists := h.Executor.registry.Get(targetID)
	if !exists {
		return nil, ErrWorkflowNotFound
	}
	if definition.Source == "user" {
		owner := ""
		if definition.Metadata != nil {
			if value, exists := definition.Metadata["ownerUserId"]; exists && value != nil {
				owner = strings.TrimSpace(fmt.Sprint(value))
			}
		}
		if execution.UserID == "" || owner == "" || owner != execution.UserID {
			return nil, fmt.Errorf("%w: nested user workflow owner mismatch", ErrScopeDenied)
		}
	}
	execution.Depth++
	execution.InvocationID = fmt.Sprintf("%s/%s", execution.InvocationID, node.ID)
	execution.IdempotencyKey = fmt.Sprintf("%s/%s", execution.IdempotencyKey, node.ID)
	result, err := h.Executor.Execute(ctx, ExecuteRequest{WorkflowID: targetID, Input: input, Context: execution})
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, fmt.Errorf("nested workflow failed: %s", result.Error)
	}
	return result.Output, nil
}

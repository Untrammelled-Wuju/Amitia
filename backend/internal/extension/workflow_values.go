package extension

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var templatePattern = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_.-]*)(?:\s*\|\s*(json|string|number|date|default|truncate)(?::([^}]*))?)?\s*\}\}`)

func validateCondition(expression *ConditionExpression, depth, maximum int) error {
	if expression == nil {
		return nil
	}
	if depth >= maximum {
		return fmt.Errorf("条件表达式深度超过限制")
	}
	allowed := map[string]bool{"eq": true, "neq": true, "gt": true, "gte": true, "lt": true, "lte": true, "and": true, "or": true, "not": true, "exists": true, "empty": true, "contains": true, "starts_with": true, "ends_with": true, "in": true}
	if !allowed[expression.Op] {
		return fmt.Errorf("未知条件操作符: %s", expression.Op)
	}
	if len(expression.Args) > 32 {
		return fmt.Errorf("条件参数超过限制")
	}
	for index := range expression.Args {
		if err := validateCondition(&expression.Args[index], depth+1, maximum); err != nil {
			return err
		}
	}
	return nil
}

func evalCondition(expression *ConditionExpression, context map[string]interface{}, maximum int) (bool, error) {
	return evalConditionDepth(expression, context, 0, maximum)
}

func evalConditionDepth(expression *ConditionExpression, context map[string]interface{}, depth, maximum int) (bool, error) {
	if err := validateCondition(expression, depth, maximum); err != nil {
		return false, err
	}
	resolve := func(value interface{}) (interface{}, error) { return resolveValue(value, context) }
	switch expression.Op {
	case "and":
		for index := range expression.Args {
			ok, err := evalConditionDepth(&expression.Args[index], context, depth+1, maximum)
			if err != nil || !ok {
				return ok, err
			}
		}
		return true, nil
	case "or":
		for index := range expression.Args {
			ok, err := evalConditionDepth(&expression.Args[index], context, depth+1, maximum)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	case "not":
		if len(expression.Args) != 1 {
			return false, fmt.Errorf("not 需要一个参数")
		}
		ok, err := evalConditionDepth(&expression.Args[0], context, depth+1, maximum)
		return !ok, err
	case "exists", "empty":
		value, err := resolve(expression.Value)
		if err != nil {
			if expression.Op == "exists" {
				return false, nil
			}
			return true, nil
		}
		empty := value == nil || reflect.ValueOf(value).Kind() == reflect.String && value == ""
		if expression.Op == "exists" {
			return value != nil, nil
		}
		return empty, nil
	default:
		left, err := resolve(expression.Left)
		if err != nil {
			return false, err
		}
		right, err := resolve(expression.Right)
		if err != nil {
			return false, err
		}
		switch expression.Op {
		case "eq":
			return reflect.DeepEqual(left, right), nil
		case "neq":
			return !reflect.DeepEqual(left, right), nil
		case "gt", "gte", "lt", "lte":
			lf, lok := asFloat(left)
			rf, rok := asFloat(right)
			if !lok || !rok {
				return false, fmt.Errorf("比较操作需要数字")
			}
			if expression.Op == "gt" {
				return lf > rf, nil
			}
			if expression.Op == "gte" {
				return lf >= rf, nil
			}
			if expression.Op == "lt" {
				return lf < rf, nil
			}
			return lf <= rf, nil
		case "contains":
			return strings.Contains(fmt.Sprint(left), fmt.Sprint(right)), nil
		case "starts_with":
			return strings.HasPrefix(fmt.Sprint(left), fmt.Sprint(right)), nil
		case "ends_with":
			return strings.HasSuffix(fmt.Sprint(left), fmt.Sprint(right)), nil
		case "in":
			array, ok := right.([]interface{})
			if !ok {
				return false, fmt.Errorf("in 右侧需要数组")
			}
			for _, item := range array {
				if reflect.DeepEqual(left, item) {
					return true, nil
				}
			}
			return false, nil
		}
	}
	return false, fmt.Errorf("无法执行条件")
}

func resolveValue(value interface{}, context map[string]interface{}) (interface{}, error) {
	object, ok := value.(map[string]interface{})
	if ok {
		if ref, refOK := object["ref"].(string); refOK {
			return resolveReference(ref, context)
		}
	}
	return value, nil
}

func resolveReference(reference string, context map[string]interface{}) (interface{}, error) {
	parts := strings.Split(reference, ".")
	if len(parts) < 2 || containsForbiddenPath(parts) {
		return nil, fmt.Errorf("引用非法: %s", reference)
	}
	var current interface{} = context
	for _, part := range parts {
		object, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("引用路径不是对象: %s", reference)
		}
		current, ok = object[part]
		if !ok {
			return nil, fmt.Errorf("引用不存在: %s", reference)
		}
	}
	return current, nil
}

func resolveJSON(raw json.RawMessage, context map[string]interface{}, maxTemplateLength int, allowSecrets bool) (json.RawMessage, error) {
	var value interface{}
	if err := json.Unmarshal(normalizeJSON(raw), &value); err != nil {
		return nil, err
	}
	resolved, err := resolveJSONValue(value, context, maxTemplateLength, allowSecrets)
	if err != nil {
		return nil, err
	}
	return json.Marshal(resolved)
}

func resolveJSONValue(value interface{}, context map[string]interface{}, maxTemplateLength int, allowSecrets bool) (interface{}, error) {
	switch typed := value.(type) {
	case map[string]interface{}:
		if len(typed) == 1 {
			if name, ok := typed["$secret"].(string); ok {
				if !allowSecrets {
					return nil, fmt.Errorf("此处禁止 Secret 引用")
				}
				name = strings.TrimSpace(name)
				if name == "" || strings.Contains(name, ".") || containsForbiddenPath([]string{name}) {
					return nil, fmt.Errorf("Secret 引用非法")
				}
				return resolveReference("secrets."+name, context)
			}
			if ref, ok := typed["$ref"].(string); ok {
				if strings.HasPrefix(ref, "secrets.") && !allowSecrets {
					return nil, fmt.Errorf("此处禁止 Secret 引用")
				}
				return resolveReference(ref, context)
			}
		}
		result := map[string]interface{}{}
		for key, item := range typed {
			resolved, err := resolveJSONValue(item, context, maxTemplateLength, allowSecrets)
			if err != nil {
				return nil, err
			}
			result[key] = resolved
		}
		return result, nil
	case []interface{}:
		result := make([]interface{}, len(typed))
		for index, item := range typed {
			resolved, err := resolveJSONValue(item, context, maxTemplateLength, allowSecrets)
			if err != nil {
				return nil, err
			}
			result[index] = resolved
		}
		return result, nil
	case string:
		if strings.Contains(typed, "{{") {
			return renderTemplate(typed, context, maxTemplateLength, allowSecrets)
		}
		return typed, nil
	default:
		return typed, nil
	}
}

func renderTemplate(template string, context map[string]interface{}, maximum int, allowSecrets bool) (string, error) {
	if len(template) > maximum {
		return "", fmt.Errorf("模板长度超过限制")
	}
	if strings.Contains(template, "{{{") || strings.Contains(template, "(") || strings.Contains(template, "[") {
		return "", fmt.Errorf("模板包含禁止表达式")
	}
	indexes := templatePattern.FindAllStringSubmatchIndex(template, -1)
	if strings.Contains(template, "{{") && len(indexes) == 0 {
		return "", fmt.Errorf("模板语法非法")
	}
	var builder strings.Builder
	previous := 0
	for _, index := range indexes {
		builder.WriteString(template[previous:index[0]])
		ref := template[index[2]:index[3]]
		if strings.HasPrefix(ref, "secrets.") && !allowSecrets {
			return "", fmt.Errorf("禁止在可见模板中回显 Secret")
		}
		value, err := resolveReference(ref, context)
		if err != nil {
			return "", err
		}
		formatter := "string"
		argument := ""
		if index[4] >= 0 {
			formatter = template[index[4]:index[5]]
		}
		if index[6] >= 0 {
			argument = strings.TrimSpace(template[index[6]:index[7]])
		}
		rendered, err := formatTemplateValue(value, formatter, argument)
		if err != nil {
			return "", err
		}
		builder.WriteString(rendered)
		previous = index[1]
	}
	builder.WriteString(template[previous:])
	if builder.Len() > maximum {
		return "", fmt.Errorf("模板输出超过限制")
	}
	return builder.String(), nil
}

func formatTemplateValue(value interface{}, formatter, argument string) (string, error) {
	switch formatter {
	case "json":
		raw, err := json.Marshal(value)
		return string(raw), err
	case "string":
		return fmt.Sprint(value), nil
	case "number":
		number, ok := asFloat(value)
		if !ok {
			return "", fmt.Errorf("值不能格式化为数字")
		}
		return strconv.FormatFloat(number, 'f', -1, 64), nil
	case "date":
		text := fmt.Sprint(value)
		parsed, err := time.Parse(time.RFC3339, text)
		if err != nil {
			return "", fmt.Errorf("值不能格式化为日期")
		}
		layout := argument
		if layout == "" {
			layout = time.RFC3339
		}
		return parsed.Format(layout), nil
	case "default":
		if value == nil || fmt.Sprint(value) == "" {
			return strings.Trim(argument, "\"'"), nil
		}
		return fmt.Sprint(value), nil
	case "truncate":
		length, err := strconv.Atoi(argument)
		if err != nil || length < 0 {
			return "", fmt.Errorf("truncate 参数非法")
		}
		runes := []rune(fmt.Sprint(value))
		if len(runes) > length {
			runes = runes[:length]
		}
		return string(runes), nil
	default:
		return "", fmt.Errorf("未知模板格式化器")
	}
}

func asFloat(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func transformJSON(input map[string]interface{}, context map[string]interface{}, maximum int) (interface{}, error) {
	op, _ := input["op"].(string)
	source := input["value"]
	if source == nil {
		source = input["source"]
	}
	source, err := resolveValue(source, context)
	if err != nil {
		return nil, err
	}
	object, _ := source.(map[string]interface{})
	fields := stringSlice(input["fields"])
	switch op {
	case "pick":
		result := map[string]interface{}{}
		for _, field := range fields {
			if value, ok := object[field]; ok {
				result[field] = value
			}
		}
		return result, nil
	case "omit":
		result := cloneMap(object)
		for _, field := range fields {
			delete(result, field)
		}
		return result, nil
	case "rename":
		result := cloneMap(object)
		mapping, _ := input["mapping"].(map[string]interface{})
		for from, target := range mapping {
			to := fmt.Sprint(target)
			if value, ok := result[from]; ok {
				result[to] = value
				delete(result, from)
			}
		}
		return result, nil
	case "set":
		result := cloneMap(object)
		values, _ := input["values"].(map[string]interface{})
		for key, value := range values {
			resolved, err := resolveValue(value, context)
			if err != nil {
				return nil, err
			}
			result[key] = resolved
		}
		return result, nil
	case "merge":
		result := cloneMap(object)
		other, _ := input["with"].(map[string]interface{})
		for key, value := range other {
			result[key] = value
		}
		return result, nil
	case "flatten":
		nested, ok := source.([]interface{})
		if !ok {
			return nil, fmt.Errorf("flatten 需要数组")
		}
		result := []interface{}{}
		for _, item := range nested {
			if array, ok := item.([]interface{}); ok {
				result = append(result, array...)
			} else {
				result = append(result, item)
			}
			if len(result) > maximum {
				return nil, fmt.Errorf("数组结果超过限制")
			}
		}
		return result, nil
	case "array_map":
		array, ok := source.([]interface{})
		if !ok {
			return nil, fmt.Errorf("array_map 需要数组")
		}
		if len(array) > maximum {
			return nil, fmt.Errorf("数组超过限制")
		}
		field := strings.TrimSpace(fmt.Sprint(input["field"]))
		fields := stringSlice(input["fields"])
		if field == "" && len(fields) == 0 {
			return nil, fmt.Errorf("array_map 必须指定 field 或 fields")
		}
		result := make([]interface{}, 0, len(array))
		for _, item := range array {
			object, ok := item.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("array_map 元素必须为对象")
			}
			if field != "" {
				result = append(result, object[field])
				continue
			}
			mapped := map[string]interface{}{}
			for _, name := range fields {
				if value, exists := object[name]; exists {
					mapped[name] = value
				}
			}
			result = append(result, mapped)
		}
		return result, nil
	case "array_filter":
		array, ok := source.([]interface{})
		if !ok {
			return nil, fmt.Errorf("array_filter 需要数组")
		}
		if len(array) > maximum {
			return nil, fmt.Errorf("数组超过限制")
		}
		field := strings.TrimSpace(fmt.Sprint(input["field"]))
		operator := strings.TrimSpace(fmt.Sprint(input["operator"]))
		if field == "" || !map[string]bool{"eq": true, "neq": true, "gt": true, "gte": true, "lt": true, "lte": true, "contains": true, "starts_with": true, "ends_with": true}[operator] {
			return nil, fmt.Errorf("array_filter 的 field 或 operator 非法")
		}
		expected := input["expected"]
		result := make([]interface{}, 0, len(array))
		for _, item := range array {
			object, ok := item.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("array_filter 元素必须为对象")
			}
			matched, err := compareTransformValue(object[field], expected, operator)
			if err != nil {
				return nil, err
			}
			if matched {
				result = append(result, item)
			}
		}
		return result, nil
	case "array_take":
		array, ok := source.([]interface{})
		if !ok {
			return nil, fmt.Errorf("array_take 需要数组")
		}
		count, _ := asFloat(input["count"])
		if int(count) > len(array) {
			count = float64(len(array))
		}
		if count < 0 {
			count = 0
		}
		return append([]interface{}(nil), array[:int(count)]...), nil
	case "array_sort":
		array, ok := source.([]interface{})
		if !ok {
			return nil, fmt.Errorf("array_sort 需要数组")
		}
		if len(array) > maximum {
			return nil, fmt.Errorf("数组超过限制")
		}
		result := append([]interface{}(nil), array...)
		field := fmt.Sprint(input["field"])
		if field == "" {
			return nil, fmt.Errorf("array_sort 必须指定字段")
		}
		sort.SliceStable(result, func(i, j int) bool {
			left, _ := result[i].(map[string]interface{})
			right, _ := result[j].(map[string]interface{})
			return fmt.Sprint(left[field]) < fmt.Sprint(right[field])
		})
		return result, nil
	case "to_string":
		return fmt.Sprint(source), nil
	case "to_number":
		number, ok := asFloat(source)
		if !ok {
			return nil, fmt.Errorf("无法转换为数字")
		}
		return number, nil
	case "to_boolean":
		switch typed := source.(type) {
		case bool:
			return typed, nil
		case string:
			value, err := strconv.ParseBool(typed)
			return value, err
		default:
			return source != nil, nil
		}
	default:
		return nil, fmt.Errorf("未知变换操作: %s", op)
	}
}

func compareTransformValue(left, right interface{}, operator string) (bool, error) {
	switch operator {
	case "eq":
		return reflect.DeepEqual(left, right), nil
	case "neq":
		return !reflect.DeepEqual(left, right), nil
	case "contains":
		return strings.Contains(fmt.Sprint(left), fmt.Sprint(right)), nil
	case "starts_with":
		return strings.HasPrefix(fmt.Sprint(left), fmt.Sprint(right)), nil
	case "ends_with":
		return strings.HasSuffix(fmt.Sprint(left), fmt.Sprint(right)), nil
	default:
		leftNumber, leftOK := asFloat(left)
		rightNumber, rightOK := asFloat(right)
		if !leftOK || !rightOK {
			return false, fmt.Errorf("array_filter 数值比较需要数字")
		}
		switch operator {
		case "gt":
			return leftNumber > rightNumber, nil
		case "gte":
			return leftNumber >= rightNumber, nil
		case "lt":
			return leftNumber < rightNumber, nil
		case "lte":
			return leftNumber <= rightNumber, nil
		}
	}
	return false, fmt.Errorf("array_filter operator 非法")
}

func stringSlice(value interface{}) []string {
	array, _ := value.([]interface{})
	result := make([]string, 0, len(array))
	for _, item := range array {
		result = append(result, fmt.Sprint(item))
	}
	return result
}
func cloneMap(source map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	for key, value := range source {
		result[key] = value
	}
	return result
}

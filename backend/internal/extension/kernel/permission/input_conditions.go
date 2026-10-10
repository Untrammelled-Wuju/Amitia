package permission

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
)

func ValidateInputConditions(encoded, input json.RawMessage) error {
	if len(encoded) == 0 {
		return nil
	}
	if len(encoded) > 4096 || len(input) > 1<<20 || !json.Valid(encoded) || !json.Valid(input) {
		return fmt.Errorf("权限条件或实际输入超过上限")
	}
	var conditions []PermissionCondition
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&conditions); err != nil || conditions == nil || len(conditions) > 64 {
		return fmt.Errorf("当前权限条件格式尚不支持，不能忽略后执行")
	}
	var document any
	decoder = json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("权限条件要求有效实际输入")
	}
	for _, condition := range conditions {
		if condition.Field == "" || len(condition.Field) > 256 || strings.TrimSpace(condition.Field) != condition.Field {
			return fmt.Errorf("权限条件字段无效")
		}
		value, exists := document, true
		for _, field := range strings.Split(condition.Field, ".") {
			if field == "" {
				return fmt.Errorf("权限条件字段无效")
			}
			object, ok := value.(map[string]any)
			if !ok {
				exists = false
				break
			}
			value, exists = object[field]
			if !exists {
				break
			}
		}
		var expected any
		decoder = json.NewDecoder(bytes.NewReader(condition.Value))
		decoder.UseNumber()
		if err := decoder.Decode(&expected); err != nil {
			return fmt.Errorf("权限条件比较值无效")
		}
		matched := false
		switch condition.Operator {
		case "eq":
			matched = exists && equalConditionValue(value, expected)
		case "ne":
			matched = exists && !equalConditionValue(value, expected)
		case "exists":
			flag, ok := expected.(bool)
			matched = ok && exists == flag
		case "in":
			options, ok := expected.([]any)
			if ok && len(options) <= 64 && exists {
				for _, option := range options {
					matched = matched || equalConditionValue(value, option)
				}
			}
		default:
			return fmt.Errorf("当前权限条件操作符尚不支持：%s", condition.Operator)
		}
		if !matched {
			return fmt.Errorf("实际输入未满足字段 %s 的权限条件", condition.Field)
		}
	}
	return nil
}

func equalConditionValue(left, right any) bool {
	if number, ok := left.(json.Number); ok {
		other, ok := right.(json.Number)
		if !ok {
			return false
		}
		if !boundedConditionNumber(number.String()) || !boundedConditionNumber(other.String()) {
			return false
		}
		a, aok := new(big.Rat).SetString(number.String())
		b, bok := new(big.Rat).SetString(other.String())
		return aok && bok && a.Cmp(b) == 0
	}
	return reflect.DeepEqual(left, right)
}

func boundedConditionNumber(value string) bool {
	if len(value) > 4096 {
		return false
	}
	if position := strings.IndexAny(value, "eE"); position >= 0 {
		exponent, err := strconv.Atoi(value[position+1:])
		return err == nil && exponent >= -4096 && exponent <= 4096
	}
	return true
}

package canary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
)

type DualWriteManager struct{}

func NewDualWriteManager() *DualWriteManager {
	return &DualWriteManager{}
}

func (m *DualWriteManager) ValidateDualWrite(ctx context.Context, policy DualWritePolicy) error {
	if policy.ExternalSideEffect {
		return fmt.Errorf("canary: dual write not allowed for external side effects")
	}
	if !policy.RequiredIdempotent {
		return fmt.Errorf("canary: dual write requires idempotent operations")
	}
	return nil
}

type ConsistencyResult struct {
	Consistent   bool     `json:"consistent"`
	DifferFields []string `json:"differ_fields,omitempty"`
}

func (m *DualWriteManager) CheckConsistency(ctx context.Context, oldResult, newResult json.RawMessage) (ConsistencyResult, error) {
	result := ConsistencyResult{Consistent: true}

	var oldVal, newVal interface{}
	oldErr := json.Unmarshal(oldResult, &oldVal)
	newErr := json.Unmarshal(newResult, &newVal)

	if oldErr != nil || newErr != nil {
		if bytes.Equal(oldResult, newResult) {
			return result, nil
		}
		result.Consistent = false
		result.DifferFields = []string{"<raw_bytes>"}
		return result, fmt.Errorf("canary: dual write result inconsistency detected (byte fallback)")
	}

	if reflect.DeepEqual(oldVal, newVal) {
		return result, nil
	}

	differFields := collectDifferFields("", oldVal, newVal)
	result.Consistent = false
	result.DifferFields = differFields
	return result, fmt.Errorf("canary: dual write result inconsistency detected in fields: %v", differFields)
}

func collectDifferFields(prefix string, oldVal, newVal interface{}) []string {
	var diffs []string

	switch oldTyped := oldVal.(type) {
	case map[string]interface{}:
		newTyped, ok := newVal.(map[string]interface{})
		if !ok {
			diffs = append(diffs, joinField(prefix, "<type_mismatch>"))
			return diffs
		}
		for k, ov := range oldTyped {
			path := joinField(prefix, k)
			nv, exists := newTyped[k]
			if !exists {
				diffs = append(diffs, path)
				continue
			}
			diffs = append(diffs, collectDifferFields(path, ov, nv)...)
		}
		for k := range newTyped {
			if _, exists := oldTyped[k]; !exists {
				diffs = append(diffs, joinField(prefix, k))
			}
		}
	case []interface{}:
		newTyped, ok := newVal.([]interface{})
		if !ok {
			diffs = append(diffs, joinField(prefix, "<type_mismatch>"))
			return diffs
		}
		maxLen := len(oldTyped)
		if len(newTyped) > maxLen {
			maxLen = len(newTyped)
		}
		for i := 0; i < maxLen; i++ {
			path := fmt.Sprintf("%s[%d]", prefix, i)
			if i >= len(oldTyped) || i >= len(newTyped) {
				diffs = append(diffs, path)
				continue
			}
			diffs = append(diffs, collectDifferFields(path, oldTyped[i], newTyped[i])...)
		}
	default:
		if !reflect.DeepEqual(oldVal, newVal) {
			if prefix == "" {
				diffs = append(diffs, "<root>")
			} else {
				diffs = append(diffs, prefix)
			}
		}
	}

	return diffs
}

func joinField(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

type CompensateResult struct {
	Strategy       string `json:"strategy"`
	Action         string `json:"action"`
	Success        bool   `json:"success"`
	RequiresManual bool   `json:"requires_manual"`
	Message        string `json:"message,omitempty"`
}

func (m *DualWriteManager) Compensate(ctx context.Context, policy DualWritePolicy, side string) (CompensateResult, error) {
	result := CompensateResult{
		Strategy: policy.CompensationStrategy,
	}

	if policy.ExternalSideEffect {
		result.RequiresManual = true
		result.Message = fmt.Sprintf("canary: side %s has external side effect, manual intervention required", side)
		return result, fmt.Errorf("%s", result.Message)
	}

	switch policy.CompensationStrategy {
	case "reverse":
		result.Action = "reverse"
		result.Success = true
		result.Message = fmt.Sprintf("canary: reverse operation recorded for side %s", side)
	case "retry":
		result.Action = "retry"
		result.Success = true
		result.Message = fmt.Sprintf("canary: retry scheduled for side %s", side)
	case "skip":
		result.Action = "skip"
		result.Success = true
		result.Message = fmt.Sprintf("canary: compensation skipped for side %s", side)
	case "manual":
		result.RequiresManual = true
		result.Message = fmt.Sprintf("canary: side %s requires manual handling", side)
		return result, fmt.Errorf("%s", result.Message)
	default:
		result.Action = "unknown"
		result.Message = fmt.Sprintf("canary: unknown compensation strategy %q for side %s", policy.CompensationStrategy, side)
		return result, fmt.Errorf("%s", result.Message)
	}

	return result, nil
}

type DualWriteResult struct {
	OldResult  json.RawMessage `json:"old_result"`
	NewResult  json.RawMessage `json:"new_result"`
	Consistent bool            `json:"consistent"`
	Errors     []string        `json:"errors"`
}

func (m *DualWriteManager) ExecuteDualWrite(ctx context.Context, policy DualWritePolicy, oldWriter func(ctx context.Context) (json.RawMessage, error), newWriter func(ctx context.Context) (json.RawMessage, error)) (DualWriteResult, error) {
	result := DualWriteResult{
		Errors: []string{},
	}

	if policy.ExternalSideEffect {
		return result, fmt.Errorf("canary: dual write rejected for external side effects")
	}

	if policy.RequiredIdempotent {
		if err := m.ValidateDualWrite(ctx, policy); err != nil {
			return result, err
		}
	}

	var (
		oldRes, newRes json.RawMessage
		oldErr, newErr error
		wg             sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		oldRes, oldErr = oldWriter(ctx)
	}()
	go func() {
		defer wg.Done()
		newRes, newErr = newWriter(ctx)
	}()
	wg.Wait()

	if oldErr != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("old writer failed: %v", oldErr))
	}
	if newErr != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("new writer failed: %v", newErr))
	}

	if policy.RecordBothSides {
		result.OldResult = oldRes
		result.NewResult = newRes
	}

	if oldErr != nil || newErr != nil {
		return result, fmt.Errorf("canary: dual write encountered writer errors")
	}

	if policy.ValidateConsistency {
		consResult, err := m.CheckConsistency(ctx, oldRes, newRes)
		result.Consistent = consResult.Consistent
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			if policy.ConflictResolution != "" {
				_, compErr := m.Compensate(ctx, policy, "new")
				if compErr != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("compensation failed: %v", compErr))
				}
			}
		}
	}

	return result, nil
}

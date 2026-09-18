package matrices

import (
	"context"
	"encoding/json"
	"testing"

	conformance "github.com/u-ai/mock-conformance-go"
)

func TestQuota_Suite(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "quota_fill_10",
			Category: conformance.CatQuota,
			Name:     "Quota fill with count 10 returns exact count match",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "queue.fill", conformance.MustMarshal(map[string]any{"count": 10}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					count, ok := resp["count"].(float64)
					if !ok || int(count) != 10 {
						tr.Error = "expected count to be 10"
						return nil
					}
					tr.Passed = true
					return nil
				})
				if err != nil {
					tr.Passed = false
					tr.Error = err.Error()
				}
				return tr, nil
			},
		},
		{
			ID:       "quota_fill_50",
			Category: conformance.CatQuota,
			Name:     "Quota fill with count 50 returns exact count match",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "queue.fill", conformance.MustMarshal(map[string]any{"count": 50}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					count, ok := resp["count"].(float64)
					if !ok || int(count) != 50 {
						tr.Error = "expected count to be 50"
						return nil
					}
					tr.Passed = true
					return nil
				})
				if err != nil {
					tr.Passed = false
					tr.Error = err.Error()
				}
				return tr, nil
			},
		},
		{
			ID:       "quota_fill_100",
			Category: conformance.CatQuota,
			Name:     "Quota fill with count 100 returns exact count match",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "queue.fill", conformance.MustMarshal(map[string]any{"count": 100}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					count, ok := resp["count"].(float64)
					if !ok || int(count) != 100 {
						tr.Error = "expected count to be 100"
						return nil
					}
					tr.Passed = true
					return nil
				})
				if err != nil {
					tr.Passed = false
					tr.Error = err.Error()
				}
				return tr, nil
			},
		},
		{
			ID:       "quota_fill_count_key_present",
			Category: conformance.CatQuota,
			Name:     "Quota fill response contains filled and count keys",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "queue.fill", conformance.MustMarshal(map[string]any{"count": 10}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					if _, ok := resp["filled"]; !ok {
						tr.Error = "missing filled key"
						return nil
					}
					if _, ok := resp["count"]; !ok {
						tr.Error = "missing count key"
						return nil
					}
					tr.Passed = true
					return nil
				})
				if err != nil {
					tr.Passed = false
					tr.Error = err.Error()
				}
				return tr, nil
			},
		},
		{
			ID:       "quota_fill_echo_sequential",
			Category: conformance.CatQuota,
			Name:     "Quota fill returns consistent filled=true across sequential calls",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					for _, expected := range []int{10, 50, 100} {
						env, err := h.CallRPC(ctx, "mock.fault", "queue.fill", conformance.MustMarshal(map[string]any{"count": expected}))
						if err != nil {
							tr.Error = "rpc error: " + err.Error()
							return nil
						}
						var resp map[string]any
						if err := json.Unmarshal(env.Payload, &resp); err != nil {
							tr.Error = "unmarshal: " + err.Error()
							return nil
						}
						filled, ok := resp["filled"].(bool)
						if !ok || !filled {
							tr.Error = "expected filled=true"
							return nil
						}
						count, ok := resp["count"].(float64)
						if !ok || int(count) != expected {
							tr.Error = "count mismatch"
							return nil
						}
					}
					tr.Passed = true
					return nil
				})
				if err != nil {
					tr.Passed = false
					tr.Error = err.Error()
				}
				return tr, nil
			},
		},
		{
			ID:       "quota_fill_filled_always_true",
			Category: conformance.CatQuota,
			Name:     "Quota fill filled field is boolean true for count 10",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "queue.fill", conformance.MustMarshal(map[string]any{"count": 10}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					filled, ok := resp["filled"]
					if !ok {
						tr.Error = "missing filled key"
						return nil
					}
					b, isBool := filled.(bool)
					if !isBool || !b {
						tr.Error = "filled is not bool true"
						return nil
					}
					tr.Passed = true
					return nil
				})
				if err != nil {
					tr.Passed = false
					tr.Error = err.Error()
				}
				return tr, nil
			},
		},
	}
	conformance.RunMatrix(t, conformance.CatQuota, cases)
}

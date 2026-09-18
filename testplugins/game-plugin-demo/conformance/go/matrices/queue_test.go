package matrices

import (
	"context"
	"encoding/json"
	"testing"

	conformance "github.com/u-ai/mock-conformance-go"
)

func TestQueue_Suite(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "queue_fill_small",
			Category: conformance.CatQueue,
			Name:     "Queue fill with count 5 returns filled true and matching count",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "queue.fill", conformance.MustMarshal(map[string]any{"count": 5}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					filled, ok := resp["filled"].(bool)
					if !ok || !filled {
						tr.Error = "expected filled to be true"
						return nil
					}
					count, ok := resp["count"].(float64)
					if !ok || int(count) != 5 {
						tr.Error = "expected count to be 5"
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
			ID:       "queue_fill_medium",
			Category: conformance.CatQueue,
			Name:     "Queue fill with count 25 returns filled true and matching count",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "queue.fill", conformance.MustMarshal(map[string]any{"count": 25}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					filled, ok := resp["filled"].(bool)
					if !ok || !filled {
						tr.Error = "expected filled to be true"
						return nil
					}
					count, ok := resp["count"].(float64)
					if !ok || int(count) != 25 {
						tr.Error = "expected count to be 25"
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
			ID:       "queue_fill_default",
			Category: conformance.CatQueue,
			Name:     "Queue fill with zero count defaults to count 1",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "queue.fill", conformance.MustMarshal(map[string]any{}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					filled, ok := resp["filled"].(bool)
					if !ok || !filled {
						tr.Error = "expected filled to be true"
						return nil
					}
					count, ok := resp["count"].(float64)
					if !ok || int(count) != 1 {
						tr.Error = "expected count to default to 1"
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
			ID:       "queue_consume_slow_configured",
			Category: conformance.CatQueue,
			Name:     "Queue consume slow configures count and slowMs correctly",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "queue.consume_slow", conformance.MustMarshal(map[string]any{"count": 8, "slowMs": 200}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					configured, ok := resp["configured"].(bool)
					if !ok || !configured {
						tr.Error = "expected configured to be true"
						return nil
					}
					count, ok := resp["count"].(float64)
					if !ok || int(count) != 8 {
						tr.Error = "expected count to be 8"
						return nil
					}
					slowMs, ok := resp["slowMs"].(float64)
					if !ok || int(slowMs) != 200 {
						tr.Error = "expected slowMs to be 200"
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
			ID:       "queue_consume_slow_zero",
			Category: conformance.CatQueue,
			Name:     "Queue consume slow with zero values configures correctly",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "queue.consume_slow", conformance.MustMarshal(map[string]any{"count": 0, "slowMs": 0}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					configured, ok := resp["configured"].(bool)
					if !ok || !configured {
						tr.Error = "expected configured to be true"
						return nil
					}
					count, ok := resp["count"].(float64)
					if !ok || int(count) != 0 {
						tr.Error = "expected count to be 0"
						return nil
					}
					slowMs, ok := resp["slowMs"].(float64)
					if !ok || int(slowMs) != 0 {
						tr.Error = "expected slowMs to be 0"
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
			ID:       "queue_fill_large",
			Category: conformance.CatQueue,
			Name:     "Queue fill with count 100 returns filled true and matching count",
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
					filled, ok := resp["filled"].(bool)
					if !ok || !filled {
						tr.Error = "expected filled to be true"
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
	}
	conformance.RunMatrix(t, conformance.CatQueue, cases)
}

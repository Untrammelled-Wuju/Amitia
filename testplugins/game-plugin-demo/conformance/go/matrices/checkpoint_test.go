package matrices

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	conformance "github.com/u-ai/mock-conformance-go"
)

func TestCheckpoint_Suite(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "checkpoint_01",
			Category: conformance.CatCheckpoint,
			Name:     "set state and read back returns exact value match",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatCheckpoint)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					setResp, err := h.CallRPC(ctx, "mock.state", "mock.state.set", conformance.MustMarshal(map[string]any{
						"stateId": "checkpoint-key-1",
						"value":   conformance.MustMarshal("checkpoint-value-1"),
					}))
					if err != nil {
						return fmt.Errorf("state set: %w", err)
					}
					var setResult map[string]any
					if err := json.Unmarshal(setResp.Payload, &setResult); err != nil {
						return fmt.Errorf("unmarshal set: %w", err)
					}
					if setResult["acked"] != true {
						return fmt.Errorf("state set not acked")
					}

					getResp, err := h.CallRPC(ctx, "mock.state", "mock.state.get", conformance.MustMarshal(map[string]any{
						"stateId": "checkpoint-key-1",
					}))
					if err != nil {
						return fmt.Errorf("state get: %w", err)
					}
					var getResult map[string]any
					if err := json.Unmarshal(getResp.Payload, &getResult); err != nil {
						return fmt.Errorf("unmarshal get: %w", err)
					}
					if getResult["found"] != true {
						return fmt.Errorf("state not found after set: %v", getResult)
					}

					return nil
				})

				if err != nil {
					return tr, err
				}

				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "checkpoint_02",
			Category: conformance.CatCheckpoint,
			Name:     "set multiple keys and read all back preserves values",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatCheckpoint)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					keys := []string{"multi-key-a", "multi-key-b", "multi-key-c"}
					values := []string{"alpha", "beta", "gamma"}

					for i, key := range keys {
						setResp, err := h.CallRPC(ctx, "mock.state", "mock.state.set", conformance.MustMarshal(map[string]any{
							"stateId": key,
							"value":   conformance.MustMarshal(values[i]),
						}))
						if err != nil {
							return fmt.Errorf("set %s: %w", key, err)
						}
						var setResult map[string]any
						if err := json.Unmarshal(setResp.Payload, &setResult); err != nil {
							return fmt.Errorf("unmarshal set %s: %w", key, err)
						}
						if setResult["acked"] != true {
							return fmt.Errorf("set %s not acked", key)
						}
					}

					for i, key := range keys {
						getResp, err := h.CallRPC(ctx, "mock.state", "mock.state.get", conformance.MustMarshal(map[string]any{
							"stateId": key,
						}))
						if err != nil {
							return fmt.Errorf("get %s: %w", key, err)
						}
						var getResult map[string]any
						if err := json.Unmarshal(getResp.Payload, &getResult); err != nil {
							return fmt.Errorf("unmarshal get %s: %w", key, err)
						}
						if getResult["found"] != true {
							return fmt.Errorf("key %s not found", key)
						}
						if val, ok := getResult["value"].(string); !ok || val != values[i] {
							return fmt.Errorf("key %s value mismatch: got %v, want %s", key, getResult["value"], values[i])
						}
					}

					return nil
				})

				if err != nil {
					return tr, err
				}

				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "checkpoint_03",
			Category: conformance.CatCheckpoint,
			Name:     "state consistency across sequential set and get operations",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatCheckpoint)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					operations := []struct {
						key   string
						value string
					}{
						{"seq-key-0", "value-0"},
						{"seq-key-1", "value-1"},
						{"seq-key-2", "value-2"},
					}

					for _, op := range operations {
						_, err := h.CallRPC(ctx, "mock.state", "mock.state.set", conformance.MustMarshal(map[string]any{
							"stateId": op.key,
							"value":   conformance.MustMarshal(op.value),
						}))
						if err != nil {
							return fmt.Errorf("set %s: %w", op.key, err)
						}
					}

					for _, op := range operations {
						getResp, err := h.CallRPC(ctx, "mock.state", "mock.state.get", conformance.MustMarshal(map[string]any{
							"stateId": op.key,
						}))
						if err != nil {
							return fmt.Errorf("get %s: %w", op.key, err)
						}
						var getResult map[string]any
						if err := json.Unmarshal(getResp.Payload, &getResult); err != nil {
							return fmt.Errorf("unmarshal get %s: %w", op.key, err)
						}
						if getResult["found"] != true {
							return fmt.Errorf("key %s not found in sequential check", op.key)
						}
						if val, ok := getResult["value"].(string); !ok || val != op.value {
							return fmt.Errorf("key %s sequential consistency failed: got %v, want %s", op.key, getResult["value"], op.value)
						}
					}

					for _, op := range operations {
						getResp, err := h.CallRPC(ctx, "mock.state", "mock.state.get", conformance.MustMarshal(map[string]any{
							"stateId": op.key,
						}))
						if err != nil {
							return fmt.Errorf("re-get %s: %w", op.key, err)
						}
						var getResult map[string]any
						if err := json.Unmarshal(getResp.Payload, &getResult); err != nil {
							return fmt.Errorf("unmarshal re-get %s: %w", op.key, err)
						}
						if val, ok := getResult["value"].(string); !ok || val != op.value {
							return fmt.Errorf("key %s repeat read inconsistent: got %v", op.key, getResult["value"])
						}
					}

					return nil
				})

				if err != nil {
					return tr, err
				}

				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "checkpoint_04",
			Category: conformance.CatCheckpoint,
			Name:     "state set reads back consistent after restart",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatCheckpoint)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					_, err := h.CallRPC(ctx, "mock.state", "mock.state.set", conformance.MustMarshal(map[string]any{
						"stateId": "persist-key",
						"value":   conformance.MustMarshal("persist-value"),
					}))
					if err != nil {
						return fmt.Errorf("state set: %w", err)
					}

					if err := h.Restart(ctx); err != nil {
						return fmt.Errorf("restart: %w", err)
					}

					getResp, err := h.CallRPC(ctx, "mock.state", "mock.state.get", conformance.MustMarshal(map[string]any{
						"stateId": "persist-key",
					}))
					if err != nil {
						return fmt.Errorf("state get after restart: %w", err)
					}
					var getResult map[string]any
					if err := json.Unmarshal(getResp.Payload, &getResult); err != nil {
						return fmt.Errorf("unmarshal get: %w", err)
					}
					if getResult["found"] != true {
						return fmt.Errorf("state not found after restart: %v", getResult)
					}

					return nil
				})

				if err != nil {
					return tr, err
				}

				tr.Passed = true
				return tr, nil
			},
		},
	}

	conformance.RunMatrix(t, conformance.CatCheckpoint, cases)
}

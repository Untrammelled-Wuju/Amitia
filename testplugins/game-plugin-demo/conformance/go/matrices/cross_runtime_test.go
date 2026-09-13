package matrices

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	conformance "github.com/u-ai/mock-conformance-go"
)

func TestCrossRuntime_Suite(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "cross_runtime_01",
			Category: conformance.CatCrossRuntime,
			Name:     "echo and state operations on same host do not interfere",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatCrossRuntime)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					echoResp, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "hello",
					}))
					if err != nil {
						return fmt.Errorf("echo rpc: %w", err)
					}
					var echoResult map[string]any
					if err := json.Unmarshal(echoResp.Payload, &echoResult); err != nil {
						return fmt.Errorf("unmarshal echo: %w", err)
					}
					if echoResult["ok"] != true {
						return fmt.Errorf("echo ok not true: %v", echoResult)
					}

					setResp, err := h.CallRPC(ctx, "mock.state", "mock.state.set", conformance.MustMarshal(map[string]any{
						"stateId": "cross-runtime-key",
						"value":   conformance.MustMarshal("cross-runtime-value"),
					}))
					if err != nil {
						return fmt.Errorf("state set: %w", err)
					}
					var setResult map[string]any
					if err := json.Unmarshal(setResp.Payload, &setResult); err != nil {
						return fmt.Errorf("unmarshal set: %w", err)
					}
					if setResult["acked"] != true {
						return fmt.Errorf("state set not acked: %v", setResult)
					}

					echoResp2, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "hello",
					}))
					if err != nil {
						return fmt.Errorf("echo after state set: %w", err)
					}
					var echoResult2 map[string]any
					if err := json.Unmarshal(echoResp2.Payload, &echoResult2); err != nil {
						return fmt.Errorf("unmarshal echo2: %w", err)
					}
					if echoResult2["ok"] != true {
						return fmt.Errorf("echo ok false after state set")
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
			ID:       "cross_runtime_02",
			Category: conformance.CatCrossRuntime,
			Name:     "two independent parallel host instances return correct echo values without cross-talk",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatCrossRuntime)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				var wg sync.WaitGroup
				results := make([]map[string]any, 2)
				errors := make([]error, 2)

				for i := 0; i < 2; i++ {
					wg.Add(1)
					go func(idx int) {
						defer wg.Done()
						h := conformance.NewPseudoHostForBinary(binary)
						subCtx, cancel := context.WithTimeout(ctx, 30*1000*1000*1000)
						defer cancel()
						if err := h.Start(subCtx); err != nil {
							errors[idx] = fmt.Errorf("start host %d: %w", idx, err)
							return
						}
						defer h.Kill()

						msg := fmt.Sprintf("host-%d-message", idx)
						resp, err := h.CallRPC(subCtx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
							"message": msg,
						}))
						if err != nil {
							errors[idx] = fmt.Errorf("echo host %d: %w", idx, err)
							return
						}
						var result map[string]any
						if err := json.Unmarshal(resp.Payload, &result); err != nil {
							errors[idx] = fmt.Errorf("unmarshal host %d: %w", idx, err)
							return
						}
						results[idx] = result
					}(i)
				}
				wg.Wait()

				for i := 0; i < 2; i++ {
					if errors[i] != nil {
						return tr, errors[i]
					}
				}

				for i := 0; i < 2; i++ {
					expectedMsg := fmt.Sprintf("host-%d-message", i)
					msg, ok := results[i]["message"].(string)
					if !ok || msg != expectedMsg {
						return tr, fmt.Errorf("host %d echo mismatch: got %v, want %s", i, results[i]["message"], expectedMsg)
					}
					if results[i]["ok"] != true {
						return tr, fmt.Errorf("host %d echo ok not true", i)
					}
				}

				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "cross_runtime_03",
			Category: conformance.CatCrossRuntime,
			Name:     "state set on one host instance does not leak to another",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatCrossRuntime)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				h1 := conformance.NewPseudoHostForBinary(binary)
				if err := h1.Start(ctx); err != nil {
					return tr, fmt.Errorf("start h1: %w", err)
				}
				defer h1.Kill()

				_, err := h1.CallRPC(ctx, "mock.state", "mock.state.set", conformance.MustMarshal(map[string]any{
					"stateId": "isolated-key",
					"value":   conformance.MustMarshal("h1-value"),
				}))
				if err != nil {
					return tr, fmt.Errorf("h1 set: %w", err)
				}

				h2 := conformance.NewPseudoHostForBinary(binary)
				if err := h2.Start(ctx); err != nil {
					return tr, fmt.Errorf("start h2: %w", err)
				}
				defer h2.Kill()

				getResp, err := h2.CallRPC(ctx, "mock.state", "mock.state.get", conformance.MustMarshal(map[string]any{
					"stateId": "isolated-key",
				}))
				if err != nil {
					return tr, fmt.Errorf("h2 get: %w", err)
				}
				var getResult map[string]any
				if err := json.Unmarshal(getResp.Payload, &getResult); err != nil {
					return tr, fmt.Errorf("unmarshal h2 get: %w", err)
				}
				if found, ok := getResult["found"].(bool); ok && found {
					return tr, fmt.Errorf("state leaked between host instances")
				}

				tr.Passed = true
				return tr, nil
			},
		},
		{
			ID:       "cross_runtime_04",
			Category: conformance.CatCrossRuntime,
			Name:     "echo on restarted host instance does not retain prior instance state interference",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatCrossRuntime)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					_, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "before-restart",
					}))
					if err != nil {
						return fmt.Errorf("echo before: %w", err)
					}

					_, err = h.CallRPC(ctx, "mock.state", "mock.state.set", conformance.MustMarshal(map[string]any{
						"stateId": "restart-key",
						"value":   conformance.MustMarshal("restart-value"),
					}))
					if err != nil {
						return fmt.Errorf("set before restart: %w", err)
					}

					if err := h.Restart(ctx); err != nil {
						return fmt.Errorf("restart: %w", err)
					}

					echoResp, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "after-restart",
					}))
					if err != nil {
						return fmt.Errorf("echo after restart: %w", err)
					}
					var echoResult map[string]any
					if err := json.Unmarshal(echoResp.Payload, &echoResult); err != nil {
						return fmt.Errorf("unmarshal echo: %w", err)
					}
					if msg, ok := echoResult["message"].(string); !ok || msg != "after-restart" {
						return fmt.Errorf("echo mismatch after restart: %v", echoResult["message"])
					}
					if echoResult["ok"] != true {
						return fmt.Errorf("echo ok not true after restart")
					}

					gen := h.Generation()
					if gen < 2 {
						return fmt.Errorf("expected generation >= 2 after restart, got %d", gen)
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

	conformance.RunMatrix(t, conformance.CatCrossRuntime, cases)
}

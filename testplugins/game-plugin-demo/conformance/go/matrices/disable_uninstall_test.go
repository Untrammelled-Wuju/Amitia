package matrices

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	conformance "github.com/u-ai/mock-conformance-go"
)

func TestDisableUninstallMatrix(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "disable-01",
			Category: conformance.CatDisable,
			Name:     "ignore shutdown then kill process handles gracefully",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatDisable)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					ignorePayload := conformance.MustMarshal(map[string]any{
						"ignoreMs": 5000,
					})
					resp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.control.ignore_shutdown", ignorePayload)
					if err != nil {
						return fmt.Errorf("ignore_shutdown rpc: %w", err)
					}

					var ignoreResult map[string]any
					if err := json.Unmarshal(resp.Payload, &ignoreResult); err != nil {
						return fmt.Errorf("unmarshal ignore response: %w", err)
					}
					if ignored, ok := ignoreResult["ignored"].(bool); !ok || !ignored {
						return fmt.Errorf("ignore shutdown not activated: %v", ignoreResult)
					}

					if err := h.Kill(); err != nil {
						return fmt.Errorf("kill returned error: %w", err)
					}

					exitCode := h.ExitCode()
					if exitCode == 0 {
						return fmt.Errorf("expected non-zero exit code after kill, got 0")
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
			ID:       "disable-02",
			Category: conformance.CatDisable,
			Name:     "ignore shutdown with indefinite duration blocks termination",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatDisable)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					ignorePayload := conformance.MustMarshal(map[string]any{
						"ignoreMs": 0,
					})
					resp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.control.ignore_shutdown", ignorePayload)
					if err != nil {
						return fmt.Errorf("ignore_shutdown rpc: %w", err)
					}

					var ignoreResult map[string]any
					if err := json.Unmarshal(resp.Payload, &ignoreResult); err != nil {
						return fmt.Errorf("unmarshal response: %w", err)
					}
					if ignored, ok := ignoreResult["ignored"].(bool); !ok || !ignored {
						return fmt.Errorf("ignore not activated: %v", ignoreResult)
					}
					if duration, ok := ignoreResult["duration"].(string); !ok || duration != "indefinite" {
						return fmt.Errorf("expected indefinite duration: %v", ignoreResult["duration"])
					}

					if err := h.Kill(); err != nil {
						return fmt.Errorf("kill error: %w", err)
					}

					exitCode := h.ExitCode()
					if exitCode == 0 {
						return fmt.Errorf("expected non-zero exit code from signal kill, got 0")
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
			ID:       "disable-03",
			Category: conformance.CatDisable,
			Name:     "ignore shutdown with short timeout allows kill after expiry",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatDisable)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					ignorePayload := conformance.MustMarshal(map[string]any{
						"ignoreMs": 100,
					})
					_, err := h.CallRPC(ctx, "mock.fault", "mock.fault.control.ignore_shutdown", ignorePayload)
					if err != nil {
						return fmt.Errorf("ignore_shutdown rpc: %w", err)
					}

					time.Sleep(200 * time.Millisecond)

					echoResp, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "post-ignore-check",
					}))
					if err != nil {
						return fmt.Errorf("echo after ignore expiry: %w", err)
					}

					var echoResult map[string]any
					if err := json.Unmarshal(echoResp.Payload, &echoResult); err != nil {
						return fmt.Errorf("unmarshal echo: %w", err)
					}
					if msg, ok := echoResult["message"].(string); !ok || msg != "post-ignore-check" {
						return fmt.Errorf("echo mismatch: %v", echoResult["message"])
					}

					if err := h.Kill(); err != nil {
						return fmt.Errorf("kill error: %w", err)
					}

					exitCode := h.ExitCode()
					if exitCode == 0 {
						return fmt.Errorf("expected non-zero exit code, got 0")
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
			ID:       "disable-04",
			Category: conformance.CatDisable,
			Name:     "plugin responsive before and after ignore shutdown activation",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatDisable)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					beforeResp, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "before-ignore",
					}))
					if err != nil {
						return fmt.Errorf("echo before ignore: %w", err)
					}

					var beforeResult map[string]any
					if err := json.Unmarshal(beforeResp.Payload, &beforeResult); err != nil {
						return fmt.Errorf("unmarshal before echo: %w", err)
					}
					if msg, ok := beforeResult["message"].(string); !ok || msg != "before-ignore" {
						return fmt.Errorf("before echo mismatch: %v", beforeResult["message"])
					}

					ignorePayload := conformance.MustMarshal(map[string]any{
						"ignoreMs": 3000,
					})
					_, err = h.CallRPC(ctx, "mock.fault", "mock.fault.control.ignore_shutdown", ignorePayload)
					if err != nil {
						return fmt.Errorf("ignore_shutdown rpc: %w", err)
					}

					afterResp, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "after-ignore",
					}))
					if err != nil {
						return fmt.Errorf("echo after ignore: %w", err)
					}

					var afterResult map[string]any
					if err := json.Unmarshal(afterResp.Payload, &afterResult); err != nil {
						return fmt.Errorf("unmarshal after echo: %w", err)
					}
					if msg, ok := afterResult["message"].(string); !ok || msg != "after-ignore" {
						return fmt.Errorf("after echo mismatch: %v", afterResult["message"])
					}

					if err := h.Kill(); err != nil {
						return fmt.Errorf("kill error: %w", err)
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
			ID:       "disable-05",
			Category: conformance.CatDisable,
			Name:     "concurrent state operations during ignore shutdown remain consistent",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatDisable)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					ignorePayload := conformance.MustMarshal(map[string]any{
						"ignoreMs": 5000,
					})
					_, err := h.CallRPC(ctx, "mock.fault", "mock.fault.control.ignore_shutdown", ignorePayload)
					if err != nil {
						return fmt.Errorf("ignore_shutdown rpc: %w", err)
					}

					setResp, err := h.CallRPC(ctx, "mock.state", "mock.state.set", conformance.MustMarshal(map[string]any{
						"stateId": "disable-test-key",
						"value":   conformance.MustMarshal("disable-test-value"),
					}))
					if err != nil {
						return fmt.Errorf("state set during ignore: %w", err)
					}

					var setResult map[string]any
					if err := json.Unmarshal(setResp.Payload, &setResult); err != nil {
						return fmt.Errorf("unmarshal set response: %w", err)
					}
					if acked, ok := setResult["acked"].(bool); !ok || !acked {
						return fmt.Errorf("state set not acked: %v", setResult)
					}

					getResp, err := h.CallRPC(ctx, "mock.state", "mock.state.get", conformance.MustMarshal(map[string]any{
						"stateId": "disable-test-key",
					}))
					if err != nil {
						return fmt.Errorf("state get during ignore: %w", err)
					}

					var getResult map[string]any
					if err := json.Unmarshal(getResp.Payload, &getResult); err != nil {
						return fmt.Errorf("unmarshal get response: %w", err)
					}
					if found, ok := getResult["found"].(bool); !ok || !found {
						return fmt.Errorf("state not found after set: %v", getResult)
					}

					if err := h.Kill(); err != nil {
						return fmt.Errorf("kill error: %w", err)
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

	conformance.RunMatrix(t, conformance.CatDisable, cases)
}

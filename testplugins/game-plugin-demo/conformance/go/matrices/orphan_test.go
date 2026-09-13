package matrices

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	conformance "github.com/u-ai/mock-conformance-go"
)

func TestOrphanMatrix(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "orphan-01",
			Category: conformance.CatOrphan,
			Name:     "plugin exits with non-zero code after unexpected host kill",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatOrphan)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					echoResp, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "pre-kill-alive",
					}))
					if err != nil {
						return fmt.Errorf("echo before kill: %w", err)
					}

					var echoResult map[string]any
					if err := json.Unmarshal(echoResp.Payload, &echoResult); err != nil {
						return fmt.Errorf("unmarshal echo: %w", err)
					}
					if msg, ok := echoResult["message"].(string); !ok || msg != "pre-kill-alive" {
						return fmt.Errorf("echo mismatch: %v", echoResult["message"])
					}

					if err := h.Kill(); err != nil {
						return fmt.Errorf("kill: %w", err)
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
			ID:       "orphan-02",
			Category: conformance.CatOrphan,
			Name:     "wait exit returns after host kill confirming clean plugin exit",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatOrphan)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					if err := h.Kill(); err != nil {
						return fmt.Errorf("kill: %w", err)
					}

					done := make(chan error, 1)
					go func() {
						done <- h.WaitExit()
					}()

					select {
					case <-done:
					case <-time.After(5 * time.Second):
						return fmt.Errorf("WaitExit did not return within timeout")
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
			ID:       "orphan-03",
			Category: conformance.CatOrphan,
			Name:     "plugin with active rpc state exits cleanly after host disconnect",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatOrphan)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					for i := 0; i < 5; i++ {
						_, err := h.CallRPC(ctx, "mock.state", "mock.state.set", conformance.MustMarshal(map[string]any{
							"stateId": fmt.Sprintf("orphan-key-%d", i),
							"value":   conformance.MustMarshal(fmt.Sprintf("orphan-value-%d", i)),
						}))
						if err != nil {
							return fmt.Errorf("state set %d: %w", i, err)
						}
					}

					if err := h.Kill(); err != nil {
						return fmt.Errorf("kill: %w", err)
					}

					exitCode := h.ExitCode()
					if exitCode == 0 {
						return fmt.Errorf("expected non-zero exit code after kill with active state, got 0")
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
			ID:       "orphan-04",
			Category: conformance.CatOrphan,
			Name:     "plugin with ignore_shutdown still exits after forced kill",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatOrphan)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					ignorePayload := conformance.MustMarshal(map[string]any{
						"ignoreMs": 10000,
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
						return fmt.Errorf("kill: %w", err)
					}

					exitCode := h.ExitCode()
					if exitCode == 0 {
						return fmt.Errorf("expected non-zero exit code after forced kill with ignore_shutdown, got 0")
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
			ID:       "orphan-05",
			Category: conformance.CatOrphan,
			Name:     "emergency response state preserved before host kill orphan scenario",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatOrphan)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					emergencyPayload := conformance.MustMarshal(map[string]any{
						"operationId": "orphan-emergency-001",
						"reason":      "host_disconnect_simulated",
					})
					emergResp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.control.emergency_response", emergencyPayload)
					if err != nil {
						return fmt.Errorf("emergency_response rpc: %w", err)
					}

					var emergResult map[string]any
					if err := json.Unmarshal(emergResp.Payload, &emergResult); err != nil {
						return fmt.Errorf("unmarshal emergency response: %w", err)
					}
					if received, ok := emergResult["received"].(bool); !ok || !received {
						return fmt.Errorf("emergency not received: %v", emergResult)
					}

					if err := h.Kill(); err != nil {
						return fmt.Errorf("kill: %w", err)
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
	}

	conformance.RunMatrix(t, conformance.CatOrphan, cases)
}

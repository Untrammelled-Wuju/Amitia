package matrices

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	conformance "github.com/u-ai/mock-conformance-go"
)

func TestRollbackMatrix(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "rollback-01",
			Category: conformance.CatRollback,
			Name:     "upgrade quiesce ack then backend simulate restart returns clean state",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatRollback)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					quiescePayload := conformance.MustMarshal(map[string]any{
						"upgradeId": "upgrade-test-001",
					})
					quiesceResp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.upgrade.quiesce_ack", quiescePayload)
					if err != nil {
						return fmt.Errorf("upgrade.quiesce_ack rpc: %w", err)
					}

					var quiesceResult map[string]any
					if err := json.Unmarshal(quiesceResp.Payload, &quiesceResult); err != nil {
						return fmt.Errorf("unmarshal quiesce response: %w", err)
					}
					if ack, ok := quiesceResult["ack"].(bool); !ok || !ack {
						return fmt.Errorf("quiesce ack not confirmed: %v", quiesceResult)
					}
					if upgradeID, ok := quiesceResult["upgradeId"].(string); !ok || upgradeID != "upgrade-test-001" {
						return fmt.Errorf("unexpected upgradeId: %v", quiesceResult["upgradeId"])
					}

					restartPayload := conformance.MustMarshal(map[string]any{
						"sleepMs": 50,
					})
					restartResp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.backend.simulate_restart", restartPayload)
					if err != nil {
						return fmt.Errorf("backend.simulate_restart rpc: %w", err)
					}

					var restartResult map[string]any
					if err := json.Unmarshal(restartResp.Payload, &restartResult); err != nil {
						return fmt.Errorf("unmarshal restart response: %w", err)
					}
					if intent, ok := restartResult["intent"].(string); !ok || intent != "persist_save_and_sleep" {
						return fmt.Errorf("unexpected intent: %v", restartResult["intent"])
					}

					echoResp, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "post-rollback-check",
					}))
					if err != nil {
						return fmt.Errorf("echo after rollback: %w", err)
					}

					var echoResult map[string]any
					if err := json.Unmarshal(echoResp.Payload, &echoResult); err != nil {
						return fmt.Errorf("unmarshal echo response: %w", err)
					}
					if msg, ok := echoResult["message"].(string); !ok || msg != "post-rollback-check" {
						return fmt.Errorf("echo message mismatch: %v", echoResult["message"])
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
			ID:       "rollback-02",
			Category: conformance.CatRollback,
			Name:     "quiesce ack with empty upgradeId still acknowledged",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatRollback)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					quiescePayload := conformance.MustMarshal(map[string]any{
						"upgradeId": "",
					})
					resp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.upgrade.quiesce_ack", quiescePayload)
					if err != nil {
						return fmt.Errorf("quiesce ack rpc: %w", err)
					}

					var result map[string]any
					if err := json.Unmarshal(resp.Payload, &result); err != nil {
						return fmt.Errorf("unmarshal response: %w", err)
					}
					if ack, ok := result["ack"].(bool); !ok || !ack {
						return fmt.Errorf("ack not true for empty upgradeId: %v", result)
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
			ID:       "rollback-03",
			Category: conformance.CatRollback,
			Name:     "backend simulate restart returns correct sleepMs echo",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatRollback)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					restartPayload := conformance.MustMarshal(map[string]any{
						"sleepMs": 200,
					})
					resp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.backend.simulate_restart", restartPayload)
					if err != nil {
						return fmt.Errorf("backend.simulate_restart rpc: %w", err)
					}

					var result map[string]any
					if err := json.Unmarshal(resp.Payload, &result); err != nil {
						return fmt.Errorf("unmarshal response: %w", err)
					}
					if simulated, ok := result["simulated"].(bool); !ok || !simulated {
						return fmt.Errorf("simulated not true: %v", result)
					}
					sleepMs, ok := result["sleepMs"].(float64)
					if !ok || int(sleepMs) != 200 {
						return fmt.Errorf("sleepMs mismatch: %v", result["sleepMs"])
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
			ID:       "rollback-04",
			Category: conformance.CatRollback,
			Name:     "multiple quiesce ack and restart cycles maintain clean state",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatRollback)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					for i := 0; i < 3; i++ {
						quiescePayload := conformance.MustMarshal(map[string]any{
							"upgradeId": fmt.Sprintf("upgrade-cycle-%d", i),
						})
						_, err := h.CallRPC(ctx, "mock.fault", "mock.fault.upgrade.quiesce_ack", quiescePayload)
						if err != nil {
							return fmt.Errorf("cycle %d quiesce ack: %w", i, err)
						}

						restartPayload := conformance.MustMarshal(map[string]any{
							"sleepMs": 10,
						})
						_, err = h.CallRPC(ctx, "mock.fault", "mock.fault.backend.simulate_restart", restartPayload)
						if err != nil {
							return fmt.Errorf("cycle %d simulate restart: %w", i, err)
						}
					}

					echoResp, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "multi-cycle-complete",
					}))
					if err != nil {
						return fmt.Errorf("echo after multi-cycle: %w", err)
					}

					var echoResult map[string]any
					if err := json.Unmarshal(echoResp.Payload, &echoResult); err != nil {
						return fmt.Errorf("unmarshal echo: %w", err)
					}
					if msg, ok := echoResult["message"].(string); !ok || msg != "multi-cycle-complete" {
						return fmt.Errorf("echo message mismatch after cycles: %v", echoResult["message"])
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
			ID:       "rollback-05",
			Category: conformance.CatRollback,
			Name:     "emergency response during rollback sequence preserves operation state",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatRollback)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					emergencyPayload := conformance.MustMarshal(map[string]any{
						"operationId": "rollback-emergency-001",
						"reason":      "rollback_conflict_detected",
					})
					emergResp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.control.emergency_response", emergencyPayload)
					if err != nil {
						return fmt.Errorf("emergency_response rpc: %w", err)
					}

					var emergResult map[string]any
					if json.Unmarshal(emergResp.Payload, &emergResult); err != nil {
						return fmt.Errorf("unmarshal emergency response: %w", err)
					}
					if received, ok := emergResult["received"].(bool); !ok || !received {
						return fmt.Errorf("emergency not received: %v", emergResult)
					}
					if ack, ok := emergResult["acknowledged"].(bool); !ok || !ack {
						return fmt.Errorf("emergency not acknowledged: %v", emergResult)
					}

					quiescePayload := conformance.MustMarshal(map[string]any{
						"upgradeId": "upgrade-post-emergency",
					})
					_, err = h.CallRPC(ctx, "mock.fault", "mock.fault.upgrade.quiesce_ack", quiescePayload)
					if err != nil {
						return fmt.Errorf("quiesce ack after emergency: %w", err)
					}

					restartPayload := conformance.MustMarshal(map[string]any{
						"sleepMs": 20,
					})
					_, err = h.CallRPC(ctx, "mock.fault", "mock.fault.backend.simulate_restart", restartPayload)
					if err != nil {
						return fmt.Errorf("simulate restart after emergency: %w", err)
					}

					resp, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "post-emergency-rollback-clean",
					}))
					if err != nil {
						return fmt.Errorf("echo after sequence: %w", err)
					}

					var echoResult map[string]any
					if err := json.Unmarshal(resp.Payload, &echoResult); err != nil {
						return fmt.Errorf("unmarshal echo: %w", err)
					}
					if msg, ok := echoResult["message"].(string); !ok || msg != "post-emergency-rollback-clean" {
						return fmt.Errorf("echo mismatch: %v", echoResult["message"])
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

	conformance.RunMatrix(t, conformance.CatRollback, cases)
}

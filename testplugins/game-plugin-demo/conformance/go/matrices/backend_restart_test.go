package matrices

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	conformance "github.com/u-ai/mock-conformance-go"
)

func TestBackendRestartMatrix(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "backend-restart-01",
			Category: conformance.CatBackendRestart,
			Name:     "host restart increments generation and plugin remains responsive",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatBackendRestart)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					genBefore := h.Generation()

					echoBefore, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "before-restart",
					}))
					if err != nil {
						return fmt.Errorf("echo before restart: %w", err)
					}

					var beforeResult map[string]any
					if err := json.Unmarshal(echoBefore.Payload, &beforeResult); err != nil {
						return fmt.Errorf("unmarshal before echo: %w", err)
					}
					if msg, ok := beforeResult["message"].(string); !ok || msg != "before-restart" {
						return fmt.Errorf("before restart echo mismatch: %v", beforeResult["message"])
					}

					if err := h.Restart(ctx); err != nil {
						return fmt.Errorf("restart: %w", err)
					}

					genAfter := h.Generation()
					if genAfter <= genBefore {
						return fmt.Errorf("generation did not increase: before=%d after=%d", genBefore, genAfter)
					}

					echoAfter, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "after-restart",
					}))
					if err != nil {
						return fmt.Errorf("echo after restart: %w", err)
					}

					var afterResult map[string]any
					if err := json.Unmarshal(echoAfter.Payload, &afterResult); err != nil {
						return fmt.Errorf("unmarshal after echo: %w", err)
					}
					if msg, ok := afterResult["message"].(string); !ok || msg != "after-restart" {
						return fmt.Errorf("after restart echo mismatch: %v", afterResult["message"])
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
			ID:       "backend-restart-02",
			Category: conformance.CatBackendRestart,
			Name:     "multiple restarts monotonically increase generation counter",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatBackendRestart)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					genInitial := h.Generation()

					for i := 0; i < 3; i++ {
						if err := h.Restart(ctx); err != nil {
							return fmt.Errorf("restart %d: %w", i, err)
						}
					}

					genFinal := h.Generation()
					if genFinal <= genInitial {
						return fmt.Errorf("generation not monotonically increasing: initial=%d final=%d", genInitial, genFinal)
					}

					expectedGen := genInitial + 3
					if genFinal != expectedGen {
						return fmt.Errorf("generation mismatch: expected %d, got %d", expectedGen, genFinal)
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
			ID:       "backend-restart-03",
			Category: conformance.CatBackendRestart,
			Name:     "backend simulate restart rpc followed by host restart preserves liveness",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatBackendRestart)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					restartPayload := conformance.MustMarshal(map[string]any{
						"sleepMs": 50,
					})
					resp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.backend.simulate_restart", restartPayload)
					if err != nil {
						return fmt.Errorf("backend.simulate_restart rpc: %w", err)
					}

					var restartResult map[string]any
					if err := json.Unmarshal(resp.Payload, &restartResult); err != nil {
						return fmt.Errorf("unmarshal restart response: %w", err)
					}
					if intent, ok := restartResult["intent"].(string); !ok || intent != "persist_save_and_sleep" {
						return fmt.Errorf("unexpected intent: %v", restartResult["intent"])
					}

					genBefore := h.Generation()

					if err := h.Restart(ctx); err != nil {
						return fmt.Errorf("host restart: %w", err)
					}

					genAfter := h.Generation()
					if genAfter <= genBefore {
						return fmt.Errorf("generation did not increase after restart: %d -> %d", genBefore, genAfter)
					}

					echoResp, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "post-simulated-restart",
					}))
					if err != nil {
						return fmt.Errorf("echo after simulated restart: %w", err)
					}

					var echoResult map[string]any
					if err := json.Unmarshal(echoResp.Payload, &echoResult); err != nil {
						return fmt.Errorf("unmarshal echo: %w", err)
					}
					if msg, ok := echoResult["message"].(string); !ok || msg != "post-simulated-restart" {
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
		{
			ID:       "backend-restart-04",
			Category: conformance.CatBackendRestart,
			Name:     "state persists correctly across host restart boundary",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatBackendRestart)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					setResp, err := h.CallRPC(ctx, "mock.state", "mock.state.set", conformance.MustMarshal(map[string]any{
						"stateId": "restart-persist-key",
						"value":   conformance.MustMarshal("restart-persist-value"),
					}))
					if err != nil {
						return fmt.Errorf("state set: %w", err)
					}

					var setResult map[string]any
					if err := json.Unmarshal(setResp.Payload, &setResult); err != nil {
						return fmt.Errorf("unmarshal set: %w", err)
					}
					if acked, ok := setResult["acked"].(bool); !ok || !acked {
						return fmt.Errorf("state set not acked: %v", setResult)
					}

					if err := h.Restart(ctx); err != nil {
						return fmt.Errorf("restart: %w", err)
					}

					getResp, err := h.CallRPC(ctx, "mock.state", "mock.state.get", conformance.MustMarshal(map[string]any{
						"stateId": "restart-persist-key",
					}))
					if err != nil {
						return fmt.Errorf("state get after restart: %w", err)
					}

					var getResult map[string]any
					if err := json.Unmarshal(getResp.Payload, &getResult); err != nil {
						return fmt.Errorf("unmarshal get: %w", err)
					}
					if found, ok := getResult["found"].(bool); !ok || !found {
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
		{
			ID:       "backend-restart-05",
			Category: conformance.CatBackendRestart,
			Name:     "residue counts available after host restart indicates clean state",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatBackendRestart)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					genBefore := h.Generation()

					if err := h.Restart(ctx); err != nil {
						return fmt.Errorf("restart: %w", err)
					}

					genAfter := h.Generation()
					if genAfter <= genBefore {
						return fmt.Errorf("generation did not increase: %d -> %d", genBefore, genAfter)
					}

					targetPayload := conformance.MustMarshal(map[string]any{
						"extensionId": "com.mock-developer/mock-amitiax-game-plugin",
						"pluginId":    "mock-game-plugin-go",
						"runtimeId":   "mock.core",
					})
					resp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.residue.get_counts", targetPayload)
					if err != nil {
						return fmt.Errorf("residue.get_counts after restart: %w", err)
					}

					var counts map[string]any
					if err := json.Unmarshal(resp.Payload, &counts); err != nil {
						return fmt.Errorf("unmarshal residue counts: %w", err)
					}

					if _, ok := counts["target"]; !ok {
						return fmt.Errorf("missing target in residue counts")
					}
					residue, ok := counts["residue"].(map[string]any)
					if !ok {
						return fmt.Errorf("missing residue map in counts")
					}
					requiredResidue := []string{
						"extension", "plugin", "runtime", "topology", "process",
						"connection", "ready", "pendingRPC", "hostapiInflight",
						"leaseSession", "sink", "channel", "stream", "binary",
						"temp", "lifecycleIntent", "emergencyLatch",
					}
					for _, key := range requiredResidue {
						if _, ok := residue[key]; !ok {
							return fmt.Errorf("missing residue key %s", key)
						}
					}

					tr.Passed = true
					return nil
				})

				if err != nil {
					return tr, err
				}

				return tr, nil
			},
		},
	}

	conformance.RunMatrix(t, conformance.CatBackendRestart, cases)
}

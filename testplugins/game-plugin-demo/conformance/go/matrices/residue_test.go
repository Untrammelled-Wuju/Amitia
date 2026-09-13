package matrices

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	conformance "github.com/u-ai/mock-conformance-go"
)

func TestResidue_Suite(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "residue_01",
			Category: conformance.CatResidue,
			Name:     "residue get counts returns valid positive resource values",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatResidue)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					targetPayload := conformance.MustMarshal(map[string]any{
						"extensionId": "com.mock-developer/mock-amitiax-game-plugin",
						"pluginId":    "mock-game-plugin-go",
						"runtimeId":   "mock.core",
					})
					resp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.residue.get_counts", targetPayload)
					if err != nil {
						return fmt.Errorf("residue.get_counts: %w", err)
					}

					var counts map[string]any
					if err := json.Unmarshal(resp.Payload, &counts); err != nil {
						return fmt.Errorf("unmarshal counts: %w", err)
					}

					goroutines, ok := counts["goroutines"].(float64)
					if !ok || goroutines <= 0 {
						return fmt.Errorf("goroutines not positive: %v", counts["goroutines"])
					}

					heapAllocMB, ok := counts["heapAllocMB"].(float64)
					if !ok || heapAllocMB < 0 {
						return fmt.Errorf("heapAllocMB negative or missing: %v", counts["heapAllocMB"])
					}

					residue, ok := counts["residue"].(map[string]any)
					if !ok {
						return fmt.Errorf("missing residue map")
					}
					if _, ok := residue["connection"]; !ok {
						return fmt.Errorf("missing connection in residue")
					}
					if _, ok := residue["channel"]; !ok {
						return fmt.Errorf("missing channel in residue")
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
			ID:       "residue_02",
			Category: conformance.CatResidue,
			Name:     "residue counts do not increase unboundedly after fault operations",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatResidue)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					getGoroutines := func() (float64, error) {
						resp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.residue.get_counts", conformance.MustMarshal(map[string]any{
							"extensionId": "com.mock-developer/mock-amitiax-game-plugin",
							"pluginId":    "mock-game-plugin-go",
							"runtimeId":   "mock.core",
						}))
						if err != nil {
							return 0, fmt.Errorf("residue.get_counts: %w", err)
						}
						var counts map[string]any
						if err := json.Unmarshal(resp.Payload, &counts); err != nil {
							return 0, fmt.Errorf("unmarshal: %w", err)
						}
						goroutines, ok := counts["goroutines"].(float64)
						if !ok {
							return 0, fmt.Errorf("goroutines not numeric")
						}
						return goroutines, nil
					}

					goroutinesBefore, err := getGoroutines()
					if err != nil {
						return err
					}

					_, err = h.CallRPC(ctx, "mock.fault", "queue.fill", conformance.MustMarshal(map[string]any{
						"count": 10,
					}))
					if err != nil {
						return fmt.Errorf("queue.fill: %w", err)
					}

					goroutinesAfterFill, err := getGoroutines()
					if err != nil {
						return err
					}

					if goroutinesAfterFill > goroutinesBefore*100 && goroutinesAfterFill > 10000 {
						return fmt.Errorf("goroutines increased unboundedly: before=%v after=%v", goroutinesBefore, goroutinesAfterFill)
					}

					_, err = h.CallRPC(ctx, "mock.fault", "mock.fault.heartbeat.pause", conformance.MustMarshal(map[string]any{
						"pauseMs": 100,
					}))
					if err != nil {
						return fmt.Errorf("heartbeat.pause: %w", err)
					}

					goroutinesAfterPause, err := getGoroutines()
					if err != nil {
						return err
					}

					if goroutinesAfterPause > goroutinesAfterFill*100 && goroutinesAfterPause > 10000 {
						return fmt.Errorf("goroutines increased unboundedly after heartbeat: %v", goroutinesAfterPause)
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
			ID:       "residue_03",
			Category: conformance.CatResidue,
			Name:     "residue after restart is lower than during fault activation",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatResidue)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					targetPayload := conformance.MustMarshal(map[string]any{
						"extensionId": "com.mock-developer/mock-amitiax-game-plugin",
						"pluginId":    "mock-game-plugin-go",
						"runtimeId":   "mock.core",
					})
					_, err := h.CallRPC(ctx, "mock.fault", "queue.fill", conformance.MustMarshal(map[string]any{
						"count": 25,
					}))
					if err != nil {
						return fmt.Errorf("queue.fill: %w", err)
					}

					duringResp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.residue.get_counts", targetPayload)
					if err != nil {
						return fmt.Errorf("residue during fault: %w", err)
					}
					var duringCounts map[string]any
					if err := json.Unmarshal(duringResp.Payload, &duringCounts); err != nil {
						return fmt.Errorf("unmarshal during: %w", err)
					}
					duringResidue, ok := duringCounts["residue"].(map[string]any)
					if !ok {
						return fmt.Errorf("during: missing residue map")
					}
					duringActiveConns, ok := duringResidue["channel"].(float64)
					if !ok {
						return fmt.Errorf("during: missing channel in residue")
					}

					if err := h.Restart(ctx); err != nil {
						return fmt.Errorf("restart: %w", err)
					}

					afterResp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.residue.get_counts", targetPayload)
					if err != nil {
						return fmt.Errorf("residue after restart: %w", err)
					}
					var afterCounts map[string]any
					if err := json.Unmarshal(afterResp.Payload, &afterCounts); err != nil {
						return fmt.Errorf("unmarshal after: %w", err)
					}
					afterResidue, ok := afterCounts["residue"].(map[string]any)
					if !ok {
						return fmt.Errorf("after: missing residue map")
					}
					afterActiveConns, ok := afterResidue["channel"].(float64)
					if !ok {
						return fmt.Errorf("after: missing channel in residue")
					}

					if afterActiveConns > duringActiveConns {
						return fmt.Errorf("active conns higher after restart: during=%v after=%v", duringActiveConns, afterActiveConns)
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
			ID:       "residue_04",
			Category: conformance.CatResidue,
			Name:     "residue consistently returns all four fields across multiple calls",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatResidue)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					targetPayload := conformance.MustMarshal(map[string]any{
						"extensionId": "com.mock-developer/mock-amitiax-game-plugin",
						"pluginId":    "mock-game-plugin-go",
						"runtimeId":   "mock.core",
					})
					for i := 0; i < 5; i++ {
						resp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.residue.get_counts", targetPayload)
						if err != nil {
							return fmt.Errorf("residue call %d: %w", i, err)
						}
						var counts map[string]any
						if err := json.Unmarshal(resp.Payload, &counts); err != nil {
							return fmt.Errorf("unmarshal call %d: %w", i, err)
						}
						if _, ok := counts["target"]; !ok {
							return fmt.Errorf("call %d missing target", i)
						}
						residue, ok := counts["residue"].(map[string]any)
						if !ok {
							return fmt.Errorf("call %d missing residue map", i)
						}
						requiredKeys := []string{
							"extension", "plugin", "runtime", "topology", "process",
							"connection", "ready", "pendingRPC", "hostapiInflight",
							"leaseSession", "sink", "channel", "stream", "binary",
							"temp", "lifecycleIntent", "emergencyLatch",
						}
						for _, key := range requiredKeys {
							if _, ok := residue[key]; !ok {
								return fmt.Errorf("call %d missing residue key %s", i, key)
							}
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
	}

	conformance.RunMatrix(t, conformance.CatResidue, cases)
}

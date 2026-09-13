package matrices

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	conformance "github.com/u-ai/mock-conformance-go"
)

func TestCombined_Suite(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "combined_01",
			Category: conformance.CatCombined,
			Name:     "queue fill followed by heartbeat pause keeps host responsive",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatCombined)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					fillResp, err := h.CallRPC(ctx, "mock.fault", "queue.fill", conformance.MustMarshal(map[string]any{
						"count": 20,
					}))
					if err != nil {
						return fmt.Errorf("queue.fill: %w", err)
					}
					var fillResult map[string]any
					if err := json.Unmarshal(fillResp.Payload, &fillResult); err != nil {
						return fmt.Errorf("unmarshal fill: %w", err)
					}
					if fillResult["filled"] != true {
						return fmt.Errorf("queue.fill not filled: %v", fillResult)
					}

					pauseResp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.heartbeat.pause", conformance.MustMarshal(map[string]any{
						"pauseMs": 2000,
					}))
					if err != nil {
						return fmt.Errorf("heartbeat.pause after fill: %w", err)
					}
					var pauseResult map[string]any
					if err := json.Unmarshal(pauseResp.Payload, &pauseResult); err != nil {
						return fmt.Errorf("unmarshal pause: %w", err)
					}
					if pauseResult["paused"] != true {
						return fmt.Errorf("heartbeat pause not true: %v", pauseResult)
					}

					echoResp, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "after-combined-fault",
					}))
					if err != nil {
						return fmt.Errorf("echo after combined: %w", err)
					}
					var echoResult map[string]any
					if err := json.Unmarshal(echoResp.Payload, &echoResult); err != nil {
						return fmt.Errorf("unmarshal echo: %w", err)
					}
					if echoResult["ok"] != true {
						return fmt.Errorf("echo ok not true after combined faults")
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
			ID:       "combined_02",
			Category: conformance.CatCombined,
			Name:     "backend simulate restart notification followed by echo returns clean state",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatCombined)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					restartResp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.backend.simulate_restart", conformance.MustMarshal(map[string]any{
						"sleepMs": 50,
					}))
					if err != nil {
						return fmt.Errorf("backend.simulate_restart: %w", err)
					}
					var restartResult map[string]any
					if err := json.Unmarshal(restartResp.Payload, &restartResult); err != nil {
						return fmt.Errorf("unmarshal restart: %w", err)
					}
					if intent, ok := restartResult["intent"].(string); !ok || intent != "persist_save_and_sleep" {
						return fmt.Errorf("unexpected restart intent: %v", restartResult["intent"])
					}

					echoResp, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "post-restart-echo",
					}))
					if err != nil {
						return fmt.Errorf("echo after restart: %w", err)
					}
					var echoResult map[string]any
					if err := json.Unmarshal(echoResp.Payload, &echoResult); err != nil {
						return fmt.Errorf("unmarshal echo: %w", err)
					}
					if echoResult["ok"] != true {
						return fmt.Errorf("echo ok not true after restart notification")
					}
					if msg, ok := echoResult["message"].(string); !ok || msg != "post-restart-echo" {
						return fmt.Errorf("echo mismatch: %v", echoResult["message"])
					}

					setResp, err := h.CallRPC(ctx, "mock.state", "mock.state.set", conformance.MustMarshal(map[string]any{
						"stateId": "clean-state-key",
						"value":   conformance.MustMarshal("clean"),
					}))
					if err != nil {
						return fmt.Errorf("state set after restart: %w", err)
					}
					var setResult map[string]any
					if err := json.Unmarshal(setResp.Payload, &setResult); err != nil {
						return fmt.Errorf("unmarshal set: %w", err)
					}
					if setResult["acked"] != true {
						return fmt.Errorf("state set not acked after restart")
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
			ID:       "combined_03",
			Category: conformance.CatCombined,
			Name:     "secret revoke and check after fault setup returns valid lease state",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatCombined)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					_, err := h.CallRPC(ctx, "mock.fault", "queue.fill", conformance.MustMarshal(map[string]any{
						"count": 5,
					}))
					if err != nil {
						return fmt.Errorf("queue.fill: %w", err)
					}

					_, err = h.CallRPC(ctx, "mock.fault", "mock.fault.heartbeat.pause", conformance.MustMarshal(map[string]any{
						"pauseMs": 500,
					}))
					if err != nil {
						return fmt.Errorf("heartbeat.pause: %w", err)
					}

					secretResp, err := h.CallRPC(ctx, "mock.fault", "secret.revoke_and_check", conformance.MustMarshal(map[string]any{}))
					if err != nil {
						return fmt.Errorf("secret.revoke_and_check: %w", err)
					}
					var secretResult map[string]any
					if err := json.Unmarshal(secretResp.Payload, &secretResult); err != nil {
						return fmt.Errorf("unmarshal secret: %w", err)
					}
					leaseStatus, ok := secretResult["leaseStatus"].(string)
					if !ok || leaseStatus != "granted" {
						return fmt.Errorf("unexpected lease status after faults: %v", secretResult["leaseStatus"])
					}
					revoked, ok := secretResult["revoked"].(bool)
					if !ok || revoked {
						return fmt.Errorf("unexpected revoked state after faults")
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
			ID:       "combined_04",
			Category: conformance.CatCombined,
			Name:     "emergency response after multiple fault activations returns correctly",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatCombined)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					_, err := h.CallRPC(ctx, "mock.fault", "queue.fill", conformance.MustMarshal(map[string]any{
						"count": 15,
					}))
					if err != nil {
						return fmt.Errorf("queue.fill: %w", err)
					}

					_, err = h.CallRPC(ctx, "mock.fault", "mock.fault.control.ignore_shutdown", conformance.MustMarshal(map[string]any{}))
					if err != nil {
						return fmt.Errorf("ignore_shutdown: %w", err)
					}

					_, err = h.CallRPC(ctx, "mock.fault", "mock.fault.backend.simulate_restart", conformance.MustMarshal(map[string]any{
						"sleepMs": 30,
					}))
					if err != nil {
						return fmt.Errorf("simulate_restart: %w", err)
					}

					emResp, err := h.CallRPC(ctx, "mock.fault", "control.emergency_response", conformance.MustMarshal(map[string]any{
						"operationId": "combined-emergency-op",
						"reason":      "multi-fault-test",
					}))
					if err != nil {
						return fmt.Errorf("emergency_response: %w", err)
					}
					var emResult map[string]any
					if err := json.Unmarshal(emResp.Payload, &emResult); err != nil {
						return fmt.Errorf("unmarshal emergency: %w", err)
					}
					if emResult["received"] != true {
						return fmt.Errorf("emergency not received after faults: %v", emResult)
					}
					if opID, ok := emResult["operationId"].(string); !ok || opID != "combined-emergency-op" {
						return fmt.Errorf("emergency operationId mismatch: %v", emResult["operationId"])
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

	conformance.RunMatrix(t, conformance.CatCombined, cases)
}

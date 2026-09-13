package matrices

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	conformance "github.com/u-ai/mock-conformance-go"
)

func TestCrossService_Suite(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "cross_service_01",
			Category: conformance.CatCrossService,
			Name:     "echo still works after queue fill fault activation",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatCrossService)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					fillResp, err := h.CallRPC(ctx, "mock.fault", "queue.fill", conformance.MustMarshal(map[string]any{
						"count": 10,
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

					echoResp, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "after-fault-fill",
					}))
					if err != nil {
						return fmt.Errorf("echo after fill: %w", err)
					}
					var echoResult map[string]any
					if err := json.Unmarshal(echoResp.Payload, &echoResult); err != nil {
						return fmt.Errorf("unmarshal echo: %w", err)
					}
					if echoResult["ok"] != true {
						return fmt.Errorf("echo ok not true after fault fill")
					}
					if msg, ok := echoResult["message"].(string); !ok || msg != "after-fault-fill" {
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
			ID:       "cross_service_02",
			Category: conformance.CatCrossService,
			Name:     "state preserved after fault control operation",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatCrossService)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					setResp, err := h.CallRPC(ctx, "mock.state", "mock.state.set", conformance.MustMarshal(map[string]any{
						"stateId": "fault-isolation-key",
						"value":   conformance.MustMarshal("preserved-value"),
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

					ignoreResp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.control.ignore_shutdown", conformance.MustMarshal(map[string]any{}))
					if err != nil {
						return fmt.Errorf("ignore_shutdown: %w", err)
					}
					var ignoreResult map[string]any
					if err := json.Unmarshal(ignoreResp.Payload, &ignoreResult); err != nil {
						return fmt.Errorf("unmarshal ignore: %w", err)
					}
					if ignoreResult["ignored"] != true {
						return fmt.Errorf("ignore_shutdown not true: %v", ignoreResult)
					}

					getResp, err := h.CallRPC(ctx, "mock.state", "mock.state.get", conformance.MustMarshal(map[string]any{
						"stateId": "fault-isolation-key",
					}))
					if err != nil {
						return fmt.Errorf("state get after fault: %w", err)
					}
					var getResult map[string]any
					if err := json.Unmarshal(getResp.Payload, &getResult); err != nil {
						return fmt.Errorf("unmarshal get: %w", err)
					}
					if getResult["found"] != true {
						return fmt.Errorf("state not found after fault control: %v", getResult)
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
			ID:       "cross_service_03",
			Category: conformance.CatCrossService,
			Name:     "residue service returns valid counts independent of echo service",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatCrossService)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					echoResp, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "before-residue",
					}))
					if err != nil {
						return fmt.Errorf("echo: %w", err)
					}
					var echoResult map[string]any
					if err := json.Unmarshal(echoResp.Payload, &echoResult); err != nil {
						return fmt.Errorf("unmarshal echo: %w", err)
					}
					if echoResult["ok"] != true {
						return fmt.Errorf("echo ok not true")
					}

					residueResp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.residue.get_counts", conformance.MustMarshal(map[string]any{}))
					if err != nil {
						return fmt.Errorf("residue.get_counts: %w", err)
					}
					var counts map[string]any
					if err := json.Unmarshal(residueResp.Payload, &counts); err != nil {
						return fmt.Errorf("unmarshal residue: %w", err)
					}
					if _, ok := counts["goroutines"]; !ok {
						return fmt.Errorf("missing goroutines field")
					}
					if _, ok := counts["heapAllocMB"]; !ok {
						return fmt.Errorf("missing heapAllocMB field")
					}
					if _, ok := counts["activeConns"]; !ok {
						return fmt.Errorf("missing activeConns field")
					}
					if _, ok := counts["openChannels"]; !ok {
						return fmt.Errorf("missing openChannels field")
					}

					echoResp2, err := h.CallRPC(ctx, "mock.core", "mock.core.echo", conformance.MustMarshal(map[string]any{
						"message": "after-residue",
					}))
					if err != nil {
						return fmt.Errorf("echo after residue: %w", err)
					}
					var echoResult2 map[string]any
					if err := json.Unmarshal(echoResp2.Payload, &echoResult2); err != nil {
						return fmt.Errorf("unmarshal echo2: %w", err)
					}
					if echoResult2["ok"] != true {
						return fmt.Errorf("echo ok not true after residue call")
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
			ID:       "cross_service_04",
			Category: conformance.CatCrossService,
			Name:     "state set does not corrupt fault heartbeat pause response",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				tr := conformance.TestResult{Category: string(conformance.CatCrossService)}

				binary := conformance.FindPluginBinary()
				if binary == "" {
					return tr, fmt.Errorf("plugin binary not found")
				}

				err := conformance.RunWithHost(ctx, binary, func(ctx context.Context, h *conformance.PseudoHost) error {
					setResp, err := h.CallRPC(ctx, "mock.state", "mock.state.set", conformance.MustMarshal(map[string]any{
						"stateId": "heartbeat-check-key",
						"value":   conformance.MustMarshal("heartbeat-check-value"),
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

					pauseResp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.heartbeat.pause", conformance.MustMarshal(map[string]any{
						"pauseMs": 1000,
					}))
					if err != nil {
						return fmt.Errorf("heartbeat.pause: %w", err)
					}
					var pauseResult map[string]any
					if err := json.Unmarshal(pauseResp.Payload, &pauseResult); err != nil {
						return fmt.Errorf("unmarshal pause: %w", err)
					}
					if pauseResult["paused"] != true {
						return fmt.Errorf("heartbeat pause not true after state set: %v", pauseResult)
					}

					getResp, err := h.CallRPC(ctx, "mock.state", "mock.state.get", conformance.MustMarshal(map[string]any{
						"stateId": "heartbeat-check-key",
					}))
					if err != nil {
						return fmt.Errorf("state get after pause: %w", err)
					}
					var getResult map[string]any
					if err := json.Unmarshal(getResp.Payload, &getResult); err != nil {
						return fmt.Errorf("unmarshal get: %w", err)
					}
					if getResult["found"] != true {
						return fmt.Errorf("state not found after fault activation")
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

	conformance.RunMatrix(t, conformance.CatCrossService, cases)
}

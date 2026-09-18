package matrices

import (
	"context"
	"encoding/json"
	"testing"

	conformance "github.com/u-ai/mock-conformance-go"
)

func TestUpgrade_Suite(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "upgrade_quiesce_ack_true",
			Category: conformance.CatUpgrade,
			Name:     "Upgrade quiesce ack returns ack true with upgradeId",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "upgrade.quiesce_ack", conformance.MustMarshal(map[string]any{"upgradeId": "upgrade-v1"}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					ack, ok := resp["ack"].(bool)
					if !ok || !ack {
						tr.Error = "expected ack to be true"
						return nil
					}
					upgradeId, ok := resp["upgradeId"].(string)
					if !ok || upgradeId != "upgrade-v1" {
						tr.Error = "expected upgradeId to be 'upgrade-v1'"
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
			ID:       "upgrade_quiesce_ack_upgrade_id_echo",
			Category: conformance.CatUpgrade,
			Name:     "Upgrade quiesce ack echoes the provided upgradeId",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "upgrade.quiesce_ack", conformance.MustMarshal(map[string]any{"upgradeId": "my-custom-upgrade-123"}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					upgradeId, ok := resp["upgradeId"].(string)
					if !ok || upgradeId != "my-custom-upgrade-123" {
						tr.Error = "expected upgradeId to be 'my-custom-upgrade-123'"
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
			ID:       "upgrade_quiesce_ack_has_required_keys",
			Category: conformance.CatUpgrade,
			Name:     "Upgrade quiesce ack response contains ack and upgradeId keys",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "upgrade.quiesce_ack", conformance.MustMarshal(map[string]any{"upgradeId": "check-keys"}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					if _, ok := resp["ack"]; !ok {
						tr.Error = "missing ack key"
						return nil
					}
					if _, ok := resp["upgradeId"]; !ok {
						tr.Error = "missing upgradeId key"
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
			ID:       "upgrade_quiesce_ack_multiple_versions",
			Category: conformance.CatUpgrade,
			Name:     "Upgrade quiesce ack handles multiple distinct upgradeIds correctly",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					ids := []string{"v1.0.0", "v2.0.0", "v3.0.0-rc1"}
					for _, id := range ids {
						env, err := h.CallRPC(ctx, "mock.fault", "upgrade.quiesce_ack", conformance.MustMarshal(map[string]any{"upgradeId": id}))
						if err != nil {
							tr.Error = "rpc error: " + err.Error()
							return nil
						}
						var resp map[string]any
						if err := json.Unmarshal(env.Payload, &resp); err != nil {
							tr.Error = "unmarshal: " + err.Error()
							return nil
						}
						ack, ok := resp["ack"].(bool)
						if !ok || !ack {
							tr.Error = "ack not true for " + id
							return nil
						}
						upgradeId, ok := resp["upgradeId"].(string)
						if !ok || upgradeId != id {
							tr.Error = "upgradeId mismatch for " + id
							return nil
						}
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
			ID:       "upgrade_quiesce_ack_type",
			Category: conformance.CatUpgrade,
			Name:     "Upgrade quiesce ack field is boolean type",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "upgrade.quiesce_ack", conformance.MustMarshal(map[string]any{"upgradeId": "type-check"}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					ack := resp["ack"]
					if ack == nil {
						tr.Error = "ack is nil"
						return nil
					}
					_, isBool := ack.(bool)
					if !isBool {
						tr.Error = "ack is not bool type"
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
			ID:       "upgrade_quiesce_ack_upgrade_id_type",
			Category: conformance.CatUpgrade,
			Name:     "Upgrade quiesce ack upgradeId field is string type",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "upgrade.quiesce_ack", conformance.MustMarshal(map[string]any{"upgradeId": "string-check"}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					upgradeId := resp["upgradeId"]
					if upgradeId == nil {
						tr.Error = "upgradeId is nil"
						return nil
					}
					_, isString := upgradeId.(string)
					if !isString {
						tr.Error = "upgradeId is not string type"
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
	conformance.RunMatrix(t, conformance.CatUpgrade, cases)
}

package matrices

import (
	"context"
	"encoding/json"
	"testing"

	conformance "github.com/u-ai/mock-conformance-go"
)

func TestSecretRevoke_Suite(t *testing.T) {
	cases := []conformance.TestCase{
		{
			ID:       "secret_revoke_returns_lease_status",
			Category: conformance.CatSecretRevoke,
			Name:     "Secret revoke and check returns leaseStatus granted",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "secret.revoke_and_check", conformance.MustMarshal(map[string]any{}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					leaseStatus, ok := resp["leaseStatus"].(string)
					if !ok || leaseStatus != "granted" {
						tr.Error = "expected leaseStatus to be 'granted'"
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
			ID:       "secret_revoke_returns_revoked_false",
			Category: conformance.CatSecretRevoke,
			Name:     "Secret revoke and check returns revoked false",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "secret.revoke_and_check", conformance.MustMarshal(map[string]any{}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					revoked, ok := resp["revoked"].(bool)
					if !ok || revoked {
						tr.Error = "expected revoked to be false"
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
			ID:       "secret_revoke_has_required_keys",
			Category: conformance.CatSecretRevoke,
			Name:     "Secret revoke response contains both leaseStatus and revoked keys",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "secret.revoke_and_check", conformance.MustMarshal(map[string]any{}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					if _, ok := resp["leaseStatus"]; !ok {
						tr.Error = "missing leaseStatus key"
						return nil
					}
					if _, ok := resp["revoked"]; !ok {
						tr.Error = "missing revoked key"
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
			ID:       "secret_revoke_sequential_consistent",
			Category: conformance.CatSecretRevoke,
			Name:     "Secret revoke returns consistent results across sequential calls",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					for i := 0; i < 3; i++ {
						env, err := h.CallRPC(ctx, "mock.fault", "secret.revoke_and_check", conformance.MustMarshal(map[string]any{}))
						if err != nil {
							tr.Error = "rpc error: " + err.Error()
							return nil
						}
						var resp map[string]any
						if err := json.Unmarshal(env.Payload, &resp); err != nil {
							tr.Error = "unmarshal: " + err.Error()
							return nil
						}
						leaseStatus, ok := resp["leaseStatus"].(string)
						if !ok || leaseStatus != "granted" {
							tr.Error = "inconsistent leaseStatus"
							return nil
						}
						revoked, ok := resp["revoked"].(bool)
						if !ok || revoked {
							tr.Error = "inconsistent revoked"
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
			ID:       "secret_revoke_lease_status_type",
			Category: conformance.CatSecretRevoke,
			Name:     "Secret revoke leaseStatus field is a string type",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "secret.revoke_and_check", conformance.MustMarshal(map[string]any{}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					leaseStatus := resp["leaseStatus"]
					if leaseStatus == nil {
						tr.Error = "leaseStatus is nil"
						return nil
					}
					_, isString := leaseStatus.(string)
					if !isString {
						tr.Error = "leaseStatus is not string type"
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
			ID:       "secret_revoke_revoked_type",
			Category: conformance.CatSecretRevoke,
			Name:     "Secret revoke revoked field is a boolean type",
			Run: func(ctx context.Context, host *conformance.PseudoHost) (conformance.TestResult, error) {
				var tr conformance.TestResult
				err := conformance.RunWithHost(ctx, conformance.FindPluginBinary(), func(ctx context.Context, h *conformance.PseudoHost) error {
					env, err := h.CallRPC(ctx, "mock.fault", "secret.revoke_and_check", conformance.MustMarshal(map[string]any{}))
					if err != nil {
						tr.Error = err.Error()
						return nil
					}
					var resp map[string]any
					if err := json.Unmarshal(env.Payload, &resp); err != nil {
						tr.Error = "unmarshal: " + err.Error()
						return nil
					}
					revoked := resp["revoked"]
					if revoked == nil {
						tr.Error = "revoked is nil"
						return nil
					}
					_, isBool := revoked.(bool)
					if !isBool {
						tr.Error = "revoked is not bool type"
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
	conformance.RunMatrix(t, conformance.CatSecretRevoke, cases)
}

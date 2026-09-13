package matrices

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/u-ai/mock-conformance-go"
)

func TestSmokeRealPseudoHost(t *testing.T) {
	for i := 0; i < 3; i++ {
		binary := conformance.FindPluginBinary()
		if binary == "" {
			t.Fatalf("iter %d: plugin binary not found", i)
		}
		h := conformance.NewPseudoHostForBinary(binary)
		ctx := t.Context()
		if err := h.Start(ctx); err != nil {
			t.Fatalf("iter %d start: %v", i, err)
		}
		targetPayload := conformance.MustMarshal(map[string]any{
			"extensionId": "com.mock-developer/mock-amitiax-game-plugin",
			"pluginId":    "mock-game-plugin-go",
			"runtimeId":   "mock.core",
		})
		resp, err := h.CallRPC(ctx, "mock.fault", "mock.fault.residue.get_counts", targetPayload)
		if err != nil {
			t.Fatalf("iter %d RPC err: %v", i, err)
		}
		var counts map[string]any
		if err := json.Unmarshal(resp.Payload, &counts); err != nil {
			t.Fatalf("iter %d unmarshal: %v", i, err)
		}
		if _, ok := counts["target"]; !ok {
			t.Fatalf("iter %d: missing target in residue counts", i)
		}
		if _, ok := counts["residue"]; !ok {
			t.Fatalf("iter %d: missing residue in residue counts", i)
		}
		t.Logf("iter %d RPC OK payload=%s", i, string(resp.Payload))
		_ = h.Kill()
		fmt.Printf("iter %d done\n", i)
	}
}

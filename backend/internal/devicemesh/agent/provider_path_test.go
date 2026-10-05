package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderPathRejectsCyclesIdentityMismatchAndExcessiveDepth(t *testing.T) {
	for _, test := range []struct {
		name   string
		core   string
		path   []string
		accept bool
	}{
		{"direct", "core-b", []string{"core-b"}, true},
		{"successor", "core-b", []string{"core-b", "core-c"}, true},
		{"cycle", "core-b", []string{"core-b", "core-a"}, false},
		{"repeated", "core-b", []string{"core-b", "core-c", "core-b"}, false},
		{"wrong-root", "core-b", []string{"core-c"}, false},
		{"wrong-identity", "core-c", []string{"core-c"}, false},
		{"missing-node", "core-b", []string{"core-b", ""}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/public/device-mesh/v1/pairing/status" || r.Header.Get("Authorization") != "" {
					t.Error("unexpected topology request or credential disclosure")
				}
				json.NewEncoder(w).Encode(map[string]any{"spaceId": test.core, "providerPath": test.path})
			}))
			defer server.Close()
			path, err := NewBootstrapClient().ProviderPath(t.Context(), server.URL, "core-b", "core-a")
			if (err == nil) != test.accept || test.accept && len(path) != len(test.path) {
				t.Fatalf("path=%v error=%v", path, err)
			}
		})
	}
}

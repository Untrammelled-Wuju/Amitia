package acquisition

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestRemoteCatalogSourceCanonicalPluginCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"schemaVersion":"amitia.plugin.catalog/v1",
			"id":"com.example.market",
			"source":{"id":"com.example.market","type":"community","url":"https://plugins.example.com/catalog.json"},
			"entries":[{
				"extensionId":"com.example/tools",
				"name":"Tools",
				"description":"Tools plugin",
				"version":"1.2.3",
				"trust":"official",
				"publisher":{"id":"com.example","displayName":"Example"},
				"package":{"uri":"https://plugins.example.com/tools.amitiax","sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","fileName":"tools.amitiax","sizeBytes":42},
				"signature":{"status":"verified","keyId":"key-1"},
				"providedCapabilities":["tools.example"],
				"categories":["developer"]
			}]
		}`))
	}))
	defer server.Close()

	source := NewRemoteCatalogSource(server.URL)
	items, err := source.Search(context.Background(), AcquisitionRequest{CapabilityID: capability.CapabilityID("tools.example")})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("candidates = %d, want 1", len(items))
	}
	item := items[0]
	if item.ExtensionID != "com.example/tools" || item.Version != "1.2.3" {
		t.Fatalf("unexpected candidate identity: %+v", item)
	}
	if item.Install.ExtensionPackage == nil || item.Install.ExtensionPackage.Hash != "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
		t.Fatalf("package descriptor lost: %+v", item.Install)
	}
	if item.Source.Registry != "com.example.market" || item.Source.Publisher != "com.example" || !item.Source.Verified {
		t.Fatalf("source provenance lost: %+v", item.Source)
	}
	if item.Trust.Level != TrustUnverified || item.Trust.SignatureVerified {
		t.Fatalf("catalog claims must not become package verification: %+v", item.Trust)
	}
	if item.Metadata["catalogId"] != "com.example.market" {
		t.Fatalf("catalog provenance missing: %+v", item.Metadata)
	}
}

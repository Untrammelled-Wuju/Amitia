package kernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type recordingVectorStore struct {
	namespace  string
	collection string
	points     []ExtensionVectorPoint
	query      string
}

func (s *recordingVectorStore) Upsert(_ context.Context, namespace, collection string, points []ExtensionVectorPoint) error {
	s.namespace = namespace
	s.collection = collection
	s.points = append([]ExtensionVectorPoint(nil), points...)
	return nil
}

func (s *recordingVectorStore) Search(_ context.Context, namespace, collection, query string, _ []float32, _ int, _ map[string]string) ([]ExtensionVectorSearchResult, error) {
	s.namespace = namespace
	s.collection = collection
	s.query = query
	return []ExtensionVectorSearchResult{{ID: "point-1", Score: 0.9}}, nil
}

func (s *recordingVectorStore) Delete(context.Context, string, string, []string) error {
	return nil
}

func TestVectorHostAPINamespacesByExtensionAndModule(t *testing.T) {
	store := &recordingVectorStore{}
	gateway := newConversationMessageTestGateway()
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{VectorStore: store}); err != nil {
		t.Fatal(err)
	}
	identity := runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.example/media"), ModuleID: "runtime"}
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "vector-upsert",
		RuntimeIdentity: identity,
		Method:          host_api.MethodVectorUpsert,
		Version:         1,
		Input:           json.RawMessage(`{"collection":"semantic","points":[{"id":"point-1","text":"开心","payload":{"kind":"image"}}]}`),
	})
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("upsert failed: %s %+v", result.Status, result.Error)
	}
	if store.namespace != "com.example/media/runtime" || store.collection != "semantic" || len(store.points) != 1 {
		t.Fatalf("unexpected vector upsert: %#v", store)
	}
	result = gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "vector-search",
		RuntimeIdentity: identity,
		Method:          host_api.MethodVectorSearch,
		Version:         1,
		Input:           json.RawMessage(`{"collection":"semantic","query":"开心","limit":5}`),
	})
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("search failed: %s %+v", result.Status, result.Error)
	}
	if store.query != "开心" {
		t.Fatalf("unexpected search query: %s", store.query)
	}
}

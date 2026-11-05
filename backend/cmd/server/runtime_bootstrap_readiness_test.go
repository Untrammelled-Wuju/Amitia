package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/u-ai/backend/config"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

type graphReadinessClientStub struct {
	err error
}

func (c graphReadinessClientStub) Health(context.Context) error {
	return c.err
}

func TestGraphStoreReadinessProbesSurreal(t *testing.T) {
	adapter := &graphStoreProviderAdapter{}
	if err := adapter.Ready(context.Background()); err == nil {
		t.Fatal("uninitialized graph store was reported ready")
	}

	probeErr := errors.New("surreal connection refused")
	adapter.graphSvc = &graphReadinessServiceStub{}
	adapter.client = graphReadinessClientStub{err: probeErr}
	err := adapter.Ready(context.Background())
	if !errors.Is(err, probeErr) {
		t.Fatalf("readiness error did not preserve probe cause: %v", err)
	}
}

type graphReadinessServiceStub struct{}

func (s *graphReadinessServiceStub) SyncNode(string, string, string, map[string]interface{}) error {
	return nil
}

func (s *graphReadinessServiceStub) SyncEdge(string, string, string, float64) error {
	return nil
}

func (s *graphReadinessServiceStub) DeleteNode(string) error {
	return nil
}

func (s *graphReadinessServiceStub) DeleteNodeForSpace(string, string) error {
	return nil
}

func (s *graphReadinessServiceStub) DeleteNodeIfOrphan(string) error {
	return nil
}

func (s *graphReadinessServiceStub) DeleteNodesByProperty(string, string, string) error {
	return nil
}

func (s *graphReadinessServiceStub) QueryNeighbors(string, int, string) (map[string]interface{}, error) {
	return nil, nil
}

func (s *graphReadinessServiceStub) FindPaths(string, string, int) ([]map[string]interface{}, error) {
	return nil, nil
}

func (s *graphReadinessServiceStub) FindPathsForSpace(string, string, int, string) ([]map[string]interface{}, error) {
	return nil, nil
}

func (s *graphReadinessServiceStub) DeleteOrphanNodes() error {
	return nil
}

func (s *graphReadinessServiceStub) GetStats(string) (map[string]interface{}, error) {
	return nil, nil
}

func (s *graphReadinessServiceStub) GetAllNodes(string) ([]map[string]interface{}, error) {
	return nil, nil
}

func (s *graphReadinessServiceStub) GetAllEdges(string) ([]map[string]interface{}, error) {
	return nil, nil
}

func (s *graphReadinessServiceStub) Name() string {
	return "stub"
}

func (s *graphReadinessServiceStub) Process(context.Context, string, []map[string]string, string) error {
	return nil
}

func TestVectorStoreReadinessProbesQdrant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/readyz" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host, portText, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	originalClient := qdrantDB.Client
	qdrantDB.Client = &qdrant.Client{}
	t.Cleanup(func() { qdrantDB.Client = originalClient })

	adapter := &vectorStoreProviderAdapter{
		qdrant: config.QdrantConfig{Host: host, Port: port},
	}
	if err := adapter.Ready(context.Background()); err != nil {
		t.Fatalf("live qdrant was rejected: %v", err)
	}

	server.Close()
	err = adapter.Ready(context.Background())
	if err == nil {
		t.Fatal("stopped qdrant was reported ready")
	}
}

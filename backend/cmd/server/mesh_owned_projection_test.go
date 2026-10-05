package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/graph"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

func TestOwnedProjectionLiveWriteAndDelete(t *testing.T) {
	if os.Getenv("AMITIA_TEST_PROJECTIONS") != "1" {
		t.Skip("live projection test disabled")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	p := setupMeshLocalDataPort(t)
	originalConfig := config.AppCfg
	config.InitConfig("../../config")
	t.Cleanup(func() { config.AppCfg = originalConfig })
	graphConfig := config.AppCfg.Providers.GraphStore.SurrealDB
	if password := os.Getenv("AMITIA_TEST_SURREAL_PASSWORD"); password != "" {
		graphConfig.Password = password
	}
	gc, err := graph.NewClient(graphConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer gc.Close()
	p.services.Graph = graph.NewSwitchableService(graph.NewService(gc))
	qc, err := qdrant.NewClient(&qdrant.Config{Host: "127.0.0.1", Port: 19179, SkipCompatibilityCheck: true})
	if err != nil {
		t.Fatal(err)
	}
	defer qc.Close()
	originalClient := qdrantDB.Client
	qdrantDB.Client = qc
	defer func() { qdrantDB.Client = originalClient }()
	id := uuid.NewString()
	kernelDB := p.services.KernelContainer.DeviceRegistry.Database()
	fpBytes := sha256.Sum256([]byte(id))
	fp := hex.EncodeToString(fpBytes[:])
	collection := "amitia_owned_v1_2_" + fp[:16]
	defer qc.DeleteCollection(context.Background(), collection)
	for kind, body := range map[string]any{
		"memory": map[string]any{"content": map[string]any{"value": "test"}},
		"vector": map[string]any{"content": map[string]any{"values": []float32{1, 0}, "modelFingerprint": fp}},
		"graph":  map[string]any{"content": map[string]any{"relation": "test", "target": "test"}},
	} {
		raw, _ := json.Marshal(body)
		source := id
		if kind == "memory" {
			source = ""
		}
		_, err := kernelDB.ExecContext(ctx, `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,source_id,revision,body,updated_at) VALUES(?,?,?,?,?,1,?,?)`, p.ownerID, kind, id, "one", source, raw, time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			t.Fatal(err)
		}
	}
	jobs, err := p.store.PendingProjections(ctx)
	if err != nil || len(jobs) != 2 {
		t.Fatalf("seed jobs: %d %v", len(jobs), err)
	}
	w := &meshProjectionWriter{port: p, collections: map[string]bool{}}
	for _, job := range jobs {
		if job.Resource.Kind == "graph" {
			defer p.services.Graph.(graph.OwnedProjectionPort).DeleteOwnedProjection(context.Background(), projectionPointID(job.Resource))
		}
	}
	if err := w.tick(ctx); err != nil {
		t.Fatal(err)
	}
	for _, job := range jobs {
		point := projectionPointID(job.Resource)
		if job.Resource.Kind == "vector" {
			rows, err := qc.Get(ctx, &qdrant.GetPoints{CollectionName: collection, Ids: []*qdrant.PointId{qdrant.NewID(point)}})
			if err != nil || len(rows) != 1 {
				t.Fatalf("vector acknowledgement: %d %v", len(rows), err)
			}
		} else {
			rows, err := surrealdb.Query[[]map[string]any](ctx, gc.DB(), "SELECT * FROM $record;", map[string]any{"record": models.NewRecordID("amitia_owned_graph_v1", point)})
			if err != nil || rows == nil || len(*rows) != 1 || len((*rows)[0].Result) != 1 {
				t.Fatalf("graph acknowledgement: %v", err)
			}
		}
	}
	query := coordination.DataQuery{Vector: []float32{1, 0}, VectorModel: fp}
	if resources, err := p.semanticSearch(ctx, "one", query); err != nil || len(resources) != 2 || resources[0].ID != id {
		t.Fatalf("projected semantic query: %d %v", len(resources), err)
	}
	if resources, err := p.semanticSearch(ctx, "two", query); err != nil || len(resources) != 0 {
		t.Fatalf("cross-role semantic query: %d %v", len(resources), err)
	}
	orphan := collection + "_interrupted"
	defer qc.DeleteCollection(context.Background(), orphan)
	if err := qc.CreateCollection(ctx, &qdrant.CreateCollection{CollectionName: orphan, VectorsConfig: &qdrant.VectorsConfig{Config: &qdrant.VectorsConfig_Params{Params: &qdrant.VectorParams{Size: 2, Distance: qdrant.Distance_Cosine}}}}); err != nil {
		t.Fatal(err)
	}
	var vectorJob coordination.ProjectionJob
	for _, job := range jobs {
		if job.Resource.Kind == "vector" {
			vectorJob = job
		}
	}
	point := projectionPointID(vectorJob.Resource)
	if err := p.store.TrackProjectionLocation(ctx, vectorJob.Resource, orphan); err != nil {
		t.Fatal(err)
	}
	if _, err := qc.Upsert(ctx, &qdrant.UpsertPoints{CollectionName: orphan, Wait: qdrant.PtrOf(true), Points: []*qdrant.PointStruct{{Id: qdrant.NewID(point), Vectors: qdrant.NewVectors(1, 0)}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := kernelDB.ExecContext(ctx, `UPDATE kernel_device_owned_resources SET revision=revision+1 WHERE owner_id=? AND kind='memory' AND resource_id=?`, p.ownerID, id); err != nil {
		t.Fatal(err)
	}
	if resources, err := p.semanticSearch(ctx, "one", query); err != nil || len(resources) != 2 || resources[0].Revision != 2 {
		t.Fatalf("stale projection fallback: %d %v", len(resources), err)
	}
	restartedWriter := &meshProjectionWriter{port: p, collections: map[string]bool{}}
	if err := restartedWriter.tick(ctx); err != nil {
		t.Fatal(err)
	}
	if rows, err := qc.Get(ctx, &qdrant.GetPoints{CollectionName: orphan, Ids: []*qdrant.PointId{qdrant.NewID(point)}}); err != nil || len(rows) != 0 {
		t.Fatalf("interrupted projection cleanup: %d %v", len(rows), err)
	}
	if _, err := kernelDB.ExecContext(ctx, `UPDATE kernel_device_owned_resources SET deleted=1,revision=revision+1 WHERE owner_id=? AND resource_id=?`, p.ownerID, id); err != nil {
		t.Fatal(err)
	}
	if err := w.tick(ctx); err != nil {
		t.Fatal(err)
	}
	if resources, err := p.semanticSearch(ctx, "one", query); err != nil || len(resources) != 0 {
		t.Fatalf("deleted source semantic query: %d %v", len(resources), err)
	}
	for _, job := range jobs {
		point := projectionPointID(job.Resource)
		if job.Resource.Kind == "vector" {
			rows, err := qc.Get(ctx, &qdrant.GetPoints{CollectionName: collection, Ids: []*qdrant.PointId{qdrant.NewID(point)}})
			if err != nil || len(rows) != 0 {
				t.Fatalf("vector cleanup: %d %v", len(rows), err)
			}
		} else {
			rows, err := surrealdb.Query[[]map[string]any](ctx, gc.DB(), "SELECT * FROM $record;", map[string]any{"record": models.NewRecordID("amitia_owned_graph_v1", point)})
			if err != nil || rows == nil || len(*rows) != 1 || len((*rows)[0].Result) != 0 {
				t.Fatal(fmt.Errorf("graph cleanup unconfirmed: %v", err))
			}
		}
	}
}

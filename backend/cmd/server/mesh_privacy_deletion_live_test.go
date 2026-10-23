package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
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

func TestPrivacyDeletionLiveProjectionErasureAndRecovery(t *testing.T) {
	if os.Getenv("AMITIA_TEST_DELETION_LIVE") != "1" {
		t.Skip("requires explicitly selected project database endpoints")
	}
	qport, err := strconv.Atoi(os.Getenv("AMITIA_TEST_DELETION_QDRANT_PORT"))
	if err != nil {
		t.Fatal("project Qdrant endpoint unavailable")
	}
	sport, err := strconv.Atoi(os.Getenv("AMITIA_TEST_DELETION_SURREAL_PORT"))
	if err != nil {
		t.Fatal("project SurrealDB endpoint unavailable")
	}
	namespace := "privacy_audit_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	gc, err := graph.NewClient(config.SurrealConfig{Host: "127.0.0.1", Port: sport, Namespace: namespace, Database: "isolated_test", Username: os.Getenv("AMITIA_TEST_DELETION_SURREAL_USER"), Password: os.Getenv("AMITIA_TEST_DELETION_SURREAL_PASSWORD")})
	if err != nil {
		t.Fatal(err)
	}
	defer gc.Close()
	defer func() {
		results, err := surrealdb.Query[any](context.Background(), gc.DB(), "REMOVE NAMESPACE "+namespace+";", nil)
		if err != nil {
			t.Error("isolated namespace cleanup failed", err)
		}
		if results != nil {
			for _, result := range *results {
				if result.Status != "OK" {
					t.Error("isolated namespace cleanup was not confirmed")
				}
			}
		}
	}()

	qc, err := qdrant.NewClient(&qdrant.Config{Host: "127.0.0.1", Port: qport, APIKey: os.Getenv("AMITIA_TEST_DELETION_QDRANT_KEY"), SkipCompatibilityCheck: true})
	if err != nil {
		t.Fatal(err)
	}
	defer qc.Close()
	previous := qdrantDB.Client
	qdrantDB.Client = qc
	defer func() { qdrantDB.Client = previous }()
	schemas := newThreeCoreSchemas(t)
	source := newThreeCoreFixture(t, "projection-delete-source", schemas)
	core := newThreeCoreFixture(t, "projection-delete-core", schemas)
	registerPrivacyFixture(core)
	pairThreeCoreFixtures(t, source, core)
	port := source.services.DeviceMesh.LocalDeviceDataPort.(*meshLocalDataPort)
	liveGraph := graph.NewSwitchableService(graph.NewService(gc))
	source.services.Graph = liveGraph
	id := uuid.NewString()
	digest := sha256.Sum256([]byte(id))
	fingerprint := hex.EncodeToString(digest[:])
	collection := "amitia_owned_v1_2_" + fingerprint[:16]
	defer qc.DeleteCollection(context.Background(), collection)
	seedOwnedChatManagement(t, port, "conversation", id, "one", `{"title":"isolated projection test"}`)
	raw, _ := json.Marshal(map[string]any{"id": id + "/message", "conversationId": id, "requestId": id, "content": "isolated secret", "role": "user"})
	seedOwnedChatManagement(t, port, "message", id+"/message", "one", string(raw))
	for kind, body := range map[string]any{
		"memory": map[string]any{"kind": "fact", "conversationId": id, "executionScope": map[string]any{"requestId": id, "roleId": "one"}, "content": map[string]any{"value": "isolated secret"}},
		"vector": map[string]any{"conversationId": id, "content": map[string]any{"values": []float32{1, 0}, "modelFingerprint": fingerprint}},
		"graph":  map[string]any{"conversationId": id, "content": map[string]any{"relation": "private", "target": "isolated"}},
	} {
		raw, _ := json.Marshal(body)
		parent := id
		if kind == "memory" {
			parent = ""
		}
		if _, err := port.services.KernelContainer.DeviceRegistry.Database().Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,source_id,revision,deleted,body,updated_at) VALUES(?,?,?,'one',?,1,0,?,?)`, port.ownerID, kind, id, parent, raw, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	writer := &meshProjectionWriter{port: port, collections: map[string]bool{}}
	initialJobs, err := port.store.PendingProjections(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(initialJobs) != 2 {
		t.Fatalf("expected 2 pending index writes, got %d", len(initialJobs))
	}
	for _, job := range initialJobs {
		usable := coordination.ProjectionUsable(job, time.Now())
		if !usable {
			t.Fatalf("test projection was not usable: %s", job.Resource.Kind)
		}
		var location string
		var err error
		if job.Resource.Kind == "vector" {
			err = qc.CreateCollection(t.Context(), &qdrant.CreateCollection{CollectionName: collection, VectorsConfig: &qdrant.VectorsConfig{Config: &qdrant.VectorsConfig_Params{Params: &qdrant.VectorParams{Size: 2, Distance: qdrant.Distance_Cosine}}}})
			if err == nil {
				_, err = qc.Upsert(t.Context(), &qdrant.UpsertPoints{CollectionName: collection, Wait: qdrant.PtrOf(true), Points: []*qdrant.PointStruct{{Id: qdrant.NewID(projectionPointID(job.Resource)), Vectors: &qdrant.Vectors{VectorsOptions: &qdrant.Vectors_Vector{Vector: &qdrant.Vector{Data: []float32{1, 0}}}}}}})
			}
			if err == nil {
				err = port.store.TrackProjectionLocation(t.Context(), job.Resource, collection)
			}
			location = collection
		} else {
			location, err = writer.graph(t.Context(), job, usable)
		}
		if err != nil {
			t.Fatalf("live index setup failed for %s: %v", job.Resource.Kind, err)
		}
		if err := port.store.FinishProjection(t.Context(), job, location, true); err != nil {
			t.Fatal(err)
		}
	}
	vectorResource, err := port.store.Get(t.Context(), "vector", id)
	if err != nil || vectorResource == nil {
		t.Fatal("test vector resource missing")
	}
	graphResource, err := port.store.Get(t.Context(), "graph", id)
	if err != nil || graphResource == nil {
		t.Fatal("test graph resource missing")
	}
	vectorPoint, graphPoint := projectionPointID(*vectorResource), projectionPointID(*graphResource)
	if vectorPoint == "" || graphPoint == "" {
		t.Fatal("projected resources missing")
	}
	points, err := qc.Get(t.Context(), &qdrant.GetPoints{CollectionName: collection, Ids: []*qdrant.PointId{qdrant.NewID(vectorPoint)}})
	if err != nil || len(points) != 1 {
		t.Fatalf("vector write verification: points=%d error=%v", len(points), err)
	}
	records, err := surrealdb.Query[[]map[string]any](t.Context(), gc.DB(), "SELECT * FROM $record;", map[string]any{"record": models.NewRecordID("amitia_owned_graph_v1", graphPoint)})
	if err != nil || records == nil || len((*records)[0].Result) != 1 {
		t.Fatal("graph was not actually written")
	}
	input := map[string]any{"targetId": id + "/message", "targetType": "message", "scope": "all"}
	preview := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, source, http.MethodPost, "/api/privacy/deletion/preview", input, 200))
	targets := preview["targets"].([]any)
	if len(targets) != 1 {
		t.Fatal(preview)
	}
	input["target"], input["requestId"] = targets[0], uuid.NewString()
	source.services.Graph = nil
	result := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, source, http.MethodPost, "/api/privacy/deletion/request", input, 200))
	if result["status"] == "completed" || result["retrievalBlocked"] != true {
		t.Fatalf("offline graph falsely completed: %v", result)
	}
	source.services.Graph = liveGraph
	core.request(t, source, http.MethodPost, "/api/privacy/deletion/cleanup", nil, 200)
	status := decodeThreeCoreMemoryResponse[map[string]any](t, core.request(t, source, http.MethodGet, "/api/privacy/deletion/status/"+result["id"].(string), nil, 200))
	if status["status"] != "completed" {
		t.Fatalf("recovery did not finish: %v", status)
	}
	points, err = qc.Get(t.Context(), &qdrant.GetPoints{CollectionName: collection, Ids: []*qdrant.PointId{qdrant.NewID(vectorPoint)}})
	if err != nil || len(points) != 0 {
		t.Fatal("real Qdrant point retained")
	}
	records, err = surrealdb.Query[[]map[string]any](t.Context(), gc.DB(), "SELECT * FROM $record;", map[string]any{"record": models.NewRecordID("amitia_owned_graph_v1", graphPoint)})
	if err != nil || records == nil || len((*records)[0].Result) != 0 {
		t.Fatal("real SurrealDB record retained")
	}
	t.Log("real TLS Owner deletion erased Qdrant and SurrealDB records after an unavailable graph provider recovered")
}

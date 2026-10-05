package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/graph"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

type meshProjectionWriter struct {
	port        *meshLocalDataPort
	collections map[string]bool
}

func projectionPointID(resource coordination.Resource) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(resource.OwnerID+"\x00"+resource.Kind+"\x00"+resource.ID)).String()
}

func (w *meshProjectionWriter) vector(ctx context.Context, job coordination.ProjectionJob, usable bool) (string, error) {
	client := qdrantDB.Client
	if client == nil {
		return job.Location, errors.New("向量投影服务暂不可用")
	}
	id := projectionPointID(job.Resource)
	locations, locationErr := w.port.store.ProjectionLocations(ctx, job.Resource)
	if locationErr != nil {
		return job.Location, locationErr
	}
	if job.Location != "" {
		locations = append(locations, job.Location)
	}
	remove := func(location string) error {
		if location == "" {
			return nil
		}
		exists, err := client.CollectionExists(ctx, location)
		if err != nil {
			return err
		}
		if exists {
			_, err = client.Delete(ctx, &qdrant.DeletePoints{CollectionName: location, Wait: qdrant.PtrOf(true), Points: qdrant.NewPointsSelector(qdrant.NewID(id))})
			if err != nil {
				return err
			}
		}
		return w.port.store.ForgetProjectionLocation(ctx, job.Resource, location)
	}
	clear := func() (string, error) {
		for _, location := range locations {
			if err := remove(location); err != nil {
				return job.Location, err
			}
		}
		return "", nil
	}
	if !usable {
		return clear()
	}
	var body struct {
		Content struct {
			Values []float32 `json:"values"`
			Model  string    `json:"modelFingerprint"`
		} `json:"content"`
	}
	if json.Unmarshal(job.Resource.Body, &body) != nil || len(body.Content.Model) != 64 || len(body.Content.Values) == 0 {
		return clear()
	}
	if _, err := hex.DecodeString(body.Content.Model); err != nil {
		return clear()
	}
	if err := coordination.ValidateQueryVector(coordination.DataQuery{Vector: body.Content.Values, VectorModel: body.Content.Model}); err != nil {
		return job.Location, err
	}
	name := fmt.Sprintf("amitia_owned_v1_%d_%s", len(body.Content.Values), body.Content.Model[:16])
	for _, location := range locations {
		if location != name {
			if err := remove(location); err != nil {
				return job.Location, err
			}
		}
	}
	if err := w.port.store.TrackProjectionLocation(ctx, job.Resource, name); err != nil {
		return job.Location, err
	}
	if !w.collections[name] {
		exists, err := client.CollectionExists(ctx, name)
		if err != nil {
			return name, err
		}
		if !exists {
			err = client.CreateCollection(ctx, &qdrant.CreateCollection{CollectionName: name, VectorsConfig: &qdrant.VectorsConfig{Config: &qdrant.VectorsConfig_Params{Params: &qdrant.VectorParams{Size: uint64(len(body.Content.Values)), Distance: qdrant.Distance_Cosine}}}})
			if err != nil {
				return name, err
			}
		}
		w.collections[name] = true
		if err := w.port.services.DB.WithContext(ctx).Exec(`INSERT INTO qdrant_collection_versions(collection_name,vector_dim,distance,created_at,schema_version) VALUES(?,?,'Cosine',?,'device_owned_v1') ON CONFLICT(collection_name) DO UPDATE SET schema_version=excluded.schema_version`, name, len(body.Content.Values), time.Now().UTC().Format(time.RFC3339Nano)).Error; err != nil {
			delete(w.collections, name)
			return name, err
		}
	}
	_, err := client.Upsert(ctx, &qdrant.UpsertPoints{CollectionName: name, Wait: qdrant.PtrOf(true), Points: []*qdrant.PointStruct{{Id: qdrant.NewID(id), Vectors: qdrant.NewVectors(body.Content.Values...), Payload: qdrant.NewValueMap(map[string]any{"ownerId": job.Resource.OwnerID, "roleId": job.Resource.RoleID, "resourceId": job.Resource.ID, "sourceId": job.Resource.SourceID, "revision": job.Resource.Revision, "sourceRevision": job.Source.Revision, "modelFingerprint": body.Content.Model})}}})
	if err != nil {
		delete(w.collections, name)
		return name, err
	}
	if err := w.port.store.ValidateProjection(ctx, job); err != nil {
		_ = remove(name)
		return name, err
	}
	return name, nil
}

func (w *meshProjectionWriter) graph(ctx context.Context, job coordination.ProjectionJob, usable bool) (string, error) {
	writer, ok := w.port.services.Graph.(graph.OwnedProjectionPort)
	if !ok {
		return job.Location, errors.New("图谱投影服务不可用")
	}
	id := projectionPointID(job.Resource)
	if !usable {
		return "", writer.DeleteOwnedProjection(ctx, id)
	}
	var body map[string]any
	if err := json.Unmarshal(job.Resource.Body, &body); err != nil {
		return job.Location, err
	}
	err := writer.WriteOwnedProjection(ctx, id, map[string]any{"ownerId": job.Resource.OwnerID, "roleId": job.Resource.RoleID, "resourceId": job.Resource.ID, "sourceId": job.Resource.SourceID, "revision": job.Resource.Revision, "sourceRevision": job.Source.Revision, "body": body})
	if err == nil {
		err = w.port.services.DB.WithContext(ctx).Exec(`INSERT INTO surreal_schema_versions(schema_version,entity_types,edge_types,created_at,projection_contract) VALUES('device_owned_v1','amitia_owned_graph_v1','',?,'owner-role-source-revision') ON CONFLICT(schema_version) DO UPDATE SET projection_contract=excluded.projection_contract`, time.Now().UTC().Format(time.RFC3339Nano)).Error
	}
	if err == nil {
		err = w.port.store.ValidateProjection(ctx, job)
		if err != nil {
			_ = writer.DeleteOwnedProjection(ctx, id)
		}
	}
	return "amitia_owned_graph_v1", err
}

func (w *meshProjectionWriter) tick(ctx context.Context) error {
	jobs, err := w.port.store.PendingProjections(ctx)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		current, cancel := context.WithTimeout(ctx, 10*time.Second)
		usable := coordination.ProjectionUsable(job, time.Now())
		var location string
		if job.Resource.Kind == "vector" {
			location, err = w.vector(current, job, usable)
		} else {
			location, err = w.graph(current, job, usable)
		}
		cancel()
		if finishErr := w.port.store.FinishProjection(ctx, job, location, err == nil); finishErr != nil && !errors.Is(finishErr, coordination.ErrResourceVersion) {
			return finishErr
		}
	}
	return nil
}

func runOwnedProjections(ctx context.Context, port *meshLocalDataPort) {
	w := &meshProjectionWriter{port: port, collections: map[string]bool{}}
	_ = w.tick(ctx)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = w.tick(ctx)
		}
	}
}

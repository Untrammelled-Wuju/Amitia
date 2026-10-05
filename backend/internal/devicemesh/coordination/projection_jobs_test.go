package coordination_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestProjectionJobsAreOwnerLocalAndSourceDeletionSchedulesCleanup(t *testing.T) {
	db, _ := setup(t)
	store := coordination.NewOwnershipStore(db, "a")
	commit := ownedCommit("projection-seed")
	commit.Mutations = []coordination.Mutation{
		{Kind: "memory", ID: "source", RoleID: "role", Body: json.RawMessage(`{"content":{"value":"tea"}}`)},
		{Kind: "vector", ID: "vector", RoleID: "role", SourceID: "source", Body: json.RawMessage(`{"content":{"values":[1,0],"modelFingerprint":"model"}}`)},
		{Kind: "graph", ID: "graph", RoleID: "role", SourceID: "source", Body: json.RawMessage(`{"content":{"relation":"likes","target":"tea"}}`)},
	}
	if _, err := store.Apply(t.Context(), commit); err != nil {
		t.Fatal(err)
	}
	jobs, err := store.PendingProjections(t.Context())
	if err != nil || len(jobs) != 2 {
		t.Fatalf("projection jobs=%v err=%v", jobs, err)
	}
	for _, job := range jobs {
		if !coordination.ProjectionUsable(job, time.Now()) || job.Resource.OwnerID != "a" || job.Source.ID != "source" {
			t.Fatalf("invalid source proof: %+v", job)
		}
		if err := store.FinishProjection(t.Context(), job, "source-owned-location", true); err != nil {
			t.Fatal(err)
		}
	}
	if rows, err := coordination.NewOwnershipStore(db, "core").PendingProjections(t.Context()); err != nil || len(rows) != 0 {
		t.Fatalf("device projection mirrored to Core: %v %v", rows, err)
	}
	commit.Scope.RequestID = "projection-delete"
	commit.Mutations = []coordination.Mutation{{Kind: "memory", ID: "source", RoleID: "role", ExpectedRevision: 1, Deleted: true, Body: json.RawMessage(`null`)}}
	if _, err := store.Apply(t.Context(), commit); err != nil {
		t.Fatal(err)
	}
	jobs, err = store.PendingProjections(t.Context())
	if err != nil || len(jobs) != 2 {
		t.Fatalf("cleanup jobs=%v err=%v", jobs, err)
	}
	for _, job := range jobs {
		if coordination.ProjectionUsable(job, time.Now()) || job.Location != "source-owned-location" {
			t.Fatalf("deleted source retained as usable: %+v", job)
		}
		if err := store.FinishProjection(t.Context(), job, job.Location, false); err != nil {
			t.Fatal(err)
		}
	}
	if jobs, err := store.PendingProjections(t.Context()); err != nil || len(jobs) != 0 {
		t.Fatalf("failed cleanup did not back off: %v %v", jobs, err)
	}
	if err := store.RebuildProjections(t.Context(), "role"); err != nil {
		t.Fatal(err)
	}
	jobs, err = store.PendingProjections(t.Context())
	if err != nil || len(jobs) != 2 {
		t.Fatalf("rebuild omitted cleanup: %v %v", jobs, err)
	}
	old := jobs[0]
	if _, err := db.ExecContext(t.Context(), `UPDATE kernel_device_owned_resources SET revision=revision+1 WHERE owner_id='a' AND kind=? AND resource_id=?`, old.Resource.Kind, old.Resource.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishProjection(t.Context(), old, "location", true); !errors.Is(err, coordination.ErrResourceVersion) {
		t.Fatalf("stale projection marked ready: %v", err)
	}
}

func TestProjectionStatusAndRebuildAreIsolatedByOwnerRoleAndKind(t *testing.T) {
	db, _ := setup(t)
	store := coordination.NewOwnershipStore(db, "a")
	for _, row := range []struct{ owner, role, kind string }{{"a", "role", "vector"}, {"a", "other-role", "graph"}, {"b", "role", "vector"}} {
		if _, err := db.ExecContext(t.Context(), `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,updated_at) VALUES(?,?,'shared',?,2,'{}','2026-10-04T00:00:00Z')`, row.owner, row.kind, row.role); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(t.Context(), `INSERT INTO kernel_device_owned_projections(owner_id,kind,resource_id,observed_revision,projected_revision,next_attempt_at) VALUES(?,?,'shared',2,2,?)`, row.owner, row.kind, time.Now().Add(time.Hour).Unix()); err != nil {
			t.Fatal(err)
		}
	}
	status, err := store.ProjectionStatus(t.Context(), "role")
	if err != nil || len(status.Layers) != 2 || status.Layers[0].Total != 1 || status.Layers[0].Current != 1 || status.Layers[1].Total != 0 {
		t.Fatalf("mixed projection status: %+v %v", status, err)
	}
	if err := store.RebuildProjections(t.Context(), "role"); err != nil {
		t.Fatal(err)
	}
	status, err = store.ProjectionStatus(t.Context(), "role")
	if err != nil || status.Layers[0].Pending != 1 || status.Layers[0].Current != 0 {
		t.Fatalf("rebuild status: %+v %v", status, err)
	}
	var unrelated int
	if err := db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM kernel_device_owned_projections WHERE NOT(owner_id='a' AND kind='vector') AND projected_revision=2`).Scan(&unrelated); err != nil || unrelated != 2 {
		t.Fatalf("rebuild reset another role or owner: %d %v", unrelated, err)
	}
	if _, err := store.ProjectionStatus(t.Context(), ""); !errors.Is(err, coordination.ErrRoleRequired) {
		t.Fatalf("roleless status accepted: %v", err)
	}
}

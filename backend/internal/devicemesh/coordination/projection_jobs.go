package coordination

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

const ProjectionSchema = `CREATE TABLE IF NOT EXISTS kernel_device_owned_projections (
 owner_id TEXT NOT NULL, kind TEXT NOT NULL, resource_id TEXT NOT NULL,
 observed_revision INTEGER NOT NULL DEFAULT 0, observed_source_revision INTEGER NOT NULL DEFAULT 0,
 projected_revision INTEGER NOT NULL DEFAULT 0, projected_source_revision INTEGER NOT NULL DEFAULT 0,
 location TEXT NOT NULL DEFAULT '', attempts INTEGER NOT NULL DEFAULT 0, next_attempt_at INTEGER NOT NULL DEFAULT 0,
 PRIMARY KEY(owner_id,kind,resource_id))`

const ProjectionLocationsSchema = `CREATE TABLE IF NOT EXISTS kernel_device_owned_projection_locations (
 owner_id TEXT NOT NULL, kind TEXT NOT NULL, resource_id TEXT NOT NULL, location TEXT NOT NULL,
 PRIMARY KEY(owner_id,kind,resource_id,location))`

func (s *OwnershipStore) TrackProjectionLocation(ctx context.Context, resource Resource, location string) error {
	if resource.OwnerID != s.ownerID || resource.Kind != "vector" || location == "" || len(location) > 256 {
		return ErrWrongOwner
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO kernel_device_owned_projection_locations(owner_id,kind,resource_id,location) VALUES(?,?,?,?) ON CONFLICT DO NOTHING`, s.ownerID, resource.Kind, resource.ID, location)
	return err
}

func (s *OwnershipStore) ProjectionLocations(ctx context.Context, resource Resource) ([]string, error) {
	if resource.OwnerID != s.ownerID {
		return nil, ErrWrongOwner
	}
	rows, err := s.db.QueryContext(ctx, `SELECT location FROM kernel_device_owned_projection_locations WHERE owner_id=? AND kind=? AND resource_id=?`, s.ownerID, resource.Kind, resource.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	locations := []string{}
	for rows.Next() {
		var location string
		if err := rows.Scan(&location); err != nil {
			return nil, err
		}
		locations = append(locations, location)
	}
	return locations, rows.Err()
}

func (s *OwnershipStore) ForgetProjectionLocation(ctx context.Context, resource Resource, location string) error {
	if resource.OwnerID != s.ownerID {
		return ErrWrongOwner
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM kernel_device_owned_projection_locations WHERE owner_id=? AND kind=? AND resource_id=? AND location=?`, s.ownerID, resource.Kind, resource.ID, location)
	return err
}

type ProjectionJob struct {
	Resource Resource
	Source   *Resource
	Location string
	Attempts int
}

func (s *OwnershipStore) PendingProjections(ctx context.Context) ([]ProjectionJob, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT v.kind,v.resource_id,v.role_id,v.source_id,v.revision,v.deleted,CAST(v.body AS BLOB),m.revision,m.deleted,CAST(m.body AS BLOB),COALESCE(p.location,''),COALESCE(p.attempts,0) FROM kernel_device_owned_resources v LEFT JOIN kernel_device_owned_resources m ON m.owner_id=v.owner_id AND m.kind='memory' AND m.resource_id=v.source_id AND m.role_id=v.role_id LEFT JOIN kernel_device_owned_projections p ON p.owner_id=v.owner_id AND p.kind=v.kind AND p.resource_id=v.resource_id WHERE v.owner_id=? AND v.kind IN ('vector','graph') AND (p.resource_id IS NULL OR p.observed_revision<>v.revision OR p.observed_source_revision<>COALESCE(m.revision,0) OR p.next_attempt_at<=?) ORDER BY COALESCE(p.next_attempt_at,0),v.kind,v.resource_id LIMIT 64`, s.ownerID, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := []ProjectionJob{}
	for rows.Next() {
		job := ProjectionJob{Resource: Resource{OwnerID: s.ownerID}}
		var sourceRevision sql.NullInt64
		var sourceDeleted sql.NullBool
		var sourceBody []byte
		if err := rows.Scan(&job.Resource.Kind, &job.Resource.ID, &job.Resource.RoleID, &job.Resource.SourceID, &job.Resource.Revision, &job.Resource.Deleted, &job.Resource.Body, &sourceRevision, &sourceDeleted, &sourceBody, &job.Location, &job.Attempts); err != nil {
			return nil, err
		}
		if sourceRevision.Valid {
			job.Source = &Resource{OwnerID: s.ownerID, Kind: "memory", ID: job.Resource.SourceID, RoleID: job.Resource.RoleID, Revision: sourceRevision.Int64, Deleted: sourceDeleted.Bool, Body: json.RawMessage(sourceBody)}
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *OwnershipStore) ValidateProjection(ctx context.Context, job ProjectionJob) error {
	current, err := s.Get(ctx, job.Resource.Kind, job.Resource.ID)
	if err != nil {
		return err
	}
	if current == nil || current.OwnerID != job.Resource.OwnerID || current.RoleID != job.Resource.RoleID || current.Revision != job.Resource.Revision || current.SourceID != job.Resource.SourceID {
		return ErrResourceVersion
	}
	source, err := s.Get(ctx, "memory", job.Resource.SourceID)
	if err != nil {
		return err
	}
	if (source == nil) != (job.Source == nil) || source != nil && (source.RoleID != job.Source.RoleID || source.Revision != job.Source.Revision) {
		return ErrResourceVersion
	}
	return nil
}

func ProjectionUsable(job ProjectionJob, now time.Time) bool {
	return !job.Resource.Deleted && job.Source != nil && !job.Source.Deleted && job.Source.OwnerID == job.Resource.OwnerID && job.Source.RoleID == job.Resource.RoleID && ResourceUsable(job.Source.Body, now) && ResourceUsable(job.Resource.Body, now)
}

func (s *OwnershipStore) FinishProjection(ctx context.Context, job ProjectionJob, location string, success bool) error {
	if err := s.ValidateProjection(ctx, job); err != nil {
		return err
	}
	if job.Resource.OwnerID != s.ownerID || len(location) > 256 {
		return ErrWrongOwner
	}
	sourceRevision := int64(0)
	if job.Source != nil {
		sourceRevision = job.Source.Revision
	}
	projectedRevision, projectedSource := int64(0), int64(0)
	attempts := min(max(job.Attempts, 0), 6) + 1
	delay := time.Duration(1<<min(attempts, 6)) * 5 * time.Second
	if success {
		projectedRevision, projectedSource = job.Resource.Revision, sourceRevision
		attempts = 0
		delay = time.Minute
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO kernel_device_owned_projections(owner_id,kind,resource_id,observed_revision,observed_source_revision,projected_revision,projected_source_revision,location,attempts,next_attempt_at) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(owner_id,kind,resource_id) DO UPDATE SET observed_revision=excluded.observed_revision,observed_source_revision=excluded.observed_source_revision,projected_revision=excluded.projected_revision,projected_source_revision=excluded.projected_source_revision,location=excluded.location,attempts=excluded.attempts,next_attempt_at=excluded.next_attempt_at`, s.ownerID, job.Resource.Kind, job.Resource.ID, job.Resource.Revision, sourceRevision, projectedRevision, projectedSource, location, attempts, time.Now().Add(delay).Unix())
	return err
}

func (s *OwnershipStore) RebuildProjections(ctx context.Context, role string) error {
	if role == "" {
		return errors.New("请指定索引所属角色")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE kernel_device_owned_projections SET projected_revision=0,projected_source_revision=0,next_attempt_at=0,attempts=0 WHERE owner_id=? AND EXISTS (SELECT 1 FROM kernel_device_owned_resources r WHERE r.owner_id=kernel_device_owned_projections.owner_id AND r.kind=kernel_device_owned_projections.kind AND r.resource_id=kernel_device_owned_projections.resource_id AND r.role_id=?)`, s.ownerID, role)
	return err
}

func (s *OwnershipStore) ProjectionsReady(ctx context.Context, role, kind string) (bool, error) {
	if role == "" || kind != "vector" && kind != "graph" {
		return false, ErrWrongOwner
	}
	var pending int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM kernel_device_owned_resources v LEFT JOIN kernel_device_owned_resources m ON m.owner_id=v.owner_id AND m.kind='memory' AND m.resource_id=v.source_id AND m.role_id=v.role_id LEFT JOIN kernel_device_owned_projections p ON p.owner_id=v.owner_id AND p.kind=v.kind AND p.resource_id=v.resource_id WHERE v.owner_id=? AND v.role_id=? AND v.kind=? AND (p.resource_id IS NULL OR p.projected_revision<>v.revision OR p.projected_source_revision<>COALESCE(m.revision,0))`, s.ownerID, role, kind).Scan(&pending)
	return pending == 0, err
}

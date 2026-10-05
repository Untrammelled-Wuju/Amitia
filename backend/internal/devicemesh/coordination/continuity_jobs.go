package coordination

import (
	"context"
	"encoding/json"
	"time"
)

const ContinuityJobsSchema = `CREATE TABLE IF NOT EXISTS kernel_device_continuity_locations (
	core_id TEXT NOT NULL, owner_id TEXT NOT NULL, resource_id TEXT NOT NULL,
	scope BLOB NOT NULL, revision INTEGER NOT NULL, next_check_at INTEGER NOT NULL,
	PRIMARY KEY(core_id,owner_id,resource_id))`

type ContinuityLocation struct {
	Scope    ExecutionScope
	ID       string
	Revision int64
}

func (s *Service) TrackContinuity(ctx context.Context, scope ExecutionScope, id string, revision int64) error {
	if id == "" || revision < 1 {
		return ErrResourceVersion
	}
	if err := s.Validate(ctx, scope); err != nil {
		return err
	}
	encoded, err := json.Marshal(scope)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO kernel_device_continuity_locations(core_id,owner_id,resource_id,scope,revision,next_check_at) VALUES(?,?,?,?,?,?) ON CONFLICT(core_id,owner_id,resource_id) DO UPDATE SET scope=excluded.scope,revision=excluded.revision,next_check_at=excluded.next_check_at WHERE excluded.revision>=kernel_device_continuity_locations.revision`, scope.CoreID, scope.ResourceOwnerID, id, encoded, revision, time.Now().Unix())
	return err
}

func (s *Service) PendingContinuity(ctx context.Context) ([]ContinuityLocation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT resource_id,scope,revision FROM kernel_device_continuity_locations WHERE next_check_at<=? ORDER BY next_check_at,resource_id LIMIT 16`, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ContinuityLocation{}
	for rows.Next() {
		var row ContinuityLocation
		var encoded []byte
		if err := rows.Scan(&row.ID, &encoded, &row.Revision); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(encoded, &row.Scope); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *Service) DeferContinuity(ctx context.Context, location ContinuityLocation, delay time.Duration) error {
	_, err := s.db.ExecContext(ctx, `UPDATE kernel_device_continuity_locations SET next_check_at=? WHERE core_id=? AND owner_id=? AND resource_id=? AND revision=?`, time.Now().Add(delay).Unix(), location.Scope.CoreID, location.Scope.ResourceOwnerID, location.ID, location.Revision)
	return err
}

func (s *Service) ForgetContinuity(ctx context.Context, location ContinuityLocation) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM kernel_device_continuity_locations WHERE core_id=? AND owner_id=? AND resource_id=? AND revision=?`, location.Scope.CoreID, location.Scope.ResourceOwnerID, location.ID, location.Revision)
	return err
}

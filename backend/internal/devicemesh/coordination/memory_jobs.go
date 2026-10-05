package coordination

import (
	"context"
	"time"
)

const MemoryJobsSchema = `CREATE TABLE IF NOT EXISTS kernel_device_memory_jobs (
	space_id TEXT NOT NULL, device_id TEXT NOT NULL, request_id TEXT NOT NULL,
	target_id TEXT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, next_attempt_at INTEGER NOT NULL,
	PRIMARY KEY(space_id,device_id,request_id))`

type MemoryJobLocation struct {
	SpaceID   string
	DeviceID  string
	RequestID string
	TargetID  string
	RequestLocation
	Attempts int
}

func (s *Service) PendingMemoryJobs(ctx context.Context) ([]MemoryJobLocation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT j.space_id,j.device_id,j.request_id,j.target_id,r.payload_hash,r.owner_id,r.conversation_id,r.role_id,r.mode_revision,r.provider_epoch,r.state,j.attempts FROM kernel_device_memory_jobs j JOIN kernel_device_business_requests r ON r.space_id=j.space_id AND r.device_id=j.device_id AND r.request_id=j.request_id WHERE r.state='completed' AND j.next_attempt_at<=? ORDER BY j.next_attempt_at,j.request_id LIMIT 16`, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]MemoryJobLocation, 0)
	for rows.Next() {
		var job MemoryJobLocation
		if err := rows.Scan(&job.SpaceID, &job.DeviceID, &job.RequestID, &job.TargetID, &job.Hash, &job.OwnerID, &job.ConversationID, &job.RoleID, &job.ModeRevision, &job.ProviderEpoch, &job.State, &job.Attempts); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Service) FinishMemoryJob(ctx context.Context, scope ExecutionScope) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE kernel_device_business_requests SET state='memory_saved' WHERE space_id=? AND device_id=? AND request_id=? AND owner_id=?`, scope.SpaceID, scope.InitiatorDeviceID, scope.RequestID, scope.ResourceOwnerID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM kernel_device_memory_jobs WHERE space_id=? AND device_id=? AND request_id=?`, scope.SpaceID, scope.InitiatorDeviceID, scope.RequestID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) RetryMemoryJob(ctx context.Context, job MemoryJobLocation, paused bool) error {
	if paused {
		_, err := s.db.ExecContext(ctx, `DELETE FROM kernel_device_memory_jobs WHERE space_id=? AND device_id=? AND request_id=?`, job.SpaceID, job.DeviceID, job.RequestID)
		return err
	}
	shift := job.Attempts
	if shift < 0 {
		shift = 0
	}
	if shift > 6 {
		shift = 6
	}
	delay := 5 * time.Second * time.Duration(1<<shift)
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	_, err := s.db.ExecContext(ctx, `UPDATE kernel_device_memory_jobs SET attempts=attempts+1,next_attempt_at=? WHERE space_id=? AND device_id=? AND request_id=? AND attempts=?`, time.Now().Add(delay).Unix(), job.SpaceID, job.DeviceID, job.RequestID, job.Attempts)
	return err
}

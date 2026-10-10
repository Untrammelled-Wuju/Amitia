package coordination

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

const RemoteAuthoritySchema = `CREATE TABLE IF NOT EXISTS kernel_device_remote_authority (
	call_id TEXT PRIMARY KEY, space_id TEXT NOT NULL, caller_id TEXT NOT NULL,
	target_id TEXT NOT NULL, session_id TEXT NOT NULL, generation INTEGER NOT NULL, created_at TEXT NOT NULL)`

const CancelledAuthoritySchema = `CREATE TABLE IF NOT EXISTS kernel_device_cancelled_authority (
	space_id TEXT NOT NULL, call_id TEXT NOT NULL, PRIMARY KEY(space_id,call_id))`

const SourceAuthoritySchema = `CREATE TABLE IF NOT EXISTS kernel_device_source_authority (
	space_id TEXT NOT NULL, device_id TEXT NOT NULL, closed_permission_revision INTEGER NOT NULL,
	PRIMARY KEY(space_id,device_id))`

var ErrAuthorityUnconfirmed = errors.New("设备存在尚未确认完成的远端数据请求，请恢复设备连接后再变更权限")

type AuthorityBarrier func(context.Context, string, string, int64, []string) error

type RemoteAuthoritySource struct{ SpaceID, DeviceID string }

func TrackCurrentRemoteAuthority(ctx context.Context, space, target, session string, generation int64) (string, func(context.Context) error, error) {
	scope, owned := FromContext(ctx)
	if !owned {
		return "", nil, nil
	}
	service, ok := ctx.Value(authorityServiceKey{}).(*Service)
	if !ok || scope.SpaceID != space || scope.TargetDeviceID != target {
		return "", nil, ErrWrongOwner
	}
	if err := ValidateCurrent(ctx); err != nil {
		return "", nil, err
	}
	id, err := service.TrackRemoteAuthority(ctx, scope, session, generation)
	if err != nil {
		return "", nil, err
	}
	return id, func(current context.Context) error { return service.ConfirmRemoteAuthority(current, id) }, nil
}

func (s *Service) PendingRemoteSources(ctx context.Context) ([]RemoteAuthoritySource, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT space_id,target_id FROM kernel_device_remote_authority ORDER BY space_id,target_id LIMIT 128`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sources := []RemoteAuthoritySource{}
	for rows.Next() {
		var source RemoteAuthoritySource
		if err := rows.Scan(&source.SpaceID, &source.DeviceID); err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func (s *Service) SetAuthorityBarrier(barrier AuthorityBarrier) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authorityBarrier = barrier
}

func (s *Service) TrackRemoteAuthority(ctx context.Context, scope ExecutionScope, session string, generation int64) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.Validate(ctx, scope); err != nil {
		return "", err
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM kernel_device_remote_authority WHERE space_id=? AND target_id=?`, scope.SpaceID, scope.TargetDeviceID).Scan(&count); err != nil {
		return "", err
	}
	if count >= 256 {
		return "", ErrAuthorityUnconfirmed
	}
	id := uuid.NewString()
	_, err := s.db.ExecContext(ctx, `INSERT INTO kernel_device_remote_authority(call_id,space_id,caller_id,target_id,session_id,generation,created_at) VALUES(?,?,?,?,?,?,?)`, id, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, session, generation, time.Now().UTC().Format(time.RFC3339Nano))
	return id, err
}

func (s *Service) ReconcileRemoteAuthority(ctx context.Context, space, target, session string, generation int64, reconcile func(context.Context, []string) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.QueryContext(ctx, `SELECT call_id FROM kernel_device_remote_authority WHERE space_id=? AND target_id=? AND (session_id<>? OR generation<?) ORDER BY call_id LIMIT 256`, space, target, session, generation)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(ids) == 0 {
		return err
	}
	if err := reconcile(ctx, ids); err != nil {
		return errors.Join(ErrAuthorityUnconfirmed, err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `DELETE FROM kernel_device_remote_authority WHERE call_id=?`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func CancelSourceAuthority(ctx context.Context, db *sql.DB, space string, ids []string) error {
	if space == "" || len(ids) == 0 || len(ids) > 256 {
		return ErrWrongOwner
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if id == "" || len(id) > 512 {
			return ErrWrongOwner
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_cancelled_authority(space_id,call_id) VALUES(?,?) ON CONFLICT DO NOTHING`, space, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func ValidateSourceCall(ctx context.Context, db *sql.DB, space, id string) error {
	return validateSourceCallQuery(ctx, db, space, id)
}

type sourceAuthorityQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func validateSourceCallQuery(ctx context.Context, db sourceAuthorityQuery, space, id string) error {
	var found int
	err := db.QueryRowContext(ctx, `SELECT 1 FROM kernel_device_cancelled_authority WHERE space_id=? AND call_id=?`, space, id).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return ErrScopeExpired
}

func (s *Service) ConfirmRemoteAuthority(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM kernel_device_remote_authority WHERE call_id=?`, id)
	return err
}

func (s *Service) fenceAuthorityLocked(ctx context.Context, space, device string, closedRevision int64) error {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT target_id FROM kernel_device_remote_authority WHERE space_id=? AND (caller_id=? OR target_id=?) ORDER BY target_id`, space, device, device)
	if err != nil {
		return err
	}
	sources := []string{}
	for rows.Next() {
		var source string
		if err := rows.Scan(&source); err != nil {
			rows.Close()
			return err
		}
		sources = append(sources, source)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return nil
	}
	if s.authorityBarrier == nil {
		return ErrAuthorityUnconfirmed
	}
	excluded, _ := ctx.Value(managementExecutionKey{}).(activeExecution)
	key := space + "\x00" + device
	for id, cancel := range s.active[key] {
		if excluded.service == s && excluded.id == id {
			continue
		}
		cancel(ErrScopeExpired)
		delete(s.active[key], id)
	}
	if err := s.authorityBarrier(ctx, space, device, closedRevision, sources); err != nil {
		return errors.Join(ErrAuthorityUnconfirmed, err)
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM kernel_device_remote_authority WHERE space_id=? AND (caller_id=? OR target_id=?)`, space, device, device)
	return err
}

func FenceSourceAuthority(ctx context.Context, db *sql.DB, space, device string, closedRevision int64) error {
	if space == "" || device == "" || closedRevision < 1 {
		return ErrWrongOwner
	}
	_, err := db.ExecContext(ctx, `INSERT INTO kernel_device_source_authority(space_id,device_id,closed_permission_revision) VALUES(?,?,?) ON CONFLICT(space_id,device_id) DO UPDATE SET closed_permission_revision=max(closed_permission_revision,excluded.closed_permission_revision)`, space, device, closedRevision)
	return err
}

func (s *Service) fenceAllRemoteAuthorityLocked(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT space_id,caller_id FROM kernel_device_remote_authority UNION SELECT DISTINCT space_id,target_id FROM kernel_device_remote_authority`)
	if err != nil {
		return err
	}
	type authority struct{ space, device string }
	actors := []authority{}
	for rows.Next() {
		var actor authority
		if err := rows.Scan(&actor.space, &actor.device); err != nil {
			rows.Close()
			return err
		}
		actors = append(actors, actor)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, actor := range actors {
		policy, err := s.Get(ctx, actor.space, actor.device)
		if err != nil {
			return err
		}
		if err := s.fenceAuthorityLocked(ctx, actor.space, actor.device, policy.PermissionRevision); err != nil {
			return err
		}
	}
	return nil
}

func ValidateSourceAuthority(ctx context.Context, db *sql.DB, scope ExecutionScope) error {
	return validateSourceAuthorityQuery(ctx, db, scope)
}

func validateSourceAuthorityQuery(ctx context.Context, db sourceAuthorityQuery, scope ExecutionScope) error {
	for _, actor := range []struct {
		id       string
		revision int64
	}{{scope.InitiatorDeviceID, scope.PermissionRevision}, {scope.TargetDeviceID, scope.TargetPermissionRevision}} {
		var closed int64
		err := db.QueryRowContext(ctx, `SELECT closed_permission_revision FROM kernel_device_source_authority WHERE space_id=? AND device_id=?`, scope.SpaceID, actor.id).Scan(&closed)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if actor.revision <= closed {
			return ErrScopeExpired
		}
	}
	return nil
}

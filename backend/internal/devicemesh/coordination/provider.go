package coordination

import (
	"context"
	"database/sql"
	"errors"
)

const ProviderSchema = `CREATE TABLE IF NOT EXISTS kernel_device_provider_binding (
	id INTEGER PRIMARY KEY CHECK(id=1), core_id TEXT NOT NULL, epoch INTEGER NOT NULL DEFAULT 1)`

var ErrProviderCycle = errors.New("服务提供者连接形成循环，拒绝绑定")

func (s *Service) Provider(ctx context.Context) (string, int64, error) {
	var core string
	var epoch int64
	err := s.db.QueryRowContext(ctx, `SELECT core_id,epoch FROM kernel_device_provider_binding WHERE id=1`).Scan(&core, &epoch)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, nil
	}
	return core, epoch, err
}

func ValidateProviderPath(localCore string, path []string) error {
	if localCore == "" || len(path) == 0 || len(path) > 16 {
		return ErrProviderCycle
	}
	seen := map[string]bool{localCore: true}
	for _, core := range path {
		if core == "" || seen[core] {
			return ErrProviderCycle
		}
		seen[core] = true
	}
	return nil
}

func (s *Service) BindProvider(ctx context.Context, coreID string) (int64, bool, error) {
	if coreID == "" {
		return 0, false, errors.New("服务提供者身份不能为空")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, _, err := s.Provider(ctx)
	if err != nil {
		return 0, false, err
	}
	if current != coreID {
		if err := s.fenceAllRemoteAuthorityLocked(ctx); err != nil {
			return 0, false, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_provider_binding(id,core_id,epoch) VALUES(1,'',1) ON CONFLICT DO NOTHING`); err != nil {
		return 0, false, err
	}
	var previous string
	var epoch int64
	if err := tx.QueryRowContext(ctx, `SELECT core_id,epoch FROM kernel_device_provider_binding WHERE id=1`).Scan(&previous, &epoch); err != nil {
		return 0, false, err
	}
	if previous == coreID {
		return epoch, false, nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE kernel_device_provider_binding SET core_id=?,epoch=epoch+1 WHERE id=1`, coreID); err != nil {
		return 0, false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE kernel_device_coordination SET administrator=0,permission_revision=permission_revision+1,provider_epoch=provider_epoch+1`); err != nil {
		return 0, false, err
	}
	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	for key, active := range s.active {
		for _, cancel := range active {
			cancel(ErrScopeExpired)
		}
		delete(s.active, key)
	}
	return epoch + 1, true, nil
}

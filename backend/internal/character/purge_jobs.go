package character

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

const (
	RolePurgeStatePending = "pending"
	RolePurgeStateDead    = "dead"
	rolePurgeMaxAttempts  = 8
)

type RoleDataResidueChecker interface {
	HasRoleDataResidue(ctx context.Context, roleID string) (bool, error)
}

var rolePurgeWake = make(chan struct{}, 1)

func NotifyRolePurgeQueue() {
	select {
	case rolePurgeWake <- struct{}{}:
	default:
	}
}

func RolePurgeWake() <-chan struct{} {
	return rolePurgeWake
}

func EnsureRolePurgeJobsSchema(db *gorm.DB) error {
	if db == nil {
		return errors.New("db is required")
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS role_data_purge_jobs (
	role_id TEXT PRIMARY KEY,
	state TEXT NOT NULL DEFAULT 'pending',
	attempts INTEGER NOT NULL DEFAULT 0,
	next_attempt_at TEXT NOT NULL DEFAULT '',
	last_error TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
)`).Error; err != nil {
		return fmt.Errorf("创建角色数据清理任务表失败: %w", err)
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_role_purge_due ON role_data_purge_jobs(state, next_attempt_at)`).Error; err != nil {
		return fmt.Errorf("创建角色数据清理任务索引失败: %w", err)
	}
	return nil
}

func enqueueRolePurgeTx(tx *gorm.DB, roleID string) error {
	if tx == nil {
		return errors.New("db is required")
	}
	if roleID == "" {
		return errors.New("角色标识为空，无法写入数据清理任务")
	}
	now := time.Now().UTC()
	if err := tx.Exec(`INSERT INTO role_data_purge_jobs(role_id, state, attempts, next_attempt_at, last_error, created_at, updated_at)
VALUES(?, ?, 0, '', '', ?, ?)
ON CONFLICT(role_id) DO UPDATE SET state = ?, next_attempt_at = '', updated_at = ?`,
		roleID, RolePurgeStatePending, now, now, RolePurgeStatePending, now).Error; err != nil {
		return fmt.Errorf("写入角色数据清理任务失败: %w", err)
	}
	return nil
}

func rolePurgeBackoff(attempt int) time.Duration {
	switch {
	case attempt <= 1:
		return time.Second
	case attempt == 2:
		return 5 * time.Second
	case attempt == 3:
		return 30 * time.Second
	case attempt == 4:
		return 2 * time.Minute
	case attempt == 5:
		return 5 * time.Minute
	case attempt == 6:
		return 15 * time.Minute
	default:
		return 60 * time.Minute
	}
}

func runRolePurgeJob(ctx context.Context, db *gorm.DB, purger RoleDataPurger, roleID string) error {
	if db == nil {
		return errors.New("db is required")
	}
	if purger == nil {
		return errors.New("角色关联数据清理入口不可用")
	}
	if err := purger.PurgeRoleData(ctx, roleID); err != nil {
		now := time.Now().UTC()
		var row struct {
			Attempts int
		}
		if scanErr := db.Raw(`SELECT attempts FROM role_data_purge_jobs WHERE role_id = ?`, roleID).Scan(&row).Error; scanErr != nil {
			log.Printf("[original-error] 读取角色数据清理任务失败 role=%s: %v", roleID, scanErr)
		}
		attempts := row.Attempts + 1
		state := RolePurgeStatePending
		if attempts >= rolePurgeMaxAttempts {
			state = RolePurgeStateDead
		}
		if updateErr := db.Exec(`UPDATE role_data_purge_jobs SET attempts = ?, state = ?, next_attempt_at = ?, last_error = ?, updated_at = ? WHERE role_id = ?`,
			attempts, state, now.Add(rolePurgeBackoff(attempts)).Format(time.RFC3339Nano), err.Error(), now, roleID).Error; updateErr != nil {
			log.Printf("[original-error] 记录角色数据清理失败状态失败 role=%s: %v", roleID, updateErr)
		}
		return err
	}
	if err := db.Exec(`DELETE FROM role_data_purge_jobs WHERE role_id = ?`, roleID).Error; err != nil {
		return fmt.Errorf("删除角色数据清理任务失败: %w", err)
	}
	return nil
}

func ProcessDueRolePurgeJobs(ctx context.Context, db *gorm.DB, purger RoleDataPurger, limit int) int {
	if db == nil || purger == nil {
		return 0
	}
	if limit <= 0 {
		limit = 20
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var rows []struct {
		RoleID string
	}
	if err := db.Raw(`SELECT role_id FROM role_data_purge_jobs WHERE state = ? AND (next_attempt_at = '' OR next_attempt_at <= ?) ORDER BY next_attempt_at ASC LIMIT ?`,
		RolePurgeStatePending, now, limit).Scan(&rows).Error; err != nil {
		log.Printf("[original-error] 扫描角色数据清理任务失败: %v", err)
		return 0
	}
	processed := 0
	for _, row := range rows {
		if ctx.Err() != nil {
			return processed
		}
		if err := runRolePurgeJob(ctx, db, purger, row.RoleID); err != nil {
			log.Printf("[original-error] 角色关联数据清理失败 role=%s: %v", row.RoleID, err)
			continue
		}
		processed++
	}
	return processed
}

func SweepOrphanRolePurgeJobs(ctx context.Context, db *gorm.DB, purger RoleDataPurger) int {
	checker, ok := purger.(RoleDataResidueChecker)
	if !ok {
		return 0
	}
	var rows []struct {
		ID string
	}
	if err := db.Raw(`SELECT id FROM characters WHERE deleted_at IS NOT NULL AND deleted_at <> ''`).Scan(&rows).Error; err != nil {
		log.Printf("[original-error] 扫描已删除角色失败: %v", err)
		return 0
	}
	now := time.Now().UTC()
	enqueued := 0
	for _, row := range rows {
		if ctx.Err() != nil {
			return enqueued
		}
		residue, err := checker.HasRoleDataResidue(ctx, row.ID)
		if err != nil {
			log.Printf("[original-error] 检查角色残留数据失败 role=%s: %v", row.ID, err)
			continue
		}
		if !residue {
			continue
		}
		if err := db.Exec(`INSERT INTO role_data_purge_jobs(role_id, state, attempts, next_attempt_at, last_error, created_at, updated_at)
VALUES(?, ?, 0, '', '', ?, ?)
ON CONFLICT(role_id) DO UPDATE SET state = ?, next_attempt_at = '', updated_at = ?`,
			row.ID, RolePurgeStatePending, now, now, RolePurgeStatePending, now).Error; err != nil {
			log.Printf("[original-error] 写入角色数据清理补偿任务失败 role=%s: %v", row.ID, err)
			continue
		}
		enqueued++
	}
	return enqueued
}

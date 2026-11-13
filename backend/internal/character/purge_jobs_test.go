package character

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type scriptedPurger struct {
	failures int
	calls    int
	reasons  []string
}

func (p *scriptedPurger) PurgeRoleData(context.Context, string) error {
	p.calls++
	if p.calls <= p.failures {
		return errors.New("database is locked")
	}
	return nil
}

func purgeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&Character{}); err != nil {
		t.Fatal(err)
	}
	if err := EnsureRolePurgeJobsSchema(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func purgeJobState(t *testing.T, db *gorm.DB, roleID string) (string, int) {
	t.Helper()
	var row struct {
		State    string
		Attempts int
	}
	if err := db.Raw(`SELECT state, attempts FROM role_data_purge_jobs WHERE role_id = ?`, roleID).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row.State, row.Attempts
}

func TestDeleteForSpaceQueuesPurgeWhenCleanupFails(t *testing.T) {
	db := purgeTestDB(t)
	repo := NewRepository(app.NewAppContext(db, nil))
	if err := repo.Create(&Character{ID: "char-purge", SpaceID: "local-space", Name: "Amitia", Identity: "伙伴", IsDefault: 1}); err != nil {
		t.Fatal(err)
	}
	purger := &scriptedPurger{failures: 1}

	service := &service{repo: repo, db: db}
	if err := service.DeleteForSpace("char-purge", "local-space"); err != nil {
		t.Fatalf("删除角色必须成功，清理失败转入后台: %v", err)
	}
	if purger.calls != 0 {
		t.Fatalf("删除请求不得同步执行关联数据清理: calls=%d", purger.calls)
	}
	if state, attempts := purgeJobState(t, db, "char-purge"); state != RolePurgeStatePending || attempts != 0 {
		t.Fatalf("删除提交后必须留下待清理任务: state=%s attempts=%d", state, attempts)
	}
	var deletedAt *string
	if err := db.Raw(`SELECT deleted_at FROM characters WHERE id = ?`, "char-purge").Scan(&deletedAt).Error; err != nil {
		t.Fatal(err)
	}
	if deletedAt == nil || *deletedAt == "" {
		t.Fatal("角色必须已经删除")
	}

	ProcessDueRolePurgeJobs(context.Background(), db, purger, 10)
	if state, attempts := purgeJobState(t, db, "char-purge"); state != RolePurgeStatePending || attempts != 1 {
		t.Fatalf("清理失败必须保留重试任务: state=%s attempts=%d", state, attempts)
	}
	if err := db.Exec(`UPDATE role_data_purge_jobs SET next_attempt_at = '' WHERE role_id = ?`, "char-purge").Error; err != nil {
		t.Fatal(err)
	}
	if err := runRolePurgeJob(context.Background(), db, purger, "char-purge"); err != nil {
		t.Fatalf("重试必须清理成功: %v", err)
	}
	var remaining int
	if err := db.Raw(`SELECT COUNT(*) FROM role_data_purge_jobs WHERE role_id = ?`, "char-purge").Scan(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("清理成功后任务必须删除: remaining=%d", remaining)
	}
}

func TestRolePurgeJobBacksOffAndStopsAfterMaxAttempts(t *testing.T) {
	db := purgeTestDB(t)
	if err := enqueueRolePurgeTx(db, "char-retry"); err != nil {
		t.Fatal(err)
	}
	purger := &scriptedPurger{failures: 100}
	if err := runRolePurgeJob(context.Background(), db, purger, "char-retry"); err == nil {
		t.Fatal("清理失败必须返回原始错误")
	}
	if state, attempts := purgeJobState(t, db, "char-retry"); state != RolePurgeStatePending || attempts != 1 {
		t.Fatalf("首次失败必须保持 pending: state=%s attempts=%d", state, attempts)
	}
	if processed := ProcessDueRolePurgeJobs(context.Background(), db, purger, 10); processed != 0 {
		t.Fatalf("退避期内的任务不得被重复执行: processed=%d", processed)
	}
	if err := db.Exec(`UPDATE role_data_purge_jobs SET attempts = 7, next_attempt_at = '' WHERE role_id = ?`, "char-retry").Error; err != nil {
		t.Fatal(err)
	}
	ProcessDueRolePurgeJobs(context.Background(), db, purger, 10)
	if state, attempts := purgeJobState(t, db, "char-retry"); state != RolePurgeStateDead || attempts != rolePurgeMaxAttempts {
		t.Fatalf("达到上限必须停止重试: state=%s attempts=%d", state, attempts)
	}
}

func TestRepeatedDeleteIsIdempotentAndRequeuesPurge(t *testing.T) {
	db := purgeTestDB(t)
	repo := NewRepository(app.NewAppContext(db, nil))
	if err := repo.Create(&Character{ID: "char-twice", SpaceID: "local-space", Name: "Amitia", Identity: "伙伴"}); err != nil {
		t.Fatal(err)
	}
	service := &service{repo: repo, db: db}
	if err := service.DeleteForSpace("char-twice", "local-space"); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteForSpace("char-twice", "local-space"); err != nil {
		t.Fatalf("重复删除必须幂等: %v", err)
	}
	if state, _ := purgeJobState(t, db, "char-twice"); state != RolePurgeStatePending {
		t.Fatalf("重复删除必须重新排队清理: %s", state)
	}
}

func TestNotifyRolePurgeQueueNeverBlocksAndCoalesces(t *testing.T) {
	for drained := false; !drained; {
		select {
		case <-RolePurgeWake():
		default:
			drained = true
		}
	}
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1024; i++ {
			NotifyRolePurgeQueue()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("NotifyRolePurgeQueue 不得阻塞删除请求")
	}
	select {
	case <-RolePurgeWake():
	default:
		t.Fatal("删除提交后必须唤醒清理 worker")
	}
	select {
	case <-RolePurgeWake():
		t.Fatal("重复唤醒必须合并为一次")
	default:
	}
}

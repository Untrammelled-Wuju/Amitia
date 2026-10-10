package executionjournal

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *Store) ValidateDatabaseWiring(db *sql.DB) error {
	if s == nil || db == nil || s.db != db {
		return errors.New("持久执行记录必须使用当前设备认证数据库")
	}
	return s.ValidateWiring()
}

func (s *Store) ValidateWiring() error {
	if s == nil || s.db == nil {
		return errors.New("持久执行记录缺少数据库")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, statement := range []string{
		"SELECT device_id,action_id,payload_hash,core_id,status,result,updated_at FROM kernel_device_execution_journal LIMIT 0",
		"SELECT device_id,workflow_id,node_id,token FROM kernel_device_execution_fences LIMIT 0",
	} {
		rows, err := s.db.QueryContext(ctx, statement)
		if err != nil {
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	return nil
}

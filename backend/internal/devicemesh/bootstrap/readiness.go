package bootstrap

import (
	"database/sql"
	"errors"
)

func (s *Service) ValidateDatabaseWiring(db *sql.DB) error {
	if s == nil || db == nil || s.db != db || s.repo == nil || s.repo.db != db || s.exchangeFn == nil || s.trustFn == nil || s.ticketTTL <= 0 || s.credTTL <= 0 {
		return errors.New("设备配对必须使用当前认证数据库及原事务内的凭据交换与信任提交")
	}
	return nil
}

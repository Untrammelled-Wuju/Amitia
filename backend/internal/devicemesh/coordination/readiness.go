package coordination

import (
	"database/sql"
	"errors"
)

func (s *Service) ValidateDatabaseWiring(db *sql.DB) error {
	if s == nil || db == nil || s.db != db || s.active == nil {
		return errors.New("协调策略必须使用当前设备认证数据库及执行取消管理")
	}
	return nil
}

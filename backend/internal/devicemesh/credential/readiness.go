package credential

import (
	"database/sql"
	"errors"
)

func (s *Service) ValidateDatabaseWiring(db *sql.DB) error {
	if s == nil || db == nil || s.repo == nil || s.repo.db != db || !s.requestProof || s.ttlSeconds <= 0 || s.clock == nil {
		return errors.New("设备凭据必须使用当前认证数据库并启用请求签名校验")
	}
	return nil
}

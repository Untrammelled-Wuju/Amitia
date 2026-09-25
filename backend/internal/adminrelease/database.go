// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package adminrelease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type mysqlMigration struct {
	Version    string
	Statements []string
}

var adminMigrations = []mysqlMigration{
	{
		Version: "202609240001",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS admin_users (
				id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
				username VARCHAR(64) NOT NULL,
				password_hash VARCHAR(512) NOT NULL,
				role VARCHAR(32) NOT NULL,
				active TINYINT(1) NOT NULL DEFAULT 1,
				created_at DATETIME(6) NOT NULL,
				updated_at DATETIME(6) NOT NULL,
				PRIMARY KEY (id),
				UNIQUE KEY uk_admin_users_username (username),
				KEY idx_admin_users_role (role)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			`CREATE TABLE IF NOT EXISTS admin_sessions (
				id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
				user_id BIGINT UNSIGNED NOT NULL,
				token_hash CHAR(64) NOT NULL,
				csrf_hash CHAR(64) NOT NULL,
				expires_at DATETIME(6) NOT NULL,
				last_seen_at DATETIME(6) NOT NULL,
				created_at DATETIME(6) NOT NULL,
				PRIMARY KEY (id),
				UNIQUE KEY uk_admin_sessions_token_hash (token_hash),
				KEY idx_admin_sessions_user_id (user_id),
				KEY idx_admin_sessions_expires_at (expires_at),
				CONSTRAINT fk_admin_sessions_user FOREIGN KEY (user_id) REFERENCES admin_users(id) ON DELETE CASCADE
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			`CREATE TABLE IF NOT EXISTS update_releases (
				id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
				product VARCHAR(16) NOT NULL,
				channel VARCHAR(16) NOT NULL,
				version VARCHAR(64) NOT NULL,
				version_code BIGINT NOT NULL DEFAULT 0,
				status VARCHAR(24) NOT NULL,
				release_name VARCHAR(160) NOT NULL DEFAULT '',
				release_notes LONGTEXT NOT NULL,
				mandatory TINYINT(1) NOT NULL DEFAULT 0,
				min_version_code BIGINT NOT NULL DEFAULT 0,
				rollout_percentage INT NOT NULL DEFAULT 100,
				abi VARCHAR(32) NOT NULL DEFAULT '',
				package_name VARCHAR(160) NOT NULL DEFAULT '',
				publish_path VARCHAR(512) NOT NULL DEFAULT '',
				created_by BIGINT UNSIGNED NOT NULL,
				published_at DATETIME(6) NULL,
				created_at DATETIME(6) NOT NULL,
				updated_at DATETIME(6) NOT NULL,
				PRIMARY KEY (id),
				UNIQUE KEY uk_update_release_version (product, channel, version),
				KEY idx_update_releases_status (status),
				KEY idx_update_releases_created_by (created_by),
				CONSTRAINT fk_update_releases_created_by FOREIGN KEY (created_by) REFERENCES admin_users(id)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			`CREATE TABLE IF NOT EXISTS update_release_artifacts (
				id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
				release_id BIGINT UNSIGNED NOT NULL,
				kind VARCHAR(32) NOT NULL,
				original_name VARCHAR(512) NOT NULL,
				stored_path VARCHAR(1024) NOT NULL,
				size BIGINT NOT NULL,
				sha256 CHAR(64) NOT NULL,
				sha512 CHAR(128) NOT NULL,
				sha512_base64 VARCHAR(256) NOT NULL,
				created_at DATETIME(6) NOT NULL,
				updated_at DATETIME(6) NOT NULL,
				PRIMARY KEY (id),
				UNIQUE KEY uk_update_release_artifact_kind (release_id, kind),
				CONSTRAINT fk_update_release_artifacts_release FOREIGN KEY (release_id) REFERENCES update_releases(id) ON DELETE CASCADE
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			`CREATE TABLE IF NOT EXISTS admin_audit_logs (
				id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
				actor_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
				actor_name VARCHAR(64) NOT NULL DEFAULT '',
				action VARCHAR(64) NOT NULL,
				product VARCHAR(16) NOT NULL DEFAULT '',
				channel VARCHAR(16) NOT NULL DEFAULT '',
				release_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
				target VARCHAR(512) NOT NULL DEFAULT '',
				result VARCHAR(24) NOT NULL,
				message LONGTEXT NOT NULL,
				created_at DATETIME(6) NOT NULL,
				PRIMARY KEY (id),
				KEY idx_admin_audit_logs_actor_id (actor_id),
				KEY idx_admin_audit_logs_action (action),
				KEY idx_admin_audit_logs_product (product),
				KEY idx_admin_audit_logs_channel (channel),
				KEY idx_admin_audit_logs_release_id (release_id),
				KEY idx_admin_audit_logs_result (result),
				KEY idx_admin_audit_logs_created_at (created_at)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		},
	},
}

func OpenDatabase(config MySQLConfig) (*gorm.DB, error) {
	location, err := time.LoadLocation(config.Loc)
	if err != nil {
		return nil, fmt.Errorf("加载 MySQL 时区失败: %w", err)
	}
	driverConfig := mysqldriver.Config{
		User:                 config.Username,
		Passwd:               config.Password,
		Net:                  "tcp",
		Addr:                 net.JoinHostPort(config.Host, fmt.Sprintf("%d", config.Port)),
		DBName:               config.Database,
		ParseTime:            true,
		Loc:                  location,
		AllowNativePasswords: true,
		CheckConnLiveness:    true,
		Timeout:              5 * time.Second,
		ReadTimeout:          30 * time.Second,
		WriteTimeout:         30 * time.Second,
		Params: map[string]string{
			"charset":   config.Charset,
			"collation": "utf8mb4_unicode_ci",
		},
	}
	db, err := gorm.Open(gormmysql.Open(driverConfig.FormatDSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("MySQL Ping 失败: %w", err)
	}
	if err := applyMigrations(db); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

func applyMigrations(db *gorm.DB) error {
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(32) NOT NULL,
		checksum CHAR(64) NOT NULL,
		applied_at DATETIME(6) NOT NULL,
		PRIMARY KEY (version)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`).Error; err != nil {
		return fmt.Errorf("创建迁移表失败: %w", err)
	}
	for _, migration := range adminMigrations {
		checksum := migrationChecksum(migration)
		var existing struct {
			Checksum string
		}
		err := db.Raw("SELECT checksum FROM schema_migrations WHERE version = ?", migration.Version).Scan(&existing).Error
		if err != nil {
			return fmt.Errorf("读取迁移状态失败: %w", err)
		}
		if existing.Checksum != "" {
			if !strings.EqualFold(existing.Checksum, checksum) {
				return fmt.Errorf("迁移 %s 校验值已变化", migration.Version)
			}
			continue
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			for _, statement := range migration.Statements {
				if err := tx.Exec(statement).Error; err != nil {
					return err
				}
			}
			return tx.Exec(
				"INSERT INTO schema_migrations (version, checksum, applied_at) VALUES (?, ?, ?)",
				migration.Version,
				checksum,
				time.Now(),
			).Error
		}); err != nil {
			return fmt.Errorf("执行迁移 %s 失败: %w", migration.Version, err)
		}
	}
	return nil
}

func migrationChecksum(migration mysqlMigration) string {
	sum := sha256.Sum256([]byte(migration.Version + "\n" + strings.Join(migration.Statements, "\n")))
	return hex.EncodeToString(sum[:])
}

func (s *Server) checkDatabase(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func (s *Server) withChannelLock(ctx context.Context, product, channel string, fn func() error) error {
	lockName := fmt.Sprintf("amitia:release:%s:%s", product, channel)
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	connection, err := sqlDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()
	var acquired int
	if err := connection.QueryRowContext(ctx, "SELECT GET_LOCK(?, 15)", lockName).Scan(&acquired); err != nil {
		return err
	}
	if acquired != 1 {
		return errors.New("发布通道正在被其他发布任务占用")
	}
	defer connection.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", lockName)
	return fn()
}

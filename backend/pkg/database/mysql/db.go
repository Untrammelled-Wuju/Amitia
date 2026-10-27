// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package mysql

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/glebarez/go-sqlite"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewSQLite(dataDir string) *gorm.DB {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}
	dbPath := filepath.Join(dataDir, "app.db")
	log.Printf("[DB] 连接 SQLite: %s", dbPath)
	writerDSN := dbPath + "?_txlock=immediate&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)"
	readerDSN := dbPath + "?_pragma=busy_timeout(10000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=query_only(1)"

	pool, err := newSQLiteRoutingPool(writerDSN, readerDSN, dbPath)
	if err != nil {
		log.Fatalf("SQLite 连接失败: %v", err)
	}

	db, err := gorm.Open(&sqlite.Dialector{DriverName: "sqlite", DSN: writerDSN, Conn: pool}, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("SQLite 连接失败: %v", err)
	}
	registerSQLiteRecoveryCallbacks(db, pool)

	fmt.Println("[DB] SQLite 连接成功")
	return db
}

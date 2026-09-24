// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package mysql

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

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

	writer, err := sql.Open("sqlite", writerDSN)
	if err != nil {
		log.Fatalf("SQLite 写连接创建失败: %v", err)
	}
	reader, err := sql.Open("sqlite", readerDSN)
	if err != nil {
		log.Fatalf("SQLite 读连接创建失败: %v", err)
	}
	writer.SetMaxIdleConns(1)
	writer.SetMaxOpenConns(1)
	writer.SetConnMaxLifetime(time.Hour)
	reader.SetMaxIdleConns(10)
	reader.SetMaxOpenConns(10)
	reader.SetConnMaxLifetime(time.Hour)
	pool := &sqliteRoutingPool{writer: writer, reader: reader}

	db, err := gorm.Open(&sqlite.Dialector{DriverName: "sqlite", DSN: writerDSN, Conn: pool}, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("SQLite 连接失败: %v", err)
	}

	if err := pool.Ping(); err != nil {
		log.Fatalf("SQLite Ping 失败: %v", err)
	}

	fmt.Println("[DB] SQLite 连接成功")
	return db
}

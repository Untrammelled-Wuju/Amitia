package mysql

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync/atomic"

	"gorm.io/gorm"
)

type sqliteRoutingPool struct {
	writer *sql.DB
	reader *sql.DB
}

type StorageStats struct {
	WriterMaxOpen   int    `json:"writerMaxOpen"`
	WriterOpen      int    `json:"writerOpen"`
	WriterInUse     int    `json:"writerInUse"`
	ReaderMaxOpen   int    `json:"readerMaxOpen"`
	ReaderOpen      int    `json:"readerOpen"`
	ReaderInUse     int    `json:"readerInUse"`
	WriteOperations uint64 `json:"writeOperations"`
	ReadOperations  uint64 `json:"readOperations"`
	BusyErrors      uint64 `json:"busyErrors"`
	WriterWaitCount int64  `json:"writerWaitCount"`
	ReaderWaitCount int64  `json:"readerWaitCount"`
}

var sqliteCounters struct {
	writeOperations atomic.Uint64
	readOperations  atomic.Uint64
	busyErrors      atomic.Uint64
}

func (p *sqliteRoutingPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return p.pool(query).PrepareContext(ctx, query)
}

func (p *sqliteRoutingPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	sqliteCounters.writeOperations.Add(1)
	result, err := p.writer.ExecContext(ctx, query, args...)
	recordSQLiteBusy(err)
	return result, err
}

func (p *sqliteRoutingPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if isWriteQuery(query) {
		sqliteCounters.writeOperations.Add(1)
	} else {
		sqliteCounters.readOperations.Add(1)
	}
	rows, err := p.pool(query).QueryContext(ctx, query, args...)
	recordSQLiteBusy(err)
	return rows, err
}

func (p *sqliteRoutingPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if isWriteQuery(query) {
		sqliteCounters.writeOperations.Add(1)
	} else {
		sqliteCounters.readOperations.Add(1)
	}
	return p.pool(query).QueryRowContext(ctx, query, args...)
}

func (p *sqliteRoutingPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	sqliteCounters.writeOperations.Add(1)
	tx, err := p.writer.BeginTx(ctx, opts)
	recordSQLiteBusy(err)
	return tx, err
}

func (p *sqliteRoutingPool) GetDBConn() (*sql.DB, error) {
	return p.writer, nil
}

func (p *sqliteRoutingPool) Ping() error {
	if err := p.writer.Ping(); err != nil {
		return err
	}
	return p.reader.Ping()
}

func (p *sqliteRoutingPool) stats() StorageStats {
	writerStats := p.writer.Stats()
	readerStats := p.reader.Stats()
	return StorageStats{
		WriterMaxOpen:   writerStats.MaxOpenConnections,
		WriterOpen:      writerStats.OpenConnections,
		WriterInUse:     writerStats.InUse,
		ReaderMaxOpen:   readerStats.MaxOpenConnections,
		ReaderOpen:      readerStats.OpenConnections,
		ReaderInUse:     readerStats.InUse,
		WriteOperations: sqliteCounters.writeOperations.Load(),
		ReadOperations:  sqliteCounters.readOperations.Load(),
		BusyErrors:      sqliteCounters.busyErrors.Load(),
		WriterWaitCount: writerStats.WaitCount,
		ReaderWaitCount: readerStats.WaitCount,
	}
}

func SQLiteStats(db *gorm.DB) StorageStats {
	if db == nil {
		return StorageStats{}
	}
	pool, ok := db.ConnPool.(*sqliteRoutingPool)
	if !ok || pool == nil {
		return StorageStats{}
	}
	return pool.stats()
}

func (p *sqliteRoutingPool) pool(query string) *sql.DB {
	if isWriteQuery(query) {
		return p.writer
	}
	return p.reader
}

func recordSQLiteBusy(err error) {
	if isSQLiteBusy(err) {
		sqliteCounters.busyErrors.Add(1)
	}
}

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	var sqliteErr interface {
		Code() int
	}
	if errors.As(err, &sqliteErr) {
		code := sqliteErr.Code()
		return code == 5 || code == 6 || code == 261 || code == 262 || code == 517 || code == 518
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") ||
		strings.Contains(message, "sqlite_busy") ||
		strings.Contains(message, "database table is locked")
}

func isWriteQuery(query string) bool {
	query = strings.TrimSpace(strings.ToLower(query))
	for strings.HasPrefix(query, "--") {
		if newline := strings.IndexByte(query, '\n'); newline >= 0 {
			query = strings.TrimSpace(query[newline+1:])
			continue
		}
		return false
	}
	if query == "" {
		return false
	}
	first := query
	if index := strings.IndexAny(first, " \t\r\n("); index >= 0 {
		first = first[:index]
	}
	switch first {
	case "select", "pragma", "explain", "values":
		if first == "pragma" && strings.Contains(query, "=") {
			return true
		}
		return false
	case "with":
		return containsWriteKeyword(query)
	}
	return true
}

func containsWriteKeyword(query string) bool {
	for _, keyword := range []string{" insert ", " update ", " delete ", " replace ", " create ", " alter ", " drop ", " vacuum "} {
		if strings.Contains(query, keyword) {
			return true
		}
	}
	return false
}

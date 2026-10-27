package mysql

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

const (
	sqliteBusyMaxAttempts  = 4
	sqliteBusyInitialDelay = 50 * time.Millisecond
	sqliteBusyMaxDelay     = 500 * time.Millisecond
)

type sqliteRoutingPool struct {
	writer      *sql.DB
	reader      *sql.DB
	recoverMu   sync.Mutex
	identityMu  sync.RWMutex
	writerDSN   string
	readerDSN   string
	dbPath      string
	walIdentity os.FileInfo
	shmIdentity os.FileInfo
	generation  atomic.Uint64
}

type StorageStats struct {
	WriterMaxOpen    int    `json:"writerMaxOpen"`
	WriterOpen       int    `json:"writerOpen"`
	WriterInUse      int    `json:"writerInUse"`
	ReaderMaxOpen    int    `json:"readerMaxOpen"`
	ReaderOpen       int    `json:"readerOpen"`
	ReaderInUse      int    `json:"readerInUse"`
	WriteOperations  uint64 `json:"writeOperations"`
	ReadOperations   uint64 `json:"readOperations"`
	BusyErrors       uint64 `json:"busyErrors"`
	WriterWaitCount  int64  `json:"writerWaitCount"`
	ReaderWaitCount  int64  `json:"readerWaitCount"`
	IOErrors         uint64 `json:"ioErrors"`
	PoolRecoveries   uint64 `json:"poolRecoveries"`
	RecoveryFailures uint64 `json:"recoveryFailures"`
	PoolGeneration   uint64 `json:"poolGeneration"`
	WALChanged       bool   `json:"walChanged"`
	SHMChanged       bool   `json:"shmChanged"`
	LastIOError      string `json:"lastIoError,omitempty"`
}

var sqliteCounters struct {
	writeOperations  atomic.Uint64
	readOperations   atomic.Uint64
	busyErrors       atomic.Uint64
	ioErrors         atomic.Uint64
	poolRecoveries   atomic.Uint64
	recoveryFailures atomic.Uint64
	lastIOError      atomic.Value
}

func (p *sqliteRoutingPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	stmt, err := p.pool(query).PrepareContext(ctx, query)
	recordSQLiteBusy(err)
	if err != nil && isSQLiteUnavailable(err) {
		p.recoverState(p.generation.Load())
	}
	return stmt, err
}

func (p *sqliteRoutingPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	sqliteCounters.writeOperations.Add(1)
	generation := p.generation.Load()
	var result sql.Result
	err := retrySQLiteBusy(ctx, func() error {
		var attemptErr error
		result, attemptErr = p.writer.ExecContext(ctx, query, args...)
		recordSQLiteBusy(attemptErr)
		return attemptErr
	})
	if isSQLiteUnavailable(err) {
		p.recoverState(generation)
	}
	return result, err
}

func (p *sqliteRoutingPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	generation := p.generation.Load()
	if isWriteQuery(query) {
		sqliteCounters.writeOperations.Add(1)
	} else {
		sqliteCounters.readOperations.Add(1)
	}
	rows, err := p.pool(query).QueryContext(ctx, query, args...)
	recordSQLiteBusy(err)
	if isSQLiteUnavailable(err) {
		p.recoverState(generation)
	}
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
	generation := p.generation.Load()
	var tx *sql.Tx
	err := retrySQLiteBusy(ctx, func() error {
		var attemptErr error
		tx, attemptErr = p.writer.BeginTx(ctx, opts)
		recordSQLiteBusy(attemptErr)
		return attemptErr
	})
	if isSQLiteUnavailable(err) {
		p.recoverState(generation)
	}
	return tx, err
}

func (p *sqliteRoutingPool) GetDBConn() (*sql.DB, error) {
	return p.writer, nil
}

func (p *sqliteRoutingPool) Ping() error {
	generation := p.generation.Load()
	if err := p.writer.Ping(); err != nil {
		recordSQLiteBusy(err)
		if isSQLiteUnavailable(err) {
			p.recoverState(generation)
		}
		return err
	}
	if err := p.reader.Ping(); err != nil {
		recordSQLiteBusy(err)
		if isSQLiteUnavailable(err) {
			p.recoverState(generation)
		}
		return err
	}
	return nil
}

func (p *sqliteRoutingPool) stats() StorageStats {
	writerStats := p.writer.Stats()
	readerStats := p.reader.Stats()
	p.identityMu.RLock()
	walIdentity := p.walIdentity
	shmIdentity := p.shmIdentity
	p.identityMu.RUnlock()
	return StorageStats{
		WriterMaxOpen:    writerStats.MaxOpenConnections,
		WriterOpen:       writerStats.OpenConnections,
		WriterInUse:      writerStats.InUse,
		ReaderMaxOpen:    readerStats.MaxOpenConnections,
		ReaderOpen:       readerStats.OpenConnections,
		ReaderInUse:      readerStats.InUse,
		WriteOperations:  sqliteCounters.writeOperations.Load(),
		ReadOperations:   sqliteCounters.readOperations.Load(),
		BusyErrors:       sqliteCounters.busyErrors.Load(),
		WriterWaitCount:  writerStats.WaitCount,
		ReaderWaitCount:  readerStats.WaitCount,
		IOErrors:         sqliteCounters.ioErrors.Load(),
		PoolRecoveries:   sqliteCounters.poolRecoveries.Load(),
		RecoveryFailures: sqliteCounters.recoveryFailures.Load(),
		PoolGeneration:   p.generation.Load(),
		WALChanged:       fileIdentityChanged(p.dbPath+"-wal", walIdentity),
		SHMChanged:       fileIdentityChanged(p.dbPath+"-shm", shmIdentity),
		LastIOError:      lastSQLiteIOError(),
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

func (p *sqliteRoutingPool) recoverState(generation uint64) {
	p.recoverMu.Lock()
	defer p.recoverMu.Unlock()
	if p.generation.Load() != generation {
		return
	}
	p.writer.SetMaxIdleConns(0)
	p.reader.SetMaxIdleConns(0)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.writer.PingContext(ctx); err != nil {
		sqliteCounters.recoveryFailures.Add(1)
		storeLastSQLiteIOError(err)
		p.restoreIdleLimits()
		return
	}
	if err := p.reader.PingContext(ctx); err != nil {
		sqliteCounters.recoveryFailures.Add(1)
		storeLastSQLiteIOError(err)
		p.restoreIdleLimits()
		return
	}
	p.restoreIdleLimits()
	p.generation.Add(1)
	p.captureFileIdentities()
	sqliteCounters.poolRecoveries.Add(1)
}

func newSQLiteRoutingPool(writerDSN, readerDSN, dbPath string) (*sqliteRoutingPool, error) {
	writer, reader, err := openSQLiteConnections(writerDSN, readerDSN)
	if err != nil {
		return nil, err
	}
	pool := &sqliteRoutingPool{
		writerDSN: writerDSN,
		readerDSN: readerDSN,
		dbPath:    dbPath,
		writer:    writer,
		reader:    reader,
	}
	pool.captureFileIdentities()
	return pool, nil
}

func (p *sqliteRoutingPool) restoreIdleLimits() {
	p.writer.SetMaxIdleConns(1)
	p.reader.SetMaxIdleConns(10)
}

func (p *sqliteRoutingPool) captureFileIdentities() {
	walIdentity, _ := os.Stat(p.dbPath + "-wal")
	shmIdentity, _ := os.Stat(p.dbPath + "-shm")
	p.identityMu.Lock()
	p.walIdentity = walIdentity
	p.shmIdentity = shmIdentity
	p.identityMu.Unlock()
}

func openSQLiteConnections(writerDSN, readerDSN string) (*sql.DB, *sql.DB, error) {
	writer, err := sql.Open("sqlite", writerDSN)
	if err != nil {
		return nil, nil, err
	}
	reader, err := sql.Open("sqlite", readerDSN)
	if err != nil {
		_ = writer.Close()
		return nil, nil, err
	}
	writer.SetMaxIdleConns(1)
	writer.SetMaxOpenConns(1)
	writer.SetConnMaxLifetime(time.Hour)
	reader.SetMaxIdleConns(10)
	reader.SetMaxOpenConns(10)
	reader.SetConnMaxLifetime(time.Hour)
	if err := writer.Ping(); err != nil {
		_ = writer.Close()
		_ = reader.Close()
		return nil, nil, err
	}
	if err := reader.Ping(); err != nil {
		_ = writer.Close()
		_ = reader.Close()
		return nil, nil, err
	}
	return writer, reader, nil
}

func recordSQLiteBusy(err error) {
	if isSQLiteBusy(err) {
		sqliteCounters.busyErrors.Add(1)
	}
	if isSQLiteUnavailable(err) {
		sqliteCounters.ioErrors.Add(1)
		storeLastSQLiteIOError(err)
	}
}

func retrySQLiteBusy(ctx context.Context, operation func() error) error {
	delay := sqliteBusyInitialDelay
	var err error
	for attempt := 0; attempt < sqliteBusyMaxAttempts; attempt++ {
		err = operation()
		if err == nil || !isSQLiteBusy(err) {
			return err
		}
		if attempt == sqliteBusyMaxAttempts-1 {
			break
		}
		wait := delay/2 + time.Duration(rand.IntN(int(delay/2)+1))
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		if delay < sqliteBusyMaxDelay {
			delay *= 2
			if delay > sqliteBusyMaxDelay {
				delay = sqliteBusyMaxDelay
			}
		}
	}
	return err
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

func isSQLiteUnavailable(err error) bool {
	if err == nil {
		return false
	}
	var sqliteErr interface {
		Code() int
	}
	if errors.As(err, &sqliteErr) {
		code := sqliteErr.Code()
		if code == 11 || code&0xff == 10 {
			return true
		}
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "disk i/o error") ||
		strings.Contains(message, "database disk image is malformed") ||
		strings.Contains(message, "database disk image is corrupt") ||
		strings.Contains(message, "failed to read page")
}

func storeLastSQLiteIOError(err error) {
	if err != nil {
		sqliteCounters.lastIOError.Store(err.Error())
	}
}

func lastSQLiteIOError() string {
	value := sqliteCounters.lastIOError.Load()
	if value == nil {
		return ""
	}
	text, _ := value.(string)
	return text
}

func fileIdentityChanged(path string, previous os.FileInfo) bool {
	if previous == nil {
		return false
	}
	current, err := os.Stat(path)
	if err != nil {
		return true
	}
	return !os.SameFile(previous, current)
}

func registerSQLiteRecoveryCallbacks(db *gorm.DB, pool *sqliteRoutingPool) {
	if db == nil || pool == nil {
		return
	}
	callback := func(tx *gorm.DB) {
		if tx == nil {
			return
		}
		scheduleSQLiteRecovery(pool, tx.Error)
	}
	db.Callback().Query().After("gorm:query").Register("amitia:sqlite:recovery:query", callback)
	db.Callback().Create().After("gorm:create").Register("amitia:sqlite:recovery:create", callback)
	db.Callback().Update().After("gorm:update").Register("amitia:sqlite:recovery:update", callback)
	db.Callback().Delete().After("gorm:delete").Register("amitia:sqlite:recovery:delete", callback)
	db.Callback().Row().After("gorm:row").Register("amitia:sqlite:recovery:row", callback)
}

func scheduleSQLiteRecovery(pool *sqliteRoutingPool, err error) {
	if pool == nil || !isSQLiteUnavailable(err) {
		return
	}
	recordSQLiteBusy(err)
	generation := pool.generation.Load()
	go pool.recoverState(generation)
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

package kernel

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
)

const migrationSQLExecuteTimeout = 10 * time.Second
const migrationSQLQueryTimeout = 5 * time.Second
const maxMigrationQueryRows = 1000

type MigrationSandboxDeps struct {
	DB          *sql.DB
	ExtensionID string
}

func setupMigrationSandboxRoutes(gateway *host_api.DefaultGateway, deps MigrationSandboxDeps) error {
	if deps.DB == nil {
		return fmt.Errorf("kernel: migration sandbox requires database")
	}

	executeRoute := host_api.Route{
		Method:          host_api.MethodMigrationSQLExecute,
		Version:         1,
		Permission:      host_api.RoutePermissionForMethod(host_api.MethodMigrationSQLExecute),
		ScopePolicy:     host_api.RouteScopeForMethod(host_api.MethodMigrationSQLExecute),
		RiskLevel:       host_api.RiskHigh,
		SideEffectLevel: host_api.SideEffectWrite,
		Timeout:         migrationSQLExecuteTimeout,
		Handler:         createMigrationSQLExecuteHandler(deps.DB),
	}
	if err := gateway.RegisterRoute(executeRoute); err != nil {
		return fmt.Errorf("kernel: register migration.sql.execute: %w", err)
	}

	queryRoute := host_api.Route{
		Method:          host_api.MethodMigrationSQLQuery,
		Version:         1,
		Permission:      host_api.RoutePermissionForMethod(host_api.MethodMigrationSQLQuery),
		ScopePolicy:     host_api.RouteScopeForMethod(host_api.MethodMigrationSQLQuery),
		RiskLevel:       host_api.RiskLow,
		SideEffectLevel: host_api.SideEffectReadOnly,
		Timeout:         migrationSQLQueryTimeout,
		Handler:         createMigrationSQLQueryHandler(deps.DB),
	}
	if err := gateway.RegisterRoute(queryRoute); err != nil {
		return fmt.Errorf("kernel: register migration.sql.query: %w", err)
	}

	return nil
}

func createMigrationSQLExecuteHandler(db *sql.DB) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		extID := string(req.RuntimeIdentity.ExtensionID)
		if extID == "" {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error: &host_api.Error{
					Code:    host_api.ErrorCodeInputInvalid,
					Message: "migration.sql.execute: extension id required",
				},
			}, nil
		}

		var p struct {
			SQL string `json:"sql"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
			}, nil
		}
		if strings.TrimSpace(p.SQL) == "" {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "migration.sql.execute: sql is required"},
			}, nil
		}

		parsedStmts, validateErr := migration.ValidateRawStatements(p.SQL, extID)
		if validateErr != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error: &host_api.Error{
					Code:    PackageErrCodeMigrationSandboxViolation,
					Message: fmt.Sprintf("migration sandbox violation: %v", validateErr),
				},
			}, nil
		}

		conn, err := db.Conn(ctx)
		if err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: err.Error()},
			}, nil
		}
		committed := false
		defer func() {
			if !committed {
				_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
			}
			_ = conn.Close()
		}()

		beforeHash, hashErr := computeMigrationSandboxSchemaHash(ctx, conn, extID)
		if hashErr != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: fmt.Sprintf("compute before hash: %v", hashErr)},
			}, nil
		}

		if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: fmt.Sprintf("begin immediate: %v", err)},
			}, nil
		}

		var totalRowsAffected int64
		var statementsExecuted int
		for _, stmt := range parsedStmts {
			result, execErr := conn.ExecContext(ctx, stmt.Raw)
			if execErr != nil {
				return host_api.CallResult{
					Status: host_api.StatusFailed,
					Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: fmt.Sprintf("execute sql: %v", execErr)},
				}, nil
			}
			if rows, rowsErr := result.RowsAffected(); rowsErr == nil {
				totalRowsAffected += rows
			}
			statementsExecuted++
		}

		afterHash, hashErr := computeMigrationSandboxSchemaHash(ctx, conn, extID)
		if hashErr != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: fmt.Sprintf("compute after hash: %v", hashErr)},
			}, nil
		}

		if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: fmt.Sprintf("commit: %v", err)},
			}, nil
		}
		committed = true

		output, _ := json.Marshal(map[string]any{
			"ok":                  true,
			"statementsExecuted":  statementsExecuted,
			"rowsAffected":        totalRowsAffected,
			"beforeSchemaHash":    beforeHash,
			"afterSchemaHash":     afterHash,
			"extensionId":         extID,
		})
		return host_api.CallResult{
			Status: host_api.StatusSuccess,
			Output: output,
		}, nil
	}
}

func createMigrationSQLQueryHandler(db *sql.DB) host_api.Handler {
	return func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
		extID := string(req.RuntimeIdentity.ExtensionID)
		if extID == "" {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error: &host_api.Error{
					Code:    host_api.ErrorCodeInputInvalid,
					Message: "migration.sql.query: extension id required",
				},
			}, nil
		}

		var p struct {
			SQL    string `json:"sql"`
			Limit  int    `json:"limit"`
		}
		if err := json.Unmarshal(req.Input, &p); err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: err.Error()},
			}, nil
		}
		if strings.TrimSpace(p.SQL) == "" {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInputInvalid, Message: "migration.sql.query: sql is required"},
			}, nil
		}
		if p.Limit <= 0 || p.Limit > maxMigrationQueryRows {
			p.Limit = maxMigrationQueryRows
		}

		lowerSQL := strings.ToLower(strings.TrimSpace(p.SQL))
		if !strings.HasPrefix(lowerSQL, "select") && !strings.HasPrefix(lowerSQL, "with") {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error: &host_api.Error{
					Code:    PackageErrCodeMigrationSandboxViolation,
					Message: "migration.sql.query: only SELECT/WITH statements allowed",
				},
			}, nil
		}

		parsedStmts, validateErr := migration.ValidateRawStatements(p.SQL, extID)
		if validateErr != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error: &host_api.Error{
					Code:    PackageErrCodeMigrationSandboxViolation,
					Message: fmt.Sprintf("migration sandbox violation: %v", validateErr),
				},
			}, nil
		}
		if len(parsedStmts) != 1 {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error: &host_api.Error{
					Code:    host_api.ErrorCodeInputInvalid,
					Message: "migration.sql.query: exactly one statement required",
				},
			}, nil
		}

		rows, err := db.QueryContext(ctx, parsedStmts[0].Raw)
		if err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
			}, nil
		}
		defer rows.Close()

		columns, err := rows.Columns()
		if err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
			}, nil
		}

		var results []map[string]interface{}
		for rows.Next() {
			if len(results) >= p.Limit {
				break
			}
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}
			if err := rows.Scan(valuePtrs...); err != nil {
				return host_api.CallResult{
					Status: host_api.StatusFailed,
					Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
				}, nil
			}
			record := make(map[string]interface{}, len(columns))
			for i, col := range columns {
				val := values[i]
				b, ok := val.([]byte)
				if ok {
					record[col] = string(b)
				} else {
					record[col] = val
				}
			}
			results = append(results, record)
		}
		if err := rows.Err(); err != nil {
			return host_api.CallResult{
				Status: host_api.StatusFailed,
				Error:  &host_api.Error{Code: host_api.ErrorCodeInternal, Message: err.Error()},
			}, nil
		}

		output, _ := json.Marshal(map[string]any{
			"rows":   results,
			"total":  len(results),
			"limit":  p.Limit,
			"columns": columns,
		})
		return host_api.CallResult{
			Status: host_api.StatusSuccess,
			Output: output,
		}, nil
	}
}

func computeMigrationSandboxSchemaHash(ctx context.Context, executor migrationQueryExecutor, extensionID string) (string, error) {
	nsPrefix := migration.ExtensionNamespacePrefix(extensionID) + "%"
	rows, err := executor.QueryContext(ctx, "SELECT name, sql FROM sqlite_master WHERE type='table' AND name LIKE ? ORDER BY name", nsPrefix)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	h := sha256.New()
	for rows.Next() {
		var name, sqlText string
		if err := rows.Scan(&name, &sqlText); err != nil {
			return "", err
		}
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write([]byte(sqlText))
		h.Write([]byte{0})
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	sum := h.Sum(nil)
	return "sha256:" + hex.EncodeToString(sum), nil
}

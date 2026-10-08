package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/configwrite"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/securityaudit"
	"gorm.io/gorm"
)

func (s *service) AdministratorActionContext(ctx context.Context, operation, id string) (map[string]interface{}, error) {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	switch operation {
	case "audit-clear", "safety-clear", "safety-handle", "usage-clear":
		var auditSpaceID string
		if operation == "audit-clear" {
			var err error
			auditSpaceID, err = auditSpace(ctx)
			if err != nil {
				return nil, err
			}
		}
		var result map[string]interface{}
		err := configwrite.Transaction(s.db.WithContext(ctx), func(tx *gorm.DB) error {
			var write *gorm.DB
			switch operation {
			case "audit-clear":
				write = tx.Where("space_id = ?", auditSpaceID).Delete(&securityaudit.AuditEvent{})
				result = map[string]interface{}{"deleted": write.RowsAffected}
			case "safety-clear":
				write = tx.Exec("DELETE FROM safety_events")
				result = map[string]interface{}{"deleted": true, "removedRecords": write.RowsAffected}
			case "safety-handle":
				write = tx.Table("safety_events").Where("id=?", id).Update("handled", 1)
				result = map[string]interface{}{"handled": true, "id": id}
				if write.Error == nil && write.RowsAffected == 0 {
					return gorm.ErrRecordNotFound
				}
			case "usage-clear":
				write = tx.Table("messages").Where("tokens>0").Update("tokens", 0)
				result = map[string]interface{}{"cleared": true, "updatedRecords": write.RowsAffected}
			}
			return write.Error
		})
		if err != nil {
			return nil, err
		}
		return result, nil
	case "logs-delete", "model-errors-delete", "logs-rotate", "temp-clean":
		return s.administratorLogAction(ctx, operation)
	case "release-export", "diagnostic-export":
		var report map[string]interface{}
		prefix := "release_check"
		if operation == "release-export" {
			report = s.GetReleaseCheckHistory()
		} else {
			prefix = "diagnostic"
			report = map[string]interface{}{"health": s.Health(), "diagnosis": s.MaintenanceDiagnose(), "runtime": s.GetRuntimeStatus(), "exportedAt": time.Now().Format(time.DateTime)}
		}
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return nil, err
		}
		name := fmt.Sprintf("%s_%d.json", prefix, time.Now().UnixNano())
		if err := replaceAdministratorLog(ctx, filepath.Join(s.dataDir, name), data, 0600); err != nil {
			return nil, err
		}
		return map[string]interface{}{"exported": true, "file": name}, nil
	case "diagnose", "diagnostics-run", "check-now":
		var result map[string]interface{}
		err := coordination.CommitCurrent(ctx, func() error {
			switch operation {
			case "diagnose":
				result = s.MaintenanceDiagnose()
			case "diagnostics-run":
				result = s.RunDiagnostics()
			case "check-now":
				result = s.RunNow()
			}
			return nil
		})
		return result, err
	default:
		return nil, errors.New("未知管理员管理操作")
	}
}

func (s *service) administratorLogAction(ctx context.Context, operation string) (map[string]interface{}, error) {
	entries, err := os.ReadDir("logs")
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	result := map[string]interface{}{"processedFiles": 0, "removedLines": 0, "bytesFreed": int64(0)}
	processed, removed := 0, 0
	var freed int64
	fail := func(err error) (map[string]interface{}, error) {
		result["processedFiles"], result["removedLines"], result["bytesFreed"] = processed, removed, freed
		return result, err
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		if operation == "temp-clean" && !strings.HasSuffix(entry.Name(), ".old") || operation != "temp-clean" && !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return fail(err)
		}
		path := filepath.Join("logs", entry.Name())
		switch operation {
		case "temp-clean", "logs-delete":
			info, err := entry.Info()
			if err != nil {
				return fail(err)
			}
			if err := coordination.CommitCurrent(ctx, func() error { return os.Remove(path) }); err != nil {
				return fail(err)
			}
			freed += info.Size()
		case "logs-rotate":
			if err := coordination.CommitCurrent(ctx, func() error { return os.Rename(path, path+".old") }); err != nil {
				return fail(err)
			}
		case "model-errors-delete":
			data, err := os.ReadFile(path)
			if err != nil {
				return fail(err)
			}
			var kept []string
			fileRemoved := 0
			for _, line := range strings.Split(string(data), "\n") {
				lower := strings.ToLower(line)
				if strings.Contains(lower, "model") && (strings.Contains(lower, "error") || strings.Contains(lower, "fail")) {
					fileRemoved++
				} else {
					kept = append(kept, line)
				}
			}
			if fileRemoved == 0 {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return fail(err)
			}
			if err := replaceAdministratorLog(ctx, path, []byte(strings.Join(kept, "\n")), info.Mode().Perm()); err != nil {
				return fail(err)
			}
			removed += fileRemoved
		}
		processed++
	}
	result["processedFiles"], result["removedLines"], result["bytesFreed"] = processed, removed, freed
	switch operation {
	case "logs-delete":
		result["deleted"] = true
	case "model-errors-delete":
		result["deleted"], result["updatedFiles"] = true, processed
	case "logs-rotate":
		result["rotated"], result["count"] = true, processed
	case "temp-clean":
		result["cleaned"] = true
	}
	return result, nil
}

func replaceAdministratorLog(ctx context.Context, path string, data []byte, mode os.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".managed-log-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return coordination.CommitCurrent(ctx, func() error { return os.Rename(temporary, path) })
}

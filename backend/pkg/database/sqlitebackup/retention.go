package sqlitebackup

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type RetentionPolicy struct {
	KeepRecent    int
	KeepDailyDays int
}

var DefaultRetention = RetentionPolicy{
	KeepRecent:    5,
	KeepDailyDays: 14,
}

type backupFile struct {
	path    string
	modTime time.Time
}

func PruneBackups(dir string, currentPath string, policy RetentionPolicy) ([]string, error) {
	if policy.KeepRecent <= 0 {
		policy.KeepRecent = 5
	}
	if policy.KeepDailyDays <= 0 {
		policy.KeepDailyDays = 14
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	backups := make([]backupFile, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "backup-") || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		base := strings.TrimSuffix(entry.Name(), ".db")
		if _, err := os.Stat(filepath.Join(dir, base+".json")); err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, backupFile{path: filepath.Join(dir, entry.Name()), modTime: info.ModTime()})
	}
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].modTime.After(backups[j].modTime)
	})
	keep := make(map[string]struct{}, policy.KeepRecent+policy.KeepDailyDays+1)
	if currentPath != "" {
		keep[filepath.Clean(currentPath)] = struct{}{}
	}
	now := time.Now()
	daily := make(map[string]struct{}, policy.KeepDailyDays)
	for index, backup := range backups {
		if index < policy.KeepRecent {
			keep[backup.path] = struct{}{}
			continue
		}
		if now.Sub(backup.modTime) > time.Duration(policy.KeepDailyDays)*24*time.Hour {
			continue
		}
		day := backup.modTime.Format("2006-01-02")
		if _, exists := daily[day]; exists {
			continue
		}
		daily[day] = struct{}{}
		keep[backup.path] = struct{}{}
	}
	deleted := make([]string, 0)
	var pruneErr error
	for _, backup := range backups {
		if _, exists := keep[filepath.Clean(backup.path)]; exists {
			continue
		}
		base := strings.TrimSuffix(backup.path, ".db")
		for _, path := range []string{backup.path, backup.path + "-wal", backup.path + "-shm", base + ".json"} {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				pruneErr = errors.Join(pruneErr, err)
			}
		}
		deleted = append(deleted, backup.path)
	}
	return deleted, pruneErr
}

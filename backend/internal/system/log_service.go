// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/pkg/util"
)

func runtimeLogDirectory() string {
	return util.RuntimeLogDir(util.RuntimeRoot())
}

func recentLogFiles(logDir string) []os.FileInfo {
	entries, _ := os.ReadDir(logDir)
	files := make([]os.FileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, err := entry.Info()
		if err == nil && info.Mode().IsRegular() {
			files = append(files, info)
		}
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].ModTime().Equal(files[j].ModTime()) {
			return files[i].Name() > files[j].Name()
		}
		return files[i].ModTime().After(files[j].ModTime())
	})
	return files
}

func readLogTail(path string, maxBytes int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, os.ErrPermission
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	size := info.Size()
	start := int64(0)
	if size > maxBytes {
		start = size - maxBytes
	}
	data := make([]byte, size-start)
	n, err := file.ReadAt(data, start)
	if err != nil && n == 0 && len(data) != 0 {
		return nil, err
	}
	data = data[:n]
	if start > 0 {
		if newline := strings.IndexByte(string(data), '\n'); newline >= 0 {
			data = data[newline+1:]
		}
	}
	return data, nil
}

func (s *service) GetLogsRecent(limit int) map[string]interface{} {
	logDir := runtimeLogDirectory()
	entries := recentLogFiles(logDir)
	lines := make([]interface{}, 0)
	count := 0
	for i := 0; i < len(entries) && count < limit; i++ {
		if !entries[i].IsDir() && strings.HasSuffix(entries[i].Name(), ".log") {
			data, err := readLogTail(filepath.Join(logDir, entries[i].Name()), 1024*1024)
			if err == nil {
				fileLines := strings.Split(string(data), "\n")
				for j := len(fileLines) - 1; j >= 0 && count < limit; j-- {
					l := fileLines[j]
					if l != "" && count < limit {
						lines = append(lines, map[string]interface{}{"file": entries[i].Name(), "line": l, "time": time.Now().Format(time.DateTime)})
						count++
					}
				}
			}
		}
	}
	return map[string]interface{}{"logs": lines}
}

func (s *service) GetLogsRecentErrors(limit int) map[string]interface{} {
	logDir := runtimeLogDirectory()
	entries := recentLogFiles(logDir)
	errs := make([]interface{}, 0)
	count := 0
	for i := 0; i < len(entries) && count < limit; i++ {
		if !entries[i].IsDir() && strings.HasSuffix(entries[i].Name(), ".log") {
			data, err := readLogTail(filepath.Join(logDir, entries[i].Name()), 1024*1024)
			if err == nil {
				fileLines := strings.Split(string(data), "\n")
				for _, l := range fileLines {
					if (strings.Contains(strings.ToLower(l), "error") || strings.Contains(strings.ToLower(l), "fail")) && count < limit {
						errs = append(errs, map[string]interface{}{"file": entries[i].Name(), "line": l, "time": time.Now().Format(time.DateTime)})
						count++
					}
				}
			}
		}
	}
	return map[string]interface{}{"errors": errs}
}

func (s *service) GetLogsFiles() map[string]interface{} {
	logDir := runtimeLogDirectory()
	entries := recentLogFiles(logDir)
	files := make([]interface{}, 0)
	for _, e := range entries {
		if !e.IsDir() {
			info := e
			files = append(files, map[string]interface{}{
				"name": e.Name(), "size": info.Size(), "modTime": info.ModTime().Format(time.DateTime),
			})
		}
	}
	return map[string]interface{}{"files": files, "directory": logDir}
}

func (s *service) GetLogsFileContent(name string) string {
	logDir := runtimeLogDirectory()
	cleanName := filepath.Base(strings.TrimSpace(name))
	if cleanName == "." || cleanName == ".." || cleanName == "" || cleanName != name || strings.ContainsAny(name, `/\`) {
		return "Invalid log file name"
	}
	data, err := readLogTail(filepath.Join(logDir, cleanName), 50000)
	if err != nil {
		return "File not found: " + cleanName
	}
	content := string(data)
	return content
}

func (s *service) DeleteLogs() map[string]interface{} {
	logDir := runtimeLogDirectory()
	entries, _ := os.ReadDir(logDir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			os.Remove(filepath.Join(logDir, e.Name()))
		}
	}
	return map[string]interface{}{"deleted": true}
}

func (s *service) GetLogsModelErrors() map[string]interface{} {
	logDir := runtimeLogDirectory()
	entries := recentLogFiles(logDir)
	errs := make([]interface{}, 0)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			data, err := readLogTail(filepath.Join(logDir, e.Name()), 1024*1024)
			if err == nil {
				for _, line := range strings.Split(string(data), "\n") {
					if strings.Contains(strings.ToLower(line), "model") && (strings.Contains(strings.ToLower(line), "error") || strings.Contains(strings.ToLower(line), "fail")) {
						errs = append(errs, map[string]interface{}{"file": e.Name(), "line": line, "time": time.Now().Format(time.DateTime)})
					}
				}
			}
		}
	}
	return map[string]interface{}{"errors": errs}
}

func (s *service) DeleteLogsModelErrors() map[string]interface{} {
	logDir := runtimeLogDirectory()
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return map[string]interface{}{"deleted": false, "removedLines": 0, "error": err.Error()}
	}
	removed := 0
	updatedFiles := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		path := filepath.Join(logDir, entry.Name())
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		kept := make([]string, 0, len(lines))
		fileRemoved := 0
		for _, line := range lines {
			lower := strings.ToLower(line)
			isModelError := strings.Contains(lower, "model") && (strings.Contains(lower, "error") || strings.Contains(lower, "fail"))
			if isModelError {
				removed++
				fileRemoved++
				continue
			}
			kept = append(kept, line)
		}
		if fileRemoved == 0 {
			continue
		}
		info, statErr := os.Stat(path)
		mode := os.FileMode(0o600)
		if statErr == nil {
			mode = info.Mode().Perm()
		}
		if writeErr := os.WriteFile(path, []byte(strings.Join(kept, "\n")), mode); writeErr == nil {
			updatedFiles++
		}
	}
	return map[string]interface{}{"deleted": true, "removedLines": removed, "updatedFiles": updatedFiles}
}

func (s *service) GetLogsPromptTraces(limit int) map[string]interface{} {
	logDir := runtimeLogDirectory()
	entries := recentLogFiles(logDir)
	traces := make([]interface{}, 0)
	for i := 0; i < len(entries) && len(traces) < limit; i++ {
		if entries[i].IsDir() || !strings.HasSuffix(entries[i].Name(), ".log") {
			continue
		}
		data, err := readLogTail(filepath.Join(logDir, entries[i].Name()), 1024*1024)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for j := len(lines) - 1; j >= 0 && len(traces) < limit; j-- {
			line := strings.TrimSpace(lines[j])
			if line == "" || !strings.Contains(line, "prompt_trace") {
				continue
			}
			var item map[string]interface{}
			if err := json.Unmarshal([]byte(line), &item); err != nil {
				continue
			}
			if item["stage"] != "prompt_trace" {
				continue
			}
			traces = append(traces, item)
		}
	}
	return map[string]interface{}{"traces": traces}
}

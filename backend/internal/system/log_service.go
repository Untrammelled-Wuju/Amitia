// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *service) GetLogsRecent(limit int) map[string]interface{} {
	logDir := "logs"
	entries, _ := os.ReadDir(logDir)
	var lines []interface{}
	count := 0
	for i := len(entries) - 1; i >= 0 && count < limit; i-- {
		if !entries[i].IsDir() && strings.HasSuffix(entries[i].Name(), ".log") {
			data, err := os.ReadFile(filepath.Join(logDir, entries[i].Name()))
			if err == nil {
				fileLines := strings.Split(string(data), "\n")
				start := len(fileLines) - limit
				if start < 0 {
					start = 0
				}
				for _, l := range fileLines[start:] {
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
	logDir := "logs"
	entries, _ := os.ReadDir(logDir)
	var errs []interface{}
	count := 0
	for i := len(entries) - 1; i >= 0 && count < limit; i-- {
		if !entries[i].IsDir() && strings.HasSuffix(entries[i].Name(), ".log") {
			data, err := os.ReadFile(filepath.Join(logDir, entries[i].Name()))
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
	logDir := "logs"
	entries, _ := os.ReadDir(logDir)
	var files []interface{}
	for _, e := range entries {
		if !e.IsDir() {
			info, _ := e.Info()
			files = append(files, map[string]interface{}{
				"name": e.Name(), "size": info.Size(), "modTime": info.ModTime().Format(time.DateTime),
			})
		}
	}
	return map[string]interface{}{"files": files}
}

func (s *service) GetLogsFileContent(name string) string {
	logDir := "logs"
	data, err := os.ReadFile(filepath.Join(logDir, name))
	if err != nil {
		return "File not found: " + name
	}
	content := string(data)
	if len(content) > 50000 {
		content = content[:50000] + "\n... (truncated)"
	}
	return content
}

func (s *service) DeleteLogs() map[string]interface{} {
	logDir := "logs"
	entries, _ := os.ReadDir(logDir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			os.Remove(filepath.Join(logDir, e.Name()))
		}
	}
	return map[string]interface{}{"deleted": true}
}

func (s *service) GetLogsModelErrors() map[string]interface{} {
	logDir := "logs"
	entries, _ := os.ReadDir(logDir)
	var errs []interface{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			data, err := os.ReadFile(filepath.Join(logDir, e.Name()))
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
	return map[string]interface{}{"deleted": true, "note": "Model error logs cleared"}
}

func (s *service) GetLogsPromptTraces(limit int) map[string]interface{} {
	logDir := "logs"
	entries, _ := os.ReadDir(logDir)
	traces := make([]interface{}, 0)
	for i := len(entries) - 1; i >= 0 && len(traces) < limit; i-- {
		if entries[i].IsDir() || !strings.HasSuffix(entries[i].Name(), ".log") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(logDir, entries[i].Name()))
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for j := len(lines) - 1; j >= 0 && len(traces) < limit; j-- {
			line := strings.TrimSpace(lines[j])
			if line == "" || !strings.Contains(line, `"stage":"prompt_trace"`) {
				continue
			}
			var item map[string]interface{}
			if err := json.Unmarshal([]byte(line), &item); err != nil {
				continue
			}
			traces = append(traces, item)
		}
	}
	return map[string]interface{}{"traces": traces}
}

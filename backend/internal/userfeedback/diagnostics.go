// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package userfeedback

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/pkg/util"
)

var (
	apiKeyPattern     = regexp.MustCompile(`(?i)\b(sk|pk|ak|api[_-]?key|token|secret)[-_a-z0-9]*\b\s*[:=]?\s*["']?[A-Za-z0-9_\-]{12,}`)
	bearerPattern     = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._\-]{8,}`)
	secretPairPattern = regexp.MustCompile(`(?i)("?(?:password|passwd|pwd|token|api[_-]?key|secret|authorization|cookie)"?\s*[:=]\s*")[^"]{2,}`)
	winUserPattern    = regexp.MustCompile(`(?i)([A-Za-z]:\\Users\\)[^\\\s"']+`)
	unixUserPattern   = regexp.MustCompile(`(/(?:home|Users)/)[^/\s"']+`)
	emailPattern      = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	phonePattern      = regexp.MustCompile(`\b1[3-9]\d{9}\b`)
)

func redactText(value string, limit int) (string, int) {
	line := value
	count := 0
	apply := func(pattern *regexp.Regexp, replacement string) {
		matches := pattern.FindAllString(line, -1)
		if len(matches) == 0 {
			return
		}
		count += len(matches)
		line = pattern.ReplaceAllString(line, replacement)
	}
	apply(secretPairPattern, `$1***`)
	apply(apiKeyPattern, `$1***`)
	apply(bearerPattern, `Bearer ***`)
	apply(winUserPattern, `${1}<user>`)
	apply(unixUserPattern, `${1}<user>`)
	apply(emailPattern, `***@***`)
	apply(phonePattern, `***`)
	if limit > 0 && len(line) > limit {
		line = line[:limit] + "...[truncated]"
	}
	return line, count
}

func redactLine(line string) (string, int) {
	return redactText(line, maxLogLineLength)
}

func runtimeLogDirectory() string {
	return util.RuntimeLogDir(util.RuntimeRoot())
}

func dataDirectory() string {
	if config.AppCfg != nil {
		if dir := strings.TrimSpace(config.AppCfg.Storage.DataDir); dir != "" {
			return dir
		}
	}
	return util.RuntimeDataDir(util.RuntimeRoot(), "")
}

func screenshotDirectory() string {
	return filepath.Join(dataDirectory(), "user-feedback", "screenshots")
}

func logFilesByRecency(logDir string) []os.FileInfo {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil
	}
	files := make([]os.FileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr == nil && info.Mode().IsRegular() && strings.HasSuffix(strings.ToLower(entry.Name()), ".log") {
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

func readLogTail(path string, maxBytes int64) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", os.ErrPermission
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	size := info.Size()
	start := int64(0)
	if size > maxBytes {
		start = size - maxBytes
	}
	buffer := make([]byte, size-start)
	n, err := file.ReadAt(buffer, start)
	if err != nil && n == 0 && len(buffer) != 0 {
		return "", err
	}
	content := string(buffer[:n])
	if start > 0 {
		if newline := strings.IndexByte(content, '\n'); newline >= 0 {
			content = content[newline+1:]
		}
	}
	return content, nil
}

func looksLikeError(line string) bool {
	lower := strings.ToLower(line)
	for _, marker := range []string{"error", "fail", "panic", "exception", "fatal", "warn"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func collectLogs(logDir string, includeLogs bool) (files []string, errorLines []LogLine, recentLines []LogLine, redacted int) {
	files = []string{}
	errorLines = []LogLine{}
	recentLines = []LogLine{}
	if !includeLogs {
		if entries, err := os.ReadDir(logDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".log") {
					files = append(files, entry.Name())
				}
			}
		}
		return files, errorLines, recentLines, 0
	}
	fileInfos := logFilesByRecency(logDir)
	for _, info := range fileInfos {
		files = append(files, info.Name())
	}
	for index := 0; index < len(fileInfos) && len(recentLines) < defaultLogLineLimit; index++ {
		content, err := readLogTail(filepath.Join(logDir, fileInfos[index].Name()), 2*1024*1024)
		if err != nil {
			continue
		}
		lines := strings.Split(content, "\n")
		for position := len(lines) - 1; position >= 0 && len(recentLines) < defaultLogLineLimit; position-- {
			line := strings.TrimSpace(lines[position])
			if line == "" {
				continue
			}
			cleaned, masked := redactLine(line)
			redacted += masked
			recentLines = append(recentLines, LogLine{File: fileInfos[index].Name(), Line: cleaned})
		}
	}
	for index := 0; index < len(fileInfos) && len(errorLines) < defaultErrorLineLimit; index++ {
		content, err := readLogTail(filepath.Join(logDir, fileInfos[index].Name()), 2*1024*1024)
		if err != nil {
			continue
		}
		lines := strings.Split(content, "\n")
		for position := len(lines) - 1; position >= 0 && len(errorLines) < defaultErrorLineLimit; position-- {
			line := strings.TrimSpace(lines[position])
			if line == "" || !looksLikeError(line) {
				continue
			}
			cleaned, masked := redactLine(line)
			redacted += masked
			errorLines = append(errorLines, LogLine{File: fileInfos[index].Name(), Line: cleaned})
		}
	}
	return files, errorLines, recentLines, redacted
}

func sanitizeClientErrors(input []ClientError) ([]ClientError, int) {
	if len(input) == 0 {
		return []ClientError{}, 0
	}
	result := make([]ClientError, 0, len(input))
	redacted := 0
	for _, item := range input {
		if len(result) >= maxClientErrors {
			break
		}
		message, maskedMessage := redactLine(strings.TrimSpace(item.Message))
		stack, maskedStack := redactLine(strings.TrimSpace(item.Stack))
		source := strings.TrimSpace(item.Source)
		if len(source) > 500 {
			source = source[:500]
		}
		if message == "" && stack == "" {
			continue
		}
		redacted += maskedMessage + maskedStack
		result = append(result, ClientError{
			Message: message,
			Stack:   stack,
			Source:  source,
			At:      strings.TrimSpace(item.At),
		})
	}
	return result, redacted
}

func buildSnapshot(input *TicketInput, clientErrors []ClientError) (*DiagnosticsSnapshot, error) {
	includeLogs := input.IncludeLogs == nil || *input.IncludeLogs
	logDir := runtimeLogDirectory()
	files, errorLines, recentLines, redacted := collectLogs(logDir, includeLogs)
	sanitizedErrors, clientRedacted := sanitizeClientErrors(clientErrors)
	redacted += clientRedacted

	platform := strings.TrimSpace(input.Platform)
	if platform == "" {
		platform = runtime.GOOS
	}
	osInfo := strings.TrimSpace(input.OSInfo)
	if osInfo == "" {
		osInfo = runtime.GOOS + " " + runtime.GOARCH
	}
	runtimeMode := strings.TrimSpace(input.RuntimeMode)
	if runtimeMode == "" {
		runtimeMode = "local"
	}

	snapshot := &DiagnosticsSnapshot{
		GeneratedAt:   time.Now().Format(time.RFC3339),
		CoreVersion:   config.AppVersion(),
		AppVersion:    strings.TrimSpace(input.AppVersion),
		Platform:      platform,
		ClientType:    strings.TrimSpace(input.ClientType),
		RuntimeMode:   runtimeMode,
		OSInfo:        osInfo,
		DeviceHash:    hashDeviceID(input.DeviceID),
		Locale:        strings.TrimSpace(input.Locale),
		Timezone:      strings.TrimSpace(input.Timezone),
		LogDirectory:  logDir,
		LogFiles:      files,
		ErrorLines:    errorLines,
		RecentLines:   recentLines,
		ClientErrors:  sanitizedErrors,
		RedactedCount: redacted,
		DataSummary: map[string]string{
			"goVersion":  runtime.Version(),
			"logLines":   strconv.Itoa(len(recentLines)),
			"errorLines": strconv.Itoa(len(errorLines)),
		},
	}
	return snapshot, nil
}

func readEnvOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func hashDeviceID(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(trimmed))
	return "sha256:" + hex.EncodeToString(sum[:])[:16]
}

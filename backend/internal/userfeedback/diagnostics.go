// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package userfeedback

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
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
	textErrorPattern  = regexp.MustCompile(`(?i)(?:^|\s)(?:\[(?:warn|warning|error|fatal|panic)\]|(?:warn|warning|error|fatal|panic)(?::|\s|$)|failed(?::|\s|$))`)
	errorNamePattern  = regexp.MustCompile(`(?i)(?:^|\s)(?:typeerror|rangeerror|referenceerror|syntaxerror|assertionerror|fluttererror|unhandledpromiserejection|uncaughtexception|exception|error)(?::|\s|$)`)
	valueErrorPattern = regexp.MustCompile(`(?i)\blevel\s*[:=]\s*(?:warn|warning|error|fatal|panic|dpanic|critical)\b`)
	timePrefixPattern = regexp.MustCompile(`^\s*\[?(?:\d{4}[-/]\d{2}[-/]\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?Z?)\]?\s*`)
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

type logRoot struct {
	Path     string
	Prefix   string
	MaxDepth int
}

type discoveredLogFile struct {
	Path    string
	Label   string
	ModTime time.Time
}

type collectedLogFile struct {
	Recent []LogLine
	Errors []LogLine
}

func logSearchRoots() []logRoot {
	roots := []logRoot{
		{Path: runtimeLogDirectory(), MaxDepth: 3},
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		roots = append(roots, logRoot{Path: filepath.Join(home, ".amitia", "extensions"), Prefix: "plugin:", MaxDepth: 6})
	}
	if dir := strings.TrimSpace(os.Getenv("AMITIA_EXTENSION_DATA_DIR")); dir != "" {
		roots = append(roots, logRoot{Path: dir, Prefix: "plugin:", MaxDepth: 6})
	}
	for _, base := range []string{dataDirectory(), util.RuntimeRoot()} {
		for _, name := range []string{"extensions", "extensions-v2"} {
			roots = append(roots, logRoot{Path: filepath.Join(base, name), Prefix: "plugin:", MaxDepth: 7})
		}
	}
	seen := map[string]bool{}
	result := make([]logRoot, 0, len(roots))
	for _, root := range roots {
		path := strings.TrimSpace(root.Path)
		if path == "" {
			continue
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			absolute = filepath.Clean(path)
		}
		if seen[absolute] {
			continue
		}
		seen[absolute] = true
		root.Path = absolute
		result = append(result, root)
	}
	return result
}

func logFilesByRecency(roots []logRoot) []discoveredLogFile {
	files := []discoveredLogFile{}
	seen := map[string]bool{}
	for _, root := range roots {
		appendLogFiles(&files, seen, root)
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].ModTime.Equal(files[j].ModTime) {
			return files[i].Label > files[j].Label
		}
		return files[i].ModTime.After(files[j].ModTime)
	})
	return files
}

func appendLogFiles(files *[]discoveredLogFile, seen map[string]bool, root logRoot) {
	info, err := os.Stat(root.Path)
	if err != nil || !info.IsDir() {
		return
	}
	baseDepth := strings.Count(filepath.Clean(root.Path), string(os.PathSeparator))
	_ = filepath.WalkDir(root.Path, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			depth := strings.Count(filepath.Clean(path), string(os.PathSeparator)) - baseDepth
			if depth >= root.MaxDepth {
				return filepath.SkipDir
			}
			switch strings.ToLower(entry.Name()) {
			case ".git", ".package-staging", "artifacts", "cache", "data", "native", "node_modules", "qdrant", "surrealdb", "tmp":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".log") {
			return nil
		}
		fileInfo, infoErr := entry.Info()
		if infoErr != nil || !fileInfo.Mode().IsRegular() {
			return nil
		}
		absolute, absErr := filepath.Abs(path)
		if absErr != nil {
			absolute = filepath.Clean(path)
		}
		if seen[absolute] {
			return nil
		}
		seen[absolute] = true
		label := entry.Name()
		if root.Prefix != "" {
			relative, relErr := filepath.Rel(root.Path, path)
			if relErr != nil {
				relative = entry.Name()
			}
			label = root.Prefix + filepath.ToSlash(relative)
		}
		*files = append(*files, discoveredLogFile{Path: absolute, Label: label, ModTime: fileInfo.ModTime()})
		return nil
	})
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
	if level, ok := structuredLogLevel(line); ok {
		return errorLogLevel(level)
	}
	return valueErrorPattern.MatchString(line) || textErrorPattern.MatchString(line) || errorNamePattern.MatchString(line)
}

func errorSignature(line string) string {
	candidate := strings.TrimSpace(line)
	if index := strings.Index(candidate, "{"); index >= 0 {
		candidate = candidate[index:]
	}
	if strings.HasPrefix(candidate, "{") {
		var payload map[string]any
		if json.Unmarshal([]byte(candidate), &payload) == nil {
			level := strings.ToLower(strings.TrimSpace(stringValue(payload["@level"])))
			if level == "" {
				level = strings.ToLower(strings.TrimSpace(stringValue(payload["level"])))
			}
			message := firstNonEmpty(
				stringValue(payload["@message"]),
				stringValue(payload["msg"]),
				stringValue(payload["message"]),
			)
			if message != "" {
				return level + "\x00" + message
			}
		}
	}
	normalized := strings.ToLower(timePrefixPattern.ReplaceAllString(strings.TrimSpace(line), ""))
	return strings.Join(strings.Fields(normalized), " ")
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func structuredLogLevel(line string) (string, bool) {
	candidate := strings.TrimSpace(line)
	if index := strings.Index(candidate, "{"); index >= 0 {
		candidate = candidate[index:]
	}
	if !strings.HasPrefix(candidate, "{") {
		return "", false
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(candidate), &payload); err != nil {
		return "", false
	}
	if value, ok := payload["@level"].(string); ok {
		return strings.ToLower(strings.TrimSpace(value)), true
	}
	switch value := payload["level"].(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(value)), true
	case float64:
		return numericLogLevel(int(value)), true
	}
	return "", false
}

func numericLogLevel(level int) string {
	switch {
	case level >= 60:
		return "fatal"
	case level >= 50:
		return "error"
	case level >= 40:
		return "warn"
	case level >= 30:
		return "info"
	default:
		return "debug"
	}
}

func errorLogLevel(level string) bool {
	switch level {
	case "warn", "warning", "error", "fatal", "panic", "dpanic", "critical":
		return true
	default:
		return false
	}
}

func collectLogs(roots []logRoot, includeLogs bool) (files []string, errorLines []LogLine, recentLines []LogLine, redacted int) {
	fileInfos := logFilesByRecency(roots)
	files = make([]string, 0, len(fileInfos))
	for _, info := range fileInfos {
		files = append(files, info.Label)
	}
	errorLines = []LogLine{}
	recentLines = []LogLine{}
	if !includeLogs || len(fileInfos) == 0 {
		return files, errorLines, recentLines, 0
	}
	collected := make([]collectedLogFile, 0, len(fileInfos))
	for _, info := range fileInfos {
		content, err := readLogTail(info.Path, 2*1024*1024)
		if err != nil {
			continue
		}
		lines := strings.Split(content, "\n")
		entry := collectedLogFile{
			Recent: make([]LogLine, 0, defaultLogLineLimit),
			Errors: make([]LogLine, 0, defaultErrorLineLimit),
		}
		seenErrors := map[string]struct{}{}
		for position := len(lines) - 1; position >= 0; position-- {
			line := strings.TrimSpace(lines[position])
			if line == "" {
				continue
			}
			cleaned, masked := redactLine(line)
			redacted += masked
			if len(entry.Recent) < defaultLogLineLimit {
				entry.Recent = append(entry.Recent, LogLine{File: info.Label, Line: cleaned})
			}
			if len(entry.Errors) < defaultErrorLineLimit && looksLikeError(line) {
				signature := errorSignature(line)
				if signature != "" {
					if _, duplicate := seenErrors[signature]; !duplicate {
						seenErrors[signature] = struct{}{}
						entry.Errors = append(entry.Errors, LogLine{File: info.Label, Line: cleaned})
					}
				}
			}
			if len(entry.Recent) >= defaultLogLineLimit && len(entry.Errors) >= defaultErrorLineLimit {
				break
			}
		}
		collected = append(collected, entry)
	}
	recentLines = interleaveLogLines(collected, defaultLogLineLimit, false)
	errorLines = interleaveLogLines(collected, defaultErrorLineLimit, true)
	return files, errorLines, recentLines, redacted
}

func interleaveLogLines(files []collectedLogFile, limit int, errors bool) []LogLine {
	result := make([]LogLine, 0, limit)
	for position := 0; len(result) < limit; position++ {
		progressed := false
		for _, file := range files {
			lines := file.Recent
			if errors {
				lines = file.Errors
			}
			if position >= len(lines) {
				continue
			}
			result = append(result, lines[position])
			progressed = true
			if len(result) >= limit {
				break
			}
		}
		if !progressed {
			break
		}
	}
	return result
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
	files, errorLines, recentLines, redacted := collectLogs(logSearchRoots(), includeLogs)
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

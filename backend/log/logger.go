package log

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
)

var Logger = logrus.New()
var rotatingFileWriter *rotatelogs.RotateLogs
var rotatingFileWriterMu sync.RWMutex

const (
	maxLogFileSize = int64(50 * 1024 * 1024)
	logRetention   = 30 * 24 * time.Hour
)

func InitLogger(logPath string) error {
	if logPath != "" {
		if err := os.MkdirAll(logPath, 0755); err != nil {
			return fmt.Errorf("create log directory %s: %w", logPath, err)
		}
		if err := removeExpiredLogFiles(logPath, logRetention); err != nil {
			return err
		}
	}

	Logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "@timestamp",
			logrus.FieldKeyLevel: "@level",
			logrus.FieldKeyMsg:   "@message",
		},
	})
	Logger.SetLevel(logrus.InfoLevel)

	if logPath != "" {
		writer, err := newRotatingFileWriter(logPath, maxLogFileSize, logRetention)
		if err != nil {
			return err
		}
		rotatingFileWriterMu.Lock()
		rotatingFileWriter = writer
		rotatingFileWriterMu.Unlock()
		Logger.SetOutput(io.MultiWriter(os.Stdout, writer))
		return nil
	}

	rotatingFileWriterMu.Lock()
	rotatingFileWriter = nil
	rotatingFileWriterMu.Unlock()
	Logger.SetOutput(os.Stdout)
	return nil
}

func newRotatingFileWriter(logPath string, rotationSize int64, maxAge time.Duration) (*rotatelogs.RotateLogs, error) {
	writer, err := rotatelogs.New(
		filepath.Join(logPath, "app.%Y%m%d.log"),
		rotatelogs.WithMaxAge(maxAge),
		rotatelogs.WithRotationTime(24*time.Hour),
		rotatelogs.WithRotationSize(rotationSize),
	)
	if err != nil {
		return nil, fmt.Errorf("create rotating log writer in %s: %w", logPath, err)
	}
	return writer, nil
}

func RotateNow() (string, error) {
	rotatingFileWriterMu.RLock()
	writer := rotatingFileWriter
	rotatingFileWriterMu.RUnlock()
	if writer == nil {
		return "", fmt.Errorf("rotating file writer is not initialized")
	}
	if err := writer.Rotate(); err != nil {
		return "", fmt.Errorf("rotate runtime log: %w", err)
	}
	return writer.CurrentFileName(), nil
}

func removeExpiredLogFiles(logPath string, maxAge time.Duration) error {
	entries, err := os.ReadDir(logPath)
	if err != nil {
		return fmt.Errorf("read log directory %s: %w", logPath, err)
	}
	cutoff := time.Now().Add(-maxAge)
	var removalErrors []error
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !isRetainedLogFile(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			removalErrors = append(removalErrors, fmt.Errorf("stat log %s: %w", entry.Name(), err))
			continue
		}
		if !info.ModTime().Before(cutoff) {
			continue
		}
		target := filepath.Join(logPath, entry.Name())
		if err := os.Remove(target); err != nil {
			removalErrors = append(removalErrors, fmt.Errorf("remove expired log %s: %w", target, err))
		}
	}
	return errors.Join(removalErrors...)
}

func isRetainedLogFile(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".log") {
		return true
	}
	index := strings.LastIndex(lower, ".log.")
	if index < 0 {
		return false
	}
	suffix := lower[index+len(".log."):]
	if suffix == "" {
		return false
	}
	for _, char := range suffix {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func Info(args ...interface{})                 { Logger.Info(args...) }
func Infof(format string, args ...interface{}) { Logger.Infof(format, args...) }
func Error(args ...interface{})                { Logger.Error(args...) }
func Warn(args ...interface{})                 { Logger.Warn(args...) }
func Debug(args ...interface{})                { Logger.Debug(args...) }

func WithFields(fields logrus.Fields) *logrus.Entry { return Logger.WithFields(fields) }

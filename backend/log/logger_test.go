package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRotatingFileWriterRotatesBySize(t *testing.T) {
	directory := t.TempDir()
	writer, err := newRotatingFileWriter(directory, 20, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()

	first := strings.Repeat("a", 24)
	if _, err := writer.Write([]byte(first + "\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte(first + "\n")); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	logFiles := make([]string, 0, 2)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "app.") && strings.Contains(entry.Name(), ".log") {
			logFiles = append(logFiles, entry.Name())
		}
	}
	if len(logFiles) != 2 {
		t.Fatalf("expected two rotated files, got %v", logFiles)
	}
	if !strings.HasSuffix(logFiles[1], ".log.1") {
		t.Fatalf("expected generation suffix, got %v", logFiles)
	}
}

func TestRotatingFileWriterRemovesExpiredLogs(t *testing.T) {
	directory := t.TempDir()
	expired := filepath.Join(directory, "app.20200101.log")
	if err := os.WriteFile(expired, []byte("expired"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(expired, old, old); err != nil {
		t.Fatal(err)
	}

	writer, err := newRotatingFileWriter(directory, maxLogFileSize, logRetention)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("fresh\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		_, statErr := os.Stat(expired)
		if os.IsNotExist(statErr) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expired log was not removed: %v", statErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRemoveExpiredLogFilesCoversAllRuntimeLogs(t *testing.T) {
	directory := t.TempDir()
	expired := []string{
		filepath.Join(directory, "electron.log"),
		filepath.Join(directory, "app.20200101.log.1"),
		filepath.Join(directory, "plugin-service.log"),
	}
	for _, name := range expired {
		if err := os.WriteFile(name, []byte("expired"), 0o644); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-31 * 24 * time.Hour)
		if err := os.Chtimes(name, old, old); err != nil {
			t.Fatal(err)
		}
	}
	recent := filepath.Join(directory, "desktop.log")
	if err := os.WriteFile(recent, []byte("recent"), 0o644); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(directory, "settings.json")
	if err := os.WriteFile(unrelated, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(unrelated, old, old); err != nil {
		t.Fatal(err)
	}

	if err := removeExpiredLogFiles(directory, logRetention); err != nil {
		t.Fatal(err)
	}
	for _, name := range expired {
		if _, err := os.Stat(name); !os.IsNotExist(err) {
			t.Fatalf("expired log was not removed: %s: %v", name, err)
		}
	}
	if _, err := os.Stat(recent); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatal(err)
	}
}

func TestRotateNowReturnsManagedGeneration(t *testing.T) {
	directory := t.TempDir()
	writer, err := newRotatingFileWriter(directory, maxLogFileSize, logRetention)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if _, err := writer.Write([]byte("before-rotate\n")); err != nil {
		t.Fatal(err)
	}

	rotatingFileWriterMu.Lock()
	previous := rotatingFileWriter
	rotatingFileWriter = writer
	rotatingFileWriterMu.Unlock()
	defer func() {
		rotatingFileWriterMu.Lock()
		rotatingFileWriter = previous
		rotatingFileWriterMu.Unlock()
	}()

	current, err := RotateNow()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(current, ".log.1") {
		t.Fatalf("expected generational log path, got %s", current)
	}
}

package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogServicesUseRuntimeDirectory(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "mounted-logs")
	t.Setenv("AMITIA_RUNTIME_ROOT", root)
	t.Setenv("AMITIA_LOG_DIR", directory)
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	content := "older message\nmodel error latest\n{\"stage\": \"prompt_trace\", \"request_id\": \"test-request\"}\n"
	if err := os.WriteFile(filepath.Join(directory, "app.log"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	s := &service{}
	files := s.GetLogsFiles()
	if files["directory"] != directory || len(files["files"].([]interface{})) != 1 {
		t.Fatalf("wrong log directory: %v", files)
	}
	logs := s.GetLogsRecent(2)["logs"].([]interface{})
	if len(logs) != 2 || !strings.Contains(logs[0].(map[string]interface{})["line"].(string), "prompt_trace") {
		t.Fatalf("newest entries missing: %v", logs)
	}
	if len(s.GetLogsRecentErrors(10)["errors"].([]interface{})) != 1 || len(s.GetLogsModelErrors()["errors"].([]interface{})) != 1 {
		t.Fatal("error views did not read runtime logs")
	}
	if len(s.GetLogsPromptTraces(10)["traces"].([]interface{})) != 1 {
		t.Fatal("spaced JSON prompt trace was lost")
	}
	if s.GetLogsFileContent("app.log") != content {
		t.Fatal("file content missing")
	}
}

func TestLogRecentUsesModificationTimeAndReturnsArrays(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("AMITIA_LOG_DIR", directory)
	s := &service{}
	if s.GetLogsRecent(10)["logs"].([]interface{}) == nil || s.GetLogsFiles()["files"].([]interface{}) == nil {
		t.Fatal("empty log responses must contain arrays")
	}
	for _, name := range []string{"z-old.log", "a-new.log"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(name+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(directory, "z-old.log"), oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	logs := s.GetLogsRecent(1)["logs"].([]interface{})
	if logs[0].(map[string]interface{})["file"] != "a-new.log" {
		t.Fatalf("wrong order: %v", logs)
	}
}

func TestLogFilePreviewReadsBoundedTailAndRejectsTraversal(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("AMITIA_LOG_DIR", directory)
	content := strings.Repeat("old entry\n", 20000) + "latest entry\n"
	if err := os.WriteFile(filepath.Join(directory, "large.log"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	s := &service{}
	preview := s.GetLogsFileContent("large.log")
	if len(preview) > 50000 || !strings.HasSuffix(preview, "latest entry\n") {
		t.Fatal("preview did not use bounded tail")
	}
	for _, name := range []string{"..", "../outside.log", `..\outside.log`, ""} {
		if s.GetLogsFileContent(name) != "Invalid log file name" {
			t.Fatalf("unsafe name accepted: %q", name)
		}
	}
}

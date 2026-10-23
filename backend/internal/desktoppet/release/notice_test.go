package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureReleaseNoticeGeneratesNoticeFile(t *testing.T) {
	stagingDir := t.TempDir()
	noticePath, err := ensureReleaseNotice(stagingDir)
	if err != nil {
		t.Fatal(err)
	}
	if noticePath != "NOTICE.txt" {
		t.Fatalf("notice path=%q", noticePath)
	}
	data, err := os.ReadFile(filepath.Join(stagingDir, noticePath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "SPDX-License-Identifier: AGPL-3.0-only") {
		t.Fatalf("notice content=%q", data)
	}
}

func TestEnsureReleaseNoticeReusesExistingLicenseFile(t *testing.T) {
	stagingDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(stagingDir, "LICENSE"), []byte("legacy license\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	noticePath, err := ensureReleaseNotice(stagingDir)
	if err != nil {
		t.Fatal(err)
	}
	if noticePath != "LICENSE" {
		t.Fatalf("notice path=%q", noticePath)
	}
	if _, err := os.Stat(filepath.Join(stagingDir, "NOTICE.txt")); !os.IsNotExist(err) {
		t.Fatalf("generated notice should not exist: %v", err)
	}
}

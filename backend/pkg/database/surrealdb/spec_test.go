// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package surrealdb

import (
	"archive/zip"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/pkg/platform"
)

func surrealdbTestHeader(goos string) []byte {
	switch goos {
	case "windows":
		return []byte{'M', 'Z', 0x90, 0x00}
	case "darwin":
		return []byte{0xcf, 0xfa, 0xed, 0xfe}
	default:
		return []byte{0x7f, 'E', 'L', 'F', 0x02, 0x01}
	}
}

func foreignTestHeader(goos string) []byte {
	if goos == "windows" {
		return surrealdbTestHeader("linux")
	}
	return surrealdbTestHeader("windows")
}

func writeSurrealdbBinary(t *testing.T, path string, header []byte) {
	t.Helper()
	if err := os.WriteFile(path, append(header, 0x00, 0x00, 0x00, 0x00), 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeSurrealdbArchive(t *testing.T, zipPath, entryName string, header []byte) {
	t.Helper()
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create(entryName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(append(header, 0x00, 0x00, 0x00, 0x00)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func prepareSurrealConfig(t *testing.T, root string) {
	t.Helper()
	t.Setenv("AMITIA_RUNTIME_ROOT", root)
	if err := os.MkdirAll(filepath.Join(root, "surrealdb"), 0o755); err != nil {
		t.Fatal(err)
	}
	previous := config.AppCfg
	config.AppCfg = &config.Config{}
	config.AppCfg.Providers.GraphStore.SurrealDB = config.SurrealConfig{
		Host:     "127.0.0.1",
		Port:     49321,
		Username: "test-user",
		Password: "test-password-sentinel",
	}
	t.Cleanup(func() {
		config.AppCfg = previous
	})
}

func TestBuildSurrealProcessSpecKeepsPasswordOutOfArgs(t *testing.T) {
	root := t.TempDir()
	prepareSurrealConfig(t, root)
	binaryName := "surreal" + platform.Get().ExecutableSuffix()
	writeSurrealdbBinary(t, filepath.Join(root, "surrealdb", binaryName), surrealdbTestHeader(runtime.GOOS))

	spec, err := BuildSurrealProcessSpec("test-instance")
	if err != nil {
		t.Fatal(err)
	}
	for _, arg := range spec.Args {
		if arg == "test-password-sentinel" || arg == "--pass" {
			t.Fatalf("password leaked into args: %#v", spec.Args)
		}
	}
	if got := spec.Environment.Values["SURREAL_PASS"]; got != "test-password-sentinel" {
		t.Fatalf("SURREAL_PASS = %q", got)
	}
	if got := spec.Environment.Values["SURREAL_USER"]; got != "test-user" {
		t.Fatalf("SURREAL_USER = %q", got)
	}
	if len(spec.Environment.SensitiveKeys) != 1 || spec.Environment.SensitiveKeys[0] != "SURREAL_PASS" {
		t.Fatalf("SURREAL_PASS is not marked sensitive: %#v", spec.Environment.SensitiveKeys)
	}
}

func TestBuildSurrealProcessSpecRejectsForeignBinary(t *testing.T) {
	root := t.TempDir()
	prepareSurrealConfig(t, root)
	binaryName := "surreal" + platform.Get().ExecutableSuffix()
	writeSurrealdbBinary(t, filepath.Join(root, "surrealdb", binaryName), foreignTestHeader(runtime.GOOS))

	_, err := BuildSurrealProcessSpec("test-instance")
	if err == nil {
		t.Fatal("expected platform mismatch to fail")
	}
	message := err.Error()
	if !strings.Contains(message, "executable header") {
		t.Fatalf("error lost the platform mismatch reason: %v", err)
	}
	if !strings.Contains(message, runtime.GOOS) {
		t.Fatalf("error lost the current platform: %v", err)
	}
	if !strings.Contains(message, binaryName) {
		t.Fatalf("error lost the expected binary name: %v", err)
	}
}

func TestBuildSurrealProcessSpecRejectsForeignArchive(t *testing.T) {
	root := t.TempDir()
	prepareSurrealConfig(t, root)
	surrealDir := filepath.Join(root, "surrealdb")
	writeSurrealdbArchive(t, filepath.Join(surrealDir, "surreal.zip"), "surreal.exe", foreignTestHeader(runtime.GOOS))

	_, err := BuildSurrealProcessSpec("test-instance")
	if err == nil {
		t.Fatal("expected foreign archive to fail")
	}
	message := err.Error()
	if !strings.Contains(message, "carries") || !strings.Contains(message, "want") {
		t.Fatalf("error lost the archive content diagnosis: %v", err)
	}
	if !strings.Contains(message, "surreal.zip") {
		t.Fatalf("error lost the archive name: %v", err)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error chain lost the missing candidate archive: %v", err)
	}
}

func TestBuildSurrealProcessSpecExtractsMatchingArchive(t *testing.T) {
	root := t.TempDir()
	prepareSurrealConfig(t, root)
	surrealDir := filepath.Join(root, "surrealdb")
	binaryName := "surreal" + platform.Get().ExecutableSuffix()
	writeSurrealdbArchive(t, filepath.Join(surrealDir, "surreal.zip"), binaryName, surrealdbTestHeader(runtime.GOOS))

	spec, err := BuildSurrealProcessSpec("test-instance")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(spec.Executable) != binaryName {
		t.Fatalf("executable = %q", spec.Executable)
	}
	if _, err := os.Stat(spec.Executable); err != nil {
		t.Fatalf("extracted executable missing: %v", err)
	}
}

func TestBuildSurrealProcessSpecReportsMissingBinary(t *testing.T) {
	root := t.TempDir()
	prepareSurrealConfig(t, root)

	_, err := BuildSurrealProcessSpec("test-instance")
	if err == nil {
		t.Fatal("expected missing binary to fail")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error chain lost the original stat error: %v", err)
	}
	if !strings.Contains(err.Error(), "is unavailable for") {
		t.Fatalf("error lost the runtime platform context: %v", err)
	}
}

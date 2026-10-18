// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package surrealdb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/config"
)

func TestBuildSurrealProcessSpecKeepsPasswordOutOfArgs(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AMITIA_RUNTIME_ROOT", root)
	if err := os.MkdirAll(filepath.Join(root, "surrealdb"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "surrealdb", "surreal.exe"), []byte("test"), 0755); err != nil {
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

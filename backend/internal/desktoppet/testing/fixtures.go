// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package testing

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type ContractFixture struct {
	Name    string          `json:"name"`
	Hash    string          `json:"hash"`
	Payload json.RawMessage `json:"payload"`
}

type ContractManifest struct {
	SchemaVersion string            `json:"schemaVersion"`
	ContractHash  string            `json:"contractHash"`
	Fixtures      []ContractFixture `json:"fixtures"`
	CreatedAt     string            `json:"createdAt"`
}

func LoadFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", path, err)
	}
	return data
}

func WriteGoldenFile(t *testing.T, path string, content []byte) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("failed to create directory %s: %v", dir, err)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("failed to write golden file %s: %v", path, err)
	}
}

func FixtureExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

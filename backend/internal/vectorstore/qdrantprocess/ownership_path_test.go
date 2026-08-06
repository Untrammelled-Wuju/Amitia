// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

type testPathHost struct {
	runtimehost.RuntimeHost
	paths util.RuntimePaths
}

func (h *testPathHost) Paths() util.RuntimePaths { return h.paths }

func (h *testPathHost) Descriptor() platform.RuntimeDescriptor {
	return platform.RuntimeDescriptor{Kind: platform.RuntimeKindNativeProcess}
}

func TestResolveOwnershipRoot_Valid(t *testing.T) {
	host := &testPathHost{paths: util.RuntimePaths{TempDir: "C:\\Temp\\amitia"}}
	root, err := ResolveOwnershipRoot(host)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(root, filepath.Join("process", "qdrant")) {
		t.Fatalf("unexpected root: %s", root)
	}
}

func TestResolveOwnershipRoot_NilHost(t *testing.T) {
	_, err := ResolveOwnershipRoot(nil)
	if err == nil {
		t.Fatal("expected error for nil host")
	}
}

func TestResolveOwnershipRoot_EmptyTempDir(t *testing.T) {
	host := &testPathHost{paths: util.RuntimePaths{}}
	_, err := ResolveOwnershipRoot(host)
	if err == nil {
		t.Fatal("expected error for empty temp dir")
	}
}

func TestResolveOwnershipRoot_RelativeTempDir(t *testing.T) {
	host := &testPathHost{paths: util.RuntimePaths{TempDir: "relative"}}
	_, err := ResolveOwnershipRoot(host)
	if err == nil {
		t.Fatal("expected error for relative temp dir")
	}
}

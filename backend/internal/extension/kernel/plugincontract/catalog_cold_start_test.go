package plugincontract

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const coldStartDigestGoldenPath = "testdata/cold-start-digest.json"

func coldStartDescriptor(index int) Descriptor {
	return Descriptor{
		ID:      fmt.Sprintf("com.amitia/plugin%04d/ai", index),
		Summary: fmt.Sprintf("synthetic capability %d", index),
		Tags:    []string{"cold-start"},
		Scopes:  []ServiceScope{ServiceScopeGlobal},
		Permissions: []DescriptorPermission{
			{Capability: "storage.state.read", Risk: "low"},
		},
		Placement: ServicePlacementDevice,
	}
}

func buildColdStartIndex(t *testing.T, count int) *DescriptorIndex {
	t.Helper()
	index := NewDescriptorIndex()
	for position := 0; position < count; position++ {
		if _, err := index.Add(coldStartDescriptor(position)); err != nil {
			t.Fatalf("add descriptor %d: %v", position, err)
		}
	}
	return index
}

func TestColdStartRebuildsThousandPluginsWithoutProcesses(t *testing.T) {
	source := buildColdStartIndex(t, 1000)
	digest, err := ColdStartDigest(source)
	if err != nil {
		t.Fatalf("ColdStartDigest: %v", err)
	}
	raw, err := source.MarshalColdStart()
	if err != nil {
		t.Fatalf("MarshalColdStart: %v", err)
	}
	before := runtime.NumGoroutine()
	restored, err := RestoreDescriptorIndex(raw, nil)
	if err != nil {
		t.Fatalf("RestoreDescriptorIndex: %v", err)
	}
	after := runtime.NumGoroutine()
	if delta := after - before; delta > 0 {
		t.Fatalf("restore started %d goroutines, want 0", delta)
	}
	if restored.Len() != 1000 {
		t.Fatalf("restored %d descriptors, want 1000", restored.Len())
	}
	restoredDigest, err := ColdStartDigest(restored)
	if err != nil {
		t.Fatalf("ColdStartDigest(restored): %v", err)
	}
	if restoredDigest != digest {
		t.Fatalf("digest drifted: %s != %s", restoredDigest, digest)
	}
	repeated, err := restored.MarshalColdStart()
	if err != nil {
		t.Fatalf("MarshalColdStart(restored): %v", err)
	}
	if string(repeated) != string(raw) {
		t.Fatalf("second cold start snapshot differs from first")
	}
	if restoredSchemas := restored.Schemas(); restoredSchemas == nil || restoredSchemas.Count() != 0 {
		t.Fatalf("restored index carries schema bodies, want none")
	}
	if strings.Contains(string(raw), "\"inputSchemaRef\"") || strings.Contains(string(raw), "\"outputSchemaRef\"") {
		t.Fatalf("plain descriptors unexpectedly carry schema refs")
	}
}

func TestColdStartDoesNotSpawnProcesses(t *testing.T) {
	source, err := os.ReadFile("catalog_cold_start.go")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	for _, forbidden := range []string{"os/exec", "exec.Command", "os.StartProcess", "go func(", "net.Dial"} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("cold start source contains forbidden token %q", forbidden)
		}
	}
	for _, path := range []string{"descriptor.go", "descriptor_catalog.go", "catalog_revision.go"} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(body), "os/exec") {
			t.Fatalf("%s imports os/exec", path)
		}
	}
}

func TestColdStartRejectsCorruptSnapshots(t *testing.T) {
	duplicate := `{"schemaVersion":1,"descriptors":[{"id":"com.amitia/x/ai","summary":"a","placement":"device"},{"id":"com.amitia/x/ai","summary":"b","placement":"device"}]}`
	cases := []struct {
		name    string
		raw     string
		want    string
		unwraps bool
	}{
		{name: "empty", raw: "   ", want: "cold start snapshot is empty"},
		{name: "truncated-json", raw: `{"schemaVersion":1,"descriptors":[`, want: "cold start snapshot is not valid json", unwraps: true},
		{name: "wrong-version", raw: `{"schemaVersion":2,"descriptors":[]}`, want: "cold start snapshot schemaVersion=2, want 1"},
		{name: "missing-field", raw: `{"schemaVersion":1}`, want: "cold start snapshot has no descriptors field"},
		{name: "duplicate-id", raw: duplicate, want: "cold start restore failed at descriptor 1 (com.amitia/x/ai)"},
		{name: "invalid-id", raw: `{"schemaVersion":1,"descriptors":[{"id":"BadID","summary":"a","placement":"device"}]}`, want: "cold start restore failed at descriptor 0 (BadID)"},
		{name: "empty-summary", raw: `{"schemaVersion":1,"descriptors":[{"id":"com.amitia/x/ai","summary":" ","placement":"device"}]}`, want: "cold start restore failed at descriptor 0 (com.amitia/x/ai)"},
	}
	for _, testCase := range cases {
		_, err := RestoreDescriptorIndex([]byte(testCase.raw), nil)
		if err == nil {
			t.Fatalf("case %s: expected error", testCase.name)
		}
		if !strings.Contains(err.Error(), testCase.want) {
			t.Fatalf("case %s: error %q does not contain %q", testCase.name, err.Error(), testCase.want)
		}
		if testCase.unwraps {
			var syntaxError *json.SyntaxError
			if !errors.As(err, &syntaxError) {
				t.Fatalf("case %s: error does not unwrap to json.SyntaxError: %v", testCase.name, err)
			}
		}
	}
}

func TestColdStartRequiresSchemasForSchemaRefs(t *testing.T) {
	schemas := NewSchemaStore()
	ref, err := schemas.Put([]byte(`{"type":"object"}`))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	descriptor := coldStartDescriptor(1)
	descriptor.InputRef = &ref
	index := NewDescriptorIndex()
	index.schemas = schemas
	if _, err := index.Add(descriptor); err != nil {
		t.Fatalf("Add with schema: %v", err)
	}
	raw, err := index.MarshalColdStart()
	if err != nil {
		t.Fatalf("MarshalColdStart: %v", err)
	}
	if _, err := RestoreDescriptorIndex(raw, nil); err == nil {
		t.Fatalf("restore without schemas must fail on schema ref")
	} else if !strings.Contains(err.Error(), "cold start restore failed at descriptor 0") {
		t.Fatalf("error = %q, want descriptor position wrapper", err.Error())
	}
	restored, err := RestoreDescriptorIndex(raw, schemas)
	if err != nil {
		t.Fatalf("restore with schemas: %v", err)
	}
	if restored.Len() != 1 || restored.Schemas().Count() != 1 {
		t.Fatalf("restored len=%d schemas=%d, want 1/1", restored.Len(), restored.Schemas().Count())
	}
}

func TestColdStartKeepsEmptyCatalogAndNormalizes(t *testing.T) {
	empty, err := RestoreDescriptorIndex([]byte(`{"schemaVersion":1,"descriptors":[]}`), nil)
	if err != nil {
		t.Fatalf("restore empty: %v", err)
	}
	if empty.Len() != 0 {
		t.Fatalf("empty catalog len=%d, want 0", empty.Len())
	}
	raw := []byte(`{"schemaVersion":1,"descriptors":[{"id":"  com.amitia/x/ai  ","summary":" raw ","tags":[" b ","a","b"],"scopes":["global"],"permissions":[{"capability":" storage.state.read ","risk":"low"},{"capability":"storage.state.read","risk":"low"}],"placement":"device"}]}`)
	restored, err := RestoreDescriptorIndex(raw, nil)
	if err != nil {
		t.Fatalf("restore normalized: %v", err)
	}
	stored, ok := restored.Get("com.amitia/x/ai")
	if !ok {
		t.Fatalf("restored index lost descriptor")
	}
	if stored.Summary != "raw" {
		t.Fatalf("summary = %q, want trimmed", stored.Summary)
	}
	if len(stored.Tags) != 2 || stored.Tags[0] != "a" || stored.Tags[1] != "b" {
		t.Fatalf("tags = %v, want sorted unique [a b]", stored.Tags)
	}
	if len(stored.Permissions) != 1 || stored.Permissions[0].Capability != "storage.state.read" || stored.Permissions[0].Risk != "low" {
		t.Fatalf("permissions = %+v, want single lowercase entry", stored.Permissions)
	}
}

func TestColdStartDigestGolden(t *testing.T) {
	index := buildColdStartIndex(t, 3)
	digest, err := ColdStartDigest(index)
	if err != nil {
		t.Fatalf("ColdStartDigest: %v", err)
	}
	raw, err := index.MarshalColdStart()
	if err != nil {
		t.Fatalf("MarshalColdStart: %v", err)
	}
	golden := struct {
		SchemaVersion int    `json:"schemaVersion"`
		Descriptors   int    `json:"descriptors"`
		Digest        string `json:"digest"`
		SnapshotBytes int    `json:"snapshotBytes"`
	}{SchemaVersion: 1, Descriptors: index.Len(), Digest: digest, SnapshotBytes: len(raw)}
	encoded, err := json.MarshalIndent(golden, "", "  ")
	if err != nil {
		t.Fatalf("marshal golden: %v", err)
	}
	encoded = append(encoded, '\n')
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(coldStartDigestGoldenPath), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(coldStartDigestGoldenPath, encoded, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	want, err := os.ReadFile(coldStartDigestGoldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v", coldStartDigestGoldenPath, err)
	}
	if strings.TrimSpace(string(want)) != strings.TrimSpace(string(encoded)) {
		t.Fatalf("cold start digest drifted\nwant:\n%s\ngot:\n%s", want, encoded)
	}
}

func TestColdStartScalesLinearlyEnough(t *testing.T) {
	if testing.Short() {
		t.Skip("timing probe skipped in short mode")
	}
	source := buildColdStartIndex(t, 1000)
	raw, err := source.MarshalColdStart()
	if err != nil {
		t.Fatalf("MarshalColdStart: %v", err)
	}
	started := time.Now()
	restored, err := RestoreDescriptorIndex(raw, nil)
	if err != nil {
		t.Fatalf("RestoreDescriptorIndex: %v", err)
	}
	elapsed := time.Since(started)
	if restored.Len() != 1000 {
		t.Fatalf("restored %d, want 1000", restored.Len())
	}
	if elapsed > 5*time.Second {
		t.Fatalf("restore 1000 descriptors took %s, want under 5s", elapsed)
	}
	t.Logf("restore 1000 descriptors in %s, snapshot %d bytes", elapsed, len(raw))
}

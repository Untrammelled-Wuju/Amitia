package staging

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/pkg/resourceuri"
)

type testNativeResourceBridge struct {
	data     []byte
	released bool
}

func (b *testNativeResourceBridge) Stat(string) (int64, string, string, error) {
	return int64(len(b.data)), "application/octet-stream", "payload.bin", nil
}

func (b *testNativeResourceBridge) ReadChunk(_ string, offset, length int64) ([]byte, error) {
	end := offset + length
	if end > int64(len(b.data)) {
		end = int64(len(b.data))
	}
	return append([]byte(nil), b.data[offset:end]...), nil
}

func (b *testNativeResourceBridge) Release(string) error {
	b.released = true
	return nil
}

func TestImportWithBridgePreservesBinaryAndCanonicalURI(t *testing.T) {
	attachments := t.TempDir()
	resolver, err := resourceuri.NewPhysicalResolver(resourceuri.PhysicalRoots{Attachments: attachments})
	if err != nil {
		t.Fatal(err)
	}
	baseDir := filepath.Join(attachments, "ios-native-imports")
	importer := NewStagingImporter(baseDir, resolver)

	data := bytes.Repeat([]byte{0x00, 0xff, 0x7f, 0x31, 0x42, 0x93}, (2*MaxChunkSize)/6+100)
	bridge := &testNativeResourceBridge{data: data}
	result, err := importer.ImportWithBridge(StagingImportRequest{
		NativeStagingID: "nativeStaging:test-binary",
		Filename:        "payload.bin",
	}, bridge)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if !bridge.released {
		t.Fatal("native staging was not released")
	}
	uri, err := resourceuri.Parse(result.ResourceURI)
	if err != nil {
		t.Fatalf("parse resource uri: %v", err)
	}
	if uri.Root() != resourceuri.ResourceRootAttachments {
		t.Fatalf("expected attachments root, got %s", uri.Root())
	}
	resolved, err := resolver.Resolve(uri)
	if err != nil {
		t.Fatalf("resolve resource uri: %v", err)
	}
	got, err := os.ReadFile(resolved.LocalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("imported binary does not match source")
	}
	sum := sha256.Sum256(data)
	if result.Checksum != hex.EncodeToString(sum[:]) {
		t.Fatalf("checksum mismatch: got %s", result.Checksum)
	}
}

func TestImportWithBridgeRejectsOversizeAndReleases(t *testing.T) {
	attachments := t.TempDir()
	resolver, _ := resourceuri.NewPhysicalResolver(resourceuri.PhysicalRoots{Attachments: attachments})
	bridge := &testNativeResourceBridge{data: bytes.Repeat([]byte{1}, 32)}
	importer := NewStagingImporter(filepath.Join(attachments, "ios-native-imports"), resolver)
	_, err := importer.ImportWithBridge(StagingImportRequest{
		NativeStagingID: "nativeStaging:too-large",
		MaxReadBytes:    8,
	}, bridge)
	if err == nil {
		t.Fatal("expected oversize import to fail")
	}
	if !bridge.released {
		t.Fatal("oversize failure must release native staging")
	}
}

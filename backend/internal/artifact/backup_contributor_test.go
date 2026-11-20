package artifact

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type memoryComponentWriter struct {
	data map[string][]byte
}

func (w *memoryComponentWriter) CreateComponent(id, _ string, _ dataportability.ComponentKind) (io.WriteCloser, error) {
	return &memoryComponentCloser{writer: w, id: id}, nil
}

func (*memoryComponentWriter) WriteJSON(string, interface{}) error { return nil }

type memoryComponentCloser struct {
	writer *memoryComponentWriter
	id     string
	data   bytes.Buffer
}

func (c *memoryComponentCloser) Write(data []byte) (int, error) { return c.data.Write(data) }

func (c *memoryComponentCloser) Close() error {
	if c.writer.data == nil {
		c.writer.data = map[string][]byte{}
	}
	c.writer.data[c.id] = append([]byte(nil), c.data.Bytes()...)
	return nil
}

type memoryComponentReader struct {
	data map[string][]byte
}

func (r *memoryComponentReader) ReadComponent(id string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(r.data[id])), nil
}

func (*memoryComponentReader) ReadJSON(string, interface{}) error { return nil }

func (r *memoryComponentReader) ListComponents() []string {
	keys := make([]string, 0, len(r.data))
	for key := range r.data {
		keys = append(keys, key)
	}
	return keys
}

func TestBackupRestoresChatAttachmentBlobAndOwnershipReferences(t *testing.T) {
	dataDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(dataDir, "artifacts.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Artifact{}, &ArtifactReference{}); err != nil {
		t.Fatal(err)
	}
	blobRoot := filepath.Join(dataDir, "artifacts", "blobs", "sha256")
	if err := os.MkdirAll(blobRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	service := NewService(NewFilesystemBlobStore(blobRoot), NewRepository(db), DefaultLimits)
	if err := service.SetStorageDeviceID("core-a"); err != nil {
		t.Fatal(err)
	}
	content := bytes.Repeat([]byte("backup-attachment"), 100000)
	stored, err := service.Create(t.Context(), CreateRequest{OwnerSpaceID: "space-a", OwnerDeviceID: "device-a", Kind: KindFile, MIMEType: "application/octet-stream", Source: SourceChatUpload, Filename: "backup.bin", Reader: bytes.NewReader(content), Unbounded: true})
	if err != nil {
		t.Fatal(err)
	}
	contributor := NewBackupContributor(db, dataDir)
	writer := &memoryComponentWriter{}
	if err := contributor.Export(t.Context(), dataportability.BackupRequest{Scope: dataportability.ScopeAll}, writer); err != nil {
		t.Fatal(err)
	}
	if len(writer.data[chatArtifactsComponent]) == 0 {
		t.Fatal("attachment component was not written")
	}
	if err := db.Where("1 = 1").Delete(&ArtifactReference{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("1 = 1").Delete(&Artifact{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(blobRoot); err != nil {
		t.Fatal(err)
	}
	if err := contributor.Import(t.Context(), dataportability.ImportRequest{OperationID: "restore"}, &memoryComponentReader{data: writer.data}); err != nil {
		t.Fatal(err)
	}
	restored, err := service.GetChatUploadReference(t.Context(), "space-a", "device-a", "core-a", stored.ID)
	if err != nil || restored.BlobDigest != stored.BlobDigest || restored.SizeBytes != stored.SizeBytes {
		t.Fatalf("restored=%+v err=%v", restored, err)
	}
	chunk, err := service.ReadChatUploadChunk(t.Context(), "space-a", "device-a", "core-a", stored.ID, 0, ChatUploadChunkMaxBytes)
	if err != nil || !bytes.Equal(chunk.Data, content[:len(chunk.Data)]) {
		t.Fatalf("restored chunk=%d err=%v", len(chunk.Data), err)
	}
}

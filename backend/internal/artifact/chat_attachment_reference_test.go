package artifact

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestChatAttachmentReferenceUsesRecoverableTwoPhaseBinding(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "artifacts.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Artifact{}, &ArtifactReference{}); err != nil {
		t.Fatal(err)
	}
	service := NewService(NewFilesystemBlobStore(t.TempDir()), NewRepository(db), DefaultLimits)
	if err := service.SetStorageDeviceID("device-a"); err != nil {
		t.Fatal(err)
	}
	stored, err := service.Create(t.Context(), CreateRequest{
		OwnerSpaceID:  "space-a",
		OwnerDeviceID: "device-a",
		Kind:          KindFile,
		MIMEType:      "application/octet-stream",
		Filename:      "large.bin",
		Source:        SourceChatUpload,
		Reader:        bytes.NewReader(bytes.Repeat([]byte{0, 255, 1, 2}, 1<<19)),
		Unbounded:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	tokenA := testChatAttachmentBindingToken(t, "message-a")
	tokenB := testChatAttachmentBindingToken(t, "message-b")
	if err := service.PrepareChatAttachmentReference(t.Context(), stored.ID, tokenA, "message-a"); err != nil {
		t.Fatal(err)
	}
	records, err := service.ListChatAttachmentReferenceBindings(t.Context(), 10)
	if err != nil || len(records) != 1 || records[0].State != ChatAttachmentReferencePending || records[0].Token != tokenA {
		t.Fatalf("pending records=%+v err=%v", records, err)
	}
	if err := service.CommitChatAttachmentReference(t.Context(), stored.ID, tokenA, "message-a"); err != nil {
		t.Fatal(err)
	}
	records, err = service.ListChatAttachmentReferenceBindings(t.Context(), 10)
	if err != nil || len(records) != 1 || records[0].State != ChatAttachmentReferenceConfirmed || records[0].Token != tokenA {
		t.Fatalf("confirmed records=%+v err=%v", records, err)
	}
	if err := service.Delete(t.Context(), "space-a", stored.ID); err == nil {
		t.Fatal("confirmed message attachment was deleted")
	}
	if err := service.RemoveChatAttachmentReference(t.Context(), stored.ID, tokenB, "message-a"); err != nil {
		t.Fatal(err)
	}
	records, err = service.ListChatAttachmentReferenceBindings(t.Context(), 10)
	if err != nil || len(records) != 0 {
		t.Fatalf("remaining records=%+v err=%v", records, err)
	}
	if err := service.Delete(t.Context(), "space-a", stored.ID); err != nil {
		t.Fatalf("release confirmed attachment: %v", err)
	}
}

func testChatAttachmentBindingToken(t *testing.T, messageID string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"messageId": messageID, "scope": map[string]any{"resourceOwnerId": "device-a"}, "reference": map[string]any{"artifactId": "artifact"}})
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

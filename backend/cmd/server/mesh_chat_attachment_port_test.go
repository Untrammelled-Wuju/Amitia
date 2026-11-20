package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/artifact"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type attachmentChunkReaderStub struct {
	data []byte
}

func (s attachmentChunkReaderStub) ReadChatAttachmentChunk(_ context.Context, _ coordination.ExecutionScope, request coordination.ChatAttachmentRead) (coordination.ChatAttachmentChunk, error) {
	if request.Offset < 0 || request.Offset > int64(len(s.data)) || request.Length < 1 {
		return coordination.ChatAttachmentChunk{}, coordination.ErrWrongOwner
	}
	end := min(int64(len(s.data)), request.Offset+request.Length)
	digest := sha256.Sum256(s.data)
	return coordination.ChatAttachmentChunk{ArtifactID: request.ArtifactID, Digest: "sha256:" + hex.EncodeToString(digest[:]), Offset: request.Offset, Size: int64(len(s.data)), Data: append([]byte(nil), s.data[request.Offset:end]...), EOF: end == int64(len(s.data))}, nil
}

func TestMeshChatAttachmentStorageValidatesReferenceAndChunks(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	runtime, err := BuildArtifactRuntime(p.services.DB, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Service.SetStorageDeviceID("device-a"); err != nil {
		t.Fatal(err)
	}
	p.services.Artifact = runtime
	data := bytes.Repeat([]byte("attachment-data"), 45000)
	stored, err := runtime.Service.Create(t.Context(), artifact.CreateRequest{OwnerSpaceID: p.spaceID, OwnerDeviceID: "device-a", Kind: artifact.KindFile, MIMEType: "application/octet-stream", Source: artifact.SourceChatUpload, Filename: "data.bin", Reader: bytes.NewReader(data), Unbounded: true})
	if err != nil {
		t.Fatal(err)
	}
	reference := coordination.ChatAttachmentReference{ArtifactID: string(stored.ID), OwnerDeviceID: "device-a", StorageDeviceID: "device-a", Kind: "file", Name: "data.bin", MIMEType: "application/octet-stream", SHA256: strings.TrimPrefix(string(stored.BlobDigest), "sha256:"), SizeBytes: stored.SizeBytes}
	scope := coordination.ExecutionScope{SpaceID: p.spaceID, InitiatorDeviceID: "device-a", TargetDeviceID: "device-a", CoreID: "core", RoleOwnerID: "device-a", ResourceOwnerID: "device-a", RoleID: "one", RoleRevision: 3, RequestID: "attachment-request"}
	if err := p.ValidateChatAttachment(t.Context(), scope, reference); err != nil {
		t.Fatal(err)
	}
	chunk, err := p.ReadChatAttachmentStorageChunk(t.Context(), scope, reference, 5, 200)
	if err != nil || !bytes.Equal(chunk.Data, data[5:205]) || chunk.Digest != string(stored.BlobDigest) {
		t.Fatalf("chunk=%+v error=%v", chunk, err)
	}
	spoofedOwner := reference
	spoofedOwner.OwnerDeviceID = "device-b"
	if err := p.ValidateChatAttachment(t.Context(), scope, spoofedOwner); err == nil {
		t.Fatalf("spoofed uploader accepted: %v", err)
	}
	wrongStorage := reference
	wrongStorage.StorageDeviceID = "device-b"
	if err := p.ValidateChatAttachment(t.Context(), scope, wrongStorage); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("wrong storage accepted: %v", err)
	}
	tampered := reference
	tampered.SHA256 = strings.Repeat("0", 64)
	if _, err := p.ReadChatAttachmentStorageChunk(t.Context(), scope, tampered, 0, 10); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("tampered digest accepted: %v", err)
	}
	if err := p.SetChatAttachmentReference(t.Context(), scope, reference, "message-a", coordination.ChatAttachmentReferencePrepare); err != nil {
		t.Fatal(err)
	}
	if err := p.SetChatAttachmentReference(t.Context(), scope, reference, "message-a", coordination.ChatAttachmentReferenceCommit); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Service.Delete(t.Context(), p.spaceID, stored.ID); err == nil {
		t.Fatal("referenced attachment was deleted")
	}
	if err := p.SetChatAttachmentReference(t.Context(), scope, reference, "message-a", coordination.ChatAttachmentReferenceRemove); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Service.Delete(t.Context(), p.spaceID, stored.ID); err != nil {
		t.Fatalf("unreferenced attachment was not deleted: %v", err)
	}
}

func TestSavedChatAttachmentReferenceRequiresMessageConversationAndOwner(t *testing.T) {
	reference := coordination.ChatAttachmentReference{ArtifactID: "art-a", OwnerDeviceID: "device-a", StorageDeviceID: "core", Kind: "file", Name: "a.bin", MIMEType: "application/octet-stream", SHA256: strings.Repeat("a", 64), SizeBytes: 1}
	body, err := json.Marshal(map[string]any{"conversationId": "chat-a", "attachments": []coordination.ChatAttachmentReference{reference}})
	if err != nil {
		t.Fatal(err)
	}
	view := business.QueryResult{Snapshot: coordination.DataSnapshot{OwnerID: "device-a", Resources: []coordination.Resource{{Kind: "message", ID: "message-a", OwnerID: "device-a", Body: body}}}}
	resolved, err := savedChatAttachmentReference(view, "chat-a", "message-a", "art-a")
	if err != nil || resolved != reference {
		t.Fatalf("reference=%+v err=%v", resolved, err)
	}
	if _, err := savedChatAttachmentReference(view, "chat-b", "message-a", "art-a"); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("cross-conversation message accepted: %v", err)
	}
	if _, err := savedChatAttachmentReference(view, "chat-a", "message-a", "art-b"); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("unreferenced artifact accepted: %v", err)
	}
}

func TestRemoteChatAttachmentReaderSupportsStreamingRanges(t *testing.T) {
	data := bytes.Repeat([]byte("range-data"), 100000)
	digest := sha256.Sum256(data)
	reader := &remoteChatAttachmentReader{
		ctx:            t.Context(),
		runtime:        attachmentChunkReaderStub{data: data},
		conversationID: "chat",
		messageID:      "message",
		reference:      coordination.ChatAttachmentReference{ArtifactID: "artifact", SizeBytes: int64(len(data)), SHA256: hex.EncodeToString(digest[:])},
	}
	if _, err := reader.Seek(4097, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 9000)
	if _, err := io.ReadFull(reader, buffer); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buffer, data[4097:13097]) || reader.position != 13097 {
		t.Fatal("streamed attachment range changed")
	}
}

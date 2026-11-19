package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"testing"
)

func TestMaterializeConversationAttachmentsWritesReadOnlyCopy(t *testing.T) {
	payload := []byte("attachment-body")
	digest := sha256.Sum256(payload)
	hash := hex.EncodeToString(digest[:])
	items, err := materializeConversationAttachments([]conversationAttachmentInput{{
		Kind: "file",
		Name: "notes.txt",
		MIME: "text/plain",
		Data: base64.StdEncoding.EncodeToString(payload),
		Hash: hash,
	}})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected items: %+v", items)
	}
	item := items[0]
	if item["id"] != hash || item["filename"] != "notes.txt" || item["mimeType"] != "text/plain" {
		t.Fatalf("unexpected metadata: %+v", item)
	}
	if item["sizeBytes"] != int64(len(payload)) {
		t.Fatalf("unexpected size: %+v", item["sizeBytes"])
	}
	absolutePath, _ := item["absolutePath"].(string)
	content, readErr := os.ReadFile(absolutePath)
	if readErr != nil {
		t.Fatalf("read materialized copy: %v", readErr)
	}
	if string(content) != string(payload) {
		t.Fatalf("materialized copy mismatch: %q", string(content))
	}
	info, statErr := os.Stat(absolutePath)
	if statErr != nil {
		t.Fatalf("stat materialized copy: %v", statErr)
	}
	if info.Mode().Perm() != 0o444 {
		t.Fatalf("materialized copy must be read-only, got %v", info.Mode().Perm())
	}
}

func TestMaterializeConversationAttachmentsRejectsHashMismatch(t *testing.T) {
	payload := []byte("tampered")
	if _, err := materializeConversationAttachments([]conversationAttachmentInput{{
		Kind: "file",
		Name: "notes.txt",
		MIME: "text/plain",
		Data: base64.StdEncoding.EncodeToString(payload),
		Hash: "0000000000000000000000000000000000000000000000000000000000000000",
	}}); err == nil {
		t.Fatal("hash mismatch accepted")
	}
}

func TestMaterializeConversationAttachmentsRejectsInvalidEncoding(t *testing.T) {
	if _, err := materializeConversationAttachments([]conversationAttachmentInput{{
		Kind: "file",
		Name: "notes.txt",
		MIME: "text/plain",
		Data: "not-base64!!",
	}}); err == nil {
		t.Fatal("invalid base64 accepted")
	}
}

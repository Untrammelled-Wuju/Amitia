package workbenchterminal

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
	"time"
)

func TestTranscriptContainsOnlyRetainedBytesDigest(t *testing.T) {
	manager, _, id := sessionFixture(t)
	s := manager.sessions[id]
	expected := []byte("abc\x00\xff\r\n")
	s.mu.Lock()
	s.output.append(expected)
	s.mu.Unlock()
	result, err := manager.Export("first", id)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(expected)
	if result.OutputDigest != hex.EncodeToString(hash[:]) || result.DigestScope != "retained-bytes-only" {
		t.Fatal("digest mismatch")
	}
	decoded, err := base64.StdEncoding.DecodeString(result.Data)
	if err != nil || string(decoded) != string(expected) || result.OutputBytes != len(expected) {
		t.Fatal("export bytes mismatch")
	}
	if result.Truncated || result.EndCursor != int64(len(expected)) || result.BaseCursor != 0 {
		t.Fatal("wrong cursor range")
	}
	if result.StartedAt.IsZero() || result.CapturedAt.Before(result.StartedAt) || result.CapturedAt.After(time.Now().Add(time.Minute)) {
		t.Fatal("invalid timestamps")
	}
}

func TestTranscriptDigestIsNotHistoricalWhenBufferTruncated(t *testing.T) {
	manager, _, id := sessionFixture(t)
	s := manager.sessions[id]
	s.mu.Lock()
	s.output.append(make([]byte, bufferCapacity+40))
	s.mu.Unlock()
	result, err := manager.Export("first", id)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Truncated || result.BaseCursor != 40 || result.OutputBytes != bufferCapacity {
		t.Fatalf("wrong truncation metadata: %+v", result)
	}
	if result.DigestScope != "retained-bytes-only" {
		t.Fatal("misleading evidence scope")
	}
}

package workbenchterminal

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestOutputBufferCursorAndReplay(t *testing.T) {
	var b outputBuffer
	b.append([]byte("first"))
	encoded, next, truncated := b.read(0)
	if truncated || next != 5 || encoded != base64.StdEncoding.EncodeToString([]byte("first")) {
		t.Fatalf("unexpected first response %q %d %v", encoded, next, truncated)
	}
	b.append([]byte("second"))
	encoded, next, truncated = b.read(5)
	if truncated || next != 11 || encoded != base64.StdEncoding.EncodeToString([]byte("second")) {
		t.Fatalf("unexpected second response")
	}
	encoded, next, _ = b.read(100000)
	if encoded != "" || next != 11 {
		t.Fatalf("future cursor not clamped")
	}
}

func TestOutputBufferOverflow(t *testing.T) {
	var b outputBuffer
	b.append(bytes.Repeat([]byte("a"), bufferCapacity+150))
	if len(b.data) != bufferCapacity || b.base != 150 {
		t.Fatalf("incorrect ring buffer capacity: %d, %d", len(b.data), b.base)
	}
	encoded, next, truncated := b.read(0)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || !truncated || len(decoded) != maxReadBytes || next != int64(maxReadBytes+150) {
		t.Fatalf("bad overflow cursor")
	}
}

func TestOutputBufferLargeSingleAppend(t *testing.T) {
	var b outputBuffer
	payload := bytes.Repeat([]byte("z"), 2*bufferCapacity+10)
	b.append(payload)
	if len(b.data) != bufferCapacity || b.base != int64(len(payload)-bufferCapacity) {
		t.Fatalf("overflow too large")
	}
	if !bytes.Equal(b.data, payload[len(payload)-bufferCapacity:]) {
		t.Fatal("trailing output corrupted")
	}
}

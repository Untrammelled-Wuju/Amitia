package workbenchterminal

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestBufferReadStartAndOverlappingReplay(t *testing.T) {
	var b outputBuffer
	b.append([]byte("abcdefghijkl"))
	got, start, end, lost := b.readWithStart(4)
	if lost || start != 4 || end != 12 || got != base64.StdEncoding.EncodeToString([]byte("efghijkl")) {
		t.Fatalf("incorrect replay %q %d %d %v", got, start, end, lost)
	}
	got, start, end, lost = b.readWithStart(10000)
	if lost || start != 12 || end != 12 || got != "" {
		t.Fatalf("invalid future cursor clamp %q %d %d %v", got, start, end, lost)
	}
}

func TestBufferReadStartOverflowAndBoundaries(t *testing.T) {
	var b outputBuffer
	b.append(bytes.Repeat([]byte("x"), bufferCapacity+73))
	got, start, end, lost := b.readWithStart(0)
	if !lost || start != 73 || end != 73+maxReadBytes || len(got) != base64.StdEncoding.EncodedLen(maxReadBytes) {
		t.Fatalf("incorrect ring gap %d %d %v", start, end, lost)
	}
	got, start, end, lost = b.readWithStart(end)
	if lost || start != 73+maxReadBytes || end != 73+2*maxReadBytes || len(got) != base64.StdEncoding.EncodedLen(maxReadBytes) {
		t.Fatalf("incorrect page progress %d %d %v", start, end, lost)
	}
}

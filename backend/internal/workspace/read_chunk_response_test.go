package workspace

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestReadChunkResponsePreservesRawBytesAndOffsets(t *testing.T) {
	payload := []byte{0xe4, 0xb8, 0xad, 0xe6, 0x96}
	response := buildReadChunkResponse(ReadResult{Content: payload, Resource: "file-a", IsText: true}, 17, 5)
	decoded, err := base64.StdEncoding.DecodeString(response.ContentBase64)
	if err != nil || !bytes.Equal(decoded, payload) {
		t.Fatalf("read response lost raw bytes: %v", err)
	}
	if response.BytesRead != 5 || response.NextOffset != 22 || !response.HasMore || response.Resource != "file-a" || !response.IsText {
		t.Fatalf("incorrect read metadata: %+v", response)
	}
}

func TestReadChunkResponseShortReadAndEOF(t *testing.T) {
	for _, sample := range []struct {
		content []byte
		bytes   int64
		hasMore bool
	}{
		{[]byte("abc"), 3, false},
		{[]byte{}, 0, false},
		{[]byte("abcd"), 4, true},
	} {
		response := buildReadChunkResponse(ReadResult{Content: sample.content}, 100, 4)
		if response.BytesRead != sample.bytes || response.NextOffset != 100+sample.bytes || response.HasMore != sample.hasMore {
			t.Fatalf("unexpected range response: %+v", response)
		}
	}
}

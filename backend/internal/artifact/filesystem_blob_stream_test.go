package artifact

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
)

func TestFilesystemBlobStoreUnboundedAndCancellation(t *testing.T) {
	store := NewFilesystemBlobStore(t.TempDir())
	data := bytes.Repeat([]byte("unbounded-stream"), 100000)
	info, err := store.Put(t.Context(), bytes.NewReader(data), -1)
	if err != nil {
		t.Fatal(err)
	}
	if info.SizeBytes != int64(len(data)) {
		t.Fatalf("size = %d, want %d", info.SizeBytes, len(data))
	}
	reader, _, err := store.Open(t.Context(), info.Digest)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("stream contents changed")
	}
	if _, err := store.Put(t.Context(), bytes.NewReader(data), 1024); !errors.Is(err, ErrBlobTooLarge) {
		t.Fatalf("bounded upload error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := store.Put(ctx, bytes.NewReader(data), -1); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled upload error = %v", err)
	}
	entries, err := os.ReadDir(store.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			t.Fatalf("temporary upload left behind: %s", entry.Name())
		}
	}
}

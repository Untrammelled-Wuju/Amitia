// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package business

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/system/dataportability"
)

type fakeMemoryArchivePort struct{ entries []MemoryConsumerEntry }

func (f fakeMemoryArchivePort) Archive(context.Context, string) (MemoryArchive, error) {
	return MemoryArchive{Format: OwnedMemoryArchiveFormat, Entries: f.entries}, nil
}

func (f fakeMemoryArchivePort) RestoreArchive(context.Context, dataportability.ImportRequest, MemoryArchive) error {
	return nil
}

type recordingBackupWriter struct {
	counts  map[string]int64
	buffers map[string]*bytes.Buffer
}

func (w *recordingBackupWriter) CreateComponent(id, _ string, _ dataportability.ComponentKind) (io.WriteCloser, error) {
	buffer := &bytes.Buffer{}
	w.buffers[id] = buffer
	return nopCloser{buffer}, nil
}

func (w *recordingBackupWriter) WriteJSON(id string, value interface{}) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = w.buffers[id].Write(encoded)
	return err
}

func (w *recordingBackupWriter) SetComponentItemCount(id string, count int64) error {
	w.counts[id] = count
	return nil
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

func TestMemoryBackupReportsRealItemCount(t *testing.T) {
	entries := []MemoryConsumerEntry{
		{Resource: coordination.Resource{Kind: "memory", ID: "memory-a"}},
		{Resource: coordination.Resource{Kind: "memory", ID: "memory-b"}},
		{Resource: coordination.Resource{Kind: "fact", ID: "fact-c"}},
	}
	contributor := NewMemoryBackupContributor(fakeMemoryArchivePort{entries: entries})
	writer := &recordingBackupWriter{counts: map[string]int64{}, buffers: map[string]*bytes.Buffer{}}
	if err := contributor.Export(context.Background(), dataportability.BackupRequest{Scope: dataportability.ScopeAll}, writer); err != nil {
		t.Fatal(err)
	}
	if got := writer.counts[OwnedMemoryBackupComponent]; got != 3 {
		t.Fatalf("itemCount=%d want=3", got)
	}
	if writer.buffers[OwnedMemoryBackupComponent].Len() == 0 {
		t.Fatal("memory archive body was not written")
	}
}

//go:build linux && !android

package archive

import (
	"context"
	"io"
)

type EntryType string

const (
	EntryTypeFile      EntryType = "file"
	EntryTypeDirectory EntryType = "directory"
	EntryTypeSymlink   EntryType = "symlink"
	EntryTypeHardlink  EntryType = "hardlink"
	EntryTypeOther     EntryType = "other"
)

type ArchiveEntry struct {
	Name       string
	Path       string
	Type       EntryType
	SizeBytes  int64
	Mode       uint32
	ModifiedAt int64
	LinkTarget string
	Encrypted  bool
	Offset     int64
}

type CompressionCodec interface {
	Format() Format
	Detect(r io.Reader) (bool, error)
	PeekFormat(r io.Reader) (Format, error)
}

type ArchiveReader interface {
	Entries(ctx context.Context) ([]ArchiveEntry, error)
	Close() error
}

type ArchiveWriter interface {
	CreateEntry(ctx context.Context, entry ArchiveEntry, content io.Reader) error
	CreateEmptyDirectory(ctx context.Context, entry ArchiveEntry) error
	CreateSymlink(ctx context.Context, entry ArchiveEntry) error
	Close() error
}

type StreamingArchiveReader interface {
	ForEachEntry(ctx context.Context, fn func(ctx context.Context, entry ArchiveEntry, content io.Reader) error) error
	Close() error
}

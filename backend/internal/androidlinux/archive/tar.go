//go:build linux && !android

package archive

import (
	"archive/tar"
	"context"
	"io"
	"path/filepath"
	"strings"
	"time"
)

type TarReader struct {
	reader   *tar.Reader
	src      io.ReadCloser
	filePath string
}

func OpenTarReader(ctx context.Context, src io.ReadCloser, filePath string) (*TarReader, error) {
	return &TarReader{
		reader:   tar.NewReader(src),
		src:      src,
		filePath: filePath,
	}, nil
}

func (t *TarReader) Close() error {
	if t.src != nil {
		return t.src.Close()
	}
	return nil
}

func (t *TarReader) Entries(ctx context.Context) ([]ArchiveEntry, error) {
	entries := make([]ArchiveEntry, 0, 64)
	for {
		select {
		case <-ctx.Done():
			return entries, ErrCancelled()
		default:
		}

		header, err := t.reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return entries, ErrCorrupt(err.Error())
		}

		entry := tarHeaderToEntry(header)
		entries = append(entries, entry)
	}
	return entries, nil
}

func (t *TarReader) ForEachEntry(ctx context.Context, fn func(ctx context.Context, entry ArchiveEntry, content io.Reader) error) error {
	for {
		select {
		case <-ctx.Done():
			return ErrCancelled()
		default:
		}

		header, err := t.reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return ErrCorrupt(err.Error())
		}

		entry := tarHeaderToEntry(header)
		err = fn(ctx, entry, t.reader)
		if err != nil {
			return err
		}
	}
}

func (t *TarReader) Verify(ctx context.Context) (*VerifyResult, error) {
	result := &VerifyResult{
		Format: FormatTAR,
		Valid:  true,
	}

	for {
		select {
		case <-ctx.Done():
			return result, ErrCancelled()
		default:
		}

		header, err := t.reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Valid = false
			result.CorruptEntries++
			break
		}

		result.EntryCount++
		entry := tarHeaderToEntry(header)

		if !isSafeEntryPath(entry.Path) {
			result.UnsafeEntries++
			result.Valid = false
			continue
		}

		switch header.Typeflag {
		case tar.TypeReg, tar.TypeRegA:
			result.TotalUncompressedBytes += header.Size
		case tar.TypeLink:
			result.Warnings = append(result.Warnings, "hardlink entry: "+entry.Path)
		case tar.TypeSymlink:
			if !isSafeLinkTarget(header.Linkname) {
				result.UnsafeEntries++
				result.Valid = false
			}
		case tar.TypeBlock, tar.TypeChar, tar.TypeFifo, tar.TypeGNUSparse:
			result.UnsafeEntries++
			result.Valid = false
		}
	}

	if result.UnsafeEntries > 0 || result.CorruptEntries > 0 {
		result.Valid = false
	}

	return result, nil
}

func tarHeaderToEntry(header *tar.Header) ArchiveEntry {
	entryType := tarTypeflagToEntryType(header.Typeflag)
	return ArchiveEntry{
		Name:       filepath.Base(header.Name),
		Path:       normalizeEntryPath(header.Name),
		Type:       entryType,
		SizeBytes:  header.Size,
		Mode:       uint32(header.Mode),
		ModifiedAt: header.ModTime.Unix(),
		LinkTarget: header.Linkname,
	}
}

func tarTypeflagToEntryType(t byte) EntryType {
	switch t {
	case tar.TypeReg, tar.TypeRegA:
		return EntryTypeFile
	case tar.TypeDir:
		return EntryTypeDirectory
	case tar.TypeSymlink:
		return EntryTypeSymlink
	case tar.TypeLink:
		return EntryTypeHardlink
	default:
		return EntryTypeOther
	}
}

func isSafeLinkTarget(target string) bool {
	if target == "" {
		return false
	}
	if strings.HasPrefix(target, "/") {
		return false
	}
	if strings.HasPrefix(target, "\\") {
		return false
	}
	if strings.Contains(target, "..") {
		return false
	}
	return true
}

type TarWriter struct {
	writer *tar.Writer
	dst    io.WriteCloser
}

func OpenTarWriter(dst io.WriteCloser) *TarWriter {
	return &TarWriter{
		writer: tar.NewWriter(dst),
		dst:    dst,
	}
}

func (t *TarWriter) Close() error {
	if t.writer != nil {
		if err := t.writer.Close(); err != nil {
			return err
		}
	}
	if t.dst != nil {
		return t.dst.Close()
	}
	return nil
}

func (t *TarWriter) CreateEntry(ctx context.Context, entry ArchiveEntry, content io.Reader) error {
	if entry.Type == EntryTypeDirectory {
		return t.CreateEmptyDirectory(ctx, entry)
	}

	mode := int64(entry.Mode)
	if mode == 0 {
		mode = 0644
	}

	header := &tar.Header{
		Name:    entry.Path,
		Size:    entry.SizeBytes,
		Mode:    mode,
		ModTime: time.Unix(entry.ModifiedAt, 0),
	}

	switch entry.Type {
	case EntryTypeDirectory:
		header.Typeflag = tar.TypeDir
		header.Name += "/"
	case EntryTypeSymlink:
		header.Typeflag = tar.TypeSymlink
		header.Linkname = entry.LinkTarget
		header.Size = 0
	default:
		header.Typeflag = tar.TypeReg
	}

	if err := t.writer.WriteHeader(header); err != nil {
		return ErrWriteFailed(err.Error())
	}

	if content != nil && entry.SizeBytes > 0 {
		_, err := io.Copy(t.writer, content)
		if err != nil {
			return ErrWriteFailed(err.Error())
		}
	}
	return nil
}

func (t *TarWriter) CreateEmptyDirectory(ctx context.Context, entry ArchiveEntry) error {
	mode := int64(entry.Mode)
	if mode == 0 {
		mode = 0755
	}

	header := &tar.Header{
		Name:     entry.Path + "/",
		Typeflag: tar.TypeDir,
		Mode:     mode,
		ModTime:  time.Unix(entry.ModifiedAt, 0),
	}

	if err := t.writer.WriteHeader(header); err != nil {
		return ErrWriteFailed(err.Error())
	}
	return nil
}

func (t *TarWriter) CreateSymlink(ctx context.Context, entry ArchiveEntry) error {
	header := &tar.Header{
		Name:     entry.Path,
		Typeflag: tar.TypeSymlink,
		Linkname: entry.LinkTarget,
		Mode:     0777,
		ModTime:  time.Unix(entry.ModifiedAt, 0),
	}

	if err := t.writer.WriteHeader(header); err != nil {
		return ErrWriteFailed(err.Error())
	}
	return nil
}

type TarCodec struct{}

func (c *TarCodec) Format() Format { return FormatTAR }

func (c *TarCodec) Detect(r io.Reader) (bool, error) { return false, nil }

func (c *TarCodec) PeekFormat(r io.Reader) (Format, error) { return FormatTAR, nil }

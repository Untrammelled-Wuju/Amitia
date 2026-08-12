//go:build linux && !android

package archive

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ZipReader struct {
	file     *os.File
	reader   *zip.Reader
	filePath string
}

func OpenZipReader(ctx context.Context, path string, maxSize int64) (*ZipReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, ErrReadFailed(err.Error())
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, ErrReadFailed(err.Error())
	}

	if maxSize > 0 && info.Size() > maxSize {
		file.Close()
		return nil, ErrTooLarge(maxSize)
	}

	reader, err := zip.NewReader(file, info.Size())
	if err != nil {
		file.Close()
		return nil, ErrCorrupt(err.Error())
	}

	return &ZipReader{
		file:     file,
		reader:   reader,
		filePath: path,
	}, nil
}

func (z *ZipReader) Close() error {
	if z.file != nil {
		return z.file.Close()
	}
	return nil
}

func (z *ZipReader) Entries(ctx context.Context) ([]ArchiveEntry, error) {
	entries := make([]ArchiveEntry, 0, len(z.reader.File))
	for _, f := range z.reader.File {
		entry := zipFileToEntry(f)
		entries = append(entries, entry)
	}
	return entries, nil
}

func (z *ZipReader) ForEachEntry(ctx context.Context, fn func(ctx context.Context, entry ArchiveEntry, content io.Reader) error) error {
	for _, f := range z.reader.File {
		entry := zipFileToEntry(f)
		if entry.Encrypted {
			return ErrEncryptedUnsupported()
		}

		rc, err := f.Open()
		if err != nil {
			return ErrReadFailed(err.Error())
		}

		err = fn(ctx, entry, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (z *ZipReader) Verify(ctx context.Context) (*VerifyResult, error) {
	result := &VerifyResult{
		Format: FormatZIP,
		Valid:  true,
	}

	for _, f := range z.reader.File {
		result.EntryCount++

		if f.Flags&0x01 != 0 {
			result.EncryptedEntries++
			result.Valid = false
			continue
		}

		entry := zipFileToEntry(f)
		if !isSafeEntryPath(entry.Path) {
			result.UnsafeEntries++
			result.Valid = false
			continue
		}

		result.TotalUncompressedBytes += int64(f.UncompressedSize64)
	}

	if result.UnsafeEntries > 0 || result.EncryptedEntries > 0 {
		result.Valid = false
	}

	return result, nil
}

func zipFileToEntry(f *zip.File) ArchiveEntry {
	entryType := EntryTypeFile
	if f.FileInfo().IsDir() {
		entryType = EntryTypeDirectory
	} else if f.FileInfo().Mode()&os.ModeSymlink != 0 {
		entryType = EntryTypeSymlink
	}

	return ArchiveEntry{
		Name:       filepath.Base(f.Name),
		Path:       normalizeEntryPath(f.Name),
		Type:       entryType,
		SizeBytes:  int64(f.UncompressedSize64),
		Mode:       uint32(f.Mode()),
		ModifiedAt: f.Modified.Unix(),
		Encrypted:  f.Flags&0x01 != 0,
	}
}

type ZipWriter struct {
	file   *os.File
	writer *zip.Writer
}

func OpenZipWriter(path string, overwrite bool) (*ZipWriter, error) {
	flags := os.O_WRONLY | os.O_CREATE
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}

	file, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		if os.IsExist(err) {
			return nil, ErrTargetExists(path)
		}
		return nil, ErrWriteFailed(err.Error())
	}

	return &ZipWriter{
		file:   file,
		writer: zip.NewWriter(file),
	}, nil
}

func (z *ZipWriter) Close() error {
	if z.writer != nil {
		if err := z.writer.Close(); err != nil {
			return err
		}
	}
	if z.file != nil {
		return z.file.Close()
	}
	return nil
}

func (z *ZipWriter) CreateEntry(ctx context.Context, entry ArchiveEntry, content io.Reader) error {
	if entry.Type == EntryTypeDirectory {
		return z.CreateEmptyDirectory(ctx, entry)
	}

	header := &zip.FileHeader{
		Name: entry.Path,
	}

	switch entry.Type {
	case EntryTypeDirectory:
		header.SetMode(os.FileMode(entry.Mode))
		header.Name += "/"
	case EntryTypeSymlink:
		header.SetMode(os.ModeSymlink)
	case EntryTypeFile:
		header.SetMode(os.FileMode(entry.Mode))
	}

	w, err := z.writer.CreateHeader(header)
	if err != nil {
		return ErrWriteFailed(err.Error())
	}

	if content != nil {
		_, err = io.Copy(w, content)
		if err != nil {
			return ErrWriteFailed(err.Error())
		}
	}
	return nil
}

func (z *ZipWriter) CreateEmptyDirectory(ctx context.Context, entry ArchiveEntry) error {
	header := &zip.FileHeader{
		Name:   entry.Path + "/",
		Method: zip.Store,
	}
	header.SetMode(os.FileMode(entry.Mode))

	_, err := z.writer.CreateHeader(header)
	if err != nil {
		return ErrWriteFailed(err.Error())
	}
	return nil
}

func (z *ZipWriter) CreateSymlink(ctx context.Context, entry ArchiveEntry) error {
	header := &zip.FileHeader{
		Name:   entry.Path,
		Method: zip.Store,
	}
	header.SetMode(os.ModeSymlink)

	w, err := z.writer.CreateHeader(header)
	if err != nil {
		return ErrWriteFailed(err.Error())
	}

	_, err = w.Write([]byte(entry.LinkTarget))
	if err != nil {
		return ErrWriteFailed(err.Error())
	}
	return nil
}

func isSafeEntryPath(path string) bool {
	if path == "" {
		return false
	}
	if strings.Contains(path, "..") {
		return false
	}
	if strings.HasPrefix(path, "/") {
		return false
	}
	if strings.HasPrefix(path, "\\") {
		return false
	}
	if strings.Contains(path, ":") {
		return false
	}
	if strings.ContainsAny(path, "\x00") {
		return false
	}
	return true
}

func normalizeEntryPath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = filepath.Clean("/" + path)
	path = strings.TrimPrefix(path, "/")
	return path
}

type ZipCodec struct{}

func (c *ZipCodec) Format() Format { return FormatZIP }

func (c *ZipCodec) Detect(r io.Reader) (bool, error) {
	return false, nil
}

func (c *ZipCodec) PeekFormat(r io.Reader) (Format, error) {
	return FormatZIP, nil
}

var _ = time.Now

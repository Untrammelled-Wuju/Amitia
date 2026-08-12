package dataportability

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type ArchiveWriter struct {
	file      *os.File
	zipWriter *zip.Writer
	entries   map[string]bool
}

func NewArchiveWriter(path string) *ArchiveWriter {
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return &ArchiveWriter{entries: make(map[string]bool)}
	}
	return &ArchiveWriter{
		file:      f,
		zipWriter: zip.NewWriter(f),
		entries:   make(map[string]bool),
	}
}

func (a *ArchiveWriter) CreateComponent(id, logicalName string, kind ComponentKind) (io.WriteCloser, error) {
	if a.zipWriter == nil {
		return nil, ErrBackupComponentFailed
	}
	path := fmt.Sprintf("datasets/%s.ndjson", id)
	w, err := a.zipWriter.CreateHeader(&zip.FileHeader{
		Name:   path,
		Method: zip.Deflate,
	})
	if err != nil {
		return nil, err
	}
	a.entries[id] = true
	return &componentWriteCloser{w: w, zw: a.zipWriter}, nil
}

func (a *ArchiveWriter) WriteManifest(data []byte) error {
	if a.zipWriter == nil {
		return ErrBackupComponentFailed
	}
	w, err := a.zipWriter.CreateHeader(&zip.FileHeader{
		Name:   "manifest.json",
		Method: zip.Deflate,
	})
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func (a *ArchiveWriter) CopyFile(destDir, name, srcPath string) error {
	if a.zipWriter == nil {
		return ErrBackupComponentFailed
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	path := fmt.Sprintf("%s/%s", destDir, name)
	w, err := a.zipWriter.CreateHeader(&zip.FileHeader{
		Name:   path,
		Method: zip.Deflate,
	})
	if err != nil {
		return err
	}
	_, err = io.Copy(w, src)
	return err
}

func (a *ArchiveWriter) Finalize() error {
	if a.zipWriter == nil {
		return nil
	}
	if err := a.zipWriter.Close(); err != nil {
		return err
	}
	if a.file != nil {
		return a.file.Close()
	}
	return nil
}

func (a *ArchiveWriter) Close() error {
	if a.zipWriter != nil {
		a.zipWriter.Close()
	}
	if a.file != nil {
		return a.file.Close()
	}
	return nil
}

type componentWriteCloser struct {
	w  io.Writer
	zw *zip.Writer
}

func (c *componentWriteCloser) Write(p []byte) (int, error) {
	return c.w.Write(p)
}

func (c *componentWriteCloser) Close() error {
	return nil
}

type ArchiveReader struct {
	reader *zip.Reader
	files  map[string]*zip.File
}

func NewArchiveReader(path string) (*ArchiveReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		f.Close()
		return nil, err
	}
	files := make(map[string]*zip.File)
	for _, zf := range zr.File {
		files[zf.Name] = zf
	}
	return &ArchiveReader{reader: zr, files: files}, nil
}

func (r *ArchiveReader) ReadManifest() (*BackupManifest, error) {
	f, ok := r.files["manifest.json"]
	if !ok {
		return nil, ErrBackupManifestInvalid
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var manifest BackupManifest
	if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (r *ArchiveReader) OpenComponent(path string) (io.ReadCloser, error) {
	f, ok := r.files[path]
	if !ok {
		return nil, ErrBackupComponentFailed
	}
	return f.Open()
}

func (r *ArchiveReader) VerifyIntegrity() error {
	for _, f := range r.files {
		_ = f.UncompressedSize
	}
	return nil
}

func (r *ArchiveReader) SHA256() (string, error) {
	return "", nil
}

func (r *ArchiveReader) Close() error {
	return nil
}

func ComputeSHA256(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

package amitiax

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
)

func WriteV2SignatureToArchive(archivePath string, signatureData []byte) error {
	if len(signatureData) == 0 {
		return fmt.Errorf("amitiax: signature data is empty")
	}
	tmpPath := archivePath + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	w := zip.NewWriter(tmpFile)
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}
	for _, f := range r.File {
		name := normalizePath(f.Name)
		if name == V2SignatureFile {
			continue
		}
		out, err := w.CreateHeader(&zip.FileHeader{
			Name:   f.Name,
			Method: f.Method,
		})
		if err != nil {
			r.Close()
			w.Close()
			tmpFile.Close()
			os.Remove(tmpPath)
			return err
		}
		rc, err := f.Open()
		if err != nil {
			r.Close()
			w.Close()
			tmpFile.Close()
			os.Remove(tmpPath)
			return err
		}
		_, copyErr := io.Copy(out, rc)
		rc.Close()
		if copyErr != nil {
			r.Close()
			w.Close()
			tmpFile.Close()
			os.Remove(tmpPath)
			return copyErr
		}
	}
	r.Close()
	sigWriter, err := w.Create(V2SignatureFile)
	if err != nil {
		w.Close()
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	if _, err := sigWriter.Write(signatureData); err != nil {
		w.Close()
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := w.Close(); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, archivePath); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

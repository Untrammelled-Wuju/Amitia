package util

import (
	"archive/zip"
	"fmt"
	"io"
	amitialog "log"
	"os"
	"path/filepath"
	"strings"
)

func UnzipFile(zipPath, targetDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	targetDir, err = filepath.Abs(targetDir)
	if err != nil {
		return err
	}

	for _, file := range reader.File {
		targetPath := filepath.Join(targetDir, file.Name)
		cleanPath, err := filepath.Abs(targetPath)
		if err != nil {
			return err
		}
		if cleanPath != targetDir && !strings.HasPrefix(cleanPath, targetDir+string(os.PathSeparator)) {
			return fmt.Errorf("非法压缩包路径: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(cleanPath, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(cleanPath), 0755); err != nil {
			return err
		}
		if err := extractZipEntry(file, cleanPath); err != nil {
			if err := os.Remove(cleanPath); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return fmt.Errorf("解压 %s 失败: %w", file.Name, err)
		}
	}
	return nil
}

func extractZipEntry(file *zip.File, targetPath string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	mode := file.Mode().Perm()
	if mode == 0 {
		mode = 0644
	}
	dst, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(dst, src)
	syncErr := dst.Sync()
	closeErr := dst.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	if uint64(written) != file.UncompressedSize64 {
		return fmt.Errorf("解压字节数不完整: got %d want %d", written, file.UncompressedSize64)
	}
	return nil
}

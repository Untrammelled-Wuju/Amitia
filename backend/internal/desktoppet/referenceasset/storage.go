// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package referenceasset

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrPathTraversal   = errors.New("path traversal detected")
	ErrDirCreateFailed = errors.New("failed to create directory")
	ErrTempFileFailed  = errors.New("failed to create temp file")
	ErrRenameFailed    = errors.New("failed to rename file")
	ErrVerifyFailed    = errors.New("failed to verify output file")
)

func EnsureDir(dir string) error {
	if err := validatePath(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("%w: %v", ErrDirCreateFailed, err)
	}
	return nil
}

func WriteAtomically(path string, data []byte) error {
	if err := validatePath(path); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("%w: %v", ErrDirCreateFailed, err)
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("%w: %v", ErrTempFileFailed, err)
	}

	if err := verifyDecodable(tempPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("%w: %v", ErrVerifyFailed, err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("%w: %v", ErrRenameFailed, err)
	}

	return nil
}

func verifyDecodable(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, _, err = image.Decode(f)
	return err
}

func extensionForMIME(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}

func validatePath(path string) error {
	if path == "" {
		return ErrPathTraversal
	}
	clean := filepath.Clean(path)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return ErrPathTraversal
	}
	return nil
}

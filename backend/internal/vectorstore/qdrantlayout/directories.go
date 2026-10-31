// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantlayout

import (
	"context"
	amitialog "log"
	"os"
	"runtime"
)

type DirectoryManager interface {
	Ensure(context.Context, Layout) error
}

type directoryManager struct {
	creator DirectoryCreator
}

func NewDirectoryManager(creator DirectoryCreator) DirectoryManager {
	if creator == nil {
		creator = newDefaultDirectoryCreator()
	}
	return &directoryManager{creator: creator}
}

func (m *directoryManager) Ensure(ctx context.Context, layout Layout) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	dirs := []string{
		layout.ConfigRoot,
		layout.DataRoot,
		layout.StorageDir,
		layout.SnapshotsDir,
		layout.MigrationDir,
	}

	created := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if err := m.ensureOne(dir); err != nil {
			m.rollback(created)
			return newDirectoryCreation(dir, err)
		}
		created = append(created, dir)
	}
	return nil
}

func (m *directoryManager) ensureOne(path string) error {
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return newInvalidLayout("path exists but is not a directory: " + path)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	perm := os.FileMode(0750)
	if runtime.GOOS == "windows" {
		perm = 0755
	}
	if err := m.creator.MkdirAll(path, perm); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0750); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
	return nil
}

func (m *directoryManager) rollback(created []string) {
	for i := len(created) - 1; i >= 0; i-- {
		path := created[i]
		if isEmpty, err := isEmptyDir(path); err == nil && isEmpty {
			if err := os.Remove(path); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
		}
	}
}

func isEmptyDir(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	if err != nil {
		return true, nil
	}
	return false, nil
}

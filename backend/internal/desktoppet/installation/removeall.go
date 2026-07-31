// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func removeTree(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	err := os.RemoveAll(path)
	if err == nil {
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			return nil
		}
	}

	var paths []string
	walkErr := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, p)
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("walk directory failed: %s: %w", path, walkErr)
	}

	sort.Sort(sort.Reverse(sort.StringSlice(paths)))

	for _, p := range paths {
		os.Remove(p)
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("failed to remove directory: %s", path)
	}
	return nil
}

package trusted_service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func (s *ProcessSupervisor) CleanupOwnedRuntime(extensionID string) error {
	root := filepath.Join(s.rootDir, "services")
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		if relative != "." && len(strings.Split(relative, string(os.PathSeparator))) > 2 {
			return filepath.SkipDir
		}
		data, err := os.ReadFile(filepath.Join(current, ".amitia-owned-runtime.json"))
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		var receipt ownedRuntimeReceipt
		if err := json.Unmarshal(data, &receipt); err != nil {
			return err
		}
		if receipt.ExtensionID != extensionID {
			return filepath.SkipDir
		}
		if err := cleanupOwnedRuntime(current, extensionID); err != nil {
			return err
		}
		return filepath.SkipDir
	})
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

type ownedRuntimeReceipt struct {
	ExtensionID string   `json:"extensionId"`
	Paths       []string `json:"paths"`
	LinksFile   string   `json:"linksFile"`
}

func ownedRuntimePath(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", fmt.Errorf("invalid owned runtime path")
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("owned runtime path escapes working directory")
	}
	current := root
	parts := strings.Split(clean, string(os.PathSeparator))
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			break
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("owned runtime parent is a symlink")
		}
	}
	return filepath.Join(root, clean), nil
}

func cleanupOwnedRuntime(root, extensionID string) error {
	file := filepath.Join(root, ".amitia-owned-runtime.json")
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var receipt ownedRuntimeReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return err
	}
	if receipt.ExtensionID != extensionID {
		return fmt.Errorf("owned runtime identity mismatch")
	}
	paths := make([]string, 0, len(receipt.Paths))
	for _, relative := range receipt.Paths {
		owned, err := ownedRuntimePath(root, relative)
		if err != nil {
			return err
		}
		paths = append(paths, owned)
	}
	if receipt.LinksFile != "" {
		linksFile, err := ownedRuntimePath(root, receipt.LinksFile)
		if err != nil {
			return err
		}
		linksData, err := os.ReadFile(linksFile)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil {
			var links []struct {
				Target string `json:"target"`
				Source string `json:"source"`
			}
			if err := json.Unmarshal(linksData, &links); err != nil {
				return err
			}
			for _, link := range links {
				owned := false
				for _, candidate := range paths {
					if strings.HasPrefix(filepath.Clean(link.Source), candidate+string(os.PathSeparator)) {
						owned = true
						break
					}
				}
				if !owned {
					return fmt.Errorf("unowned runtime link source")
				}
				actual, err := os.Readlink(link.Target)
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				if err != nil {
					continue
				}
				if actual == link.Source {
					if err := os.Remove(link.Target); err != nil {
						return err
					}
				}
			}
		}
	}
	for _, owned := range paths {
		if err := os.RemoveAll(owned); err != nil {
			return err
		}
	}
	return os.Remove(file)
}

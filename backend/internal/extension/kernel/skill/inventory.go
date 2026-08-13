package skill

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ResourceInventory struct {
	Items          []ResourceItem `json:"items"`
	ScriptsPresent bool           `json:"scriptsPresent"`
	CountByType    map[string]int `json:"countByType"`
}

const (
	MaxResourceEntries   = 256
	MaxResourceEntrySize = 1 << 20
)

func CollectResourceInventory(rootDir string, policy ParsePolicy) (ResourceInventory, error) {
	inv := ResourceInventory{
		Items:       []ResourceItem{},
		CountByType: map[string]int{},
	}

	if !policy.CollectResourceIndex {
		return inv, nil
	}

	if rootDir == "" {
		return inv, nil
	}

	info, err := os.Stat(rootDir)
	if err != nil {
		return inv, fmt.Errorf("SKILL_RESOURCE_PATH_INVALID: cannot stat skill root: %w", err)
	}
	if !info.IsDir() {
		return inv, fmt.Errorf("SKILL_RESOURCE_PATH_INVALID: skill root is not a directory")
	}

	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return inv, fmt.Errorf("SKILL_RESOURCE_PATH_INVALID: cannot read skill root: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == "SKILL.md" || strings.HasPrefix(name, ".") {
			continue
		}

		fullPath := filepath.Join(rootDir, name)

		if entry.Type()&fs.ModeSymlink != 0 {
			continue
		}

		if entry.IsDir() {
			switch name {
			case "scripts":
				inv.ScriptsPresent = true
				inv.addKindItems(fullPath, name, ResourceKindScript, &inv)
			case "references":
				inv.addKindItems(fullPath, name, ResourceKindReference, &inv)
			case "assets":
				inv.addKindItems(fullPath, name, ResourceKindAsset, &inv)
			default:
				inv.addKindItems(fullPath, name, ResourceKindOther, &inv)
			}
		} else {
			inv.addItem(name, ResourceKindOther, fullPath)
		}

		if len(inv.Items) > MaxResourceEntries {
			return inv, fmt.Errorf("SKILL_RESOURCE_LIMIT_EXCEEDED")
		}
	}

	sort.Slice(inv.Items, func(i, j int) bool {
		return inv.Items[i].Path < inv.Items[j].Path
	})

	return inv, nil
}

func (inv *ResourceInventory) addKindItems(rootDir, relPrefix, kind string, target *ResourceInventory) {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Type()&fs.ModeSymlink != 0 {
			continue
		}
		subRel := relPrefix + "/" + e.Name()
		subPath := filepath.Join(rootDir, e.Name())
		if e.IsDir() {
			target.addKindItems(subPath, subRel, kind, target)
		} else {
			target.addItem(subRel, kind, subPath)
		}
		if len(target.Items) > MaxResourceEntries {
			return
		}
	}
}

func (inv *ResourceInventory) addItem(relPath, kind, fullPath string) {
	info, err := os.Stat(fullPath)
	if err != nil {
		return
	}
	if info.Size() > MaxResourceEntrySize {
		return
	}
	inv.Items = append(inv.Items, ResourceItem{
		Path:      relPath,
		Kind:      kind,
		SizeBytes: info.Size(),
	})
	inv.CountByType[kind]++
}

func ValidateResourcePath(skillRoot, relPath string) error {
	if relPath == "" {
		return fmt.Errorf("SKILL_RESOURCE_PATH_INVALID: empty path")
	}
	if filepath.IsAbs(relPath) {
		return fmt.Errorf("SKILL_RESOURCE_PATH_INVALID: absolute paths not allowed")
	}
	if len(relPath) > 0 && relPath[0] == '/' {
		return fmt.Errorf("SKILL_RESOURCE_PATH_INVALID: absolute paths not allowed")
	}
	if strings.Contains(relPath, "..") {
		return fmt.Errorf("SKILL_RESOURCE_PATH_INVALID: path traversal not allowed")
	}
	clean := filepath.Clean(relPath)
	if strings.HasPrefix(clean, "..") {
		return fmt.Errorf("SKILL_RESOURCE_PATH_INVALID: path traversal not allowed")
	}
	full := filepath.Join(skillRoot, clean)
	rootClean := filepath.Clean(skillRoot)
	if !strings.HasPrefix(full, rootClean+string(os.PathSeparator)) && full != rootClean {
		return fmt.Errorf("SKILL_RESOURCE_PATH_INVALID: path escapes skill root")
	}
	return nil
}

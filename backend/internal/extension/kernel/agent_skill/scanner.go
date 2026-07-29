package agent_skill

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

type ResourceScanner struct {
	allowedExtensions map[string]bool
	forbiddenPaths    []string
}

func NewResourceScanner() *ResourceScanner {
	return &ResourceScanner{
		allowedExtensions: map[string]bool{
			".md": true, ".txt": true, ".yaml": true, ".yml": true,
			".json": true, ".csv": true, ".xml": true,
			".png": true, ".jpg": true, ".jpeg": true, ".svg": true,
			".html": true, ".css": true,
		},
		forbiddenPaths: []string{
			"..", "~", "\\", "//", ".env", ".git", "node_modules",
		},
	}
}

func (s *ResourceScanner) ScanPaths(paths []string) ([]SkillResourceDescriptor, error) {
	var resources []SkillResourceDescriptor
	seen := map[string]bool{}

	for _, path := range paths {
		normalized := filepath.ToSlash(filepath.Clean(path))
		if normalized == "" || normalized == "." {
			continue
		}

		if err := s.validatePath(normalized); err != nil {
			return nil, fmt.Errorf("invalid resource path %q: %w", normalized, err)
		}

		if seen[normalized] {
			return nil, fmt.Errorf("duplicate resource path %q", normalized)
		}
		seen[normalized] = true

		kind := classifyResourceKind(normalized)
		resources = append(resources, SkillResourceDescriptor{
			Path:         normalized,
			Kind:         kind,
			TextReadable: isTextReadable(normalized),
		})
	}

	return resources, nil
}

func (s *ResourceScanner) validatePath(path string) error {
	if filepath.IsAbs(path) {
		return fmt.Errorf("absolute paths are not allowed")
	}

	lower := strings.ToLower(path)
	for _, forbidden := range s.forbiddenPaths {
		if strings.Contains(lower, forbidden) {
			return fmt.Errorf("path contains forbidden segment %q", forbidden)
		}
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" && !s.allowedExtensions[ext] {
		return fmt.Errorf("file extension %q is not allowed", ext)
	}

	return nil
}

func classifyResourceKind(path string) SkillResourceKind {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	switch ext {
	case ".md", ".txt":
		if strings.Contains(base, "reference") || strings.Contains(base, "rule") {
			return KindReference
		}
		return KindReference
	case ".yaml", ".yml", ".json":
		return KindConfig
	case ".html", ".css":
		return KindTemplate
	case ".js", ".ts", ".py", ".sh", ".ps1":
		return KindScript
	case ".csv", ".xml", ".tsv":
		return KindData
	default:
		return KindAsset
	}
}

func isTextReadable(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".txt", ".yaml", ".yml", ".json", ".csv", ".xml", ".html", ".css", ".tsv":
		return true
	case ".js", ".ts", ".py", ".sh", ".ps1":
		return true
	}
	return false
}

type ResourceIndexer struct{}

func NewResourceIndexer() *ResourceIndexer {
	return &ResourceIndexer{}
}

type ResourceIndex struct {
	Resources     []SkillResourceDescriptor `json:"resources"`
	TotalSize     int64                     `json:"totalSize"`
	TotalTokens   int                       `json:"totalTokens"`
	Hash          string                    `json:"hash"`
	ResourceCount int                       `json:"resourceCount"`
}

func (idx *ResourceIndexer) BuildIndex(resources []SkillResourceDescriptor) ResourceIndex {
	index := ResourceIndex{
		Resources:     resources,
		ResourceCount: len(resources),
	}

	var totalSize int64
	var totalTokens int
	for _, r := range resources {
		totalSize += r.Size
		totalTokens += r.TokenEstimate
	}
	index.TotalSize = totalSize
	index.TotalTokens = totalTokens
	index.Hash = computeIndexHash(resources)

	return index
}

func (idx *ResourceIndexer) Diff(previous, current []SkillResourceDescriptor) (added, removed, modified []SkillResourceDescriptor) {
	prevMap := map[string]SkillResourceDescriptor{}
	for _, r := range previous {
		prevMap[r.Path] = r
	}
	currMap := map[string]SkillResourceDescriptor{}
	for _, r := range current {
		currMap[r.Path] = r
	}

	for path, curr := range currMap {
		prev, existed := prevMap[path]
		if !existed {
			added = append(added, curr)
		} else if ComputeResourceHash(prev) != ComputeResourceHash(curr) {
			modified = append(modified, curr)
		}
	}

	for path, prev := range prevMap {
		if _, stillExists := currMap[path]; !stillExists {
			removed = append(removed, prev)
		}
	}

	return
}

func computeIndexHash(resources []SkillResourceDescriptor) string {
	h := sha256.New()
	for _, r := range resources {
		h.Write([]byte(r.Path))
		h.Write([]byte(r.SHA256))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package resourceuri

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/u-ai/backend/pkg/util"
)

type PhysicalRoots struct {
	Workspace   string
	Attachments string
	Data        string
	Cache       string
	Runtime     string
	Config      string
	Extensions  string
	Logs        string
	Temp        string
}

func PhysicalRootsFromRuntimePaths(paths util.RuntimePaths) PhysicalRoots {
	attachments := ""
	extensions := ""
	if paths.DataDir != "" {
		attachments = filepath.Join(paths.DataDir, "attachments")
		extensions = filepath.Join(paths.DataDir, "extensions")
	}
	return PhysicalRoots{
		Workspace:   paths.WorkspaceDir,
		Attachments: attachments,
		Data:        paths.DataDir,
		Cache:       paths.CacheDir,
		Runtime:     paths.Root,
		Config:      paths.ConfigDir,
		Extensions:  extensions,
		Logs:        paths.LogDir,
		Temp:        paths.TempDir,
	}
}

type PhysicalResolver struct {
	rootFor map[ResourceRoot]string
	keys    []ResourceRoot
}

var defaultPriority = []ResourceRoot{
	ResourceRootWorkspace,
	ResourceRootAttachments,
	ResourceRootExtensions,
	ResourceRootCache,
	ResourceRootTemp,
	ResourceRootConfig,
	ResourceRootLogs,
	ResourceRootData,
	ResourceRootRuntime,
}

func NewPhysicalResolver(roots PhysicalRoots) (*PhysicalResolver, error) {
	r := &PhysicalResolver{
		rootFor: make(map[ResourceRoot]string),
	}
	set := func(root ResourceRoot, p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = filepath.Clean(p)
		}
		r.rootFor[root] = filepath.Clean(abs)
	}
	set(ResourceRootWorkspace, roots.Workspace)
	set(ResourceRootAttachments, roots.Attachments)
	set(ResourceRootData, roots.Data)
	set(ResourceRootCache, roots.Cache)
	set(ResourceRootRuntime, roots.Runtime)
	set(ResourceRootConfig, roots.Config)
	set(ResourceRootExtensions, roots.Extensions)
	set(ResourceRootLogs, roots.Logs)
	set(ResourceRootTemp, roots.Temp)

	r.rebuildKeys()
	return r, nil
}

func (r *PhysicalResolver) rebuildKeys() {
	type item struct {
		root ResourceRoot
		len  int
	}
	items := make([]item, 0, len(r.rootFor))
	for k, v := range r.rootFor {
		items = append(items, item{root: k, len: len(v)})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].len != items[j].len {
			return items[i].len > items[j].len
		}
		return priorityIndex(items[i].root) < priorityIndex(items[j].root)
	})
	r.keys = make([]ResourceRoot, len(items))
	for i, it := range items {
		r.keys[i] = it.root
	}
}

func priorityIndex(root ResourceRoot) int {
	for i, p := range defaultPriority {
		if p == root {
			return i
		}
	}
	return len(defaultPriority)
}

func (r *PhysicalResolver) Resolve(uri ResourceURI) (ResolvedResource, error) {
	if uri.Root().IsVirtual() {
		return ResolvedResource{
			URI:       uri,
			Kind:      ResourceKindVirtual,
			LocalPath: "",
		}, ErrNonFilesystemResource
	}
	rootPath, ok := r.rootFor[uri.Root()]
	if !ok || rootPath == "" {
		return ResolvedResource{}, fmt.Errorf("%q: %w", uri.Root(), ErrRootNotConfigured)
	}
	segments := splitRelativePath(uri.RelativePath())
	joined := rootPath
	if len(segments) > 0 {
		joined = filepath.Join(rootPath, filepath.Join(segments...))
	}
	cleaned := filepath.Clean(joined)
	if err := assertWithinRoot(rootPath, cleaned); err != nil {
		return ResolvedResource{}, err
	}
	return ResolvedResource{
		URI:       uri,
		Kind:      ResourceKindFilesystem,
		LocalPath: cleaned,
	}, nil
}

func (r *PhysicalResolver) Reverse(localPath string) (ResourceURI, error) {
	trimmed := strings.TrimSpace(localPath)
	if trimmed == "" {
		return ResourceURI{}, fmt.Errorf("empty path: %w", ErrInvalidPath)
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		abs = filepath.Clean(trimmed)
	}
	abs = filepath.Clean(abs)

	var matched ResourceRoot
	found := false
	for _, root := range r.keys {
		rootPath := r.rootFor[root]
		rel, err := filepath.Rel(rootPath, abs)
		if err != nil {
			continue
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		matched = root
		found = true
		break
	}
	if !found {
		return ResourceURI{}, fmt.Errorf("%q: %w", localPath, ErrResourceOutsideRoots)
	}
	rootPath := r.rootFor[matched]
	rel, _ := filepath.Rel(rootPath, abs)
	rel = strings.ReplaceAll(rel, string(filepath.Separator), "/")
	rel = strings.TrimPrefix(rel, "./")
	if rel == "." {
		rel = ""
	}
	return ResourceURI{
		root:         matched,
		relativePath: rel,
	}, nil
}

func splitRelativePath(rel string) []string {
	if rel == "" {
		return nil
	}
	parts := strings.Split(rel, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" && p != "." {
			out = append(out, p)
		}
	}
	return out
}

func assertWithinRoot(rootPath, childPath string) error {
	rel, err := filepath.Rel(rootPath, childPath)
	if err != nil {
		return fmt.Errorf("%q: %w", childPath, ErrPathTraversal)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%q: %w", childPath, ErrPathTraversal)
	}
	return nil
}

package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/u-ai/backend/internal/workspace"
	"github.com/u-ai/backend/pkg/resourceuri"
)

type ResourceMaterializer interface {
	Materialize(context.Context, string) (string, func() error, error)
	Publish(context.Context, string, string) error
	URIForLocalPath(string) (string, error)
}

type resourceMaterializer struct {
	resolver  resourceuri.ResourceResolver
	workspace *workspace.Service
	tempDir   string
}

func NewResourceMaterializer(resolver resourceuri.ResourceResolver, workspaceService *workspace.Service, tempDir string) ResourceMaterializer {
	return &resourceMaterializer{resolver: resolver, workspace: workspaceService, tempDir: tempDir}
}

func (m *resourceMaterializer) Materialize(ctx context.Context, rawURI string) (string, func() error, error) {
	uri, err := resourceuri.Parse(rawURI)
	if err != nil {
		return "", nil, err
	}
	if uri.Root() != resourceuri.ResourceRootWorkspace {
		if m.resolver == nil {
			return "", nil, fmt.Errorf("resource resolver not configured")
		}
		resolved, err := m.resolver.Resolve(uri)
		if err != nil {
			return "", nil, err
		}
		info, err := os.Stat(resolved.LocalPath)
		if err != nil {
			return "", nil, err
		}
		if info.IsDir() {
			return "", nil, fmt.Errorf("resource is a directory")
		}
		return resolved.LocalPath, func() error { return nil }, nil
	}
	if m.workspace == nil {
		return "", nil, fmt.Errorf("workspace service not configured")
	}
	if err := os.MkdirAll(m.tempDir, 0o755); err != nil {
		return "", nil, err
	}
	f, err := os.CreateTemp(m.tempDir, "media_materialized_*")
	if err != nil {
		return "", nil, err
	}
	path := f.Name()
	if err := m.workspace.CopyTo(ctx, rawURI, f); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", nil, err
	}
	return path, func() error { return os.Remove(path) }, nil
}

func (m *resourceMaterializer) Publish(ctx context.Context, stagingPath string, targetURI string) error {
	uri, err := resourceuri.Parse(targetURI)
	if err != nil {
		return err
	}
	if uri.Root() != resourceuri.ResourceRootWorkspace {
		if m.resolver == nil {
			return fmt.Errorf("resource resolver not configured")
		}
		resolved, err := m.resolver.Resolve(uri)
		if err != nil {
			return err
		}
		if filepath.Clean(stagingPath) == filepath.Clean(resolved.LocalPath) {
			return nil
		}
		return NewResourceIO(m.tempDir).AtomicCommit(stagingPath, resolved.LocalPath)
	}
	if m.workspace == nil {
		return fmt.Errorf("workspace service not configured")
	}
	f, err := os.Open(stagingPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := m.workspace.Write(ctx, targetURI, f, workspace.WriteOptions{Overwrite: true, Atomic: true}); err != nil {
		return err
	}
	return os.Remove(stagingPath)
}

func (m *resourceMaterializer) URIForLocalPath(path string) (string, error) {
	if m.resolver == nil {
		return "", fmt.Errorf("resource resolver not configured")
	}
	uri, err := m.resolver.Reverse(path)
	if err != nil {
		return "", err
	}
	return uri.String(), nil
}

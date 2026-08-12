package workspace

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/u-ai/backend/pkg/resourceuri"
)

type Service struct {
	registry   *Registry
	physicalResolver PhysicalResolverProvider
}

type PhysicalResolverProvider interface {
	Resolve(uri resourceuri.ResourceURI) (resourceuri.ResolvedResource, error)
}

func NewService(registry *Registry, provider PhysicalResolverProvider) *Service {
	return &Service{
		registry:   registry,
		physicalResolver: provider,
	}
}

func (s *Service) ListMounts(ctx context.Context) ([]WorkspaceMount, error) {
	return s.registry.ListMounts(), nil
}

func (s *Service) Stat(ctx context.Context, uriStr string) (WorkspaceEntry, error) {
	mount, rel, err := s.resolveURIToMount(uriStr)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	backend, ok := s.registry.GetBackend(mount.Kind)
	if !ok {
		return WorkspaceEntry{}, fmt.Errorf("%w: no backend for kind %q", ErrMountUnavailable, mount.Kind)
	}
	return backend.Stat(ctx, mount, rel)
}

func (s *Service) List(ctx context.Context, uriStr string, opts ListOptions) (ListResult, error) {
	mount, rel, err := s.resolveURIToMount(uriStr)
	if err != nil {
		return ListResult{}, err
	}
	backend, ok := s.registry.GetBackend(mount.Kind)
	if !ok {
		return ListResult{}, fmt.Errorf("%w: no backend for kind %q", ErrMountUnavailable, mount.Kind)
	}
	entries, err := backend.List(ctx, mount, rel, opts)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{
		Entries: entries,
	}, nil
}

func (s *Service) Read(ctx context.Context, uriStr string, opts ReadOptions) (ReadResult, error) {
	mount, rel, err := s.resolveURIToMount(uriStr)
	if err != nil {
		return ReadResult{}, err
	}
	backend, ok := s.registry.GetBackend(mount.Kind)
	if !ok {
		return ReadResult{}, fmt.Errorf("%w: no backend for kind %q", ErrMountUnavailable, mount.Kind)
	}
	return backend.Read(ctx, mount, rel, opts)
}

func (s *Service) Write(ctx context.Context, uriStr string, src io.Reader, opts WriteOptions) (WorkspaceEntry, error) {
	mount, rel, err := s.resolveURIToMount(uriStr)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	if mount.ReadOnly {
		return WorkspaceEntry{}, ErrReadOnly
	}
	backend, ok := s.registry.GetBackend(mount.Kind)
	if !ok {
		return WorkspaceEntry{}, fmt.Errorf("%w: no backend for kind %q", ErrMountUnavailable, mount.Kind)
	}
	return backend.Write(ctx, mount, rel, src, opts)
}

func (s *Service) Mkdir(ctx context.Context, uriStr string) (WorkspaceEntry, error) {
	mount, rel, err := s.resolveURIToMount(uriStr)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	if mount.ReadOnly {
		return WorkspaceEntry{}, ErrReadOnly
	}
	backend, ok := s.registry.GetBackend(mount.Kind)
	if !ok {
		return WorkspaceEntry{}, fmt.Errorf("%w: no backend for kind %q", ErrMountUnavailable, mount.Kind)
	}
	return backend.Mkdir(ctx, mount, rel)
}

func (s *Service) Rename(ctx context.Context, uriStr string, newName string) (WorkspaceEntry, error) {
	mount, rel, err := s.resolveURIToMount(uriStr)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	if mount.ReadOnly {
		return WorkspaceEntry{}, ErrReadOnly
	}
	backend, ok := s.registry.GetBackend(mount.Kind)
	if !ok {
		return WorkspaceEntry{}, fmt.Errorf("%w: no backend for kind %q", ErrMountUnavailable, mount.Kind)
	}
	return backend.Rename(ctx, mount, rel, newName)
}

func (s *Service) Move(ctx context.Context, sourceURI, destDirURI string) (WorkspaceEntry, error) {
	srcMount, srcRel, err := s.resolveURIToMount(sourceURI)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	dstMount, dstRel, err := s.resolveURIToMount(destDirURI)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	if srcMount.ID != dstMount.ID {
		return WorkspaceEntry{}, ErrCrossMountMoveUnsupported
	}
	if srcMount.ReadOnly {
		return WorkspaceEntry{}, ErrReadOnly
	}
	backend, ok := s.registry.GetBackend(srcMount.Kind)
	if !ok {
		return WorkspaceEntry{}, fmt.Errorf("%w: no backend for kind %q", ErrMountUnavailable, srcMount.Kind)
	}
	return backend.Move(ctx, srcMount, srcRel, dstRel)
}

func (s *Service) Copy(ctx context.Context, sourceURI, destDirURI string) (WorkspaceEntry, error) {
	srcMount, srcRel, err := s.resolveURIToMount(sourceURI)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	dstMount, dstRel, err := s.resolveURIToMount(destDirURI)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	if srcMount.ID != dstMount.ID {
		return WorkspaceEntry{}, ErrCrossMountCopyUnsupported
	}
	if dstMount.ReadOnly {
		return WorkspaceEntry{}, ErrReadOnly
	}
	backend, ok := s.registry.GetBackend(srcMount.Kind)
	if !ok {
		return WorkspaceEntry{}, fmt.Errorf("%w: no backend for kind %q", ErrMountUnavailable, srcMount.Kind)
	}
	return backend.Copy(ctx, srcMount, srcRel, dstRel)
}

func (s *Service) Delete(ctx context.Context, uriStr string, opts DeleteOptions) error {
	mount, rel, err := s.resolveURIToMount(uriStr)
	if err != nil {
		return err
	}
	if mount.ReadOnly {
		return ErrReadOnly
	}
	if rel == "" {
		return ErrRootMutationDenied
	}
	backend, ok := s.registry.GetBackend(mount.Kind)
	if !ok {
		return fmt.Errorf("%w: no backend for kind %q", ErrMountUnavailable, mount.Kind)
	}
	return backend.Delete(ctx, mount, rel, opts)
}

func (s *Service) resolveURIToMount(uriStr string) (WorkspaceMount, string, error) {
	uri, err := resourceuri.Parse(uriStr)
	if err != nil {
		return WorkspaceMount{}, "", fmt.Errorf("%w: %v", ErrInvalidURI, err)
	}
	if uri.Root() != resourceuri.ResourceRootWorkspace {
		return WorkspaceMount{}, "", fmt.Errorf("%w: root must be workspace", ErrInvalidURI)
	}
	rel := uri.RelativePath()
	if strings.HasPrefix(rel, "@") {
		slashIdx := strings.Index(rel, "/")
		if slashIdx < 0 {
			return WorkspaceMount{}, "", fmt.Errorf("%w: invalid mount URI", ErrInvalidURI)
		}
		mountID := WorkspaceID(rel[1:slashIdx])
		pathPart := rel[slashIdx+1:]
		mount, ok := s.registry.GetMount(mountID)
		if !ok {
			return WorkspaceMount{}, "", fmt.Errorf("%w: mount %q not found", ErrMountNotFound, mountID)
		}
		return mount, pathPart, nil
	}
	return s.defaultMount(), rel, nil
}

func (s *Service) defaultMount() WorkspaceMount {
	return WorkspaceMount{
		ID:       "default",
		Name:     "Default",
		Kind:     WorkspaceKindLocal,
		RootURI:  "amitia://workspace/",
		Status:   WorkspaceStatusReady,
		Available: true,
	}
}

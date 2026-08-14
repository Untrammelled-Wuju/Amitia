package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/u-ai/backend/pkg/resourceuri"
)

type Service struct {
	registry           *Registry
	physicalResolver   PhysicalResolverProvider
	mountRepo          *MountRepository
	safGrantResolver   SAFGrantResolver
	remoteCredResolver RemoteCredentialResolver
}

type PhysicalResolverProvider interface {
	Resolve(uri resourceuri.ResourceURI) (resourceuri.ResolvedResource, error)
}

type SAFGrantResolver interface {
	ResolveGrant(grantID string) (SAFGrantStatus, error)
}

func NewService(registry *Registry, provider PhysicalResolverProvider) *Service {
	return &Service{
		registry:         registry,
		physicalResolver: provider,
	}
}

func NewServiceWithPersistence(registry *Registry, provider PhysicalResolverProvider, mountRepo *MountRepository, safGrantResolver SAFGrantResolver) *Service {
	return &Service{
		registry:           registry,
		physicalResolver:   provider,
		mountRepo:          mountRepo,
		safGrantResolver:   safGrantResolver,
		remoteCredResolver: nil,
	}
}

func NewServiceWithRemote(registry *Registry, provider PhysicalResolverProvider, mountRepo *MountRepository, safGrantResolver SAFGrantResolver, remoteCredResolver RemoteCredentialResolver) *Service {
	return &Service{
		registry:           registry,
		physicalResolver:   provider,
		mountRepo:          mountRepo,
		safGrantResolver:   safGrantResolver,
		remoteCredResolver: remoteCredResolver,
	}
}

func (s *Service) ListMounts(ctx context.Context) ([]WorkspaceMount, error) {
	return s.registry.ListMounts(), nil
}

func (s *Service) HasBackend(kind WorkspaceKind) bool {
	return s.registry != nil && s.registry.HasBackend(kind)
}

func (s *Service) RegisterSAFMount(ctx context.Context, name string, grantID string, readOnly bool) (WorkspaceMount, error) {
	if s.safGrantResolver != nil {
		status, err := s.safGrantResolver.ResolveGrant(grantID)
		if err != nil {
			return WorkspaceMount{}, fmt.Errorf("%w: grant validation failed: %v", ErrSAFUnavailable, err)
		}
		if !status.Valid {
			return WorkspaceMount{}, ErrSAFPermissionRevoked
		}
	}

	if s.mountRepo != nil {
		existing, err := s.mountRepo.LoadAll()
		if err == nil {
			for _, rec := range existing {
				if rec.nativeGrant == grantID {
					m, _ := s.registry.GetMountOrEmpty(WorkspaceID(rec.id))
					return m, nil
				}
			}
		}
	}

	mount, err := s.registry.RegisterSAFMount(ctx, name, grantID, readOnly)
	if err != nil {
		return mount, err
	}

	if s.mountRepo != nil {
		rec := persistenceRecord{
			id:          string(mount.ID),
			name:        mount.Name,
			kind:        mount.Kind,
			nativeGrant: grantID,
			readOnly:    readOnly,
			enabled:     true,
			createdAt:   mount.CreatedAt,
			updatedAt:   mount.UpdatedAt,
		}
		if err := s.mountRepo.Insert(rec); err != nil {
			s.registry.RemoveMount(ctx, mount.ID)
			return WorkspaceMount{}, fmt.Errorf("%w: persist mount failed: %v", ErrSAFUnavailable, err)
		}
	}

	return mount, nil
}

func (s *Service) ReplaceSAFGrant(ctx context.Context, mountID WorkspaceID, grantID string, readOnly bool) (WorkspaceMount, error) {
	if s.safGrantResolver != nil {
		status, err := s.safGrantResolver.ResolveGrant(grantID)
		if err != nil {
			return WorkspaceMount{}, fmt.Errorf("%w: grant validation failed: %v", ErrSAFUnavailable, err)
		}
		if !status.Valid {
			return WorkspaceMount{}, ErrSAFPermissionRevoked
		}
	}

	mount, ok := s.registry.ReplaceSAFGrant(mountID, grantID, readOnly)
	if !ok {
		return WorkspaceMount{}, fmt.Errorf("%w: mount %q not found", ErrMountNotFound, mountID)
	}

	if s.mountRepo != nil {
		s.mountRepo.UpdateGrant(string(mountID), grantID, readOnly, string(mount.Status))
	}

	return mount, nil
}

func (s *Service) RemoveMount(ctx context.Context, id WorkspaceID) error {
	if err := s.registry.RemoveMount(ctx, id); err != nil {
		return err
	}
	if s.mountRepo != nil {
		return s.mountRepo.Delete(string(id))
	}
	return nil
}

func (s *Service) RefreshMountStatus(ctx context.Context, id WorkspaceID) (WorkspaceMount, error) {
	mount, ok := s.registry.GetMount(id)
	if !ok {
		return WorkspaceMount{}, fmt.Errorf("%w: %q", ErrMountNotFound, id)
	}
	if mount.Kind == WorkspaceKindSAF && s.safGrantResolver != nil {
		status, err := s.safGrantResolver.ResolveGrant(mount.NativeGrant)
		if err != nil {
			return mount, nil
		}
		newStatus, available := GrantStatusToMountUpdate(mount.NativeGrant, status)
		s.registry.UpdateStatus(id, newStatus, available)
		m, _ := s.registry.GetMountOrEmpty(id)
		return m, nil
	}
	if mount.Kind == WorkspaceKindRemote {
		s.registry.UpdateStatus(id, WorkspaceStatusUnavailable, false)
		m, _ := s.registry.GetMountOrEmpty(id)
		return m, nil
	}
	return mount, nil
}

func (s *Service) RegisterRemoteMount(ctx context.Context, name string, config RemoteMountConfig, credRef string, readOnly bool) (WorkspaceMount, error) {
	if s.remoteCredResolver != nil && credRef != "" {
		_, err := s.remoteCredResolver.ResolveCredential(ctx, credRef)
		if err != nil {
			return WorkspaceMount{}, fmt.Errorf("%w: %v", ErrRemoteCredentialNotFound, err)
		}
	}

	mount, err := s.registry.RegisterRemoteMount(ctx, name, config, credRef, readOnly)
	if err != nil {
		return mount, err
	}

	if s.mountRepo != nil {
		configJSON, _ := json.Marshal(config)
		rec := persistenceRecord{
			id:            string(mount.ID),
			name:          mount.Name,
			kind:          mount.Kind,
			readOnly:      readOnly,
			enabled:       true,
			createdAt:     mount.CreatedAt,
			updatedAt:     mount.UpdatedAt,
			backendConfig: string(configJSON),
			credentialRef: credRef,
		}
		if err := s.mountRepo.Insert(rec); err != nil {
			s.registry.RemoveMount(ctx, mount.ID)
			return WorkspaceMount{}, fmt.Errorf("%w: persist remote mount failed: %v", ErrRemoteUnavailable, err)
		}
	}

	backend, ok := s.registry.GetBackend(WorkspaceKindRemote)
	if ok {
		if remoteBackend, isRemote := backend.(*RemoteBackend); isRemote {
			remoteBackend.SetStatusUpdater(mount.ID, func(status WorkspaceStatus, reason string) {
				s.registry.UpdateStatus(mount.ID, status, status == WorkspaceStatusReady || status == WorkspaceStatusReadOnly)
			})
		}
	}

	return mount, nil
}

func (s *Service) UpdateRemoteMountConfig(ctx context.Context, mountID WorkspaceID, config RemoteMountConfig, credRef string, readOnly bool) (WorkspaceMount, error) {
	mount, ok := s.registry.GetMount(mountID)
	if !ok {
		return WorkspaceMount{}, fmt.Errorf("%w: %q", ErrMountNotFound, mountID)
	}
	if mount.Kind != WorkspaceKindRemote {
		return WorkspaceMount{}, fmt.Errorf("%w: not a remote mount", ErrRemoteConfigInvalid)
	}

	if s.remoteCredResolver != nil && credRef != "" {
		_, err := s.remoteCredResolver.ResolveCredential(ctx, credRef)
		if err != nil {
			return WorkspaceMount{}, fmt.Errorf("%w: %v", ErrRemoteCredentialNotFound, err)
		}
	}

	updated, ok := s.registry.UpdateRemoteMountConfig(mountID, config, credRef, readOnly)
	if !ok {
		return WorkspaceMount{}, fmt.Errorf("%w: update failed", ErrRemoteUnavailable)
	}

	backend, ok := s.registry.GetBackend(WorkspaceKindRemote)
	if ok {
		if remoteBackend, isRemote := backend.(*RemoteBackend); isRemote {
			remoteBackend.clients.invalidate(mountID)
		}
	}

	if s.mountRepo != nil {
		configJSON, _ := json.Marshal(config)
		_ = s.mountRepo.UpdateConfig(string(mountID), string(configJSON), credRef, readOnly)
	}

	return updated, nil
}

func (s *Service) LoadAndRestoreMounts(ctx context.Context) error {
	if s.mountRepo == nil {
		return nil
	}
	records, err := s.mountRepo.LoadAll()
	if err != nil {
		return fmt.Errorf("%w: load mounts: %v", ErrSAFUnavailable, err)
	}
	for _, rec := range records {
		mount := WorkspaceMount{
			ID:            WorkspaceID(rec.id),
			Name:          rec.name,
			Kind:          rec.kind,
			ReadOnly:      rec.readOnly,
			Available:     true,
			Status:        WorkspaceStatusReady,
			RootURI:       MountURI(WorkspaceID(rec.id)),
			NativeGrant:   rec.nativeGrant,
			BackendConfig: rec.backendConfig,
			CredentialRef: rec.credentialRef,
			CreatedAt:     rec.createdAt,
			UpdatedAt:     rec.updatedAt,
		}
		switch rec.kind {
		case WorkspaceKindLocal:
			if _, err := s.registry.RegisterLocalMount(ctx, rec.name, rec.localRoot, rec.readOnly); err == nil {
			}
		case WorkspaceKindSAF:
			mount.Available = false
			mount.Status = WorkspaceStatusUnavailable
			if s.safGrantResolver != nil {
				status, statusErr := s.safGrantResolver.ResolveGrant(rec.nativeGrant)
				if statusErr == nil {
					mount.Status, mount.Available = GrantStatusToMountUpdate(rec.nativeGrant, status)
				}
			}
			if err := s.registry.RestoreMount(mount); err != nil {
				continue
			}
		case WorkspaceKindRemote:
			mount.Available = false
			mount.Status = WorkspaceStatusUnavailable
			if err := s.registry.RestoreMount(mount); err != nil {
				continue
			}
			backend, ok := s.registry.GetBackend(WorkspaceKindRemote)
			if ok {
				if remoteBackend, isRemote := backend.(*RemoteBackend); isRemote {
					remoteBackend.SetStatusUpdater(mount.ID, func(status WorkspaceStatus, reason string) {
						s.registry.UpdateStatus(mount.ID, status, status == WorkspaceStatusReady || status == WorkspaceStatusReadOnly)
					})
				}
			}
		}
	}
	return nil
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

func (s *Service) CopyTo(ctx context.Context, uriStr string, dst io.Writer) error {
	if dst == nil {
		return fmt.Errorf("destination writer is nil")
	}
	const chunkSize int64 = 1024 * 1024
	var offset int64
	for {
		result, err := s.Read(ctx, uriStr, ReadOptions{Offset: offset, MaxBytes: chunkSize})
		if err != nil {
			return err
		}
		if len(result.Content) == 0 {
			return nil
		}
		written, err := dst.Write(result.Content)
		if err != nil {
			return err
		}
		if written != len(result.Content) {
			return io.ErrShortWrite
		}
		offset += int64(written)
		if int64(len(result.Content)) < chunkSize {
			return nil
		}
	}
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
		ID:        "default",
		Name:      "Default",
		Kind:      WorkspaceKindLocal,
		RootURI:   "amitia://workspace/",
		Status:    WorkspaceStatusReady,
		Available: true,
	}
}

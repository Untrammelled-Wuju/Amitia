package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"

	"github.com/google/uuid"
)

const WorkspaceKindRemote WorkspaceKind = "remote"

type RemoteBackend struct {
	credentials    RemoteCredentialResolver
	clients        *remoteClientCache
	policy         RemotePolicy
	mu             sync.RWMutex
	statusUpdaters map[WorkspaceID]func(WorkspaceStatus, string)
}

func NewRemoteBackend(credentials RemoteCredentialResolver, policy RemotePolicy) *RemoteBackend {
	return &RemoteBackend{
		credentials:    credentials,
		clients:        newRemoteClientCache(policy),
		policy:         policy,
		statusUpdaters: make(map[WorkspaceID]func(WorkspaceStatus, string)),
	}
}

func (b *RemoteBackend) SetStatusUpdater(mountID WorkspaceID, updater func(WorkspaceStatus, string)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if updater != nil {
		b.statusUpdaters[mountID] = updater
	} else {
		delete(b.statusUpdaters, mountID)
	}
}

func (b *RemoteBackend) updateStatus(mountID WorkspaceID, status WorkspaceStatus, reason string) {
	b.mu.RLock()
	updater, ok := b.statusUpdaters[mountID]
	b.mu.RUnlock()
	if ok && updater != nil {
		updater(status, reason)
	}
}

func (b *RemoteBackend) Kind() WorkspaceKind {
	return WorkspaceKindRemote
}

func (b *RemoteBackend) Stat(ctx context.Context, mount WorkspaceMount, path string) (WorkspaceEntry, error) {
	if err := ValidateRelativePath(path); err != nil {
		return WorkspaceEntry{}, err
	}

	config, err := b.parseConfig(mount)
	if err != nil {
		return WorkspaceEntry{}, err
	}

	client, err := b.acquireClient(ctx, mount.ID, config)
	if err != nil {
		b.updateStatus(mount.ID, WorkspaceStatusUnavailable, "")
		return WorkspaceEntry{}, b.mapTransportError(err)
	}
	defer b.releaseClient(mount.ID, client)

	remotePath := b.resolveRemotePath(config, path)
	result, err := client.Stat(ctx, remotePath)
	if err != nil {
		if isAuthError(err) {
			b.updateStatus(mount.ID, WorkspaceStatusUnavailable, "auth_failed")
		}
		return WorkspaceEntry{}, b.mapTransportError(err)
	}

	b.updateStatus(mount.ID, WorkspaceStatusReady, "")
	return b.buildEntry(mount, path, result), nil
}

func (b *RemoteBackend) List(ctx context.Context, mount WorkspaceMount, dirPath string, opts ListOptions) ([]WorkspaceEntry, error) {
	if err := ValidateRelativePath(dirPath); err != nil {
		return nil, err
	}

	config, err := b.parseConfig(mount)
	if err != nil {
		return nil, err
	}

	client, err := b.acquireClient(ctx, mount.ID, config)
	if err != nil {
		b.updateStatus(mount.ID, WorkspaceStatusUnavailable, "")
		return nil, b.mapTransportError(err)
	}
	defer b.releaseClient(mount.ID, client)

	limit := b.policy.MaxListEntries
	if opts.Limit > 0 && opts.Limit < limit {
		limit = opts.Limit
	}

	remotePath := b.resolveRemotePath(config, dirPath)
	result, err := client.List(ctx, remotePath, limit)
	if err != nil {
		return nil, b.mapTransportError(err)
	}

	entries := make([]WorkspaceEntry, 0, len(result.Entries))
	for _, e := range result.Entries {
		if e.IsSymlink {
			continue
		}
		entryPath := path.Join(dirPath, e.Name)
		if dirPath == "" {
			entryPath = e.Name
		}
		entries = append(entries, b.buildEntryFromRemoteStat(mount, entryPath, e))
	}

	b.updateStatus(mount.ID, WorkspaceStatusReady, "")
	return entries, nil
}

func (b *RemoteBackend) Read(ctx context.Context, mount WorkspaceMount, filePath string, opts ReadOptions) (ReadResult, error) {
	if err := ValidateRelativePath(filePath); err != nil {
		return ReadResult{}, err
	}

	config, err := b.parseConfig(mount)
	if err != nil {
		return ReadResult{}, err
	}

	client, err := b.acquireClient(ctx, mount.ID, config)
	if err != nil {
		return ReadResult{}, b.mapTransportError(err)
	}
	defer b.releaseClient(mount.ID, client)

	remotePath := b.resolveRemotePath(config, filePath)
	maxBytes := opts.MaxBytes
	if maxBytes <= 0 || maxBytes > b.policy.MaxReadBytes {
		maxBytes = b.policy.MaxReadBytes
	}

	result, err := client.Read(ctx, remotePath, opts.Offset, maxBytes)
	if err != nil {
		return ReadResult{}, b.mapTransportError(err)
	}

	return result, nil
}

func (b *RemoteBackend) Write(ctx context.Context, mount WorkspaceMount, filePath string, src io.Reader, opts WriteOptions) (WorkspaceEntry, error) {
	if err := ValidateRelativePath(filePath); err != nil {
		return WorkspaceEntry{}, err
	}

	config, err := b.parseConfig(mount)
	if err != nil {
		return WorkspaceEntry{}, err
	}

	client, err := b.acquireClient(ctx, mount.ID, config)
	if err != nil {
		return WorkspaceEntry{}, b.mapTransportError(err)
	}
	defer b.releaseClient(mount.ID, client)

	remotePath := b.resolveRemotePath(config, filePath)
	dir := path.Dir(filePath)
	if dir == "." {
		dir = ""
	}

	if !opts.Overwrite {
		_, statErr := client.Stat(ctx, remotePath)
		if statErr == nil {
			return WorkspaceEntry{}, ErrAlreadyExists
		}
	}

	tempName := RemoteTempSuffix + uuid.NewString() + ".tmp"
	tempRemotePath := b.resolveRemotePath(config, path.Join(dir, tempName))

	if err := client.Write(ctx, tempRemotePath, src, true); err != nil {
		client.Delete(ctx, tempRemotePath, false)
		return WorkspaceEntry{}, b.mapTransportError(err)
	}

	if err := client.Rename(ctx, tempRemotePath, remotePath); err != nil {
		client.Delete(ctx, tempRemotePath, false)
		return WorkspaceEntry{}, NewRemoteError("REMOTE_OUTCOME_UNKNOWN", "rename after write failed", err)
	}

	result, statErr := client.Stat(ctx, remotePath)
	if statErr != nil {
		return WorkspaceEntry{}, b.mapTransportError(statErr)
	}

	return b.buildEntry(mount, filePath, result), nil
}

func (b *RemoteBackend) Mkdir(ctx context.Context, mount WorkspaceMount, dirPath string) (WorkspaceEntry, error) {
	if err := ValidateRelativePath(dirPath); err != nil {
		return WorkspaceEntry{}, err
	}

	config, err := b.parseConfig(mount)
	if err != nil {
		return WorkspaceEntry{}, err
	}

	client, err := b.acquireClient(ctx, mount.ID, config)
	if err != nil {
		return WorkspaceEntry{}, b.mapTransportError(err)
	}
	defer b.releaseClient(mount.ID, client)

	remotePath := b.resolveRemotePath(config, dirPath)
	if err := client.Mkdir(ctx, remotePath); err != nil {
		return WorkspaceEntry{}, b.mapTransportError(err)
	}

	result, statErr := client.Stat(ctx, remotePath)
	if statErr != nil {
		return WorkspaceEntry{
			URI:      MountURI(mount.ID) + strings.TrimPrefix(dirPath, "/"),
			Name:     path.Base(dirPath),
			Type:     WorkspaceEntryTypeDirectory,
			MIMEType: "inode/directory",
			Readable: true,
			Writable: !mount.ReadOnly,
		}, nil
	}

	return b.buildEntry(mount, dirPath, result), nil
}

func (b *RemoteBackend) Rename(ctx context.Context, mount WorkspaceMount, oldPath string, newName string) (WorkspaceEntry, error) {
	if err := ValidateRelativePath(oldPath); err != nil {
		return WorkspaceEntry{}, err
	}
	if err := ValidateRelativePath(newName); err != nil && newName != path.Base(newName) {
		return WorkspaceEntry{}, ErrPathTraversal
	}
	if newName == "" || newName != path.Base(newName) {
		return WorkspaceEntry{}, ErrInvalidPath
	}

	config, err := b.parseConfig(mount)
	if err != nil {
		return WorkspaceEntry{}, err
	}

	client, err := b.acquireClient(ctx, mount.ID, config)
	if err != nil {
		return WorkspaceEntry{}, b.mapTransportError(err)
	}
	defer b.releaseClient(mount.ID, client)

	dir := path.Dir(oldPath)
	newPath := path.Join(dir, newName)
	oldRemotePath := b.resolveRemotePath(config, oldPath)
	newRemotePath := b.resolveRemotePath(config, newPath)

	if err := client.Rename(ctx, oldRemotePath, newRemotePath); err != nil {
		return WorkspaceEntry{}, b.mapTransportError(err)
	}

	result, statErr := client.Stat(ctx, newRemotePath)
	if statErr != nil {
		return WorkspaceEntry{
			URI:      MountURI(mount.ID) + strings.TrimPrefix(newPath, "/"),
			Name:     newName,
			Type:     WorkspaceEntryTypeFile,
			Readable: true,
			Writable: !mount.ReadOnly,
		}, nil
	}

	return b.buildEntry(mount, newPath, result), nil
}

func (b *RemoteBackend) Move(ctx context.Context, mount WorkspaceMount, sourcePath string, destDirPath string) (WorkspaceEntry, error) {
	if err := ValidateRelativePath(sourcePath); err != nil {
		return WorkspaceEntry{}, err
	}
	if err := ValidateRelativePath(destDirPath); err != nil {
		return WorkspaceEntry{}, err
	}

	config, err := b.parseConfig(mount)
	if err != nil {
		return WorkspaceEntry{}, err
	}

	client, err := b.acquireClient(ctx, mount.ID, config)
	if err != nil {
		return WorkspaceEntry{}, b.mapTransportError(err)
	}
	defer b.releaseClient(mount.ID, client)

	srcRemotePath := b.resolveRemotePath(config, sourcePath)
	destRemotePath := b.resolveRemotePath(config, path.Join(destDirPath, path.Base(sourcePath)))

	if err := client.Move(ctx, srcRemotePath, destDirPath); err != nil {
		return WorkspaceEntry{}, b.mapTransportError(err)
	}

	result, statErr := client.Stat(ctx, destRemotePath)
	if statErr != nil {
		newPath := path.Join(destDirPath, path.Base(sourcePath))
		return WorkspaceEntry{
			URI:      MountURI(mount.ID) + strings.TrimPrefix(newPath, "/"),
			Name:     path.Base(sourcePath),
			Type:     WorkspaceEntryTypeFile,
			Readable: true,
			Writable: !mount.ReadOnly,
		}, nil
	}

	newPath := path.Join(destDirPath, path.Base(sourcePath))
	return b.buildEntry(mount, newPath, result), nil
}

func (b *RemoteBackend) Copy(ctx context.Context, mount WorkspaceMount, sourcePath string, destDirPath string) (WorkspaceEntry, error) {
	if err := ValidateRelativePath(sourcePath); err != nil {
		return WorkspaceEntry{}, err
	}
	if err := ValidateRelativePath(destDirPath); err != nil {
		return WorkspaceEntry{}, err
	}

	config, err := b.parseConfig(mount)
	if err != nil {
		return WorkspaceEntry{}, err
	}

	client, err := b.acquireClient(ctx, mount.ID, config)
	if err != nil {
		return WorkspaceEntry{}, b.mapTransportError(err)
	}
	defer b.releaseClient(mount.ID, client)

	srcRemotePath := b.resolveRemotePath(config, sourcePath)
	destRemotePath := b.resolveRemotePath(config, path.Join(destDirPath, path.Base(sourcePath)))

	if err := client.Copy(ctx, srcRemotePath, destDirPath); err != nil {
		return WorkspaceEntry{}, b.mapTransportError(err)
	}

	result, statErr := client.Stat(ctx, destRemotePath)
	if statErr != nil {
		newPath := path.Join(destDirPath, path.Base(sourcePath))
		return WorkspaceEntry{
			URI:      MountURI(mount.ID) + strings.TrimPrefix(newPath, "/"),
			Name:     path.Base(sourcePath),
			Type:     WorkspaceEntryTypeFile,
			Readable: true,
			Writable: !mount.ReadOnly,
		}, nil
	}

	newPath := path.Join(destDirPath, path.Base(sourcePath))
	return b.buildEntry(mount, newPath, result), nil
}

func (b *RemoteBackend) Delete(ctx context.Context, mount WorkspaceMount, targetPath string, opts DeleteOptions) error {
	if err := ValidateRelativePath(targetPath); err != nil {
		return err
	}
	if targetPath == "" {
		return ErrRootMutationDenied
	}

	config, err := b.parseConfig(mount)
	if err != nil {
		return err
	}

	client, err := b.acquireClient(ctx, mount.ID, config)
	if err != nil {
		return b.mapTransportError(err)
	}
	defer b.releaseClient(mount.ID, client)

	remotePath := b.resolveRemotePath(config, targetPath)

	if !opts.Recursive {
		result, statErr := client.Stat(ctx, remotePath)
		if statErr != nil {
			return b.mapTransportError(statErr)
		}
		if result.IsDir {
			listResult, listErr := client.List(ctx, remotePath, 1)
			if listErr != nil {
				return b.mapTransportError(listErr)
			}
			if len(listResult.Entries) > 0 {
				return ErrDirectoryNotEmpty
			}
		}
	}

	if err := client.Delete(ctx, remotePath, opts.Recursive); err != nil {
		return b.mapTransportError(err)
	}

	return nil
}

func (b *RemoteBackend) parseConfig(mount WorkspaceMount) (RemoteMountConfig, error) {
	configJSON, ok := mount.backendConfig()
	if !ok {
		return RemoteMountConfig{}, ErrRemoteConfigInvalid
	}
	var config RemoteMountConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return RemoteMountConfig{}, NewRemoteError("REMOTE_CONFIG_INVALID", "failed to parse mount config", err)
	}
	return config, nil
}

func (b *RemoteBackend) acquireClient(ctx context.Context, mountID WorkspaceID, config RemoteMountConfig) (RemoteTransport, error) {
	if b.credentials == nil {
		return nil, ErrRemoteCredentialNotFound
	}

	cred, err := b.credentials.ResolveCredential(ctx, config.CredentialRef)
	if err != nil {
		return nil, NewRemoteError("REMOTE_CREDENTIAL_RESOLUTION_FAILED", "failed to resolve credential", err)
	}

	client, err := b.clients.getOrCreate(ctx, mountID, config, cred)
	if err != nil {
		return nil, NewRemoteError("REMOTE_TRANSPORT_FAILED", "failed to create transport", err)
	}

	return client, nil
}

func (b *RemoteBackend) releaseClient(mountID WorkspaceID, client RemoteTransport) {
	b.clients.release(mountID, client)
}

func (b *RemoteBackend) resolveRemotePath(config RemoteMountConfig, relativePath string) string {
	var resolver func(string, string) string
	switch config.Protocol {
	case RemoteProtocolSFTP:
		resolver = ResolveRemotePathSFTP
	case RemoteProtocolWebDAV:
		resolver = ResolveRemotePathWebDAV
	default:
		resolver = ResolveRemotePathSFTP
	}
	return resolver(config.BasePath, relativePath)
}

func (b *RemoteBackend) mapTransportError(err error) error {
	if err == nil {
		return nil
	}
	if ofErr, ok := err.(*RemoteError); ok {
		switch ofErr.Code {
		case "REMOTE_AUTH_FAILED":
			return ErrRemoteAuthFailed
		case "REMOTE_PERMISSION_DENIED":
			return ErrPermissionDenied
		case "REMOTE_LOCKED":
			return ErrRemoteLocked
		case "REMOTE_INSUFFICIENT_STORAGE":
			return ErrRemoteInsufficientStorage
		case "REMOTE_HOST_KEY_CHANGED":
			return ErrRemoteHostKeyChanged
		case "REMOTE_TLS_FAILED":
			return ErrRemoteTLSFailed
		case "REMOTE_ENDPOINT_UNREACHABLE":
			return ErrRemoteEndpointUnreachable
		case "REMOTE_SYMLINK_UNSUPPORTED":
			return ErrRemoteSymlinkUnsupported
		case "REMOTE_OUTCOME_UNKNOWN":
			return ErrRemoteOutcomeUnknown
		default:
			return err
		}
	}
	return NewRemoteError("REMOTE_UNAVAILABLE", "", err)
}

func (b *RemoteBackend) buildEntry(mount WorkspaceMount, relPath string, result RemoteStatResult) WorkspaceEntry {
	entryType := WorkspaceEntryTypeFile
	if result.IsDir {
		entryType = WorkspaceEntryTypeDirectory
	}

	uri := MountURI(mount.ID) + strings.TrimPrefix(relPath, "/")
	if result.IsDir && !strings.HasSuffix(uri, "/") {
		uri += "/"
	}

	mimeType := result.MIMEType
	if mimeType == "" {
		if result.IsDir {
			mimeType = "inode/directory"
		} else {
			mimeType = InferRemoteMIMEType(result.Name)
		}
	}

	var sizePtr *int64
	if !result.IsDir && result.SizeBytes >= 0 {
		sizePtr = &result.SizeBytes
	}

	var modPtr *time.Time
	if !result.ModifiedAt.IsZero() {
		modPtr = &result.ModifiedAt
	}

	return WorkspaceEntry{
		URI:        uri,
		Name:       result.Name,
		Type:       entryType,
		MIMEType:   mimeType,
		SizeBytes:  sizePtr,
		ModifiedAt: modPtr,
		Readable:   true,
		Writable:   !mount.ReadOnly,
	}
}

func (b *RemoteBackend) buildEntryFromRemoteStat(mount WorkspaceMount, relPath string, result RemoteStatResult) WorkspaceEntry {
	return b.buildEntry(mount, relPath, result)
}

func isAuthError(err error) bool {
	if ofErr, ok := err.(*RemoteError); ok {
		return ofErr.Code == "REMOTE_AUTH_FAILED"
	}
	return false
}

func init() {
	RegisterKnownKind(WorkspaceKindRemote)
}

var _ WorkspaceBackend = (*RemoteBackend)(nil)

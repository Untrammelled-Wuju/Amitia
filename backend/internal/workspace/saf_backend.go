package workspace

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

type SAFBackend struct {
	bridge SAFBridge
}

func NewSAFBackend(bridge SAFBridge) *SAFBackend {
	return &SAFBackend{bridge: bridge}
}

func (b *SAFBackend) Kind() WorkspaceKind {
	return WorkspaceKindSAF
}

func (b *SAFBackend) ensureBridge() error {
	if b.bridge == nil {
		return fmt.Errorf("%w: SAF bridge not configured", ErrSAFUnavailable)
	}
	return nil
}

func (b *SAFBackend) Stat(ctx context.Context, mount WorkspaceMount, path string) (WorkspaceEntry, error) {
	if err := b.ensureBridge(); err != nil {
		return WorkspaceEntry{}, err
	}
	if err := ValidateRelativePath(path); err != nil {
		return WorkspaceEntry{}, err
	}

	grantID := mount.NativeGrant
	ref, err := b.bridge.ResolvePath(ctx, grantID, path)
	if err != nil {
		return WorkspaceEntry{}, b.mapNativeError(err)
	}

	result, err := b.bridge.Stat(ctx, grantID, ref.DocumentID, ref.Name)
	if err != nil {
		return WorkspaceEntry{}, b.mapNativeError(err)
	}

	if result.IsVirtual {
		return WorkspaceEntry{}, ErrVirtualDocumentUnsupported
	}

	return b.buildEntry(mount, path, ref.DocumentID, result, grantID)
}

func (b *SAFBackend) List(ctx context.Context, mount WorkspaceMount, path string, opts ListOptions) ([]WorkspaceEntry, error) {
	if err := b.ensureBridge(); err != nil {
		return nil, err
	}
	if err := ValidateRelativePath(path); err != nil {
		return nil, err
	}

	grantID := mount.NativeGrant
	ref, err := b.bridge.ResolvePath(ctx, grantID, path)
	if err != nil {
		return nil, b.mapNativeError(err)
	}

	limit := MaxListEntries
	if opts.Limit > 0 && opts.Limit < limit {
		limit = opts.Limit
	}

	entries, _, err := b.bridge.List(ctx, grantID, ref.DocumentID, limit)
	if err != nil {
		return nil, b.mapNativeError(err)
	}

	result := make([]WorkspaceEntry, 0, len(entries))
	for _, e := range entries {
		if e.IsVirtual {
			continue
		}
		childPath := path
		if childPath != "" && !strings.HasSuffix(childPath, "/") {
			childPath += "/"
		}
		childPath += e.Name

		entry, err := b.buildEntryFromRef(mount, childPath, "", e, grantID)
		if err != nil {
			continue
		}
		result = append(result, entry)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Type != result[j].Type {
			return result[i].Type == WorkspaceEntryTypeDirectory
		}
		return result[i].Name < result[j].Name
	})

	return result, nil
}

func (b *SAFBackend) Read(ctx context.Context, mount WorkspaceMount, path string, opts ReadOptions) (ReadResult, error) {
	if err := b.ensureBridge(); err != nil {
		return ReadResult{}, err
	}
	if err := ValidateRelativePath(path); err != nil {
		return ReadResult{}, err
	}

	grantID := mount.NativeGrant
	ref, err := b.bridge.ResolvePath(ctx, grantID, path)
	if err != nil {
		return ReadResult{}, b.mapNativeError(err)
	}

	maxBytes := int64(MaxDirectRead)
	if opts.MaxBytes > 0 && opts.MaxBytes < maxBytes {
		maxBytes = opts.MaxBytes
	}

	data, _, isText, err := b.bridge.Read(ctx, grantID, ref.DocumentID, opts.Offset, maxBytes)
	if err != nil {
		return ReadResult{}, b.mapNativeError(err)
	}

	return ReadResult{
		Content: data,
		IsText:  isText,
	}, nil
}

func (b *SAFBackend) Write(ctx context.Context, mount WorkspaceMount, path string, src io.Reader, opts WriteOptions) (WorkspaceEntry, error) {
	if err := b.ensureBridge(); err != nil {
		return WorkspaceEntry{}, err
	}
	if err := ValidateRelativePath(path); err != nil {
		return WorkspaceEntry{}, err
	}

	grantID := mount.NativeGrant
	parentPath, name := SplitPathParent(path)
	if name == "" {
		return WorkspaceEntry{}, ErrInvalidPath
	}
	if err := validateSegment(name); err != nil {
		return WorkspaceEntry{}, err
	}

	parentRef, err := b.bridge.ResolvePath(ctx, grantID, parentPath)
	if err != nil {
		return WorkspaceEntry{}, b.mapNativeError(err)
	}

	data, err := io.ReadAll(io.LimitReader(src, MaxSingleWrite+1))
	if err != nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrWriteFailed, err)
	}
	if int64(len(data)) > MaxSingleWrite {
		return WorkspaceEntry{}, fmt.Errorf("%w: write size %d exceeds max %d", ErrResourceTooLarge, len(data), MaxSingleWrite)
	}

	if !opts.Overwrite {
		entries, _, err := b.bridge.List(ctx, grantID, parentRef.DocumentID, MaxListEntries)
		if err != nil {
			return WorkspaceEntry{}, b.mapNativeError(err)
		}
		for _, e := range entries {
			if e.Name == name {
				return WorkspaceEntry{}, fmt.Errorf("%w: %q", ErrAlreadyExists, path)
			}
		}
	}

	source := SAFWriteSource{Stream: data}
	mimeType := InferMIMEType(name)

	var result SAFStatResult
	if opts.Overwrite {
		existing, err := b.findChildByName(ctx, grantID, parentRef.DocumentID, name)
		if err != nil {
			return WorkspaceEntry{}, err
		}
		result, err = b.bridge.Write(ctx, grantID, existing.DocumentID, name, source, true)
		if err != nil {
			return WorkspaceEntry{}, b.mapNativeError(err)
		}
	} else {
		createInput := SAFCreateFileInput{
			ParentDocumentID: parentRef.DocumentID,
			DisplayName:      name,
			MIMEType:         mimeType,
		}
		if _, err = b.bridge.CreateFile(ctx, grantID, createInput); err != nil {
			return WorkspaceEntry{}, b.mapNativeError(err)
		}
		createdRef, resolveErr := b.bridge.ResolvePath(ctx, grantID, path)
		if resolveErr != nil {
			return WorkspaceEntry{}, b.mapNativeError(resolveErr)
		}
		result, err = b.bridge.Write(ctx, grantID, createdRef.DocumentID, name, source, true)
		if err != nil {
			// Best-effort rollback: a failed initial write must not masquerade as
			// a successfully created empty file.
			_ = b.bridge.Delete(ctx, grantID, createdRef.DocumentID)
			return WorkspaceEntry{}, b.mapNativeError(err)
		}
	}

	return b.buildEntryFromStat(mount, path, "", result)
}

func (b *SAFBackend) Mkdir(ctx context.Context, mount WorkspaceMount, path string) (WorkspaceEntry, error) {
	if err := b.ensureBridge(); err != nil {
		return WorkspaceEntry{}, err
	}
	if err := ValidateRelativePath(path); err != nil {
		return WorkspaceEntry{}, err
	}

	grantID := mount.NativeGrant
	parentPath, name := SplitPathParent(path)
	if name == "" {
		return WorkspaceEntry{}, ErrInvalidPath
	}
	if err := validateSegment(name); err != nil {
		return WorkspaceEntry{}, err
	}

	parentRef, err := b.bridge.ResolvePath(ctx, grantID, parentPath)
	if err != nil {
		return WorkspaceEntry{}, b.mapNativeError(err)
	}

	input := SAFCreateDirInput{
		ParentDocumentID: parentRef.DocumentID,
		DisplayName:      name,
	}
	result, err := b.bridge.Mkdir(ctx, grantID, input)
	if err != nil {
		return WorkspaceEntry{}, b.mapNativeError(err)
	}

	return b.buildEntryFromStat(mount, path, "", result)
}

func (b *SAFBackend) Rename(ctx context.Context, mount WorkspaceMount, path string, newName string) (WorkspaceEntry, error) {
	if err := b.ensureBridge(); err != nil {
		return WorkspaceEntry{}, err
	}
	if err := ValidateRelativePath(path); err != nil {
		return WorkspaceEntry{}, err
	}
	if err := validateSegment(newName); err != nil {
		return WorkspaceEntry{}, err
	}

	grantID := mount.NativeGrant
	ref, err := b.bridge.ResolvePath(ctx, grantID, path)
	if err != nil {
		return WorkspaceEntry{}, b.mapNativeError(err)
	}

	result, err := b.bridge.Rename(ctx, grantID, ref.DocumentID, newName)
	if err != nil {
		return WorkspaceEntry{}, b.mapNativeError(err)
	}

	parentPath, _ := SplitPathParent(path)
	newPath := parentPath
	if newPath != "" && !strings.HasSuffix(newPath, "/") {
		newPath += "/"
	}
	newPath += newName

	return b.buildEntryFromStat(mount, newPath, "", result)
}

func (b *SAFBackend) Move(ctx context.Context, mount WorkspaceMount, source string, destinationDir string) (WorkspaceEntry, error) {
	if err := b.ensureBridge(); err != nil {
		return WorkspaceEntry{}, err
	}
	if err := ValidateRelativePath(source); err != nil {
		return WorkspaceEntry{}, err
	}
	if err := ValidateRelativePath(destinationDir); err != nil {
		return WorkspaceEntry{}, err
	}

	grantID := mount.NativeGrant
	srcRef, err := b.bridge.ResolvePath(ctx, grantID, source)
	if err != nil {
		return WorkspaceEntry{}, b.mapNativeError(err)
	}
	dstRef, err := b.bridge.ResolvePath(ctx, grantID, destinationDir)
	if err != nil {
		return WorkspaceEntry{}, b.mapNativeError(err)
	}

	result, err := b.bridge.Move(ctx, grantID, srcRef.DocumentID, dstRef.DocumentID)
	if err != nil {
		return WorkspaceEntry{}, b.mapNativeError(err)
	}

	newPath := destinationDir
	if newPath != "" && !strings.HasSuffix(newPath, "/") {
		newPath += "/"
	}
	_, srcName := SplitPathParent(source)
	newPath += srcName

	return b.buildEntryFromStat(mount, newPath, "", result)
}

func (b *SAFBackend) Copy(ctx context.Context, mount WorkspaceMount, source string, destinationDir string) (WorkspaceEntry, error) {
	if err := b.ensureBridge(); err != nil {
		return WorkspaceEntry{}, err
	}
	if err := ValidateRelativePath(source); err != nil {
		return WorkspaceEntry{}, err
	}
	if err := ValidateRelativePath(destinationDir); err != nil {
		return WorkspaceEntry{}, err
	}

	grantID := mount.NativeGrant
	srcRef, err := b.bridge.ResolvePath(ctx, grantID, source)
	if err != nil {
		return WorkspaceEntry{}, b.mapNativeError(err)
	}
	dstRef, err := b.bridge.ResolvePath(ctx, grantID, destinationDir)
	if err != nil {
		return WorkspaceEntry{}, b.mapNativeError(err)
	}

	result, err := b.bridge.Copy(ctx, grantID, srcRef.DocumentID, dstRef.DocumentID)
	if err != nil {
		return WorkspaceEntry{}, b.mapNativeError(err)
	}

	newPath := destinationDir
	if newPath != "" && !strings.HasSuffix(newPath, "/") {
		newPath += "/"
	}
	_, srcName := SplitPathParent(source)
	newPath += srcName

	return b.buildEntryFromStat(mount, newPath, "", result)
}

func (b *SAFBackend) Delete(ctx context.Context, mount WorkspaceMount, path string, opts DeleteOptions) error {
	if err := b.ensureBridge(); err != nil {
		return err
	}
	if err := ValidateRelativePath(path); err != nil {
		return err
	}

	grantID := mount.NativeGrant
	ref, err := b.bridge.ResolvePath(ctx, grantID, path)
	if err != nil {
		return b.mapNativeError(err)
	}

	statResult, err := b.bridge.Stat(ctx, grantID, ref.DocumentID, ref.Name)
	if err != nil {
		return b.mapNativeError(err)
	}

	if statResult.IsDirectory && !opts.Recursive {
		children, _, err := b.bridge.List(ctx, grantID, ref.DocumentID, 1)
		if err != nil {
			return b.mapNativeError(err)
		}
		if len(children) > 0 {
			return fmt.Errorf("%w: %q", ErrDirectoryNotEmpty, path)
		}
	}

	return b.mapNativeError(b.bridge.Delete(ctx, grantID, ref.DocumentID))
}

func (b *SAFBackend) findChildByName(ctx context.Context, grantID string, parentDocumentID string, name string) (SAFDocumentRef, error) {
	entries, _, err := b.bridge.List(ctx, grantID, parentDocumentID, MaxListEntries)
	if err != nil {
		return SAFDocumentRef{}, b.mapNativeError(err)
	}
	for _, e := range entries {
		if e.Name == name {
			documentID := strings.TrimSpace(e.DocumentID)
			if documentID == "" {
				documentID = parentDocumentID + "/" + name
			}
			return SAFDocumentRef{
				GrantID:     grantID,
				DocumentID:  documentID,
				Name:        name,
				MIMEType:    e.MIMEType,
				Flags:       e.Flags,
				IsDirectory: e.IsDirectory,
			}, nil
		}
	}
	return SAFDocumentRef{}, fmt.Errorf("%w: %q", ErrFileNotFound, name)
}

func (b *SAFBackend) buildEntry(mount WorkspaceMount, path string, documentID string, stat SAFStatResult, grantID string) (WorkspaceEntry, error) {
	if stat.IsVirtual {
		return WorkspaceEntry{}, ErrVirtualDocumentUnsupported
	}

	entryType := WorkspaceEntryTypeFile
	if stat.IsDirectory {
		entryType = WorkspaceEntryTypeDirectory
	}

	uri := BuildSAFEntryURI(mount, path)
	if stat.IsDirectory && !strings.HasSuffix(uri, "/") {
		uri += "/"
	}

	return WorkspaceEntry{
		URI:        uri,
		Name:       stat.Name,
		Type:       entryType,
		MIMEType:   stat.MIMEType,
		SizeBytes:  stat.SizeBytes,
		ModifiedAt: stat.ModifiedAt,
		Readable:   true,
		Writable:   !mount.ReadOnly && IsSAFWritable(stat.Flags),
	}, nil
}

func (b *SAFBackend) buildEntryFromStat(mount WorkspaceMount, path string, documentID string, stat SAFStatResult) (WorkspaceEntry, error) {
	return b.buildEntry(mount, path, documentID, stat, mount.NativeGrant)
}

func (b *SAFBackend) buildEntryFromRef(mount WorkspaceMount, path string, documentID string, ref SAFEntryRef, grantID string) (WorkspaceEntry, error) {
	if ref.IsVirtual {
		return WorkspaceEntry{}, ErrVirtualDocumentUnsupported
	}

	entryType := WorkspaceEntryTypeFile
	if ref.IsDirectory {
		entryType = WorkspaceEntryTypeDirectory
	}

	uri := BuildSAFEntryURI(mount, path)
	if ref.IsDirectory && !strings.HasSuffix(uri, "/") {
		uri += "/"
	}

	return WorkspaceEntry{
		URI:        uri,
		Name:       ref.Name,
		Type:       entryType,
		MIMEType:   ref.MIMEType,
		SizeBytes:  ref.SizeBytes,
		ModifiedAt: ref.ModifiedAt,
		Readable:   true,
		Writable:   !mount.ReadOnly && IsSAFWritable(ref.Flags),
	}, nil
}

func (b *SAFBackend) mapNativeError(err error) error {
	if err == nil {
		return nil
	}

	if safErrPtr, ok := err.(*SAFNativeError); ok && safErrPtr != nil {
		return b.mapNativeError(*safErrPtr)
	}
	if safErr, ok := err.(SAFNativeError); ok {
		if safErr.PermissionRevoked {
			return fmt.Errorf("%w: %v", ErrSAFPermissionRevoked, err)
		}
		if safErr.ProviderUnavailable {
			return fmt.Errorf("%w: %v", ErrSAFProviderUnavailable, err)
		}
		switch safErr.Code {
		case "NOT_FOUND", "FILE_NOT_FOUND":
			return fmt.Errorf("%w: %v", ErrFileNotFound, err)
		case "NOT_DIRECTORY":
			return fmt.Errorf("%w: %v", ErrNotDirectory, err)
		case "ALREADY_EXISTS":
			return fmt.Errorf("%w: %v", ErrAlreadyExists, err)
		case "UNSUPPORTED_OPERATION", "OPERATION_NOT_SUPPORTED":
			return fmt.Errorf("%w: %v", ErrOperationUnsupported, err)
		case "INVALID_ARGUMENT":
			return fmt.Errorf("%w: %v", ErrInvalidPath, err)
		case "WRITE_FAILED":
			return fmt.Errorf("%w: %v", ErrWriteFailed, err)
		case "CANCELLED":
			return fmt.Errorf("%w: %v", ErrOperationCancelled, err)
		case "TIMEOUT":
			return fmt.Errorf("%w: %v", ErrOperationTimeout, err)
		default:
			return fmt.Errorf("%w: %v", ErrSAFUnavailable, err)
		}
	}

	return err
}

func GrantStatusToMountUpdate(grantID string, status SAFGrantStatus) (WorkspaceStatus, bool) {
	if !status.Valid {
		return WorkspaceStatusPermissionRevoked, false
	}
	if !status.ProviderAvailable {
		return WorkspaceStatusUnavailable, false
	}
	if !status.RootExists {
		return WorkspaceStatusMissing, false
	}
	if status.Readable && !status.Writable {
		return WorkspaceStatusReadOnly, true
	}
	return WorkspaceStatusReady, true
}

func generateSAFGrantID() string {
	return "saf_" + generateOpaqueID()
}

func generateOpaqueID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

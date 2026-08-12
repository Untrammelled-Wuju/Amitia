package workspace

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type fakeSAFEntry struct {
	name        string
	mimeType    string
	size        *int64
	modifiedAt  *time.Time
	flags       int64
	isDirectory bool
	isVirtual   bool
	children    map[string]*fakeSAFEntry
	content     []byte
}

type FakeSAFBridge struct {
	mu          sync.RWMutex
	grants      map[string]bool
	trees       map[string]*fakeSAFEntry
	nextDocID   int
	documentIDs map[string]string
}

func NewFakeSAFBridge() *FakeSAFBridge {
	return &FakeSAFBridge{
		grants:      make(map[string]bool),
		trees:       make(map[string]*fakeSAFEntry),
		documentIDs: make(map[string]string),
	}
}

func (f *FakeSAFBridge) RegisterGrant(grantID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.grants[grantID] = true
	f.trees[grantID] = &fakeSAFEntry{
		name:        "root",
		mimeType:    SAFMIMETypeDir,
		isDirectory: true,
		children:    make(map[string]*fakeSAFEntry),
		flags:       SAFFlagSupportsCreate | SAFFlagSupportsDelete | SAFFlagSupportsRename | SAFFlagSupportsMove | SAFFlagSupportsCopy | SAFFlagSupportsWrite,
	}
	f.documentIDs[grantID] = grantID + ":root"
}

func (f *FakeSAFBridge) RevokeGrant(grantID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.grants[grantID] = false
}

func (f *FakeSAFBridge) RemoveGrant(grantID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.grants, grantID)
	delete(f.trees, grantID)
}

func (f *FakeSAFBridge) GrantStatus(ctx context.Context, grantID string) (SAFGrantStatus, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	valid, exists := f.grants[grantID]
	if !exists || !valid {
		return SAFGrantStatus{Valid: false}, nil
	}
	_, ok := f.trees[grantID]
	if !ok {
		return SAFGrantStatus{Valid: true, ProviderAvailable: true, RootExists: false}, nil
	}
	return SAFGrantStatus{
		Valid:             true,
		Readable:          true,
		Writable:          true,
		ProviderAvailable: true,
		RootExists:        true,
	}, nil
}

func (f *FakeSAFBridge) Stat(ctx context.Context, grantID string, documentID string, name string) (SAFStatResult, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if !f.isGrantValid(grantID) {
		return SAFStatResult{}, SAFNativeError{Code: "PERMISSION_DENIED", PermissionRevoked: true}
	}
	entry := f.findEntryByPath(grantID, documentID, name)
	if entry == nil {
		return SAFStatResult{}, SAFNativeError{Code: "NOT_FOUND"}
	}
	return SAFStatResult{
		Name:        entry.name,
		MIMEType:    entry.mimeType,
		SizeBytes:   entry.size,
		ModifiedAt:  entry.modifiedAt,
		Flags:       entry.flags,
		IsDirectory: entry.isDirectory,
		IsVirtual:   entry.isVirtual,
	}, nil
}

func (f *FakeSAFBridge) List(ctx context.Context, grantID string, documentID string, limit int) ([]SAFEntryRef, string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if !f.isGrantValid(grantID) {
		return nil, "", SAFNativeError{Code: "PERMISSION_DENIED", PermissionRevoked: true}
	}
	entry := f.findEntryByDocID(grantID, documentID)
	if entry == nil {
		return nil, "", SAFNativeError{Code: "NOT_FOUND"}
	}
	if !entry.isDirectory {
		return nil, "", SAFNativeError{Code: "NOT_DIRECTORY"}
	}
	result := make([]SAFEntryRef, 0, len(entry.children))
	for _, child := range entry.children {
		result = append(result, SAFEntryRef{
			Name:        child.name,
			MIMEType:    child.mimeType,
			SizeBytes:   child.size,
			ModifiedAt:  child.modifiedAt,
			Flags:       child.flags,
			IsDirectory: child.isDirectory,
			IsVirtual:   child.isVirtual,
		})
		if len(result) >= limit {
			break
		}
	}
	return result, "", nil
}

func (f *FakeSAFBridge) Read(ctx context.Context, grantID string, documentID string, offset int64, maxBytes int64) ([]byte, string, bool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if !f.isGrantValid(grantID) {
		return nil, "", false, SAFNativeError{Code: "PERMISSION_DENIED", PermissionRevoked: true}
	}
	entry := f.findEntryByDocID(grantID, documentID)
	if entry == nil {
		return nil, "", false, SAFNativeError{Code: "NOT_FOUND"}
	}
	if entry.isDirectory {
		return nil, "", false, SAFNativeError{Code: "NOT_FILE"}
	}
	data := entry.content
	if offset >= int64(len(data)) {
		return nil, "", true, nil
	}
	end := int64(len(data))
	if offset+maxBytes < end {
		end = offset + maxBytes
	}
	return data[offset:end], "", true, nil
}

func (f *FakeSAFBridge) Write(ctx context.Context, grantID string, documentID string, targetName string, source SAFWriteSource, overwrite bool) (SAFStatResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.isGrantValid(grantID) {
		return SAFStatResult{}, SAFNativeError{Code: "PERMISSION_DENIED", PermissionRevoked: true}
	}
	parentDocID := f.parentDocID(documentID, targetName)
	parent := f.findEntryByDocID(grantID, parentDocID)
	if parent == nil {
		return SAFStatResult{}, SAFNativeError{Code: "NOT_FOUND"}
	}
	existing, hasExisting := parent.children[targetName]
	if !hasExisting {
		return SAFStatResult{}, SAFNativeError{Code: "NOT_FOUND"}
	}
	now := time.Now().UTC()
	existing.content = source.Stream
	existing.mimeType = InferMIMEType(targetName)
	sz := int64(len(source.Stream))
	existing.size = &sz
	existing.modifiedAt = &now
	return SAFStatResult{
		Name:        existing.name,
		MIMEType:    existing.mimeType,
		SizeBytes:   existing.size,
		ModifiedAt:  existing.modifiedAt,
		Flags:       existing.flags,
		IsDirectory: existing.isDirectory,
	}, nil
}

func (f *FakeSAFBridge) Mkdir(ctx context.Context, grantID string, input SAFCreateDirInput) (SAFStatResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.isGrantValid(grantID) {
		return SAFStatResult{}, SAFNativeError{Code: "PERMISSION_DENIED", PermissionRevoked: true}
	}
	parent := f.findEntryByDocID(grantID, input.ParentDocumentID)
	if parent == nil {
		return SAFStatResult{}, SAFNativeError{Code: "NOT_FOUND"}
	}
	if !parent.isDirectory {
		return SAFStatResult{}, SAFNativeError{Code: "NOT_DIRECTORY"}
	}
	if _, exists := parent.children[input.DisplayName]; exists {
		return SAFStatResult{}, SAFNativeError{Code: "ALREADY_EXISTS"}
	}
	now := time.Now().UTC()
	newEntry := &fakeSAFEntry{
		name:        input.DisplayName,
		mimeType:    SAFMIMETypeDir,
		isDirectory: true,
		children:    make(map[string]*fakeSAFEntry),
		flags:       SAFFlagSupportsCreate | SAFFlagSupportsDelete | SAFFlagSupportsRename | SAFFlagSupportsMove | SAFFlagSupportsCopy | SAFFlagSupportsWrite,
		modifiedAt:  &now,
	}
	parent.children[input.DisplayName] = newEntry
	f.documentIDs[grantID+":"+input.ParentDocumentID+"/"+input.DisplayName] = input.ParentDocumentID + "/" + input.DisplayName
	return SAFStatResult{
		Name:        newEntry.name,
		MIMEType:    newEntry.mimeType,
		IsDirectory: true,
		Flags:       newEntry.flags,
		ModifiedAt:  newEntry.modifiedAt,
	}, nil
}

func (f *FakeSAFBridge) Rename(ctx context.Context, grantID string, documentID string, newName string) (SAFStatResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.isGrantValid(grantID) {
		return SAFStatResult{}, SAFNativeError{Code: "PERMISSION_DENIED", PermissionRevoked: true}
	}
	entry := f.findEntryByDocID(grantID, documentID)
	if entry == nil {
		return SAFStatResult{}, SAFNativeError{Code: "NOT_FOUND"}
	}
	parentDocID := f.parentDocIDFromDocID(documentID)
	parent := f.findEntryByDocID(grantID, parentDocID)
	if parent == nil {
		return SAFStatResult{}, SAFNativeError{Code: "NOT_FOUND"}
	}
	if _, exists := parent.children[newName]; exists {
		return SAFStatResult{}, SAFNativeError{Code: "ALREADY_EXISTS"}
	}
	delete(parent.children, entry.name)
	entry.name = newName
	parent.children[newName] = entry
	now := time.Now().UTC()
	entry.modifiedAt = &now
	return SAFStatResult{
		Name:        entry.name,
		MIMEType:    entry.mimeType,
		SizeBytes:   entry.size,
		ModifiedAt:  entry.modifiedAt,
		Flags:       entry.flags,
		IsDirectory: entry.isDirectory,
	}, nil
}

func (f *FakeSAFBridge) Move(ctx context.Context, grantID string, documentID string, targetParentDocumentID string) (SAFStatResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.isGrantValid(grantID) {
		return SAFStatResult{}, SAFNativeError{Code: "PERMISSION_DENIED", PermissionRevoked: true}
	}
	entry := f.findEntryByDocID(grantID, documentID)
	if entry == nil {
		return SAFStatResult{}, SAFNativeError{Code: "NOT_FOUND"}
	}
	srcParentDocID := f.parentDocIDFromDocID(documentID)
	srcParent := f.findEntryByDocID(grantID, srcParentDocID)
	if srcParent == nil {
		return SAFStatResult{}, SAFNativeError{Code: "NOT_FOUND"}
	}
	dstParent := f.findEntryByDocID(grantID, targetParentDocumentID)
	if dstParent == nil {
		return SAFStatResult{}, SAFNativeError{Code: "NOT_FOUND"}
	}
	if !dstParent.isDirectory {
		return SAFStatResult{}, SAFNativeError{Code: "NOT_DIRECTORY"}
	}
	if _, exists := dstParent.children[entry.name]; exists {
		return SAFStatResult{}, SAFNativeError{Code: "ALREADY_EXISTS"}
	}
	delete(srcParent.children, entry.name)
	dstParent.children[entry.name] = entry
	now := time.Now().UTC()
	entry.modifiedAt = &now
	return SAFStatResult{
		Name:        entry.name,
		MIMEType:    entry.mimeType,
		SizeBytes:   entry.size,
		ModifiedAt:  entry.modifiedAt,
		Flags:       entry.flags,
		IsDirectory: entry.isDirectory,
	}, nil
}

func (f *FakeSAFBridge) Copy(ctx context.Context, grantID string, documentID string, targetParentDocumentID string) (SAFStatResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.isGrantValid(grantID) {
		return SAFStatResult{}, SAFNativeError{Code: "PERMISSION_DENIED", PermissionRevoked: true}
	}
	entry := f.findEntryByDocID(grantID, documentID)
	if entry == nil {
		return SAFStatResult{}, SAFNativeError{Code: "NOT_FOUND"}
	}
	dstParent := f.findEntryByDocID(grantID, targetParentDocumentID)
	if dstParent == nil {
		return SAFStatResult{}, SAFNativeError{Code: "NOT_FOUND"}
	}
	if !dstParent.isDirectory {
		return SAFStatResult{}, SAFNativeError{Code: "NOT_DIRECTORY"}
	}
	if _, exists := dstParent.children[entry.name]; exists {
		return SAFStatResult{}, SAFNativeError{Code: "ALREADY_EXISTS"}
	}
	now := time.Now().UTC()
	copy := &fakeSAFEntry{
		name:        entry.name,
		mimeType:    entry.mimeType,
		size:        entry.size,
		modifiedAt:  &now,
		flags:       entry.flags,
		isDirectory: entry.isDirectory,
		content:     entry.content,
	}
	if entry.isDirectory {
		copy.children = make(map[string]*fakeSAFEntry)
	}
	dstParent.children[entry.name] = copy
	return SAFStatResult{
		Name:        copy.name,
		MIMEType:    copy.mimeType,
		SizeBytes:   copy.size,
		ModifiedAt:  copy.modifiedAt,
		Flags:       copy.flags,
		IsDirectory: copy.isDirectory,
	}, nil
}

func (f *FakeSAFBridge) Delete(ctx context.Context, grantID string, documentID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.isGrantValid(grantID) {
		return SAFNativeError{Code: "PERMISSION_DENIED", PermissionRevoked: true}
	}
	entry := f.findEntryByDocID(grantID, documentID)
	if entry == nil {
		return SAFNativeError{Code: "NOT_FOUND"}
	}
	parentDocID := f.parentDocIDFromDocID(documentID)
	parent := f.findEntryByDocID(grantID, parentDocID)
	if parent == nil {
		return SAFNativeError{Code: "NOT_FOUND"}
	}
	delete(parent.children, entry.name)
	return nil
}

func (f *FakeSAFBridge) ResolvePath(ctx context.Context, grantID string, relativePath string) (SAFDocumentRef, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if !f.isGrantValid(grantID) {
		return SAFDocumentRef{}, SAFNativeError{Code: "PERMISSION_DENIED", PermissionRevoked: true}
	}
	tree, ok := f.trees[grantID]
	if !ok {
		return SAFDocumentRef{}, SAFNativeError{Code: "NOT_FOUND"}
	}
	if relativePath == "" {
		return SAFDocumentRef{
			GrantID:     grantID,
			DocumentID:  grantID + ":root",
			Name:        "root",
			MIMEType:    SAFMIMETypeDir,
			Flags:       tree.flags,
			IsDirectory: true,
		}, nil
	}
	segments := strings.Split(relativePath, "/")
	current := tree
	docID := grantID + ":root"
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		child, ok := current.children[seg]
		if !ok {
			return SAFDocumentRef{}, SAFNativeError{Code: "NOT_FOUND"}
		}
		current = child
		docID = docID + "/" + seg
	}
	return SAFDocumentRef{
		GrantID:     grantID,
		DocumentID:  docID,
		Name:        current.name,
		MIMEType:    current.mimeType,
		Flags:       current.flags,
		IsDirectory: current.isDirectory,
	}, nil
}

func (f *FakeSAFBridge) CreateFile(ctx context.Context, grantID string, input SAFCreateFileInput) (SAFStatResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.isGrantValid(grantID) {
		return SAFStatResult{}, SAFNativeError{Code: "PERMISSION_DENIED", PermissionRevoked: true}
	}
	parent := f.findEntryByDocID(grantID, input.ParentDocumentID)
	if parent == nil {
		return SAFStatResult{}, SAFNativeError{Code: "NOT_FOUND"}
	}
	if !parent.isDirectory {
		return SAFStatResult{}, SAFNativeError{Code: "NOT_DIRECTORY"}
	}
	if _, exists := parent.children[input.DisplayName]; exists {
		return SAFStatResult{}, SAFNativeError{Code: "ALREADY_EXISTS"}
	}
	now := time.Now().UTC()
	newEntry := &fakeSAFEntry{
		name:        input.DisplayName,
		mimeType:    input.MIMEType,
		isDirectory: false,
		flags:       SAFFlagSupportsDelete | SAFFlagSupportsRename | SAFFlagSupportsMove | SAFFlagSupportsCopy | SAFFlagSupportsWrite,
		modifiedAt:  &now,
	}
	parent.children[input.DisplayName] = newEntry
	f.documentIDs[grantID+":"+input.ParentDocumentID+"/"+input.DisplayName] = input.ParentDocumentID + "/" + input.DisplayName
	return SAFStatResult{
		Name:        newEntry.name,
		MIMEType:    newEntry.mimeType,
		SizeBytes:   newEntry.size,
		ModifiedAt:  newEntry.modifiedAt,
		Flags:       newEntry.flags,
		IsDirectory: false,
	}, nil
}

func (f *FakeSAFBridge) isGrantValid(grantID string) bool {
	valid, exists := f.grants[grantID]
	return exists && valid
}

func (f *FakeSAFBridge) findEntryByPath(grantID string, documentID string, name string) *fakeSAFEntry {
	if name == "" || name == "root" {
		return f.trees[grantID]
	}
	return f.findEntryByDocID(grantID, documentID)
}

func (f *FakeSAFBridge) findEntryByDocID(grantID string, docID string) *fakeSAFEntry {
	if docID == grantID+":root" {
		return f.trees[grantID]
	}
	tree := f.trees[grantID]
	if tree == nil {
		return nil
	}
	relPath := strings.TrimPrefix(docID, grantID+":root/")
	if relPath == docID {
		return nil
	}
	segments := strings.Split(relPath, "/")
	current := tree
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		child, ok := current.children[seg]
		if !ok {
			return nil
		}
		current = child
	}
	return current
}

func (f *FakeSAFBridge) parentDocID(documentID string, name string) string {
	if documentID == "" {
		return ""
	}
	idx := strings.LastIndex(documentID, "/"+name)
	if idx < 0 {
		return documentID
	}
	return documentID[:idx]
}

func (f *FakeSAFBridge) parentDocIDFromDocID(documentID string) string {
	idx := strings.LastIndex(documentID, "/")
	if idx < 0 {
		return documentID
	}
	return documentID[:idx]
}

var _ SAFBridge = (*FakeSAFBridge)(nil)

func init() {
	_ = fmt.Sprintf
}

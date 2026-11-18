package kernel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/u-ai/backend/internal/workspace"
	workspacegit "github.com/u-ai/backend/internal/workspace/git"
)

const maxWorkspaceHostWriteBytes = 8 << 20

const (
	maxDshWorkspaceFileBytes = 32 << 20
	maxDshWorkspacePageBytes = 2 << 20
	maxDshWorkspaceTextBytes = 64 << 20
	maxDshWorkspacePageLines = 5000
	maxDshWorkspaceSearchFiles = 20000
	maxDshWorkspaceSearchResults = 2000
)

type ConversationWorkspace struct {
	ProjectID   string
	WorkspaceID string
	RootURI     string
	ReadOnly    bool
}

type ConversationWorkspaceResolver interface {
	ResolveConversationWorkspace(ctx context.Context, conversationID string) (ConversationWorkspace, error)
}

type workspaceFileRequest struct {
	SessionID string `json:"sessionId"`
	Path      string `json:"path"`
	BaseFile  string `json:"baseFile,omitempty"`
}

type workspaceTextFileRequest struct {
	SessionID string `json:"sessionId"`
	Path      string `json:"path"`
	BaseFile  string `json:"baseFile,omitempty"`
	Offset    int    `json:"offset,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

type workspaceByteFileRequest struct {
	SessionID string `json:"sessionId"`
	Path      string `json:"path"`
	BaseFile  string `json:"baseFile,omitempty"`
	Offset    int64  `json:"offset,omitempty"`
	Length    int64  `json:"length,omitempty"`
}

type workspaceDirectoryRequest struct {
	SessionID string `json:"sessionId"`
	Path      string `json:"path,omitempty"`
}

type workspaceSearchFileRequest struct {
	SessionID        string   `json:"sessionId"`
	Query            string   `json:"query"`
	ExcludedPaths    []string `json:"excludedPaths,omitempty"`
	IncludeHidden    bool     `json:"includeHidden,omitempty"`
	RespectGitignore bool     `json:"respectGitignore,omitempty"`
}

type WorkspaceBridge struct {
	mu                   sync.RWMutex
	workspaces           *workspace.Service
	git                  *workspacegit.GitHandler
	conversationResolver ConversationWorkspaceResolver
}

type workspaceOperationInput struct {
	Operation string          `json:"operation"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

func NewWorkspaceBridge(service *workspace.Service) *WorkspaceBridge {
	return &WorkspaceBridge{workspaces: service}
}

func (b *WorkspaceBridge) SetGitHandler(handler *workspacegit.GitHandler) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.git = handler
	b.mu.Unlock()
}

func (b *WorkspaceBridge) SetConversationWorkspaceResolver(resolver ConversationWorkspaceResolver) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.conversationResolver = resolver
	b.mu.Unlock()
}

func (b *WorkspaceBridge) Read(ctx context.Context, input json.RawMessage) (any, error) {
	service, git, err := b.dependencies()
	if err != nil {
		return nil, err
	}
	operation, payload, err := parseWorkspaceOperation(input)
	if err != nil {
		return nil, err
	}
	switch operation {
	case "conversation_workspace":
		request, err := decodeWorkspacePayload[struct {
			ConversationID string `json:"conversationId"`
		}](payload)
		if err != nil {
			return nil, err
		}
		return b.conversationWorkspace(ctx, request.ConversationID)
	case "file_stat":
		request, err := decodeWorkspacePayload[workspaceFileRequest](payload)
		if err != nil {
			return nil, err
		}
		return b.statSessionFile(ctx, request)
	case "file_read_text":
		request, err := decodeWorkspacePayload[workspaceTextFileRequest](payload)
		if err != nil {
			return nil, err
		}
		return b.readSessionTextFile(ctx, request)
	case "file_read_bytes":
		request, err := decodeWorkspacePayload[workspaceByteFileRequest](payload)
		if err != nil {
			return nil, err
		}
		return b.readSessionFileBytes(ctx, request)
	case "file_list":
		request, err := decodeWorkspacePayload[workspaceDirectoryRequest](payload)
		if err != nil {
			return nil, err
		}
		return b.listSessionDirectory(ctx, request)
	case "file_search":
		request, err := decodeWorkspacePayload[workspaceSearchFileRequest](payload)
		if err != nil {
			return nil, err
		}
		return b.searchSessionFiles(ctx, request)
	case "list_mounts":
		return service.ListMounts(ctx)
	case "list":
		request, err := decodeWorkspacePayload[struct {
			URI    string `json:"uri"`
			Limit  int    `json:"limit,omitempty"`
			Cursor string `json:"cursor,omitempty"`
		}](payload)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(request.URI) == "" {
			return nil, fmt.Errorf("uri is required")
		}
		if request.Limit <= 0 {
			request.Limit = 500
		}
		if request.Limit > 1000 {
			request.Limit = 1000
		}
		return service.List(ctx, request.URI, workspace.ListOptions{Limit: request.Limit, Cursor: request.Cursor})
	case "stat":
		request, err := decodeWorkspacePayload[struct {
			URI string `json:"uri"`
		}](payload)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(request.URI) == "" {
			return nil, fmt.Errorf("uri is required")
		}
		return service.Stat(ctx, request.URI)
	case "read_file":
		request, err := decodeWorkspacePayload[struct {
			URI      string `json:"uri"`
			Offset   int64  `json:"offset,omitempty"`
			MaxBytes int64  `json:"maxBytes,omitempty"`
			Encoding string `json:"encoding,omitempty"`
		}](payload)
		if err != nil {
			return nil, err
		}
		return readWorkspaceFile(ctx, service, request.URI, request.Offset, request.MaxBytes, request.Encoding)
	case "browse":
		request, err := decodeWorkspacePayload[struct {
			URI      string `json:"uri"`
			MaxBytes int64  `json:"maxBytes,omitempty"`
		}](payload)
		if err != nil {
			return nil, err
		}
		return browseWorkspace(ctx, service, request.URI, request.MaxBytes)
	case "git_status":
		request, err := decodeWorkspacePayload[struct {
			WorkspaceURI   string `json:"workspaceUri"`
			IncludeIgnored bool   `json:"includeIgnored,omitempty"`
		}](payload)
		if err != nil {
			return nil, err
		}
		if git == nil {
			return nil, fmt.Errorf("workspace git service is unavailable")
		}
		return git.Status(ctx, request.WorkspaceURI, request.IncludeIgnored)
	case "git_diff":
		request, err := decodeWorkspacePayload[workspacegit.GitDiffRequest](payload)
		if err != nil {
			return nil, err
		}
		if git == nil {
			return nil, fmt.Errorf("workspace git service is unavailable")
		}
		return git.Diff(ctx, request)
	case "git_log":
		request, err := decodeWorkspacePayload[workspacegit.GitLogRequest](payload)
		if err != nil {
			return nil, err
		}
		if git == nil {
			return nil, fmt.Errorf("workspace git service is unavailable")
		}
		return git.Log(ctx, request)
	case "git_branches":
		request, err := decodeWorkspacePayload[struct {
			WorkspaceURI string `json:"workspaceUri"`
		}](payload)
		if err != nil {
			return nil, err
		}
		if git == nil {
			return nil, fmt.Errorf("workspace git service is unavailable")
		}
		return git.ListBranches(ctx, request.WorkspaceURI)
	case "git_remotes":
		request, err := decodeWorkspacePayload[struct {
			WorkspaceURI string `json:"workspaceUri"`
		}](payload)
		if err != nil {
			return nil, err
		}
		if git == nil {
			return nil, fmt.Errorf("workspace git service is unavailable")
		}
		return git.ListRemotes(ctx, request.WorkspaceURI)
	case "isolated_info":
		request, err := decodeWorkspacePayload[struct {
			WorkspaceURI string `json:"workspaceUri"`
		}](payload)
		if err != nil {
			return nil, err
		}
		if git == nil {
			return nil, fmt.Errorf("workspace git service is unavailable")
		}
		return git.IsolatedInfo(ctx, request.WorkspaceURI)
	default:
		return nil, fmt.Errorf("unsupported workspace read operation %q", operation)
	}
}

func (b *WorkspaceBridge) Write(ctx context.Context, input json.RawMessage) (any, error) {
	service, git, err := b.dependencies()
	if err != nil {
		return nil, err
	}
	operation, payload, err := parseWorkspaceOperation(input)
	if err != nil {
		return nil, err
	}
	switch operation {
	case "write_file":
		request, err := decodeWorkspacePayload[struct {
			URI       string `json:"uri"`
			Content   string `json:"content"`
			Overwrite bool   `json:"overwrite"`
			Atomic    bool   `json:"atomic"`
		}](payload)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(request.URI) == "" {
			return nil, fmt.Errorf("uri is required")
		}
		if len(request.Content) > maxWorkspaceHostWriteBytes {
			return nil, fmt.Errorf("workspace content exceeds %d bytes", maxWorkspaceHostWriteBytes)
		}
		return service.Write(ctx, request.URI, strings.NewReader(request.Content), workspace.WriteOptions{
			Overwrite: request.Overwrite,
			Atomic:    request.Atomic,
		})
	case "mkdir":
		request, err := decodeWorkspacePayload[struct {
			URI string `json:"uri"`
		}](payload)
		if err != nil {
			return nil, err
		}
		return service.Mkdir(ctx, request.URI)
	case "rename":
		request, err := decodeWorkspacePayload[struct {
			URI     string `json:"uri"`
			NewName string `json:"newName"`
		}](payload)
		if err != nil {
			return nil, err
		}
		return service.Rename(ctx, request.URI, request.NewName)
	case "move":
		request, err := decodeWorkspacePayload[struct {
			SourceURI         string `json:"sourceUri"`
			DestinationDirURI string `json:"destinationDirUri"`
		}](payload)
		if err != nil {
			return nil, err
		}
		return service.Move(ctx, request.SourceURI, request.DestinationDirURI)
	case "copy":
		request, err := decodeWorkspacePayload[struct {
			SourceURI         string `json:"sourceUri"`
			DestinationDirURI string `json:"destinationDirUri"`
		}](payload)
		if err != nil {
			return nil, err
		}
		return service.Copy(ctx, request.SourceURI, request.DestinationDirURI)
	case "delete":
		request, err := decodeWorkspacePayload[struct {
			URI       string `json:"uri"`
			Recursive bool   `json:"recursive"`
		}](payload)
		if err != nil {
			return nil, err
		}
		if err := service.Delete(ctx, request.URI, workspace.DeleteOptions{Recursive: request.Recursive}); err != nil {
			return nil, err
		}
		return map[string]any{"deleted": true, "uri": request.URI}, nil
	case "git_add":
		request, err := decodeWorkspacePayload[workspacegit.GitAddRequest](payload)
		if err != nil {
			return nil, err
		}
		if git == nil {
			return nil, fmt.Errorf("workspace git service is unavailable")
		}
		return git.Add(ctx, request)
	case "git_restore":
		request, err := decodeWorkspacePayload[workspacegit.GitRestoreRequest](payload)
		if err != nil {
			return nil, err
		}
		if git == nil {
			return nil, fmt.Errorf("workspace git service is unavailable")
		}
		return git.Restore(ctx, request)
	case "git_commit":
		request, err := decodeWorkspacePayload[workspacegit.GitCommitRequest](payload)
		if err != nil {
			return nil, err
		}
		if git == nil {
			return nil, fmt.Errorf("workspace git service is unavailable")
		}
		return git.Commit(ctx, request)
	case "git_checkout":
		request, err := decodeWorkspacePayload[workspacegit.GitCheckoutRequest](payload)
		if err != nil {
			return nil, err
		}
		if git == nil {
			return nil, fmt.Errorf("workspace git service is unavailable")
		}
		return git.Checkout(ctx, request)
	case "git_fetch":
		request, err := decodeWorkspacePayload[workspacegit.GitFetchRequest](payload)
		if err != nil {
			return nil, err
		}
		if git == nil {
			return nil, fmt.Errorf("workspace git service is unavailable")
		}
		return git.Fetch(ctx, request)
	case "git_pull":
		request, err := decodeWorkspacePayload[workspacegit.GitPullRequest](payload)
		if err != nil {
			return nil, err
		}
		if git == nil {
			return nil, fmt.Errorf("workspace git service is unavailable")
		}
		return git.Pull(ctx, request)
	case "git_push":
		request, err := decodeWorkspacePayload[workspacegit.GitPushRequest](payload)
		if err != nil {
			return nil, err
		}
		if git == nil {
			return nil, fmt.Errorf("workspace git service is unavailable")
		}
		return git.Push(ctx, request)
	case "create_isolated":
		request, err := decodeWorkspacePayload[workspacegit.IsolatedCreateRequest](payload)
		if err != nil {
			return nil, err
		}
		if git == nil {
			return nil, fmt.Errorf("workspace git service is unavailable")
		}
		return git.CreateIsolated(ctx, request)
	case "delete_isolated":
		request, err := decodeWorkspacePayload[struct {
			WorkspaceURI string `json:"workspaceUri"`
			Force        bool   `json:"force"`
		}](payload)
		if err != nil {
			return nil, err
		}
		if git == nil {
			return nil, fmt.Errorf("workspace git service is unavailable")
		}
		if err := git.DeleteIsolated(ctx, request.WorkspaceURI, request.Force); err != nil {
			return nil, err
		}
		return map[string]any{"deleted": true, "workspaceUri": request.WorkspaceURI}, nil
	default:
		return nil, fmt.Errorf("unsupported workspace write operation %q", operation)
	}
}

func (b *WorkspaceBridge) conversationWorkspace(ctx context.Context, conversationID string) (any, error) {
	resolved, err := b.resolveConversationWorkspace(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"conversationId": conversationID,
		"projectId":      resolved.ProjectID,
		"workspaceId":    resolved.WorkspaceID,
		"rootUri":        resolved.RootURI,
		"readOnly":       resolved.ReadOnly,
	}, nil
}

func (b *WorkspaceBridge) resolveConversationWorkspace(ctx context.Context, conversationID string) (ConversationWorkspace, error) {
	if b == nil {
		return ConversationWorkspace{}, fmt.Errorf("workspace bridge is unavailable")
	}
	b.mu.RLock()
	resolver := b.conversationResolver
	b.mu.RUnlock()
	if resolver == nil {
		return ConversationWorkspace{}, fmt.Errorf("conversation workspace resolver is unavailable")
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return ConversationWorkspace{}, fmt.Errorf("conversationId is required")
	}
	return resolver.ResolveConversationWorkspace(ctx, conversationID)
}

func (b *WorkspaceBridge) statSessionFile(ctx context.Context, request workspaceFileRequest) (any, error) {
	service, target, entry, err := b.sessionFileTarget(ctx, request)
	if err != nil {
		return nil, err
	}
	_ = service
	if entry.Type == workspace.WorkspaceEntryTypeDirectory {
		return nil, fmt.Errorf("workspace path %q is a directory", request.Path)
	}
	return dshWorkspaceFileStat(target, entry), nil
}

func (b *WorkspaceBridge) readSessionTextFile(ctx context.Context, request workspaceTextFileRequest) (any, error) {
	service, target, entry, err := b.sessionFileTarget(ctx, workspaceFileRequest{
		SessionID: request.SessionID,
		Path:      request.Path,
		BaseFile:  request.BaseFile,
	})
	if err != nil {
		return nil, err
	}
	if entry.Type == workspace.WorkspaceEntryTypeDirectory {
		return nil, fmt.Errorf("workspace path %q is a directory", request.Path)
	}
	offset := request.Offset
	if offset <= 0 {
		offset = 1
	}
	limit := request.Limit
	if limit <= 0 {
		limit = maxDshWorkspacePageLines
	}
	if limit > maxDshWorkspacePageLines {
		return nil, fmt.Errorf("workspace text limit %d exceeds %d lines", limit, maxDshWorkspacePageLines)
	}
	content, err := readWorkspaceBytes(ctx, service, target.uri, maxDshWorkspaceTextBytes, 0, entry.SizeBytes)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(content) || strings.ContainsRune(string(content), '\x00') {
		return nil, fmt.Errorf("workspace path %q is not UTF-8 text", request.Path)
	}
	lines := strings.Split(string(content), "\n")
	if strings.HasSuffix(string(content), "\n") {
		lines = lines[:len(lines)-1]
	}
	if len(content) == 0 {
		lines = nil
	}
	start := offset - 1
	if start > len(lines) {
		start = len(lines)
	}
	end := start + limit
	if end > len(lines) {
		end = len(lines)
	}
	page := lines[start:end]
	textBytes := 0
	for index, line := range page {
		textBytes += len(line)
		if index > 0 {
			textBytes++
		}
	}
	if textBytes > maxDshWorkspacePageBytes {
		return nil, fmt.Errorf("workspace text page for %q exceeds %d bytes", request.Path, maxDshWorkspacePageBytes)
	}
	result := dshWorkspaceFileStat(target, entry)
	result["offset"] = offset
	result["text"] = strings.Join(page, "\n")
	result["lines"] = len(page)
	result["eof"] = start+len(page) >= len(lines)
	return result, nil
}

func (b *WorkspaceBridge) readSessionFileBytes(ctx context.Context, request workspaceByteFileRequest) (any, error) {
	service, target, entry, err := b.sessionFileTarget(ctx, workspaceFileRequest{
		SessionID: request.SessionID,
		Path:      request.Path,
		BaseFile:  request.BaseFile,
	})
	if err != nil {
		return nil, err
	}
	if entry.Type == workspace.WorkspaceEntryTypeDirectory {
		return nil, fmt.Errorf("workspace path %q is a directory", request.Path)
	}
	offset := request.Offset
	if offset < 0 {
		return nil, fmt.Errorf("workspace byte offset must be non-negative")
	}
	length := request.Length
	if length < 0 {
		return nil, fmt.Errorf("workspace byte length must be non-negative")
	}
	if length > maxDshWorkspacePageBytes {
		return nil, fmt.Errorf("workspace byte window %d exceeds %d bytes", length, maxDshWorkspacePageBytes)
	}
	if length == 0 {
		if entry.SizeBytes != nil && *entry.SizeBytes-offset > maxDshWorkspaceFileBytes {
			return nil, fmt.Errorf("workspace file %q exceeds %d bytes", request.Path, maxDshWorkspaceFileBytes)
		}
		length = maxDshWorkspaceFileBytes
	}
	content, err := readWorkspaceBytes(ctx, service, target.uri, length, offset, entry.SizeBytes)
	if err != nil {
		return nil, err
	}
	result := dshWorkspaceFileStat(target, entry)
	result["offset"] = offset
	result["dataBase64"] = base64.StdEncoding.EncodeToString(content)
	eof := int64(len(content)) < length
	if entry.SizeBytes != nil {
		eof = offset+int64(len(content)) >= *entry.SizeBytes
	}
	result["eof"] = eof
	return result, nil
}

func (b *WorkspaceBridge) listSessionDirectory(ctx context.Context, request workspaceDirectoryRequest) (any, error) {
	service, mount, err := b.sessionWorkspaceMount(ctx, request.SessionID)
	if err != nil {
		return nil, err
	}
	relativePath, err := sessionWorkspaceRelativeAllowRoot(*mount, request.Path)
	if err != nil {
		return nil, err
	}
	root := strings.TrimSuffix(mount.RootURI, "/")
	uri := root
	if relativePath != "." {
		uri += "/" + relativePath
	}
	listing, err := service.List(ctx, uri, workspace.ListOptions{Limit: workspace.MaxListEntries})
	if err != nil {
		return nil, err
	}
	entries := make([]map[string]any, 0, len(listing.Entries))
	for _, entry := range listing.Entries {
		entryType := "other"
		switch entry.Type {
		case workspace.WorkspaceEntryTypeFile:
			entryType = "file"
		case workspace.WorkspaceEntryTypeDirectory:
			entryType = "directory"
		}
		value := map[string]any{"name": entry.Name, "type": entryType}
		if entry.SizeBytes != nil {
			value["size"] = *entry.SizeBytes
		}
		entries = append(entries, value)
	}
	pathValue := relativePath
	if pathValue == "." {
		pathValue = ""
	}
	return map[string]any{
		"path":      pathValue,
		"entries":   entries,
		"truncated": len(listing.Entries) >= workspace.MaxListEntries,
	}, nil
}

type dshSearchMatch struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

type dshIgnoreRule struct {
	base    string
	pattern *regexp.Regexp
	negated bool
}

type dshSearchAccumulator struct {
	query            string
	excludedPaths    []string
	includeHidden    bool
	respectGitignore bool
	filesScanned     int
	truncated        bool
	incomplete       bool
	matches          map[string]string
}

var errDshWorkspaceSearchLimit = fmt.Errorf("workspace search file limit reached")

func (b *WorkspaceBridge) searchSessionFiles(ctx context.Context, request workspaceSearchFileRequest) (any, error) {
	service, mount, err := b.sessionWorkspaceMount(ctx, request.SessionID)
	if err != nil {
		return nil, err
	}
	query := strings.TrimSpace(request.Query)
	if query == "" {
		return nil, fmt.Errorf("workspace search query is required")
	}
	accumulator := &dshSearchAccumulator{
		query:            strings.ToLower(query),
		excludedPaths:    request.ExcludedPaths,
		includeHidden:    request.IncludeHidden,
		respectGitignore: request.RespectGitignore,
		matches:          map[string]string{},
	}
	if mount.Kind == workspace.WorkspaceKindLocal && strings.TrimSpace(mount.LocalRoot) != "" {
		root := filepath.Clean(mount.LocalRoot)
		info, statErr := os.Stat(root)
		if statErr != nil {
			return nil, statErr
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("workspace root %q is not a directory", root)
		}
		err = accumulator.walkLocal(ctx, root, "", nil)
	} else {
		err = accumulator.walkWorkspace(ctx, service, *mount, "")
		if accumulator.respectGitignore {
			accumulator.incomplete = true
		}
	}
	if err != nil && err != errDshWorkspaceSearchLimit {
		return nil, err
	}
	matches := make([]dshSearchMatch, 0, len(accumulator.matches))
	for relative, entryType := range accumulator.matches {
		matches = append(matches, dshSearchMatch{Path: relative, Type: entryType})
	}
	sort.Slice(matches, func(left, right int) bool {
		leftDepth := strings.Count(matches[left].Path, "/")
		rightDepth := strings.Count(matches[right].Path, "/")
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return matches[left].Path < matches[right].Path
	})
	return map[string]any{
		"matches":    matches,
		"truncated":  accumulator.truncated,
		"incomplete": accumulator.incomplete,
	}, nil
}

func (a *dshSearchAccumulator) walkLocal(ctx context.Context, root, relative string, inherited []dshIgnoreRule) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	absolute := root
	if relative != "" {
		absolute = filepath.Join(root, filepath.FromSlash(relative))
	}
	rules := inherited
	if a.respectGitignore {
		content, err := os.ReadFile(filepath.Join(absolute, ".gitignore"))
		if err == nil {
			rules = append(append([]dshIgnoreRule(nil), inherited...), parseDshIgnoreRules(relative, string(content))...)
		} else if !os.IsNotExist(err) {
			a.incomplete = true
		}
	}
	entries, err := os.ReadDir(absolute)
	if err != nil {
		if relative == "" {
			return err
		}
		a.incomplete = true
		return nil
	}
	sort.Slice(entries, func(left, right int) bool {
		return entries[left].Name() < entries[right].Name()
	})
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := entry.Name()
		if name == ".git" {
			continue
		}
		child := name
		if relative != "" {
			child = path.Join(relative, name)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		if entry.IsDir() {
			if err := a.walkLocal(ctx, root, child, rules); err != nil {
				if err == errDshWorkspaceSearchLimit {
					return err
				}
				return err
			}
			continue
		}
		if !entry.Type().IsRegular() || dshSearchHidden(child, a.includeHidden) || dshSearchExcluded(child, a.excludedPaths) || dshSearchIgnored(child, rules) {
			continue
		}
		a.filesScanned += 1
		if a.filesScanned > maxDshWorkspaceSearchFiles {
			a.incomplete = true
			return errDshWorkspaceSearchLimit
		}
		a.addMatch(child)
	}
	return nil
}

func (a *dshSearchAccumulator) walkWorkspace(ctx context.Context, service *workspace.Service, mount workspace.WorkspaceMount, relative string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root := strings.TrimSuffix(mount.RootURI, "/")
	uri := root
	if relative != "" {
		uri += "/" + relative
	}
	listing, err := service.List(ctx, uri, workspace.ListOptions{Limit: workspace.MaxListEntries})
	if err != nil {
		if relative == "" {
			return err
		}
		a.incomplete = true
		return nil
	}
	if len(listing.Entries) >= workspace.MaxListEntries {
		a.incomplete = true
	}
	sort.Slice(listing.Entries, func(left, right int) bool {
		return listing.Entries[left].Name < listing.Entries[right].Name
	})
	for _, entry := range listing.Entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Name == ".git" {
			continue
		}
		child := entry.Name
		if relative != "" {
			child = path.Join(relative, entry.Name)
		}
		if entry.Type == workspace.WorkspaceEntryTypeDirectory {
			if err := a.walkWorkspace(ctx, service, mount, child); err != nil {
				return err
			}
			continue
		}
		if dshSearchHidden(child, a.includeHidden) || dshSearchExcluded(child, a.excludedPaths) {
			continue
		}
		a.filesScanned += 1
		if a.filesScanned > maxDshWorkspaceSearchFiles {
			a.incomplete = true
			return errDshWorkspaceSearchLimit
		}
		a.addMatch(child)
	}
	return nil
}

func (a *dshSearchAccumulator) addMatch(relative string) {
	name := path.Base(relative)
	searchValue := name
	if strings.Contains(a.query, "/") {
		searchValue = relative
	}
	if !strings.Contains(strings.ToLower(searchValue), a.query) {
		return
	}
	ancestor := path.Dir(relative)
	for ancestor != "." && ancestor != "/" && ancestor != "" {
		a.matches[ancestor] = "directory"
		ancestor = path.Dir(ancestor)
	}
	if _, exists := a.matches[relative]; !exists && len(a.matches) >= maxDshWorkspaceSearchResults {
		a.truncated = true
		return
	}
	a.matches[relative] = "file"
}

func dshSearchHidden(relative string, includeHidden bool) bool {
	if includeHidden {
		return false
	}
	for _, segment := range strings.Split(relative, "/") {
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}

func dshSearchExcluded(relative string, excludedPaths []string) bool {
	relative = strings.Trim(relative, "/")
	for _, value := range excludedPaths {
		excluded := strings.Trim(strings.TrimSpace(value), "/")
		if excluded == "" {
			continue
		}
		if strings.Contains(excluded, "/") {
			if relative == excluded || strings.HasPrefix(relative, excluded+"/") {
				return true
			}
			continue
		}
		if path.Base(relative) == excluded {
			return true
		}
	}
	return false
}

func dshSearchIgnored(relative string, rules []dshIgnoreRule) bool {
	ignored := false
	for _, rule := range rules {
		if rule.matches(relative) {
			ignored = !rule.negated
		}
	}
	return ignored
}

func (r dshIgnoreRule) matches(relative string) bool {
	value := strings.Trim(relative, "/")
	if r.base != "" {
		if value == r.base {
			value = ""
		} else if strings.HasPrefix(value, r.base+"/") {
			value = strings.TrimPrefix(value, r.base+"/")
		} else {
			return false
		}
	}
	return value != "" && r.pattern.MatchString(value)
}

func parseDshIgnoreRules(base, content string) []dshIgnoreRule {
	rules := make([]dshIgnoreRule, 0)
	for _, raw := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		negated := false
		if strings.HasPrefix(line, "!") {
			negated = true
			line = strings.TrimPrefix(line, "!")
		}
		if strings.HasPrefix(line, "\\#") || strings.HasPrefix(line, "\\!") {
			line = strings.TrimPrefix(line, "\\")
		}
		line = strings.TrimSuffix(line, "/")
		if line == "" {
			continue
		}
		anchored := strings.HasPrefix(line, "/") || strings.Contains(line, "/")
		line = strings.TrimPrefix(line, "/")
		matcher, err := regexp.Compile(dshIgnorePattern(line, anchored))
		if err != nil {
			continue
		}
		rules = append(rules, dshIgnoreRule{base: strings.Trim(base, "/"), pattern: matcher, negated: negated})
	}
	return rules
}

func dshIgnorePattern(pattern string, anchored bool) string {
	var builder strings.Builder
	if anchored {
		builder.WriteString("^")
	} else {
		builder.WriteString("(?:^|/)")
	}
	for index := 0; index < len(pattern); index += 1 {
		switch pattern[index] {
		case '*':
			if index+1 < len(pattern) && pattern[index+1] == '*' {
				index += 1
				if index+1 < len(pattern) && pattern[index+1] == '/' {
					index += 1
					builder.WriteString("(?:.*/)?")
				} else {
					builder.WriteString(".*")
				}
			} else {
				builder.WriteString("[^/]*")
			}
		case '?':
			builder.WriteString("[^/]")
		default:
			builder.WriteString(regexp.QuoteMeta(string(pattern[index])))
		}
	}
	builder.WriteString("(?:/.*)?$")
	return builder.String()
}

type sessionWorkspaceTarget struct {
	uri          string
	relativePath string
	absolutePath string
	mount        workspace.WorkspaceMount
}

func (b *WorkspaceBridge) sessionFileTarget(ctx context.Context, request workspaceFileRequest) (*workspace.Service, sessionWorkspaceTarget, workspace.WorkspaceEntry, error) {
	service, resolvedMount, err := b.sessionWorkspaceMount(ctx, request.SessionID)
	if err != nil {
		return nil, sessionWorkspaceTarget{}, workspace.WorkspaceEntry{}, err
	}
	mount := *resolvedMount
	value := request.Path
	if strings.TrimSpace(request.BaseFile) != "" {
		baseRelative, err := sessionWorkspaceRelative(mount, request.BaseFile)
		if err != nil {
			return nil, sessionWorkspaceTarget{}, workspace.WorkspaceEntry{}, err
		}
		if !path.IsAbs(value) && !strings.Contains(value, "://") {
			value = path.Join(path.Dir(baseRelative), value)
		}
	}
	relativePath, err := sessionWorkspaceRelative(mount, value)
	if err != nil {
		return nil, sessionWorkspaceTarget{}, workspace.WorkspaceEntry{}, err
	}
	if relativePath == "." || relativePath == "" {
		return nil, sessionWorkspaceTarget{}, workspace.WorkspaceEntry{}, fmt.Errorf("workspace file path is required")
	}
	root := strings.TrimSuffix(mount.RootURI, "/")
	uri := root + "/" + relativePath
	absolutePath := uri
	if mount.Kind == workspace.WorkspaceKindLocal && strings.TrimSpace(mount.LocalRoot) != "" {
		absolutePath = filepath.Join(mount.LocalRoot, filepath.FromSlash(relativePath))
	}
	entry, err := service.Stat(ctx, uri)
	if err != nil {
		return nil, sessionWorkspaceTarget{}, workspace.WorkspaceEntry{}, err
	}
	return service, sessionWorkspaceTarget{
		uri:          uri,
		relativePath: relativePath,
		absolutePath: absolutePath,
		mount:        mount,
	}, entry, nil
}

func (b *WorkspaceBridge) sessionWorkspaceMount(ctx context.Context, sessionID string) (*workspace.Service, *workspace.WorkspaceMount, error) {
	service, _, err := b.dependencies()
	if err != nil {
		return nil, nil, err
	}
	resolved, err := b.resolveConversationWorkspace(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	rootURI := resolved.RootURI
	workspaceID := resolved.WorkspaceID
	mounts, err := service.ListMounts(ctx)
	if err != nil {
		return nil, nil, err
	}
	for index := range mounts {
		candidate := &mounts[index]
		if workspaceID != "" && string(candidate.ID) == workspaceID || rootURI != "" && strings.TrimSuffix(candidate.RootURI, "/") == strings.TrimSuffix(rootURI, "/") {
			return service, candidate, nil
		}
	}
	return nil, nil, fmt.Errorf("conversation workspace %q is unavailable", workspaceID)
}

func sessionWorkspaceRelative(mount workspace.WorkspaceMount, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("workspace file path is required")
	}
	root := strings.TrimSuffix(mount.RootURI, "/")
	if value == root || strings.HasPrefix(value, root+"/") {
		value = strings.TrimPrefix(value, root)
	}
	if mount.Kind == workspace.WorkspaceKindLocal && strings.TrimSpace(mount.LocalRoot) != "" && filepath.IsAbs(value) {
		relative, err := filepath.Rel(mount.LocalRoot, value)
		if err != nil {
			return "", err
		}
		value = filepath.ToSlash(relative)
	}
	value = strings.TrimPrefix(value, "/")
	cleaned := path.Clean(value)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("workspace path %q is outside the session workspace", value)
	}
	return cleaned, nil
}

func sessionWorkspaceRelativeAllowRoot(mount workspace.WorkspaceMount, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return ".", nil
	}
	return sessionWorkspaceRelative(mount, value)
}

func dshWorkspaceFileStat(target sessionWorkspaceTarget, entry workspace.WorkspaceEntry) map[string]any {
	version := "unknown"
	if entry.SizeBytes != nil {
		version = fmt.Sprintf("%d", *entry.SizeBytes)
	}
	if entry.ModifiedAt != nil {
		version = fmt.Sprintf("%d:%d", modifiedUnixNano(entry.ModifiedAt), entrySize(entry.SizeBytes))
	}
	value := map[string]any{
		"absolutePath": target.absolutePath,
		"version":      version,
	}
	if entry.SizeBytes != nil {
		value["bytes"] = *entry.SizeBytes
	}
	return value
}

func entrySize(size *int64) int64 {
	if size == nil {
		return 0
	}
	return *size
}

func modifiedUnixNano(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return value.UnixNano()
}

func readWorkspaceBytes(ctx context.Context, service *workspace.Service, uri string, limit, offset int64, size *int64) ([]byte, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("workspace read limit must be positive")
	}
	if offset < 0 {
		return nil, fmt.Errorf("workspace read offset must be non-negative")
	}
	if size != nil && offset >= *size {
		return []byte{}, nil
	}
	content := make([]byte, 0, minInt64(limit, workspace.MaxDirectRead))
	position := offset
	for int64(len(content)) < limit {
		chunkSize := minInt64(workspace.MaxDirectRead, limit-int64(len(content)))
		chunk, err := service.Read(ctx, uri, workspace.ReadOptions{
			Offset:   position,
			MaxBytes: chunkSize,
			Encoding: "utf-8",
		})
		if err != nil {
			return nil, err
		}
		if len(chunk.Content) == 0 {
			break
		}
		content = append(content, chunk.Content...)
		position += int64(len(chunk.Content))
		if int64(len(chunk.Content)) < chunkSize {
			break
		}
	}
	return content, nil
}

func minInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func (b *WorkspaceBridge) dependencies() (*workspace.Service, *workspacegit.GitHandler, error) {
	if b == nil {
		return nil, nil, fmt.Errorf("workspace service is unavailable")
	}
	b.mu.RLock()
	service := b.workspaces
	git := b.git
	b.mu.RUnlock()
	if service == nil {
		return nil, nil, fmt.Errorf("workspace service is unavailable")
	}
	return service, git, nil
}

func parseWorkspaceOperation(input json.RawMessage) (string, json.RawMessage, error) {
	var request workspaceOperationInput
	if err := json.Unmarshal(input, &request); err != nil {
		return "", nil, err
	}
	request.Operation = strings.TrimSpace(request.Operation)
	if request.Operation == "" {
		return "", nil, fmt.Errorf("operation is required")
	}
	if len(request.Payload) == 0 {
		request.Payload = json.RawMessage(`{}`)
	}
	return request.Operation, request.Payload, nil
}

func decodeWorkspacePayload[T any](payload json.RawMessage) (T, error) {
	var value T
	err := json.Unmarshal(payload, &value)
	return value, err
}

func readWorkspaceFile(
	ctx context.Context,
	service *workspace.Service,
	uri string,
	offset int64,
	maxBytes int64,
	encoding string,
) (map[string]any, error) {
	if strings.TrimSpace(uri) == "" {
		return nil, fmt.Errorf("uri is required")
	}
	if offset < 0 {
		return nil, fmt.Errorf("offset must be non-negative")
	}
	if maxBytes <= 0 {
		maxBytes = workspace.MaxDirectRead
	}
	if maxBytes > workspace.MaxDirectRead {
		maxBytes = workspace.MaxDirectRead
	}
	if encoding == "" {
		encoding = "utf-8"
	}
	result, err := service.Read(ctx, uri, workspace.ReadOptions{
		Offset:   offset,
		MaxBytes: maxBytes,
		Encoding: encoding,
	})
	if err != nil {
		return nil, err
	}
	output := map[string]any{
		"resource": result.Resource,
		"isText":   result.IsText,
		"offset":   offset,
		"maxBytes": maxBytes,
	}
	if result.IsText {
		output["content"] = string(result.Content)
	}
	return output, nil
}

func browseWorkspace(ctx context.Context, service *workspace.Service, uri string, maxBytes int64) (map[string]any, error) {
	if strings.TrimSpace(uri) == "" {
		return nil, fmt.Errorf("uri is required")
	}
	entry, err := service.Stat(ctx, uri)
	if err != nil {
		return nil, err
	}
	output := map[string]any{
		"path":  uri,
		"kind":  entry.Type,
		"entry": entry,
	}
	if entry.Type == workspace.WorkspaceEntryTypeDirectory {
		list, err := service.List(ctx, uri, workspace.ListOptions{Limit: 500})
		if err != nil {
			return nil, err
		}
		output["entries"] = list.Entries
		output["hasMore"] = list.HasMore
		output["nextCursor"] = list.NextCursor
		return output, nil
	}
	if !entry.Readable {
		return output, nil
	}
	file, err := readWorkspaceFile(ctx, service, uri, 0, maxBytes, "utf-8")
	if err != nil {
		return nil, err
	}
	for key, value := range file {
		output[key] = value
	}
	return output, nil
}

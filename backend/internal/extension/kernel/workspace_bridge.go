package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/workspace"
	workspacegit "github.com/u-ai/backend/internal/workspace/git"
)

const maxWorkspaceHostWriteBytes = 8 << 20

type WorkspaceBridge struct {
	mu         sync.RWMutex
	workspaces *workspace.Service
	git        *workspacegit.GitHandler
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

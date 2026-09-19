package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/nativebridge"
	"github.com/u-ai/backend/internal/workspace"
)

type workspaceSAFBridge struct {
	bridge nativebridge.Bridge
}

func newWorkspaceSAFBridge(bridge nativebridge.Bridge) workspace.SAFBridge {
	return &workspaceSAFBridge{bridge: bridge}
}

func (b *workspaceSAFBridge) call(ctx context.Context, operation string, payload map[string]any, result any) error {
	if b.bridge == nil {
		return workspace.SAFNativeError{Code: "PROVIDER_UNAVAILABLE", Message: "android native bridge not configured", ProviderUnavailable: true}
	}
	response, err := b.bridge.Execute(ctx, nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       uuid.NewString(),
		Platform:        "android",
		Operation:       operation,
		Payload:         payload,
	})
	if err != nil {
		return workspace.SAFNativeError{Code: "PROVIDER_UNAVAILABLE", Message: err.Error(), ProviderUnavailable: true}
	}
	if response.Status != "success" {
		message := "native SAF operation failed"
		code := "SAF_FAILED"
		if response.Error != nil {
			message = response.Error.Message
			if response.Error.DomainCode != "" {
				code = response.Error.DomainCode
			} else if response.Error.Code != "" {
				code = response.Error.Code
			}
		}
		return workspace.SAFNativeError{Code: code, Message: message, PermissionRevoked: code == "PERMISSION_REVOKED", ProviderUnavailable: code == "PROVIDER_UNAVAILABLE"}
	}
	data, err := json.Marshal(response.Result)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("decode native SAF response: %w", err)
	}
	return nil
}

func (b *workspaceSAFBridge) GrantStatus(ctx context.Context, grantID string) (workspace.SAFGrantStatus, error) {
	var result workspace.SAFGrantStatus
	return result, b.call(ctx, "workspace.saf.grant_status", map[string]any{"grantId": grantID}, &result)
}
func (b *workspaceSAFBridge) Stat(ctx context.Context, grantID, documentID, name string) (workspace.SAFStatResult, error) {
	var result workspace.SAFStatResult
	return result, b.call(ctx, "workspace.saf.stat", map[string]any{"grantId": grantID, "documentId": documentID, "name": name}, &result)
}
func (b *workspaceSAFBridge) List(ctx context.Context, grantID, documentID string, limit int) ([]workspace.SAFEntryRef, string, error) {
	var result struct {
		Entries []workspace.SAFEntryRef `json:"entries"`
		Cursor  string                  `json:"cursor"`
	}
	err := b.call(ctx, "workspace.saf.list", map[string]any{"grantId": grantID, "documentId": documentID, "limit": limit}, &result)
	return result.Entries, result.Cursor, err
}
func (b *workspaceSAFBridge) Read(ctx context.Context, grantID, documentID string, offset, maxBytes int64) ([]byte, string, bool, error) {
	var result struct {
		Data     []byte `json:"data"`
		Resource string `json:"resource"`
		IsText   bool   `json:"isText"`
	}
	err := b.call(ctx, "workspace.saf.read", map[string]any{"grantId": grantID, "documentId": documentID, "offset": offset, "maxBytes": maxBytes}, &result)
	return result.Data, result.Resource, result.IsText, err
}
func (b *workspaceSAFBridge) Write(ctx context.Context, grantID, documentID, targetName string, source workspace.SAFWriteSource, overwrite bool) (workspace.SAFStatResult, error) {
	var result workspace.SAFStatResult
	return result, b.call(ctx, "workspace.saf.write", map[string]any{"grantId": grantID, "documentId": documentID, "targetName": targetName, "source": source, "overwrite": overwrite}, &result)
}
func (b *workspaceSAFBridge) Mkdir(ctx context.Context, grantID string, input workspace.SAFCreateDirInput) (workspace.SAFStatResult, error) {
	var result workspace.SAFStatResult
	return result, b.call(ctx, "workspace.saf.mkdir", map[string]any{"grantId": grantID, "input": input}, &result)
}
func (b *workspaceSAFBridge) Rename(ctx context.Context, grantID, documentID, newName string) (workspace.SAFStatResult, error) {
	var result workspace.SAFStatResult
	return result, b.call(ctx, "workspace.saf.rename", map[string]any{"grantId": grantID, "documentId": documentID, "newName": newName}, &result)
}
func (b *workspaceSAFBridge) Move(ctx context.Context, grantID, documentID, targetParentDocumentID string) (workspace.SAFStatResult, error) {
	var result workspace.SAFStatResult
	return result, b.call(ctx, "workspace.saf.move", map[string]any{"grantId": grantID, "documentId": documentID, "targetParentDocumentId": targetParentDocumentID}, &result)
}
func (b *workspaceSAFBridge) Copy(ctx context.Context, grantID, documentID, targetParentDocumentID string) (workspace.SAFStatResult, error) {
	var result workspace.SAFStatResult
	return result, b.call(ctx, "workspace.saf.copy", map[string]any{"grantId": grantID, "documentId": documentID, "targetParentDocumentId": targetParentDocumentID}, &result)
}
func (b *workspaceSAFBridge) Delete(ctx context.Context, grantID, documentID string) error {
	return b.call(ctx, "workspace.saf.delete", map[string]any{"grantId": grantID, "documentId": documentID}, &struct{}{})
}
func (b *workspaceSAFBridge) ResolvePath(ctx context.Context, grantID, relativePath string) (workspace.SAFDocumentRef, error) {
	var result workspace.SAFDocumentRef
	return result, b.call(ctx, "workspace.saf.resolve_path", map[string]any{"grantId": grantID, "relativePath": relativePath}, &result)
}
func (b *workspaceSAFBridge) CreateFile(ctx context.Context, grantID string, input workspace.SAFCreateFileInput) (workspace.SAFStatResult, error) {
	var result workspace.SAFStatResult
	return result, b.call(ctx, "workspace.saf.create_file", map[string]any{"grantId": grantID, "input": input}, &result)
}

type workspaceSAFGrantResolver struct{ bridge workspace.SAFBridge }

func (r workspaceSAFGrantResolver) ResolveGrant(grantID string) (workspace.SAFGrantStatus, error) {
	return r.bridge.GrantStatus(context.Background(), grantID)
}

type unavailableWorkspaceCredentialResolver struct{}

func (unavailableWorkspaceCredentialResolver) ResolveCredential(context.Context, string) (*workspace.RemoteCredential, error) {
	return nil, workspace.ErrRemoteCredentialNotFound
}

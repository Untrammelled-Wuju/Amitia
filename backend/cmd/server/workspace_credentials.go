package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/workspace"
)

type workspaceRemoteCredentialResolver struct {
	store secret.Store
}

func newWorkspaceRemoteCredentialResolver(dataDir string) (*workspaceRemoteCredentialResolver, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("data dir is empty")
	}
	store, err := secret.NewEncryptedFileStore(
		filepath.Join(dataDir, "workspace-credentials.json"),
		filepath.Join(dataDir, "workspace-credentials.key"),
	)
	if err != nil {
		return nil, fmt.Errorf("init workspace credential store: %w", err)
	}
	return &workspaceRemoteCredentialResolver{store: store}, nil
}

func (r *workspaceRemoteCredentialResolver) ResolveCredential(ctx context.Context, ref string) (*workspace.RemoteCredential, error) {
	if r.store == nil {
		return nil, workspace.ErrRemoteCredentialNotFound
	}
	if _, parseErr := secret.ParseRef(ref); parseErr != nil {
		return nil, fmt.Errorf("%w: %v", workspace.ErrRemoteCredentialNotFound, parseErr)
	}
	raw, err := r.store.Get(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", workspace.ErrRemoteCredentialNotFound, err)
	}
	cred, err := decodeRemoteCredential(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", workspace.ErrRemoteCredentialNotFound, err)
	}
	return cred, nil
}

func decodeRemoteCredential(raw []byte) (*workspace.RemoteCredential, error) {
	var dto struct {
		Type        string `json:"type"`
		Username    string `json:"username"`
		Password    []byte `json:"password"`
		PrivateKey  []byte `json:"privateKey"`
		Passphrase  []byte `json:"passphrase"`
		BearerToken []byte `json:"bearerToken"`
	}
	if err := json.Unmarshal(raw, &dto); err != nil {
		return nil, fmt.Errorf("decode credential: %w", err)
	}
	out := &workspace.RemoteCredential{Type: workspace.RemoteAuthType(dto.Type), Username: dto.Username}
	switch out.Type {
	case workspace.RemoteAuthTypePassword:
		out.Password = dto.Password
	case workspace.RemoteAuthTypePrivateKey:
		out.PrivateKey = dto.PrivateKey
		out.Passphrase = dto.Passphrase
	case workspace.RemoteAuthTypeBearer:
		out.BearerToken = dto.BearerToken
	default:
		return nil, fmt.Errorf("unsupported credential type: %s", dto.Type)
	}
	return out, nil
}

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/u-ai/backend/internal/mcp"
	"github.com/u-ai/backend/internal/mcp/auth"
	"gorm.io/gorm"
)

type mcpAcquisitionPersistence struct {
	repository *mcp.Repository
	secrets    auth.SecretStore
}

func newMCPAcquisitionPersistence(repository *mcp.Repository, dataDir string) (*mcpAcquisitionPersistence, error) {
	directory := filepath.Join(dataDir, "mcp")
	secrets, err := auth.NewEncryptedFileStore(filepath.Join(directory, "secrets.json"), filepath.Join(directory, "secrets.key"))
	if err != nil {
		return nil, err
	}
	return &mcpAcquisitionPersistence{repository: repository, secrets: secrets}, nil
}

func (p *mcpAcquisitionPersistence) SaveMCPConfiguration(ctx context.Context, name, transport, command string, args []string, env map[string]string) (string, error) {
	input := mcp.ServerInput{Name: name, DisplayName: name, Transport: transport, Command: command, Args: args, Enabled: true, Source: "acquisition"}
	if transport == "streamable_http" || transport == "sse" || transport == "remote" {
		input.Endpoint = command
		input.Command = ""
		if transport == "remote" {
			input.Transport = "streamable_http"
		}
	}
	if transport == "executable" {
		input.Transport = "stdio"
	}
	if len(env) > 0 {
		input.AuthType = "stdio_env"
	}
	identity, err := mcp.NormalizeServerIdentity(input)
	if err != nil {
		return "", err
	}
	if _, err := p.repository.FindServerByIdentity(ctx, identity); err == nil {
		return "", fmt.Errorf("MCP server is already installed; use its existing configuration instead of installing a duplicate")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	server, err := p.repository.CreateServer(ctx, input)
	if err != nil {
		return "", err
	}
	if len(env) > 0 {
		raw, err := json.Marshal(env)
		if err != nil {
			_ = p.repository.DeleteServer(ctx, server.ID)
			return "", err
		}
		reference, err := p.secrets.Put(ctx, "mcp/"+server.ID+"/stdio_env", raw)
		if err != nil {
			_ = p.repository.DeleteServer(ctx, server.ID)
			return "", err
		}
		if _, err := p.repository.PutCredentialReference(ctx, server.ID, "stdio_env", reference, "", nil); err != nil {
			_ = p.secrets.Delete(ctx, reference)
			_ = p.repository.DeleteServer(ctx, server.ID)
			return "", err
		}
	}
	return server.ID, nil
}

func (p *mcpAcquisitionPersistence) MarkMCPReady(ctx context.Context, id string) error {
	return p.repository.SetServerStatus(ctx, id, "ready", "", "", nil)
}

func (p *mcpAcquisitionPersistence) RemoveMCPConfiguration(ctx context.Context, id string) error {
	references, err := p.repository.CredentialReferences(ctx, id)
	if err != nil {
		return err
	}
	if err := p.repository.DeleteServer(ctx, id); err != nil {
		return err
	}
	for _, reference := range references {
		if err := p.secrets.Delete(ctx, reference); err != nil {
			return err
		}
	}
	return nil
}

type mcpExistingConnector struct {
	repository *mcp.Repository
	runtime    func() *MCPCompatibilityRuntime
}

func (p *mcpExistingConnector) EnableExistingMCP(ctx context.Context, id string) error {
	runtime := p.runtime()
	if runtime == nil || runtime.Connections == nil {
		return fmt.Errorf("canonical MCP runtime is unavailable")
	}
	server, err := p.repository.GetServer(ctx, id)
	if err != nil {
		return err
	}
	var args []string
	if err := json.Unmarshal([]byte(server.ArgsJSON), &args); err != nil {
		return err
	}
	input := mcp.ServerInput{Name: server.Name, DisplayName: server.DisplayName, Description: server.Description,
		Transport: server.Transport, Endpoint: server.Endpoint, Command: server.Command, Args: args, WorkDir: server.WorkDir,
		AuthType: server.AuthType, Source: server.Source, Enabled: true}
	if _, err := p.repository.UpdateServer(ctx, id, input); err != nil {
		return err
	}
	if err := runtime.Connections.Connect(ctx, id); err != nil {
		input.Enabled = server.Enabled == 1
		_, _ = p.repository.UpdateServer(context.WithoutCancel(ctx), id, input)
		return err
	}
	return nil
}

func (p *mcpExistingConnector) VerifyExistingMCP(ctx context.Context, id string) (bool, error) {
	runtime := p.runtime()
	if runtime == nil || runtime.Connections == nil {
		return false, fmt.Errorf("canonical MCP runtime is unavailable")
	}
	server, err := p.repository.GetServer(ctx, id)
	if err != nil {
		return false, err
	}
	_, connected := runtime.Connections.Connection(id)
	return server.Enabled == 1 && server.Status == "ready" && connected, nil
}

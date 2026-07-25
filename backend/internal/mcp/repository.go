// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.

package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/mcp/auth"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

type ServerInput struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Transport   string   `json:"transport"`
	Endpoint    string   `json:"endpoint"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	WorkDir     string   `json:"workDir"`
	AuthType    string   `json:"authType"`
	Source      string   `json:"source"`
	Enabled     bool     `json:"enabled"`
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func NormalizeServerIdentity(input ServerInput) (string, error) {
	transportName := strings.ToLower(strings.TrimSpace(input.Transport))
	switch transportName {
	case "streamable_http":
		parsed, err := url.Parse(strings.TrimSpace(input.Endpoint))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "", fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: endpoint")
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: endpoint scheme")
		}
		if parsed.User != nil || strings.ContainsAny(parsed.Host, "\r\n") {
			return "", fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: endpoint credentials")
		}
		parsed.Scheme = strings.ToLower(parsed.Scheme)
		parsed.Host = strings.ToLower(parsed.Host)
		parsed.Fragment = ""
		return transportName + ":" + parsed.String(), nil
	case "stdio":
		command := strings.TrimSpace(input.Command)
		if command == "" {
			return "", fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: command")
		}
		if filepath.IsAbs(command) {
			command = filepath.Clean(command)
		}
		encoded, _ := json.Marshal(input.Args)
		return transportName + ":" + strings.ToLower(command) + ":" + string(encoded), nil
	default:
		return "", fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: transport")
	}
}

func ServerConfigurationHash(input ServerInput) (string, error) {
	identity, err := NormalizeServerIdentity(input)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(struct {
		Identity string   `json:"identity"`
		WorkDir  string   `json:"workDir"`
		AuthType string   `json:"authType"`
		Args     []string `json:"args"`
	}{identity, filepath.Clean(strings.TrimSpace(input.WorkDir)), strings.ToLower(strings.TrimSpace(input.AuthType)), input.Args})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func (r *Repository) CreateServer(ctx context.Context, input ServerInput) (Server, error) {
	if r == nil || r.db == nil {
		return Server{}, fmt.Errorf("MCP repository unavailable")
	}
	identity, err := NormalizeServerIdentity(input)
	if err != nil {
		return Server{}, err
	}
	hash, err := ServerConfigurationHash(input)
	if err != nil {
		return Server{}, err
	}
	args, _ := json.Marshal(input.Args)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	authType := strings.ToLower(strings.TrimSpace(input.AuthType))
	if authType == "" {
		authType = "none"
	}
	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = "manual"
	}
	record := Server{ID: uuid.NewString(), Name: strings.TrimSpace(input.Name), DisplayName: strings.TrimSpace(input.DisplayName), Description: strings.TrimSpace(input.Description), Transport: strings.ToLower(strings.TrimSpace(input.Transport)), Endpoint: strings.TrimSpace(input.Endpoint), Command: strings.TrimSpace(input.Command), ArgsJSON: string(args), WorkDir: strings.TrimSpace(input.WorkDir), ProtocolVersion: "", ServerInfoJSON: "{}", CapabilitiesJSON: "{}", Instructions: "", AuthType: authType, Enabled: boolInt(input.Enabled), Status: "disconnected", Source: source, NormalizedIdentity: identity, ConfigurationHash: hash, CreatedAt: now, UpdatedAt: now}
	if record.Name == "" {
		return Server{}, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: name")
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return Server{}, err
	}
	return record, nil
}

func (r *Repository) UpdateServer(ctx context.Context, id string, input ServerInput) (Server, error) {
	var existing Server
	if err := r.db.WithContext(ctx).First(&existing, "id = ?", id).Error; err != nil {
		return Server{}, err
	}
	identity, err := NormalizeServerIdentity(input)
	if err != nil {
		return Server{}, err
	}
	hash, err := ServerConfigurationHash(input)
	if err != nil {
		return Server{}, err
	}
	args, _ := json.Marshal(input.Args)
	updates := map[string]any{"name": strings.TrimSpace(input.Name), "display_name": strings.TrimSpace(input.DisplayName), "description": strings.TrimSpace(input.Description), "transport": strings.ToLower(strings.TrimSpace(input.Transport)), "endpoint": strings.TrimSpace(input.Endpoint), "command": strings.TrimSpace(input.Command), "args_json": string(args), "work_dir": strings.TrimSpace(input.WorkDir), "auth_type": strings.ToLower(strings.TrimSpace(input.AuthType)), "enabled": boolInt(input.Enabled), "source": strings.TrimSpace(input.Source), "normalized_identity": identity, "configuration_hash": hash, "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}
	if updates["auth_type"] == "" {
		updates["auth_type"] = "none"
	}
	if updates["source"] == "" {
		updates["source"] = existing.Source
	}
	if err := r.db.WithContext(ctx).Model(&Server{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return Server{}, err
	}
	return r.GetServer(ctx, id)
}

func (r *Repository) GetServer(ctx context.Context, id string) (Server, error) {
	var record Server
	err := r.db.WithContext(ctx).First(&record, "id = ?", strings.TrimSpace(id)).Error
	return record, err
}

func (r *Repository) ListServers(ctx context.Context) ([]Server, error) {
	var records []Server
	err := r.db.WithContext(ctx).Order("created_at ASC").Find(&records).Error
	return records, err
}

func (r *Repository) ListEnabledServers(ctx context.Context) ([]Server, error) {
	var records []Server
	err := r.db.WithContext(ctx).Where("enabled = 1").Order("created_at ASC").Find(&records).Error
	return records, err
}

func (r *Repository) DeleteServer(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var dependencies int64
		if err := tx.Model(&DependencyLink{}).Where("server_id = ?", id).Count(&dependencies).Error; err != nil {
			return err
		}
		if dependencies > 0 {
			return fmt.Errorf("MCP_SERVER_IN_USE")
		}
		models := []any{&ServerScopeBinding{}, &ServerCredential{}, &ServerCapability{}, &ToolDefinition{}, &ResourceDefinition{}, &ResourceTemplate{}, &PromptDefinition{}, &OAuthSession{}, &Task{}}
		for _, model := range models {
			if err := tx.Where("server_id = ?", id).Delete(model).Error; err != nil {
				return err
			}
		}
		return tx.Where("id = ?", id).Delete(&Server{}).Error
	})
}

func (r *Repository) SetServerStatus(ctx context.Context, id, status, code, message string, initialized *struct{ ProtocolVersion, ServerInfoJSON, CapabilitiesJSON, Instructions string }) error {
	updates := map[string]any{"status": status, "last_error_code": code, "last_error_message": message, "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}
	if status == "ready" {
		updates["last_connected_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if initialized != nil {
		updates["protocol_version"] = initialized.ProtocolVersion
		updates["server_info_json"] = initialized.ServerInfoJSON
		updates["capabilities_json"] = initialized.CapabilitiesJSON
		updates["instructions"] = initialized.Instructions
	}
	return r.db.WithContext(ctx).Model(&Server{}).Where("id = ?", id).Updates(updates).Error
}

func (r *Repository) SetScopeEnabled(ctx context.Context, serverID, scopeType, scopeID string, enabled bool) error {
	scopeType = strings.ToLower(strings.TrimSpace(scopeType))
	if scopeType != "global" && scopeType != "character" {
		return fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: scope")
	}
	if scopeType == "global" {
		scopeID = ""
	} else if strings.TrimSpace(scopeID) == "" {
		return fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: scope id")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := ServerScopeBinding{ID: uuid.NewString(), ServerID: serverID, ScopeType: scopeType, ScopeID: scopeID, Enabled: boolInt(enabled), CreatedAt: now, UpdatedAt: now}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "server_id"}, {Name: "scope_type"}, {Name: "scope_id"}}, DoUpdates: clause.Assignments(map[string]any{"enabled": boolInt(enabled), "updated_at": now})}).Create(&record).Error; err != nil {
			return err
		}
		if scopeType == "global" {
			return tx.Model(&Server{}).Where("id = ?", serverID).Updates(map[string]any{"enabled": boolInt(enabled), "updated_at": now}).Error
		}
		return nil
	})
}

func (r *Repository) ResolveScopeEnabled(ctx context.Context, serverID, characterID string) (bool, string, error) {
	if strings.TrimSpace(characterID) != "" {
		var binding ServerScopeBinding
		err := r.db.WithContext(ctx).Where("server_id = ? AND scope_type = 'character' AND scope_id = ?", serverID, characterID).First(&binding).Error
		if err == nil {
			return binding.Enabled == 1, "character", nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return false, "", err
		}
	}
	var binding ServerScopeBinding
	err := r.db.WithContext(ctx).Where("server_id = ? AND scope_type = 'global' AND scope_id = ''", serverID).First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return binding.Enabled == 1, "global", nil
}

func (r *Repository) ListServerCapabilities(ctx context.Context, serverID string) ([]ServerCapability, error) {
	var records []ServerCapability
	err := r.db.WithContext(ctx).Where("server_id = ?", serverID).Order("capability ASC").Find(&records).Error
	return records, err
}

func (r *Repository) GetServerCapability(ctx context.Context, serverID, capability string) (ServerCapability, error) {
	var record ServerCapability
	err := r.db.WithContext(ctx).Where("server_id = ? AND capability = ?", serverID, normalizeCapability(capability)).First(&record).Error
	return record, err
}

func (r *Repository) ServerCapabilityEnabled(ctx context.Context, serverID, capability string) (bool, json.RawMessage, error) {
	record, err := r.GetServerCapability(ctx, serverID, capability)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, json.RawMessage(`{}`), nil
	}
	if err != nil {
		return false, nil, err
	}
	configuration := json.RawMessage(record.Configuration)
	if !json.Valid(configuration) {
		configuration = json.RawMessage(`{}`)
	}
	return record.Enabled == 1, configuration, nil
}

func (r *Repository) SetServerCapability(ctx context.Context, serverID, capability string, enabled bool, configuration json.RawMessage) (ServerCapability, error) {
	capability = normalizeCapability(capability)
	if capability == "" {
		return ServerCapability{}, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: capability")
	}
	if len(configuration) == 0 {
		configuration = json.RawMessage(`{}`)
	}
	if !json.Valid(configuration) {
		return ServerCapability{}, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: capability configuration")
	}
	var object map[string]any
	if json.Unmarshal(configuration, &object) != nil {
		return ServerCapability{}, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: capability configuration")
	}
	if _, err := r.GetServer(ctx, serverID); err != nil {
		return ServerCapability{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := ServerCapability{ID: uuid.NewString(), ServerID: serverID, Capability: capability, Configuration: string(configuration), Enabled: boolInt(enabled), CreatedAt: now, UpdatedAt: now}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "server_id"}, {Name: "capability"}}, DoUpdates: clause.Assignments(map[string]any{"configuration_json": string(configuration), "enabled": boolInt(enabled), "updated_at": now})}).Create(&record).Error
	if err != nil {
		return ServerCapability{}, err
	}
	return r.GetServerCapability(ctx, serverID, capability)
}

func normalizeCapability(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "roots", "sampling", "elicitation", "tasks", "private_network":
		return value
	}
	return ""
}

func (r *Repository) PutCredentialReference(ctx context.Context, serverID, credentialType, reference, expiresAt string, scopes []string) (ServerCredential, error) {
	if strings.TrimSpace(reference) == "" {
		return ServerCredential{}, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: secret reference")
	}
	sort.Strings(scopes)
	scopeJSON, _ := json.Marshal(scopes)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := ServerCredential{ID: uuid.NewString(), ServerID: serverID, CredentialType: credentialType, SecretReference: reference, ExpiresAt: expiresAt, ScopesJSON: string(scopeJSON), CreatedAt: now, UpdatedAt: now}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("server_id = ? AND credential_type = ?", serverID, credentialType).Delete(&ServerCredential{}).Error; err != nil {
			return err
		}
		return tx.Create(&record).Error
	})
	return record, err
}

func (r *Repository) CredentialReference(ctx context.Context, serverID, credentialType string) (ServerCredential, error) {
	var record ServerCredential
	err := r.db.WithContext(ctx).Where("server_id = ? AND credential_type = ?", serverID, credentialType).Order("updated_at DESC").First(&record).Error
	return record, err
}

func (r *Repository) DeleteCredentialReferences(ctx context.Context, serverID string) ([]string, error) {
	var records []ServerCredential
	if err := r.db.WithContext(ctx).Where("server_id = ?", serverID).Find(&records).Error; err != nil {
		return nil, err
	}
	references := make([]string, 0, len(records))
	for _, record := range records {
		references = append(references, record.SecretReference)
	}
	return references, r.db.WithContext(ctx).Where("server_id = ?", serverID).Delete(&ServerCredential{}).Error
}

func (r *Repository) CredentialReferences(ctx context.Context, serverID string) ([]string, error) {
	var records []ServerCredential
	if err := r.db.WithContext(ctx).Where("server_id = ?", serverID).Find(&records).Error; err != nil {
		return nil, err
	}
	references := make([]string, 0, len(records))
	for _, record := range records {
		references = append(references, record.SecretReference)
	}
	return references, nil
}

func (r *Repository) CreateOAuthSession(ctx context.Context, session auth.PendingSession) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	scopes, _ := json.Marshal(session.RequestedScopes)
	record := OAuthSession{ID: session.ID, ServerID: session.ServerID, StateHash: session.StateHash, CodeVerifierReference: session.CodeVerifierReference, RedirectURI: session.RedirectURI, RequestedScopesJSON: string(scopes), Status: session.Status, ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339Nano), CreatedAt: now, UpdatedAt: now}
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *Repository) ConsumeOAuthSession(ctx context.Context, id, stateHash string) (auth.PendingSession, error) {
	var result auth.PendingSession
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record OAuthSession
		query := tx.Where("state_hash = ? AND status = 'pending'", stateHash)
		if strings.TrimSpace(id) != "" {
			query = query.Where("id = ?", id)
		}
		if err := query.First(&record).Error; err != nil {
			return err
		}
		expiresAt, err := time.Parse(time.RFC3339Nano, record.ExpiresAt)
		if err != nil || expiresAt.Before(time.Now().UTC()) {
			return fmt.Errorf("MCP_OAUTH_STATE_INVALID")
		}
		resultUpdate := tx.Model(&OAuthSession{}).Where("id = ? AND status = 'pending'", record.ID).Updates(map[string]any{"status": "consumed", "updated_at": time.Now().UTC().Format(time.RFC3339Nano)})
		if resultUpdate.Error != nil {
			return resultUpdate.Error
		}
		if resultUpdate.RowsAffected != 1 {
			return fmt.Errorf("MCP_OAUTH_STATE_INVALID")
		}
		var scopes []string
		_ = json.Unmarshal([]byte(record.RequestedScopesJSON), &scopes)
		result = auth.PendingSession{ID: record.ID, ServerID: record.ServerID, StateHash: record.StateHash, CodeVerifierReference: record.CodeVerifierReference, RedirectURI: record.RedirectURI, RequestedScopes: scopes, Status: record.Status, ExpiresAt: expiresAt}
		return nil
	})
	return result, err
}

func (r *Repository) SaveOAuthTokenReference(ctx context.Context, serverID, reference string, expiresAt time.Time, scopes []string) error {
	expires := ""
	if !expiresAt.IsZero() {
		expires = expiresAt.UTC().Format(time.RFC3339Nano)
	}
	_, err := r.PutCredentialReference(ctx, serverID, "oauth_token", reference, expires, scopes)
	return err
}

func (r *Repository) OAuthTokenReference(ctx context.Context, serverID string) (string, error) {
	record, err := r.CredentialReference(ctx, serverID, "oauth_token")
	return record.SecretReference, err
}

func (r *Repository) DeleteOAuthTokenReference(ctx context.Context, serverID string) error {
	return r.db.WithContext(ctx).Where("server_id = ? AND credential_type = 'oauth_token'", serverID).Delete(&ServerCredential{}).Error
}

func (r *Repository) DeleteOAuthState(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&OAuthSession{}).Error
}

func (r *Repository) SyncTools(ctx context.Context, serverID string, definitions []ToolDefinition) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing []ToolDefinition
		if err := tx.Where("server_id = ?", serverID).Find(&existing).Error; err != nil {
			return err
		}
		byName := map[string]ToolDefinition{}
		for _, item := range existing {
			byName[item.RemoteName] = item
		}
		if err := tx.Model(&ToolDefinition{}).Where("server_id = ?", serverID).Updates(map[string]any{"enabled": 0, "updated_at": now}).Error; err != nil {
			return err
		}
		for index := range definitions {
			definition := &definitions[index]
			if previous, ok := byName[definition.RemoteName]; ok && previous.Hash == definition.Hash {
				definition.Enabled = previous.Enabled
			}
			definition.ServerID = serverID
			definition.UpdatedAt = now
			if definition.ID == "" {
				definition.ID = uuid.NewString()
			}
			if definition.CreatedAt == "" {
				definition.CreatedAt = now
			}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "server_id"}, {Name: "remote_name"}}, DoUpdates: clause.AssignmentColumns([]string{"skill_id", "title", "description", "input_schema_json", "output_schema_json", "annotations_json", "execution_json", "capability_hints_json", "risk_level", "enabled", "hash", "updated_at"})}).Create(definition).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) SyncResources(ctx context.Context, serverID string, resources []ResourceDefinition, templates []ResourceTemplate) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ResourceDefinition{}).Where("server_id = ?", serverID).Updates(map[string]any{"enabled": 0, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&ResourceTemplate{}).Where("server_id = ?", serverID).Updates(map[string]any{"enabled": 0, "updated_at": now}).Error; err != nil {
			return err
		}
		for index := range resources {
			item := &resources[index]
			item.ServerID = serverID
			item.UpdatedAt = now
			if item.ID == "" {
				item.ID = uuid.NewString()
			}
			if item.CreatedAt == "" {
				item.CreatedAt = now
			}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "server_id"}, {Name: "uri"}}, DoUpdates: clause.AssignmentColumns([]string{"name", "title", "description", "mime_type", "enabled", "hash", "updated_at"})}).Create(item).Error; err != nil {
				return err
			}
		}
		for index := range templates {
			item := &templates[index]
			item.ServerID = serverID
			item.UpdatedAt = now
			if item.ID == "" {
				item.ID = uuid.NewString()
			}
			if item.CreatedAt == "" {
				item.CreatedAt = now
			}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "server_id"}, {Name: "uri_template"}}, DoUpdates: clause.AssignmentColumns([]string{"name", "title", "description", "mime_type", "enabled", "hash", "updated_at"})}).Create(item).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) SyncPrompts(ctx context.Context, serverID string, prompts []PromptDefinition) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&PromptDefinition{}).Where("server_id = ?", serverID).Updates(map[string]any{"enabled": 0, "updated_at": now}).Error; err != nil {
			return err
		}
		for index := range prompts {
			item := &prompts[index]
			item.ServerID = serverID
			item.UpdatedAt = now
			if item.ID == "" {
				item.ID = uuid.NewString()
			}
			if item.CreatedAt == "" {
				item.CreatedAt = now
			}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "server_id"}, {Name: "remote_name"}}, DoUpdates: clause.AssignmentColumns([]string{"title", "description", "arguments_json", "enabled", "hash", "updated_at"})}).Create(item).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) ListTools(ctx context.Context, serverID string, enabledOnly bool) ([]ToolDefinition, error) {
	var records []ToolDefinition
	query := r.db.WithContext(ctx).Where("server_id = ?", serverID)
	if enabledOnly {
		query = query.Where("enabled = 1")
	}
	err := query.Order("remote_name ASC").Find(&records).Error
	return records, err
}

func (r *Repository) GetToolBySkillID(ctx context.Context, skillID string) (ToolDefinition, error) {
	var record ToolDefinition
	err := r.db.WithContext(ctx).Where("skill_id = ?", skillID).First(&record).Error
	return record, err
}

func (r *Repository) SetToolEnabled(ctx context.Context, id string, enabled bool) error {
	return r.db.WithContext(ctx).Model(&ToolDefinition{}).Where("id = ?", id).Updates(map[string]any{"enabled": boolInt(enabled), "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}).Error
}

func (r *Repository) GetTool(ctx context.Context, id string) (ToolDefinition, error) {
	var record ToolDefinition
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	return record, err
}

func (r *Repository) ListAuditLogs(ctx context.Context, serverID string, limit int) ([]AuditLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var records []AuditLog
	query := r.db.WithContext(ctx)
	if strings.TrimSpace(serverID) != "" {
		query = query.Where("server_id = ?", serverID)
	}
	err := query.Order("created_at DESC").Limit(limit).Find(&records).Error
	return records, err
}

func (r *Repository) AddAuditLog(ctx context.Context, record AuditLog) error {
	if record.ID == "" {
		record.ID = uuid.NewString()
	}
	if record.CreatedAt == "" {
		record.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if record.SummaryJSON == "" {
		record.SummaryJSON = "{}"
	}
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *Repository) ListOperations(ctx context.Context, limit int) ([]Operation, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var records []Operation
	err := r.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&records).Error
	return records, err
}

func (r *Repository) ListAgentSkillOperations(ctx context.Context, agentSkillID, status string) ([]Operation, error) {
	var records []Operation
	query := r.db.WithContext(ctx).Where("agent_skill_id = ?", agentSkillID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Order("created_at DESC").Find(&records).Error
	return records, err
}

func (r *Repository) GetOperation(ctx context.Context, id string) (Operation, error) {
	var record Operation
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	return record, err
}

func (r *Repository) UpsertTask(ctx context.Context, task Task) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if task.ID == "" {
		task.ID = uuid.NewString()
	}
	if task.CreatedAt == "" {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	if task.LastUpdatedAt == "" {
		task.LastUpdatedAt = now
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "server_id"}, {Name: "remote_task_id"}}, DoUpdates: clause.AssignmentColumns([]string{"character_id", "run_id", "status", "status_message", "result_json", "expires_at", "last_updated_at", "updated_at"})}).Create(&task).Error
}

func (r *Repository) GetTask(ctx context.Context, serverID, remoteTaskID string) (Task, error) {
	var record Task
	err := r.db.WithContext(ctx).Where("server_id = ? AND remote_task_id = ?", serverID, remoteTaskID).First(&record).Error
	return record, err
}

func (r *Repository) FindServerByIdentity(ctx context.Context, identity string) (Server, error) {
	var record Server
	err := r.db.WithContext(ctx).Where("normalized_identity = ?", identity).First(&record).Error
	return record, err
}

func (r *Repository) CreateOperation(ctx context.Context, operationType, agentSkillID, scopeType, scopeID string, plan any) (Operation, error) {
	raw, err := json.Marshal(plan)
	if err != nil {
		return Operation{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := Operation{ID: uuid.NewString(), Type: operationType, Status: "pending", AgentSkillID: agentSkillID, ScopeType: scopeType, ScopeID: scopeID, PlanJSON: string(raw), ResultJSON: "{}", CreatedAt: now, UpdatedAt: now}
	err = r.db.WithContext(ctx).Create(&record).Error
	return record, err
}

func (r *Repository) UpdateOperation(ctx context.Context, id, status string, result any, code, message string) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&Operation{}).Where("id = ?", id).Updates(map[string]any{"status": status, "result_json": string(raw), "error_code": code, "error_message": message, "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}).Error
}

func (r *Repository) UpsertDependencyLink(ctx context.Context, link DependencyLink) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if link.ID == "" {
		link.ID = uuid.NewString()
	}
	if link.CreatedAt == "" {
		link.CreatedAt = now
	}
	link.UpdatedAt = now
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "agent_skill_extension_id"}, {Name: "server_id"}, {Name: "dependency_name"}}, DoUpdates: clause.AssignmentColumns([]string{"required", "install_status", "binding_status", "updated_at"})}).Create(&link).Error
}

func (r *Repository) ListDependencyLinks(ctx context.Context, agentSkillID string) ([]DependencyLink, error) {
	var records []DependencyLink
	err := r.db.WithContext(ctx).Where("agent_skill_extension_id = ?", agentSkillID).Order("created_at ASC").Find(&records).Error
	return records, err
}

func (r *Repository) ListDependencyLinksByServer(ctx context.Context, serverID string) ([]DependencyLink, error) {
	var records []DependencyLink
	err := r.db.WithContext(ctx).Where("server_id = ?", serverID).Order("created_at ASC").Find(&records).Error
	return records, err
}

func (r *Repository) RemoveDependencyLinks(ctx context.Context, agentSkillID string) ([]string, error) {
	var records []DependencyLink
	if err := r.db.WithContext(ctx).Where("agent_skill_extension_id = ?", agentSkillID).Find(&records).Error; err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ServerID)
	}
	return ids, r.db.WithContext(ctx).Where("agent_skill_extension_id = ?", agentSkillID).Delete(&DependencyLink{}).Error
}

func (r *Repository) DeleteDependencyLink(ctx context.Context, agentSkillID, serverID, dependencyName string) error {
	return r.db.WithContext(ctx).Where("agent_skill_extension_id = ? AND server_id = ? AND dependency_name = ?", agentSkillID, serverID, dependencyName).Delete(&DependencyLink{}).Error
}

func (r *Repository) ServerDependencyReferenceCount(ctx context.Context, serverID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&DependencyLink{}).Where("server_id = ?", serverID).Count(&count).Error
	return count, err
}

func (r *Repository) OAuthSessionServerID(ctx context.Context, id string) (string, error) {
	var record OAuthSession
	if err := r.db.WithContext(ctx).Select("server_id").Where("id = ?", id).First(&record).Error; err != nil {
		return "", err
	}
	return record.ServerID, nil
}

func (r *Repository) FindOAuthSessionByStateHash(ctx context.Context, stateHash string) (OAuthSession, error) {
	var record OAuthSession
	err := r.db.WithContext(ctx).Where("state_hash = ? AND status = 'pending'", stateHash).First(&record).Error
	return record, err
}

func (r *Repository) ListTasks(ctx context.Context, serverID string, limit int) ([]Task, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var records []Task
	query := r.db.WithContext(ctx)
	if serverID != "" {
		query = query.Where("server_id = ?", serverID)
	}
	err := query.Order("last_updated_at DESC").Limit(limit).Find(&records).Error
	return records, err
}

func (r *Repository) DeleteExpiredTasks(ctx context.Context, serverID string, now time.Time) error {
	query := r.db.WithContext(ctx).Where("expires_at <> '' AND expires_at < ?", now.UTC().Format(time.RFC3339Nano))
	if serverID != "" {
		query = query.Where("server_id = ?", serverID)
	}
	return query.Delete(&Task{}).Error
}

func (r *Repository) ListResources(ctx context.Context, serverID string, enabledOnly bool) ([]ResourceDefinition, []ResourceTemplate, error) {
	var resources []ResourceDefinition
	var templates []ResourceTemplate
	resourceQuery := r.db.WithContext(ctx).Where("server_id = ?", serverID)
	templateQuery := r.db.WithContext(ctx).Where("server_id = ?", serverID)
	if enabledOnly {
		resourceQuery = resourceQuery.Where("enabled = 1")
		templateQuery = templateQuery.Where("enabled = 1")
	}
	if err := resourceQuery.Order("uri ASC").Find(&resources).Error; err != nil {
		return nil, nil, err
	}
	if err := templateQuery.Order("uri_template ASC").Find(&templates).Error; err != nil {
		return nil, nil, err
	}
	return resources, templates, nil
}

func (r *Repository) ListPrompts(ctx context.Context, serverID string, enabledOnly bool) ([]PromptDefinition, error) {
	var records []PromptDefinition
	query := r.db.WithContext(ctx).Where("server_id = ?", serverID)
	if enabledOnly {
		query = query.Where("enabled = 1")
	}
	err := query.Order("remote_name ASC").Find(&records).Error
	return records, err
}

func (r *Repository) GetPromptByName(ctx context.Context, serverID, name string) (PromptDefinition, error) {
	var record PromptDefinition
	err := r.db.WithContext(ctx).Where("server_id = ? AND remote_name = ? AND enabled = 1", serverID, name).First(&record).Error
	return record, err
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

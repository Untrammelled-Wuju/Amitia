// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.

package dependency

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/mcp"
	"github.com/u-ai/backend/internal/mcp/discovery"
	"github.com/u-ai/backend/internal/mcp/manager"
	"github.com/u-ai/backend/internal/mcp/skill"
	"gorm.io/gorm"
)

type Service struct {
	repository  *mcp.Repository
	connections *manager.Manager
	discovery   *discovery.Service
	skills      *skill.Runtime
}

type PreviewRequest struct {
	AgentSkillExtensionID string                              `json:"agentSkillExtensionId"`
	CharacterID           string                              `json:"characterId"`
	Dependencies          []extension.AgentSkillMCPDependency `json:"dependencies"`
}
type Plan struct {
	AgentSkillExtensionID string     `json:"agentSkillExtensionId"`
	CharacterID           string     `json:"characterId"`
	Items                 []PlanItem `json:"items"`
	RequiredMissing       bool       `json:"requiredMissing"`
	RiskLevel             string     `json:"riskLevel"`
}
type PlanItem struct {
	Dependency             extension.AgentSkillMCPDependency `json:"dependency"`
	NormalizedIdentity     string                            `json:"normalizedIdentity,omitempty"`
	ServerID               string                            `json:"serverId,omitempty"`
	Installed              bool                              `json:"installed"`
	Connected              bool                              `json:"connected"`
	AuthorizationRequired  bool                              `json:"authorizationRequired"`
	StartsLocalProcess     bool                              `json:"startsLocalProcess"`
	CommandAvailable       bool                              `json:"commandAvailable"`
	CanAutoConfigure       bool                              `json:"canAutoConfigure"`
	NeedsUserConfiguration bool                              `json:"needsUserConfiguration"`
	RiskLevel              string                            `json:"riskLevel"`
}
type InstallRequest struct {
	Plan            Plan `json:"plan"`
	InstallOptional bool `json:"installOptional"`
	ConfirmHTTP     bool `json:"confirmHttp"`
	ConfirmStdio    bool `json:"confirmStdio"`
	EnableServers   bool `json:"enableServers"`
}
type InstallResult struct {
	OperationID            string               `json:"operationId"`
	Status                 string               `json:"status"`
	Links                  []mcp.DependencyLink `json:"links"`
	AuthorizationServerIDs []string             `json:"authorizationServerIds"`
	Missing                []string             `json:"missing"`
}

func New(repository *mcp.Repository, connections *manager.Manager, discoveryService *discovery.Service, skills *skill.Runtime) *Service {
	return &Service{repository: repository, connections: connections, discovery: discoveryService, skills: skills}
}

func (s *Service) Preview(ctx context.Context, request PreviewRequest) (Plan, error) {
	plan := Plan{AgentSkillExtensionID: request.AgentSkillExtensionID, CharacterID: request.CharacterID, Items: []PlanItem{}, RiskLevel: "low"}
	for _, dependency := range request.Dependencies {
		input := serverInput(dependency)
		identity := ""
		if input.Transport != "" {
			identity, _ = mcp.NormalizeServerIdentity(input)
		}
		item := PlanItem{Dependency: dependency, NormalizedIdentity: identity, AuthorizationRequired: dependency.AuthType == "oauth", StartsLocalProcess: dependency.Transport == "stdio", CommandAvailable: dependency.Transport != "stdio", CanAutoConfigure: dependency.Transport == "streamable_http" && dependency.AutoConfigure && dependency.URL != "", NeedsUserConfiguration: dependency.Transport == "" || dependency.Transport == "streamable_http" && dependency.URL == "", RiskLevel: dependencyRisk(dependency)}
		if dependency.Transport == "stdio" {
			_, err := exec.LookPath(dependency.Command)
			item.CommandAvailable = err == nil
			item.CanAutoConfigure = false
			item.NeedsUserConfiguration = !item.CommandAvailable
		}
		if identity != "" {
			if server, err := s.repository.FindServerByIdentity(ctx, identity); err == nil {
				item.ServerID = server.ID
				item.Installed = true
				item.Connected = server.Status == "ready"
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return Plan{}, err
			}
		}
		if dependency.Required && !item.Installed && (!item.CanAutoConfigure || item.NeedsUserConfiguration) {
			plan.RequiredMissing = true
		}
		if item.RiskLevel == "high" {
			plan.RiskLevel = "high"
		} else if item.RiskLevel == "medium" && plan.RiskLevel == "low" {
			plan.RiskLevel = "medium"
		}
		plan.Items = append(plan.Items, item)
	}
	return plan, nil
}

func (s *Service) Install(ctx context.Context, request InstallRequest) (InstallResult, error) {
	if strings.TrimSpace(request.Plan.AgentSkillExtensionID) == "" {
		return InstallResult{}, fmt.Errorf("MCP_DEPENDENCY_PLAN_INVALID: agent skill")
	}
	dependencies := make([]extension.AgentSkillMCPDependency, 0, len(request.Plan.Items))
	for _, item := range request.Plan.Items {
		dependencies = append(dependencies, item.Dependency)
	}
	verified, err := s.Preview(ctx, PreviewRequest{AgentSkillExtensionID: request.Plan.AgentSkillExtensionID, CharacterID: request.Plan.CharacterID, Dependencies: dependencies})
	if err != nil {
		return InstallResult{}, err
	}
	request.Plan = verified
	operation, err := s.repository.CreateOperation(ctx, "agent_skill_mcp_install", request.Plan.AgentSkillExtensionID, scopeType(request.Plan.CharacterID), request.Plan.CharacterID, request.Plan)
	if err != nil {
		return InstallResult{}, err
	}
	result := InstallResult{OperationID: operation.ID, Status: "running", Links: []mcp.DependencyLink{}, AuthorizationServerIDs: []string{}, Missing: []string{}}
	created := []string{}
	linked := []mcp.DependencyLink{}
	fail := func(code string, cause error) (InstallResult, error) {
		result.Status = "failed"
		_ = s.repository.UpdateOperation(context.Background(), operation.ID, "failed", result, code, safeMessage(cause))
		for _, link := range linked {
			_ = s.repository.DeleteDependencyLink(context.Background(), link.AgentSkillExtensionID, link.ServerID, link.DependencyName)
		}
		for _, id := range created {
			count, _ := s.repository.ServerDependencyReferenceCount(context.Background(), id)
			if count == 0 {
				_ = s.repository.DeleteServer(context.Background(), id)
			}
		}
		return result, cause
	}
	for _, item := range request.Plan.Items {
		dependency := item.Dependency
		if !dependency.Required && !request.InstallOptional {
			result.Missing = append(result.Missing, dependency.ID)
			continue
		}
		serverID := item.ServerID
		if serverID == "" {
			if dependency.Transport == "stdio" {
				if !request.ConfirmStdio || !item.CommandAvailable {
					result.Missing = append(result.Missing, dependency.ID)
					if dependency.Required {
						return fail("MCP_DEPENDENCY_REQUIRED_MISSING", fmt.Errorf("required stdio dependency is not confirmed: %s", dependency.ID))
					}
					continue
				}
			} else if dependency.Transport == "streamable_http" {
				if !request.ConfirmHTTP || dependency.URL == "" {
					result.Missing = append(result.Missing, dependency.ID)
					if dependency.Required {
						return fail("MCP_DEPENDENCY_REQUIRED_MISSING", fmt.Errorf("required HTTP dependency is not confirmed: %s", dependency.ID))
					}
					continue
				}
			} else {
				result.Missing = append(result.Missing, dependency.ID)
				if dependency.Required {
					return fail("MCP_DEPENDENCY_REQUIRED_MISSING", fmt.Errorf("dependency needs configuration: %s", dependency.ID))
				}
				continue
			}
			server, createErr := s.repository.CreateServer(ctx, serverInput(dependency))
			if createErr != nil {
				return fail("MCP_DEPENDENCY_INSTALL_FAILED", createErr)
			}
			serverID = server.ID
			created = append(created, serverID)
		}
		bindingScope := dependency.DefaultScope
		bindingID := ""
		if bindingScope == "character" {
			bindingID = request.Plan.CharacterID
			if bindingID == "" {
				return fail("MCP_DEPENDENCY_SCOPE_INVALID", fmt.Errorf("character scope is required for %s", dependency.ID))
			}
		}
		if err := s.repository.SetScopeEnabled(ctx, serverID, bindingScope, bindingID, request.EnableServers); err != nil {
			return fail("MCP_DEPENDENCY_INSTALL_FAILED", err)
		}
		link := mcp.DependencyLink{AgentSkillExtensionID: request.Plan.AgentSkillExtensionID, ServerID: serverID, DependencyName: dependency.ID, Required: boolInt(dependency.Required), InstallStatus: "installed", BindingStatus: "bound"}
		if dependency.AuthType == "oauth" {
			link.InstallStatus = "authorization_required"
			result.AuthorizationServerIDs = append(result.AuthorizationServerIDs, serverID)
		}
		if err := s.repository.UpsertDependencyLink(ctx, link); err != nil {
			return fail("MCP_DEPENDENCY_INSTALL_FAILED", err)
		}
		linked = append(linked, link)
		links, _ := s.repository.ListDependencyLinks(ctx, request.Plan.AgentSkillExtensionID)
		for _, saved := range links {
			if saved.ServerID == serverID && saved.DependencyName == dependency.ID {
				result.Links = append(result.Links, saved)
				break
			}
		}
		if request.EnableServers && dependency.AuthType != "oauth" {
			if err := s.connections.Connect(ctx, serverID); err != nil {
				if dependency.Required {
					return fail("MCP_DEPENDENCY_CONNECT_FAILED", err)
				}
				continue
			}
			if err := s.discovery.Discover(ctx, serverID); err != nil {
				if dependency.Required {
					return fail("MCP_DEPENDENCY_DISCOVERY_FAILED", err)
				}
				continue
			}
			if err := s.applyToolAllowlist(ctx, serverID, dependency.ToolAllowlist); err != nil {
				return fail("MCP_DEPENDENCY_INSTALL_FAILED", err)
			}
			if err := s.skills.RegisterServer(ctx, serverID); err != nil {
				return fail("MCP_DEPENDENCY_INSTALL_FAILED", err)
			}
		}
	}
	if len(result.AuthorizationServerIDs) > 0 {
		result.Status = "awaiting_authorization"
		_ = s.repository.UpdateOperation(ctx, operation.ID, "awaiting_authorization", result, "", "")
		return result, nil
	}
	if len(result.Missing) > 0 {
		result.Status = "degraded"
	} else {
		result.Status = "completed"
	}
	if err := s.repository.UpdateOperation(ctx, operation.ID, result.Status, result, "", ""); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Service) applyToolAllowlist(ctx context.Context, serverID string, allowlist []string) error {
	if len(allowlist) == 0 {
		return nil
	}
	allowed := map[string]bool{}
	for _, name := range allowlist {
		allowed[strings.TrimSpace(name)] = true
	}
	tools, err := s.repository.ListTools(ctx, serverID, false)
	if err != nil {
		return err
	}
	for _, tool := range tools {
		if allowed[tool.RemoteName] && tool.Enabled != 1 {
			if err := s.repository.SetToolEnabled(ctx, tool.ID, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) Uninstall(ctx context.Context, agentSkillID string) ([]string, error) {
	return s.repository.RemoveDependencyLinks(ctx, agentSkillID)
}
func (s *Service) AuthorizationCompleted(ctx context.Context, serverID string) error {
	links, err := s.repository.ListDependencyLinksByServer(ctx, serverID)
	if err != nil {
		return err
	}
	for _, link := range links {
		link.InstallStatus = "installed"
		if err := s.repository.UpsertDependencyLink(ctx, link); err != nil {
			return err
		}
		all, err := s.repository.ListDependencyLinks(ctx, link.AgentSkillExtensionID)
		if err != nil {
			return err
		}
		awaiting := false
		for _, item := range all {
			if item.InstallStatus == "authorization_required" {
				awaiting = true
				break
			}
		}
		if awaiting {
			continue
		}
		operations, err := s.repository.ListAgentSkillOperations(ctx, link.AgentSkillExtensionID, "awaiting_authorization")
		if err != nil {
			return err
		}
		for _, operation := range operations {
			if err := s.repository.UpdateOperation(ctx, operation.ID, "completed", map[string]any{"status": "completed", "links": all}, "", ""); err != nil {
				return err
			}
		}
	}
	return nil
}
func serverInput(dependency extension.AgentSkillMCPDependency) mcp.ServerInput {
	return mcp.ServerInput{Name: dependency.ID, DisplayName: dependency.ID, Description: dependency.Description, Transport: dependency.Transport, Endpoint: dependency.URL, Command: dependency.Command, Args: append([]string{}, dependency.Args...), AuthType: dependency.AuthType, Source: "agent_skill", Enabled: false}
}
func dependencyRisk(dependency extension.AgentSkillMCPDependency) string {
	if dependency.Transport == "stdio" || dependency.AuthType == "custom_headers" || dependency.AuthType == "stdio_env" {
		return "high"
	}
	for _, tool := range dependency.ToolAllowlist {
		lower := strings.ToLower(tool)
		if strings.Contains(lower, "delete") || strings.Contains(lower, "send") || strings.Contains(lower, "create") || strings.Contains(lower, "update") {
			return "high"
		}
	}
	if dependency.AuthType == "oauth" || dependency.AuthType == "bearer_token" {
		return "medium"
	}
	return "low"
}
func scopeType(characterID string) string {
	if characterID != "" {
		return "character"
	}
	return "global"
}
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
func safeMessage(err error) string {
	if err == nil {
		return ""
	}
	value := err.Error()
	if len(value) > 512 {
		value = value[:512]
	}
	return value
}

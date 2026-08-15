package acquisition

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

// Source 接口: 用于从不同来源搜索能力候选
type Source interface {
	ID() string
	Kind() CandidateKind
	Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error)
}

// InstalledSource 从已安装但未启用的 Capability 中搜索候选
type InstalledSource struct {
	service  *capability.CapabilityService
	registry *capability.ProviderRegistry
}

// NewInstalledSource 创建 InstalledSource 实例
func NewInstalledSource(service *capability.CapabilityService, registry *capability.ProviderRegistry) *InstalledSource {
	return &InstalledSource{
		service:  service,
		registry: registry,
	}
}

func (s *InstalledSource) ID() string {
	return "installed"
}

func (s *InstalledSource) Kind() CandidateKind {
	return CandidateInstalledExtension
}

func (s *InstalledSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	if s.service == nil || s.registry == nil {
		return nil, nil
	}

	requestedID := capability.CapabilityID(request.CapabilityID)

	// 如果请求指定了 CapabilityID，仅返回匹配的 provider
	if requestedID != "" {
		return s.searchByCapability(requestedID)
	}

	// 未指定 CapabilityID 时，返回所有已注册但未 enabled 的 provider
	return s.searchAllInstalled()
}

func (s *InstalledSource) searchByCapability(requestedID capability.CapabilityID) ([]CapabilityCandidate, error) {
	if !s.service.HasCapability(requestedID) {
		return nil, nil
	}

	if s.service.HasExecutableProvider(requestedID) {
		return nil, nil
	}

	defs := s.registry.ListByCapability(requestedID)
	var candidates []CapabilityCandidate
	for _, def := range defs {
		if def == nil {
			continue
		}
		candidate := CapabilityCandidate{
			ID:           string(def.ID),
			Kind:         CandidateInstalledExtension,
			Name:         string(def.CapabilityID),
			Description:  "Installed but not enabled capability",
			Version:      "",
			Capabilities: []capability.CapabilityID{def.CapabilityID},
			Install: CandidateInstallDescriptor{
				Method: InstallEnableExisting,
			},
			Trust: CandidateTrust{
				Level: TrustVerified,
			},
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func (s *InstalledSource) searchAllInstalled() ([]CapabilityCandidate, error) {
	providers := s.service.ListProviders()
	if len(providers) == 0 {
		return nil, nil
	}

	var candidates []CapabilityCandidate
	for _, def := range providers {
		if def == nil {
			continue
		}
		if s.service.HasExecutableProvider(def.CapabilityID) {
			continue
		}
		if !s.service.HasCapability(def.CapabilityID) {
			continue
		}

		candidate := CapabilityCandidate{
			ID:           string(def.ID),
			Kind:         CandidateInstalledExtension,
			Name:         string(def.CapabilityID),
			Description:  "Installed but not enabled capability",
			Version:      "",
			Capabilities: []capability.CapabilityID{def.CapabilityID},
			Install: CandidateInstallDescriptor{
				Method: InstallEnableExisting,
			},
			Trust: CandidateTrust{
				Level: TrustVerified,
			},
		}
		candidates = append(candidates, candidate)
	}

	if len(candidates) == 0 {
		return nil, nil
	}
	return candidates, nil
}

// AgentSkillSource 从已有但未启用的 Skill 中搜索候选
type AgentSkillSource struct {
	service *capability.CapabilityService
}

// NewAgentSkillSource 创建 AgentSkillSource 实例
func NewAgentSkillSource(service *capability.CapabilityService) *AgentSkillSource {
	return &AgentSkillSource{
		service: service,
	}
}

func (s *AgentSkillSource) ID() string {
	return "agent_skill"
}

func (s *AgentSkillSource) Kind() CandidateKind {
	return CandidateAgentSkill
}

func (s *AgentSkillSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	// TODO: 实现搜索已有但未启用的 Skill
	return nil, nil
}

// MCPPackageSource 从 MCP 包中搜索候选
type MCPPackageSource struct {
	// TODO: 添加 MCP 包管理相关依赖
}

// NewMCPPackageSource 创建 MCPPackageSource 实例
func NewMCPPackageSource() *MCPPackageSource {
	return &MCPPackageSource{}
}

func (s *MCPPackageSource) ID() string {
	return "mcp_package"
}

func (s *MCPPackageSource) Kind() CandidateKind {
	return CandidateMCP
}

func (s *MCPPackageSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	// TODO: 实际实现等待后续阶段补充
	return nil, nil
}

// ExtensionCatalogSource 从 Extension 目录中搜索候选
type ExtensionCatalogSource struct {
	// TODO: 添加 Extension 目录相关依赖
}

// NewExtensionCatalogSource 创建 ExtensionCatalogSource 实例
func NewExtensionCatalogSource() *ExtensionCatalogSource {
	return &ExtensionCatalogSource{}
}

func (s *ExtensionCatalogSource) ID() string {
	return "extension_catalog"
}

func (s *ExtensionCatalogSource) Kind() CandidateKind {
	return CandidateExtensionPackage
}

func (s *ExtensionCatalogSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	// TODO: 实际实现等待后续阶段补充
	return nil, nil
}

// GeneratedSkillSource 搜索可生成的 Skill 候选
type GeneratedSkillSource struct {
	// TODO: 添加 Skill 生成相关依赖
}

// NewGeneratedSkillSource 创建 GeneratedSkillSource 实例
func NewGeneratedSkillSource() *GeneratedSkillSource {
	return &GeneratedSkillSource{}
}

func (s *GeneratedSkillSource) ID() string {
	return "generated_skill"
}

func (s *GeneratedSkillSource) Kind() CandidateKind {
	return CandidateGeneratedSkill
}

func (s *GeneratedSkillSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	// 仅当 request.AllowGeneratedSkill=true 时才有候选
	if !request.AllowGeneratedSkill {
		return nil, nil
	}
	// TODO: 实际实现等待后续阶段补充
	return nil, nil
}

// RemoteCatalogSource 从远程目录中搜索候选
type RemoteCatalogSource struct {
	// TODO: 添加远程目录访问相关依赖
}

// NewRemoteCatalogSource 创建 RemoteCatalogSource 实例
func NewRemoteCatalogSource() *RemoteCatalogSource {
	return &RemoteCatalogSource{}
}

func (s *RemoteCatalogSource) ID() string {
	return "remote_catalog"
}

func (s *RemoteCatalogSource) Kind() CandidateKind {
	return CandidateExtensionPackage
}

func (s *RemoteCatalogSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	// TODO: 实际实现等待后续阶段补充
	return nil, nil
}

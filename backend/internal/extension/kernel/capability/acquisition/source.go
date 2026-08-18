package acquisition

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/extension_center"
)

// SourcePort defines the external service interfaces that Sources need to query.
// These are minimal interfaces to avoid circular dependencies.
type (
	// AgentSkillRegistryPort queries AgentSkill definitions.
	AgentSkillRegistryPort interface {
		List(filter interface{}) []interface{}
		Get(extensionID string) (interface{}, bool)
	}

	// MCPLifecyclePort queries MCP installations.
	MCPLifecyclePort interface {
		ListInstallations() []interface{}
	}

	// ExtensionCenterPort queries Extension Center cards.
	ExtensionCenterPort interface {
		ListDiscoverable(ctx context.Context) ([]extension_center.ExtensionCard, error)
		ListInstalled(ctx context.Context) ([]extension_center.ExtensionCard, error)
	}
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
	catalog *agent_skill.AgentSkillCatalog
}

// NewAgentSkillSource 创建 AgentSkillSource 实例
func NewAgentSkillSource(catalog *agent_skill.AgentSkillCatalog) *AgentSkillSource {
	return &AgentSkillSource{
		catalog: catalog,
	}
}

func (s *AgentSkillSource) ID() string {
	return "agent_skill"
}

func (s *AgentSkillSource) Kind() CandidateKind {
	return CandidateAgentSkill
}

func (s *AgentSkillSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	if s.catalog == nil {
		return nil, fmt.Errorf("agent_skill source: catalog not configured")
	}

	items := s.catalog.List(agent_skill.CatalogFilter{})
	if len(items) == 0 {
		return nil, nil
	}

	var candidates []CapabilityCandidate
	for _, item := range items {
		candidate := s.buildCandidate(item, request)
		if candidate != nil {
			candidates = append(candidates, *candidate)
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}
	return candidates, nil
}

// buildCandidate 根据 AgentSkill 条目构造候选对象
func (s *AgentSkillSource) buildCandidate(item agent_skill.AgentSkillDefinition, request AcquisitionRequest) *CapabilityCandidate {
	if item.Enabled {
		return nil
	}

	capID := string(request.CapabilityID)
	matchesQuery := capID == "" ||
		containsString(item.Name, capID) ||
		containsString(item.Description, capID) ||
		s.skillProvidesCapability(item, capID)

	if !matchesQuery {
		return nil
	}

	providedCaps := s.extractProvidedCapabilities(item)
	if len(providedCaps) == 0 {
		return nil
	}

	return &CapabilityCandidate{
		ID:           item.ExtensionID,
		Kind:         CandidateAgentSkill,
		Name:         item.Name,
		Description:  item.Description,
		Capabilities: providedCaps,
		Install: CandidateInstallDescriptor{
			Method: InstallEnableExisting,
		},
		Trust: CandidateTrust{
			Level: TrustVerified,
		},
	}
}

// skillProvidesCapability 检查 Skill 是否声明提供指定能力
func (s *AgentSkillSource) skillProvidesCapability(item agent_skill.AgentSkillDefinition, capID string) bool {
	for _, cap := range s.extractProvidedCapabilities(item) {
		if string(cap) == capID {
			return true
		}
	}
	return false
}

// extractProvidedCapabilities 从 Skill 元数据中提取真实提供能力列表
func (s *AgentSkillSource) extractProvidedCapabilities(item agent_skill.AgentSkillDefinition) []capability.CapabilityID {
	if item.Metadata != nil {
		if raw, ok := item.Metadata["providesCapabilities"].([]any); ok {
			var caps []capability.CapabilityID
			for _, r := range raw {
				if s, ok := r.(string); ok && s != "" {
					caps = append(caps, capability.CapabilityID(s))
				}
			}
			if len(caps) > 0 {
				return caps
			}
		}
	}
	return nil
}

// MCPPackageSource 从 MCP 包中搜索候选
type MCPPackageSource struct {
	lifecycle interface {
		ListInstallations() []interface{}
	}
}

// NewMCPPackageSource 创建 MCPPackageSource 实例
func NewMCPPackageSource(lifecycle interface {
	ListInstallations() []interface{}
}) *MCPPackageSource {
	return &MCPPackageSource{
		lifecycle: lifecycle,
	}
}

func (s *MCPPackageSource) ID() string {
	return "mcp_package"
}

func (s *MCPPackageSource) Kind() CandidateKind {
	return CandidateMCP
}

func (s *MCPPackageSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	if s.lifecycle == nil {
		return nil, fmt.Errorf("mcp_package source: lifecycle not configured")
	}

	installations := s.lifecycle.ListInstallations()
	if len(installations) == 0 {
		return nil, nil
	}

	var candidates []CapabilityCandidate
	for _, inst := range installations {
		candidate := s.buildCandidate(inst, request)
		if candidate != nil {
			candidates = append(candidates, *candidate)
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}
	return candidates, nil
}

// buildCandidate 根据 MCP 安装条目构造候选对象
func (s *MCPPackageSource) buildCandidate(item interface{}, request AcquisitionRequest) *CapabilityCandidate {
	type mcpInstallInterface interface {
		GetBindingID() string
		GetServerName() string
		GetInstallState() string
		GetProvidedCapabilities() []string
	}

	if inst, ok := item.(mcpInstallInterface); ok {
		providedCaps := inst.GetProvidedCapabilities()
		if len(providedCaps) == 0 {
			return nil
		}

		capID := string(request.CapabilityID)
		matchesQuery := capID == "" || containsCapability(providedCaps, capID)
		if !matchesQuery {
			return nil
		}

		state := inst.GetInstallState()
		if state == "connected" {
			return nil
		}

		var caps []capability.CapabilityID
		for _, c := range providedCaps {
			caps = append(caps, capability.CapabilityID(c))
		}

		return &CapabilityCandidate{
			ID:           inst.GetBindingID(),
			Kind:         CandidateMCP,
			Name:         inst.GetServerName(),
			Description:  fmt.Sprintf("MCP server: %s (state: %s)", inst.GetServerName(), state),
			Capabilities: caps,
			Install: CandidateInstallDescriptor{
				Method: InstallMCP,
				MCP: &MCPInstallDescriptor{
					ServerName: inst.GetServerName(),
				},
			},
			Trust: CandidateTrust{
				Level: TrustVerified,
			},
		}
	}
	return nil
}

// ExtensionCatalogSource 从 Extension 目录中搜索候选
type ExtensionCatalogSource struct {
	center ExtensionCenterPort
}

// NewExtensionCatalogSource 创建 ExtensionCatalogSource 实例
func NewExtensionCatalogSource(center ExtensionCenterPort) *ExtensionCatalogSource {
	return &ExtensionCatalogSource{
		center: center,
	}
}

func (s *ExtensionCatalogSource) ID() string {
	return "extension_catalog"
}

func (s *ExtensionCatalogSource) Kind() CandidateKind {
	return CandidateExtensionPackage
}

func (s *ExtensionCatalogSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	if s.center == nil {
		return nil, fmt.Errorf("extension_catalog source: center not configured")
	}

	notInstalled, err := s.center.ListDiscoverable(ctx)
	if err != nil {
		return nil, fmt.Errorf("extension_catalog source: list discoverable: %w", err)
	}

	installed, err := s.center.ListInstalled(ctx)
	if err != nil {
		return nil, fmt.Errorf("extension_catalog source: list installed: %w", err)
	}

	allCards := append(notInstalled, installed...)
	if len(allCards) == 0 {
		return nil, nil
	}

	var candidates []CapabilityCandidate
	for _, card := range allCards {
		if !s.matchesRequest(card, request) {
			continue
		}

		candidate := s.buildCandidate(card)
		candidates = append(candidates, candidate)
	}

	if len(candidates) == 0 {
		return nil, nil
	}
	return candidates, nil
}

func (s *ExtensionCatalogSource) matchesRequest(card extension_center.ExtensionCard, request AcquisitionRequest) bool {
	capID := string(request.CapabilityID)
	if capID == "" {
		return true
	}

	if containsString(card.DisplayName, capID) {
		return true
	}
	if containsString(card.Description, capID) {
		return true
	}
	if containsString(card.ExtensionID, capID) {
		return true
	}
	for _, tag := range card.ContributionTags {
		if containsString(string(tag), capID) {
			return true
		}
	}
	return false
}

func (s *ExtensionCatalogSource) buildCandidate(card extension_center.ExtensionCard) CapabilityCandidate {
	method := InstallExtension
	if card.Status == extension_center.ExtensionStatusInstalledDisabled {
		method = InstallEnableExisting
	}

	providedCaps := s.extractProvidedCapabilities(card)

	install := CandidateInstallDescriptor{
		Method: method,
	}
	if card.PackageURI != "" {
		install.ExtensionPackage = &ExtensionInstallDescriptor{
			PackageURI: card.PackageURI,
			Hash:       card.Hash,
		}
	}

	return CapabilityCandidate{
		ID:           card.ExtensionID,
		Kind:         CandidateExtensionPackage,
		Name:         card.DisplayName,
		Description:  card.Description,
		Version:      card.Version,
		Capabilities: providedCaps,
		Install:      install,
		Trust: CandidateTrust{
			Level: TrustLevel(card.Trust),
		},
		Metadata: map[string]any{
			"extensionId":      card.ExtensionID,
			"status":           card.Status,
			"trust":            card.Trust,
			"contributionTags": card.ContributionTags,
			"source":           card.Source,
			"signatureStatus":  string(card.Trust),
		},
	}
}

func (s *ExtensionCatalogSource) extractProvidedCapabilities(card extension_center.ExtensionCard) []capability.CapabilityID {
	var caps []capability.CapabilityID
	for _, capID := range card.ProvidedCapabilities {
		if capID != "" {
			caps = append(caps, capability.CapabilityID(capID))
		}
	}
	return caps
}

// GeneratedSkillSource 搜索可生成的 Skill 候选
type GeneratedSkillSource struct {
	allowGenerated bool
}

// NewGeneratedSkillSource 创建 GeneratedSkillSource 实例
func NewGeneratedSkillSource(allowGenerated bool) *GeneratedSkillSource {
	return &GeneratedSkillSource{
		allowGenerated: allowGenerated,
	}
}

func (s *GeneratedSkillSource) ID() string {
	return "generated_skill"
}

func (s *GeneratedSkillSource) Kind() CandidateKind {
	return CandidateGeneratedSkill
}

func (s *GeneratedSkillSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	if !request.AllowGeneratedSkill || !s.allowGenerated {
		return nil, nil
	}

	capID := string(request.CapabilityID)
	if capID == "" {
		return nil, nil
	}

	promptStub := request.Description
	if promptStub == "" {
		promptStub = fmt.Sprintf("Generate a skill that provides %s capability", capID)
	}

	return []CapabilityCandidate{
		{
			ID:           "generated_" + string(capID),
			Kind:         CandidateGeneratedSkill,
			Name:         "Generated skill for: " + string(capID),
			Description:  fmt.Sprintf("AI-generated skill to provide capability %s. Requires validation before use.", capID),
			Capabilities: []capability.CapabilityID{request.CapabilityID},
			Install: CandidateInstallDescriptor{
				Method: InstallGeneratedSkill,
				GeneratedSkill: &GeneratedSkillDescriptor{
					PromptStub:  promptStub,
					Description: fmt.Sprintf("Provide %s capability", capID),
				},
			},
			Trust: CandidateTrust{
				Level: TrustUnverified,
			},
			Metadata: map[string]any{
				"generated":  true,
				"unverified": true,
			},
		},
	}, nil
}


// NewSkillSource 从远程 Skill 仓库搜索可安装的 Skill 候选
type NewSkillSource struct {
	remoteCatalog interface {
		ListSkills(ctx context.Context) ([]RemoteSkillEntry, error)
	}
}

// RemoteSkillEntry 表示远程仓库中的 Skill 条目
type RemoteSkillEntry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	SourceURI   string   `json:"sourceUri"`
	Hash        string   `json:"hash"`
	Provides    []string `json:"provides"`
}

// NewSkillSource 创建 NewSkillSource 实例
func NewNewSkillSource(remoteCatalog interface {
	ListSkills(ctx context.Context) ([]RemoteSkillEntry, error)
}) *NewSkillSource {
	return &NewSkillSource{remoteCatalog: remoteCatalog}
}

func (s *NewSkillSource) ID() string {
	return "new_skill"
}

func (s *NewSkillSource) Kind() CandidateKind {
	return CandidateAgentSkill
}

func (s *NewSkillSource) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	if s.remoteCatalog == nil {
		return nil, nil
	}

	entries, err := s.remoteCatalog.ListSkills(ctx)
	if err != nil {
		return nil, fmt.Errorf("remote skill catalog: %w", err)
	}

	capID := string(request.CapabilityID)
	var candidates []CapabilityCandidate
	for _, entry := range entries {
		matches := capID == ""
		if !matches {
			for _, provides := range entry.Provides {
				if provides == capID {
					matches = true
					break
				}
			}
		}
		if !matches {
			continue
		}

		var caps []capability.CapabilityID
		for _, p := range entry.Provides {
			caps = append(caps, capability.CapabilityID(p))
		}
		if len(caps) == 0 {
			caps = []capability.CapabilityID{request.CapabilityID}
		}

		candidates = append(candidates, CapabilityCandidate{
			ID:           "skill_" + entry.ID,
			Kind:         CandidateAgentSkill,
			Name:         entry.Name,
			Description:  entry.Description,
			Capabilities: caps,
			Install: CandidateInstallDescriptor{
				Method: InstallSkill,
				Skill: &SkillInstallDescriptor{
					SourceURI: entry.SourceURI,
					SkillName: entry.Name,
					Hash:      entry.Hash,
				},
			},
			Trust: CandidateTrust{
				Level: TrustUnverified,
			},
			Metadata: map[string]any{
				"sourceUri": entry.SourceURI,
				"remote":    true,
			},
		})
	}

	return candidates, nil
}

// containsString 检查 substr 是否包含在 s 中（大小写不敏感）
func containsString(s, substr string) bool {
	if substr == "" {
		return true
	}
	return len(s) >= len(substr) && containsSubstring(s, substr)
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if toLower(s[i+j]) != toLower(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func toLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

// containsCapability 检查 capID 是否在列表中
func containsCapability(caps []string, capID string) bool {
	for _, c := range caps {
		if c == capID {
			return true
		}
	}
	return false
}

// RemoteSkillCatalog HTTP 实现，从远程 Skill 仓库获取 Skill 列表
type RemoteSkillCatalog struct {
	client  *http.Client
	apiURL  string
}

// NewRemoteSkillCatalog 创建 RemoteSkillCatalog 实例
func NewRemoteSkillCatalog(apiURL string) *RemoteSkillCatalog {
	return &RemoteSkillCatalog{
		client: &http.Client{Timeout: 15 * time.Second},
		apiURL: apiURL,
	}
}

func (c *RemoteSkillCatalog) ListSkills(ctx context.Context) ([]RemoteSkillEntry, error) {
	if c.apiURL == "" {
		return nil, fmt.Errorf("remote skill catalog: apiURL is empty")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("remote skill catalog: create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remote skill catalog: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remote skill catalog: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("remote skill catalog: read body: %w", err)
	}

	var entries []RemoteSkillEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("remote skill catalog: parse response: %w", err)
	}

	return entries, nil
}

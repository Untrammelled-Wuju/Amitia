package agentplatform

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/secret"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CreateCredentialGrantInput struct {
	AgentID      string     `json:"agentId"`
	ConnectionID string     `json:"connectionId"`
	Provider     string     `json:"provider"`
	AccountID    string     `json:"accountId"`
	Scopes       []string   `json:"scopes"`
	SecretRef    string     `json:"secretRef"`
	SecretValue  string     `json:"secretValue"`
	ExpiresAt    *time.Time `json:"expiresAt"`
}

type CapabilityLeaseResult struct {
	Lease CapabilityLease `json:"lease"`
	Token string          `json:"token"`
}

type CreateNetworkPolicyInput struct {
	AgentID             string   `json:"agentId"`
	Name                string   `json:"name"`
	DefaultAction       string   `json:"defaultAction"`
	AllowDomains        []string `json:"allowDomains"`
	DenyDomains         []string `json:"denyDomains"`
	AllowedPorts        []int    `json:"allowedPorts"`
	UploadPolicy        string   `json:"uploadPolicy"`
	SensitiveDataPolicy string   `json:"sensitiveDataPolicy"`
}

type UpdateNetworkPolicyInput struct {
	Name                string   `json:"name"`
	DefaultAction       string   `json:"defaultAction"`
	AllowDomains        []string `json:"allowDomains"`
	DenyDomains         []string `json:"denyDomains"`
	AllowedPorts        []int    `json:"allowedPorts"`
	UploadPolicy        string   `json:"uploadPolicy"`
	SensitiveDataPolicy string   `json:"sensitiveDataPolicy"`
	ExpectedRevision    int64    `json:"expectedRevision"`
}

type EgressRequest struct {
	AgentID       string `json:"agentId"`
	URL           string `json:"url"`
	Method        string `json:"method"`
	PayloadHash   string `json:"payloadHash"`
	SensitiveData bool   `json:"sensitiveData"`
}

type AcquireResourceLeaseInput struct {
	AgentID      string `json:"agentId"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	TaskID       string `json:"taskId"`
	Mode         string `json:"mode"`
	Fingerprint  string `json:"fingerprint"`
	TTLSeconds   int    `json:"ttlSeconds"`
}

type CreateBudgetPolicyInput struct {
	ScopeType         string `json:"scopeType"`
	ScopeID           string `json:"scopeId"`
	Period            string `json:"period"`
	TokenLimit        int64  `json:"tokenLimit"`
	CostLimitMicroUSD int64  `json:"costLimitMicroUsd"`
	ToolCallLimit     int64  `json:"toolCallLimit"`
	ConcurrencyLimit  int    `json:"concurrencyLimit"`
	WarningPercent    int    `json:"warningPercent"`
	HardStop          bool   `json:"hardStop"`
}

type BudgetReservation struct {
	Policy  BudgetPolicy `json:"policy"`
	Allowed bool         `json:"allowed"`
	Warning bool         `json:"warning"`
	Reason  string       `json:"reason"`
}

type RecordBudgetUsageInput struct {
	RequestID    string `json:"requestId"`
	InputTokens  int64  `json:"inputTokens"`
	OutputTokens int64  `json:"outputTokens"`
	CostMicroUSD int64  `json:"costMicroUsd"`
	ToolCalls    int64  `json:"toolCalls"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
}

type AcquireBudgetLeaseInput struct {
	ScopeType  string `json:"scopeType"`
	ScopeID    string `json:"scopeId"`
	HolderID   string `json:"holderId"`
	RequestID  string `json:"requestId"`
	TTLSeconds int    `json:"ttlSeconds"`
}

func (s *Service) CreateCredentialGrant(ctx context.Context, spaceID string, input CreateCredentialGrantInput) (CredentialGrant, error) {
	provider := strings.TrimSpace(input.Provider)
	if provider == "" || strings.TrimSpace(input.ConnectionID) == "" || strings.TrimSpace(input.AgentID) == "" {
		return CredentialGrant{}, errors.New("Agent、连接和应用 Provider 不能为空")
	}
	scopesJSON, _, err := hashJSON(input.Scopes)
	if err != nil {
		return CredentialGrant{}, err
	}
	now := time.Now().UTC()
	secretRef := strings.TrimSpace(input.SecretRef)
	if strings.TrimSpace(input.SecretValue) != "" {
		if s.secretBroker == nil {
			return CredentialGrant{}, errors.New("加密凭据存储不可用")
		}
		ref, storeErr := s.secretBroker.Store(ctx, "agent-platform/"+input.AgentID+"/"+input.ConnectionID, []byte(input.SecretValue))
		if storeErr != nil {
			return CredentialGrant{}, fmt.Errorf("保存 Agent 凭据: %w", storeErr)
		}
		secretRef = string(ref)
	}
	grant := CredentialGrant{
		ID: newID("grant"), SpaceID: spaceID, AgentID: input.AgentID,
		ConnectionID: input.ConnectionID, Provider: provider, AccountID: strings.TrimSpace(input.AccountID),
		ScopeJSON: scopesJSON, SecretRef: secretRef, Status: "active",
		ExpiresAt: input.ExpiresAt, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&grant).Error; err != nil {
		return CredentialGrant{}, fmt.Errorf("创建 Agent 凭据授权: %w", err)
	}
	if err := s.recordActivity(ctx, ActivityEvent{SpaceID: spaceID, AgentID: input.AgentID, Kind: "credential", State: "granted", Title: "应用授权已添加", Detail: provider}); err != nil {
		return CredentialGrant{}, err
	}
	return grant, nil
}

func (s *Service) ListCredentialGrants(ctx context.Context, spaceID, agentID string) ([]CredentialGrant, error) {
	grants := make([]CredentialGrant, 0)
	if err := s.db.WithContext(ctx).Where("space_id = ? AND agent_id = ?", spaceID, agentID).Order("created_at DESC").Find(&grants).Error; err != nil {
		return nil, fmt.Errorf("读取 Agent 授权: %w", err)
	}
	return grants, nil
}

func (s *Service) ListCapabilityLeases(ctx context.Context, spaceID, agentID string, activeOnly bool) ([]CapabilityLease, error) {
	query := s.db.WithContext(ctx).Where("space_id = ?", spaceID)
	if strings.TrimSpace(agentID) != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if activeOnly {
		query = query.Where("status = ? AND expires_at > ?", "active", time.Now().UTC())
	}
	leases := make([]CapabilityLease, 0)
	if err := query.Order("created_at DESC").Find(&leases).Error; err != nil {
		return nil, fmt.Errorf("读取能力令牌: %w", err)
	}
	return leases, nil
}

func (s *Service) RevokeCredentialGrant(ctx context.Context, spaceID, grantID string) (CredentialGrant, error) {
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).Model(&CredentialGrant{}).Where("id = ? AND space_id = ? AND status = ?", grantID, spaceID, "active").Updates(map[string]any{"status": "revoked", "revoked_at": now, "updated_at": now})
	if result.Error != nil {
		return CredentialGrant{}, fmt.Errorf("撤销 Agent 授权: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return CredentialGrant{}, errors.New("Agent 授权不存在或已经撤销")
	}
	if err := s.db.WithContext(ctx).Model(&CapabilityLease{}).Where("grant_id = ? AND status = ?", grantID, "active").Updates(map[string]any{"status": "revoked", "revoked_at": now}).Error; err != nil {
		return CredentialGrant{}, fmt.Errorf("撤销关联能力令牌: %w", err)
	}
	var grant CredentialGrant
	if err := s.db.WithContext(ctx).Where("id = ?", grantID).First(&grant).Error; err != nil {
		return CredentialGrant{}, fmt.Errorf("读取撤销后的授权: %w", err)
	}
	if grant.SecretRef != "" && s.secretBroker != nil {
		if err := s.secretBroker.Delete(ctx, secret.SecretRef(grant.SecretRef)); err != nil {
			return CredentialGrant{}, fmt.Errorf("删除 Agent 凭据: %w", err)
		}
	}
	if err := s.recordActivity(ctx, ActivityEvent{SpaceID: spaceID, AgentID: grant.AgentID, Kind: "credential", State: "revoked", Title: "应用授权已撤销", Detail: grant.Provider}); err != nil {
		return CredentialGrant{}, err
	}
	return grant, nil
}

func (s *Service) CreateCapabilityLease(ctx context.Context, spaceID, grantID, capability, resource string, maxUses int, ttl time.Duration) (CapabilityLeaseResult, error) {
	var grant CredentialGrant
	if err := s.db.WithContext(ctx).Where("id = ? AND space_id = ? AND status = ?", grantID, spaceID, "active").First(&grant).Error; err != nil {
		return CapabilityLeaseResult{}, fmt.Errorf("读取 Agent 授权: %w", err)
	}
	if grant.ExpiresAt != nil && !grant.ExpiresAt.After(time.Now().UTC()) {
		return CapabilityLeaseResult{}, errors.New("Agent 授权已经过期")
	}
	var scopes []string
	if err := json.Unmarshal([]byte(grant.ScopeJSON), &scopes); err != nil {
		return CapabilityLeaseResult{}, fmt.Errorf("解析 Agent 授权范围: %w", err)
	}
	if !scopeAllows(scopes, capability) {
		return CapabilityLeaseResult{}, fmt.Errorf("Agent 授权不包含能力 %s", capability)
	}
	if maxUses <= 0 {
		maxUses = 1
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return CapabilityLeaseResult{}, fmt.Errorf("生成能力令牌: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	now := time.Now().UTC()
	lease := CapabilityLease{
		ID: newID("lease"), SpaceID: spaceID, AgentID: grant.AgentID, GrantID: grant.ID,
		TokenHash: hex.EncodeToString(sum[:]), Capability: capability, Resource: resource,
		MaxUses: maxUses, Status: "active", ExpiresAt: now.Add(ttl), CreatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&lease).Error; err != nil {
		return CapabilityLeaseResult{}, fmt.Errorf("创建能力令牌: %w", err)
	}
	return CapabilityLeaseResult{Lease: lease, Token: token}, nil
}

func (s *Service) ConsumeCapabilityLease(ctx context.Context, spaceID, token, capability, resource string) (CapabilityLease, error) {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	var lease CapabilityLease
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id = ? AND token_hash = ?", spaceID, hex.EncodeToString(sum[:])).First(&lease).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("能力令牌不存在")
			}
			return fmt.Errorf("读取能力令牌: %w", err)
		}
		if lease.Status != "active" || !lease.ExpiresAt.After(time.Now().UTC()) {
			return errors.New("能力令牌已经失效")
		}
		if lease.Capability != capability {
			return fmt.Errorf("能力令牌范围不匹配: %s", capability)
		}
		if resource != "" && lease.Resource != "" && lease.Resource != resource {
			return fmt.Errorf("能力令牌资源不匹配: %s", resource)
		}
		if lease.UsedCount >= lease.MaxUses {
			return errors.New("能力令牌已经用完")
		}
		lease.UsedCount++
		updates := map[string]any{"used_count": lease.UsedCount}
		if lease.UsedCount >= lease.MaxUses {
			updates["status"] = "consumed"
		}
		return tx.Model(&CapabilityLease{}).Where("id = ?", lease.ID).Updates(updates).Error
	})
	if err != nil {
		return CapabilityLease{}, err
	}
	return lease, nil
}

func (s *Service) CreateNetworkPolicy(ctx context.Context, spaceID string, input CreateNetworkPolicyInput) (NetworkPolicy, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return NetworkPolicy{}, errors.New("网络策略名称不能为空")
	}
	defaultAction := strings.TrimSpace(input.DefaultAction)
	if defaultAction == "" {
		defaultAction = "deny"
	}
	if defaultAction != "allow" && defaultAction != "deny" {
		return NetworkPolicy{}, errors.New("默认网络动作必须是 allow 或 deny")
	}
	allowJSON, _, err := hashJSON(normalizeDomains(input.AllowDomains))
	if err != nil {
		return NetworkPolicy{}, err
	}
	denyJSON, _, err := hashJSON(normalizeDomains(input.DenyDomains))
	if err != nil {
		return NetworkPolicy{}, err
	}
	ports := input.AllowedPorts
	if len(ports) == 0 {
		ports = []int{443}
	}
	portsJSON, _, err := hashJSON(ports)
	if err != nil {
		return NetworkPolicy{}, err
	}
	uploadPolicy := strings.TrimSpace(input.UploadPolicy)
	if uploadPolicy == "" {
		uploadPolicy = "approval"
	}
	sensitivePolicy := strings.TrimSpace(input.SensitiveDataPolicy)
	if sensitivePolicy == "" {
		sensitivePolicy = "block"
	}
	now := time.Now().UTC()
	policy := NetworkPolicy{
		ID: newID("networkpolicy"), SpaceID: spaceID, AgentID: input.AgentID, Name: name,
		DefaultAction: defaultAction, AllowDomains: allowJSON, DenyDomains: denyJSON,
		AllowedPorts: portsJSON, UploadPolicy: uploadPolicy, SensitiveDataPolicy: sensitivePolicy,
		Revision: 1, Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&policy).Error; err != nil {
		return NetworkPolicy{}, fmt.Errorf("创建网络策略: %w", err)
	}
	return policy, nil
}

func (s *Service) ListNetworkPolicies(ctx context.Context, spaceID, agentID string) ([]NetworkPolicy, error) {
	query := s.db.WithContext(ctx).Where("space_id = ?", spaceID)
	if strings.TrimSpace(agentID) != "" {
		query = query.Where("agent_id = ? OR agent_id = ''", agentID)
	}
	policies := make([]NetworkPolicy, 0)
	if err := query.Order("updated_at DESC").Find(&policies).Error; err != nil {
		return nil, fmt.Errorf("读取网络策略: %w", err)
	}
	return policies, nil
}

func (s *Service) UpdateNetworkPolicy(ctx context.Context, spaceID, policyID string, input UpdateNetworkPolicyInput) (NetworkPolicy, error) {
	var policy NetworkPolicy
	if err := s.db.WithContext(ctx).Where("id = ? AND space_id = ?", policyID, spaceID).First(&policy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NetworkPolicy{}, errors.New("网络策略不存在")
		}
		return NetworkPolicy{}, fmt.Errorf("读取网络策略: %w", err)
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = policy.Name
	}
	defaultAction := strings.TrimSpace(input.DefaultAction)
	if defaultAction == "" {
		defaultAction = policy.DefaultAction
	}
	if defaultAction != "allow" && defaultAction != "deny" {
		return NetworkPolicy{}, errors.New("默认网络动作必须是 allow 或 deny")
	}
	allowJSON, _, err := hashJSON(normalizeDomains(input.AllowDomains))
	if err != nil {
		return NetworkPolicy{}, err
	}
	denyJSON, _, err := hashJSON(normalizeDomains(input.DenyDomains))
	if err != nil {
		return NetworkPolicy{}, err
	}
	ports := input.AllowedPorts
	if len(ports) == 0 {
		ports = []int{443}
	}
	portsJSON, _, err := hashJSON(ports)
	if err != nil {
		return NetworkPolicy{}, err
	}
	uploadPolicy := strings.TrimSpace(input.UploadPolicy)
	if uploadPolicy == "" {
		uploadPolicy = policy.UploadPolicy
	}
	sensitivePolicy := strings.TrimSpace(input.SensitiveDataPolicy)
	if sensitivePolicy == "" {
		sensitivePolicy = policy.SensitiveDataPolicy
	}
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).Model(&NetworkPolicy{}).Where(
		"id = ? AND space_id = ? AND revision = ?", policy.ID, spaceID, input.ExpectedRevision,
	).Updates(map[string]any{
		"name": name, "default_action": defaultAction, "allow_domains": allowJSON,
		"deny_domains": denyJSON, "allowed_ports": portsJSON, "upload_policy": uploadPolicy,
		"sensitive_data_policy": sensitivePolicy, "revision": input.ExpectedRevision + 1,
		"updated_at": now,
	})
	if result.Error != nil {
		return NetworkPolicy{}, fmt.Errorf("更新网络策略: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return NetworkPolicy{}, errors.New("网络策略版本已变化，请刷新后重试")
	}
	if err := s.db.WithContext(ctx).Where("id = ? AND space_id = ?", policy.ID, spaceID).First(&policy).Error; err != nil {
		return NetworkPolicy{}, fmt.Errorf("读取更新后的网络策略: %w", err)
	}
	if err := s.recordActivity(ctx, ActivityEvent{
		SpaceID: spaceID, AgentID: policy.AgentID, Kind: "network.policy",
		State: "updated", Title: "Agent 网络策略已更新", Detail: policy.Name,
		PayloadJSON: mustJSON(policy),
	}); err != nil {
		return NetworkPolicy{}, err
	}
	return policy, nil
}

func (s *Service) EvaluateEgress(ctx context.Context, spaceID string, input EgressRequest) (EgressDecision, error) {
	parsed, err := url.Parse(strings.TrimSpace(input.URL))
	if err != nil {
		return EgressDecision{}, fmt.Errorf("解析网络地址: %w", err)
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return EgressDecision{}, errors.New("网络地址缺少主机名")
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
		decision := EgressDecision{ID: newID("egress"), SpaceID: spaceID, AgentID: input.AgentID, Host: host, Port: portForURL(parsed), Method: input.Method, PayloadHash: input.PayloadHash, Allowed: false, Reason: "默认禁止访问私有和本机网络"}
		decision.CreatedAt = time.Now().UTC()
		if err := s.db.WithContext(ctx).Create(&decision).Error; err != nil {
			return EgressDecision{}, fmt.Errorf("记录网络决策: %w", err)
		}
		return decision, nil
	}
	query := s.db.WithContext(ctx).Where("space_id = ? AND enabled = ?", spaceID, true)
	if input.AgentID != "" {
		query = query.Where("agent_id = ? OR agent_id = ''", input.AgentID)
	}
	var policies []NetworkPolicy
	if err := query.Order("agent_id DESC, updated_at DESC").Find(&policies).Error; err != nil {
		return EgressDecision{}, fmt.Errorf("读取网络策略: %w", err)
	}
	decision := EgressDecision{
		ID: newID("egress"), SpaceID: spaceID, AgentID: input.AgentID, Host: host,
		Port: portForURL(parsed), Method: strings.ToUpper(strings.TrimSpace(input.Method)),
		PayloadHash: input.PayloadHash, Allowed: false, Reason: "没有可用网络策略",
		CreatedAt: time.Now().UTC(),
	}
	if len(policies) > 0 {
		policy := policies[0]
		decision.PolicyID = policy.ID
		var allow []string
		var deny []string
		var ports []int
		if err := json.Unmarshal([]byte(policy.AllowDomains), &allow); err != nil {
			return EgressDecision{}, fmt.Errorf("解析允许域名: %w", err)
		}
		if err := json.Unmarshal([]byte(policy.DenyDomains), &deny); err != nil {
			return EgressDecision{}, fmt.Errorf("解析禁止域名: %w", err)
		}
		if err := json.Unmarshal([]byte(policy.AllowedPorts), &ports); err != nil {
			return EgressDecision{}, fmt.Errorf("解析允许端口: %w", err)
		}
		portAllowed := containsInt(ports, decision.Port)
		if domainMatchesAny(host, deny) {
			decision.Reason = "命中禁止域名"
		} else if !portAllowed {
			decision.Reason = "端口不在允许列表"
		} else if domainMatchesAny(host, allow) || policy.DefaultAction == "allow" {
			decision.Allowed = true
			decision.Reason = "命中 Agent 网络策略"
		} else {
			decision.Reason = "域名不在允许列表"
		}
		if input.SensitiveData && policy.SensitiveDataPolicy == "block" {
			decision.Allowed = false
			decision.Reason = "敏感数据禁止外传"
		} else if input.SensitiveData || policy.UploadPolicy == "approval" {
			decision.RequiresApproval = decision.Allowed && (input.SensitiveData || strings.ToUpper(decision.Method) != "GET")
		}
	}
	if err := s.db.WithContext(ctx).Create(&decision).Error; err != nil {
		return EgressDecision{}, fmt.Errorf("记录网络决策: %w", err)
	}
	return decision, nil
}

func (s *Service) ListEgressDecisions(ctx context.Context, spaceID, agentID string, limit int) ([]EgressDecision, error) {
	query := s.db.WithContext(ctx).Where("space_id = ?", spaceID)
	if strings.TrimSpace(agentID) != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	decisions := make([]EgressDecision, 0)
	if err := query.Order("created_at DESC").Limit(trimLimit(limit, 200, 1000)).Find(&decisions).Error; err != nil {
		return nil, fmt.Errorf("读取网络决策审计: %w", err)
	}
	return decisions, nil
}

func (s *Service) AcquireResourceLease(ctx context.Context, spaceID string, input AcquireResourceLeaseInput) (ResourceLease, error) {
	if strings.TrimSpace(input.ResourceType) == "" || strings.TrimSpace(input.ResourceID) == "" || strings.TrimSpace(input.AgentID) == "" {
		return ResourceLease{}, errors.New("资源类型、资源 ID 和 Agent ID 不能为空")
	}
	mode := strings.TrimSpace(input.Mode)
	if mode == "" {
		mode = "exclusive"
	}
	if mode != "exclusive" && mode != "shared" {
		return ResourceLease{}, errors.New("资源租约模式必须是 exclusive 或 shared")
	}
	if input.TTLSeconds <= 0 {
		input.TTLSeconds = 900
	}
	if input.TTLSeconds > 86400 {
		input.TTLSeconds = 86400
	}
	now := time.Now().UTC()
	lease := ResourceLease{
		ID: newID("resourcelease"), SpaceID: spaceID, ResourceType: input.ResourceType,
		ResourceID: input.ResourceID, HolderAgentID: input.AgentID, TaskID: input.TaskID,
		Mode: mode, Fingerprint: input.Fingerprint, Status: "active",
		ExpiresAt: now.Add(time.Duration(input.TTLSeconds) * time.Second), CreatedAt: now,
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ResourceLease{}).Where("space_id = ? AND expires_at <= ? AND status = ?", spaceID, now, "active").Updates(map[string]any{"status": "expired", "released_at": now}).Error; err != nil {
			return fmt.Errorf("清理过期资源租约: %w", err)
		}
		var conflicts []ResourceLease
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id = ? AND resource_type = ? AND resource_id = ? AND status = ? AND expires_at > ? AND holder_agent_id <> ?", spaceID, input.ResourceType, input.ResourceID, "active", now, input.AgentID).Find(&conflicts).Error; err != nil {
			return fmt.Errorf("检查资源冲突: %w", err)
		}
		for _, conflict := range conflicts {
			if mode == "exclusive" || conflict.Mode == "exclusive" {
				return fmt.Errorf("资源 %s/%s 已被 Agent %s 锁定", input.ResourceType, input.ResourceID, conflict.HolderAgentID)
			}
		}
		if err := tx.Create(&lease).Error; err != nil {
			return fmt.Errorf("创建资源租约: %w", err)
		}
		return nil
	})
	if err != nil {
		return ResourceLease{}, err
	}
	if err := s.recordActivity(ctx, ActivityEvent{SpaceID: spaceID, AgentID: input.AgentID, Kind: "resource.lease", State: "acquired", Title: "资源租约已获取", Detail: input.ResourceType + "/" + input.ResourceID}); err != nil {
		return ResourceLease{}, err
	}
	return lease, nil
}

func (s *Service) ReleaseResourceLease(ctx context.Context, spaceID, leaseID, agentID string) (ResourceLease, error) {
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).Model(&ResourceLease{}).Where("id = ? AND space_id = ? AND holder_agent_id = ? AND status = ?", leaseID, spaceID, agentID, "active").Updates(map[string]any{"status": "released", "released_at": now})
	if result.Error != nil {
		return ResourceLease{}, fmt.Errorf("释放资源租约: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ResourceLease{}, errors.New("资源租约不存在或不属于当前 Agent")
	}
	var lease ResourceLease
	if err := s.db.WithContext(ctx).Where("id = ?", leaseID).First(&lease).Error; err != nil {
		return ResourceLease{}, fmt.Errorf("读取资源租约: %w", err)
	}
	return lease, nil
}

func (s *Service) ListResourceLeases(ctx context.Context, spaceID, resourceType, resourceID string, activeOnly bool) ([]ResourceLease, error) {
	query := s.db.WithContext(ctx).Where("space_id = ?", spaceID)
	if strings.TrimSpace(resourceType) != "" {
		query = query.Where("resource_type = ?", resourceType)
	}
	if strings.TrimSpace(resourceID) != "" {
		query = query.Where("resource_id = ?", resourceID)
	}
	if activeOnly {
		query = query.Where("status = ? AND expires_at > ?", "active", time.Now().UTC())
	}
	leases := make([]ResourceLease, 0)
	if err := query.Order("created_at DESC").Find(&leases).Error; err != nil {
		return nil, fmt.Errorf("读取资源租约: %w", err)
	}
	return leases, nil
}

func (s *Service) CreateBudgetPolicy(ctx context.Context, spaceID string, input CreateBudgetPolicyInput) (BudgetPolicy, error) {
	scopeType := strings.TrimSpace(input.ScopeType)
	if scopeType != "agent" && scopeType != "team" && scopeType != "space" {
		return BudgetPolicy{}, errors.New("预算范围必须是 agent、team 或 space")
	}
	if strings.TrimSpace(input.ScopeID) == "" {
		return BudgetPolicy{}, errors.New("预算范围 ID 不能为空")
	}
	period := strings.TrimSpace(input.Period)
	if period == "" {
		period = "monthly"
	}
	if input.WarningPercent <= 0 || input.WarningPercent > 100 {
		input.WarningPercent = 80
	}
	if input.ConcurrencyLimit <= 0 {
		input.ConcurrencyLimit = 1
	}
	now := time.Now().UTC()
	policy := BudgetPolicy{
		ID: newID("budget"), SpaceID: spaceID, ScopeType: scopeType, ScopeID: input.ScopeID,
		Period: period, TokenLimit: input.TokenLimit, CostLimitMicroUSD: input.CostLimitMicroUSD,
		ToolCallLimit: input.ToolCallLimit, ConcurrencyLimit: input.ConcurrencyLimit,
		WarningPercent: input.WarningPercent, HardStop: input.HardStop, RevisedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&policy).Error; err != nil {
		return BudgetPolicy{}, fmt.Errorf("创建预算策略: %w", err)
	}
	return policy, nil
}

func (s *Service) ListBudgetPolicies(ctx context.Context, spaceID, scopeType, scopeID string) ([]BudgetPolicy, error) {
	query := s.db.WithContext(ctx).Where("space_id = ?", spaceID)
	if strings.TrimSpace(scopeType) != "" {
		query = query.Where("scope_type = ?", scopeType)
	}
	if strings.TrimSpace(scopeID) != "" {
		query = query.Where("scope_id = ?", scopeID)
	}
	policies := make([]BudgetPolicy, 0)
	if err := query.Order("revised_at DESC").Find(&policies).Error; err != nil {
		return nil, fmt.Errorf("读取预算策略: %w", err)
	}
	return policies, nil
}

func (s *Service) CheckBudget(ctx context.Context, spaceID, scopeType, scopeID string) (BudgetReservation, error) {
	policy, usage, err := s.budgetSnapshot(ctx, spaceID, scopeType, scopeID)
	if err != nil {
		return BudgetReservation{}, err
	}
	if policy == nil {
		return BudgetReservation{Allowed: true, Reason: "未配置预算限制"}, nil
	}
	result := BudgetReservation{Policy: *policy, Allowed: true}
	if policy.TokenLimit > 0 {
		ratio := float64(usage.InputTokens+usage.OutputTokens) / float64(policy.TokenLimit)
		result.Warning = ratio*100 >= float64(policy.WarningPercent)
		if ratio >= 1 && policy.HardStop {
			result.Allowed = false
			result.Reason = "Token 预算已经耗尽"
		}
	}
	if policy.CostLimitMicroUSD > 0 {
		ratio := float64(usage.CostMicroUSD) / float64(policy.CostLimitMicroUSD)
		result.Warning = result.Warning || ratio*100 >= float64(policy.WarningPercent)
		if ratio >= 1 && policy.HardStop {
			result.Allowed = false
			result.Reason = "费用预算已经耗尽"
		}
	}
	if policy.ToolCallLimit > 0 && usage.ToolCalls >= policy.ToolCallLimit && policy.HardStop {
		result.Allowed = false
		result.Reason = "工具调用预算已经耗尽"
	}
	if result.Allowed && result.Reason == "" {
		result.Reason = "预算检查通过"
	}
	return result, nil
}

func (s *Service) AcquireBudgetLease(ctx context.Context, spaceID string, input AcquireBudgetLeaseInput) (BudgetLease, BudgetReservation, error) {
	policy, _, err := s.budgetSnapshot(ctx, spaceID, input.ScopeType, input.ScopeID)
	if err != nil {
		return BudgetLease{}, BudgetReservation{}, err
	}
	reservation, err := s.CheckBudget(ctx, spaceID, input.ScopeType, input.ScopeID)
	if err != nil {
		return BudgetLease{}, BudgetReservation{}, err
	}
	if !reservation.Allowed || policy == nil {
		return BudgetLease{}, reservation, errors.New(reservation.Reason)
	}
	if input.TTLSeconds <= 0 {
		input.TTLSeconds = 900
	}
	if input.TTLSeconds > 86400 {
		input.TTLSeconds = 86400
	}
	now := time.Now().UTC()
	lease := BudgetLease{
		ID: newID("budgetlease"), SpaceID: spaceID, PolicyID: policy.ID,
		ScopeType: input.ScopeType, ScopeID: input.ScopeID, HolderID: input.HolderID,
		RequestID: input.RequestID, Status: "active",
		ExpiresAt: now.Add(time.Duration(input.TTLSeconds) * time.Second), CreatedAt: now,
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&BudgetLease{}).Where("space_id = ? AND policy_id = ? AND expires_at <= ? AND status = ?", spaceID, policy.ID, now, "active").Updates(map[string]any{"status": "expired", "released_at": now}).Error; err != nil {
			return fmt.Errorf("清理过期预算占用: %w", err)
		}
		var active int64
		if err := tx.Model(&BudgetLease{}).Where("space_id = ? AND policy_id = ? AND status = ? AND expires_at > ?", spaceID, policy.ID, "active", now).Count(&active).Error; err != nil {
			return fmt.Errorf("读取预算并发占用: %w", err)
		}
		if active >= int64(policy.ConcurrencyLimit) {
			return fmt.Errorf("预算并发上限已满: %d", policy.ConcurrencyLimit)
		}
		if err := tx.Create(&lease).Error; err != nil {
			return fmt.Errorf("创建预算占用: %w", err)
		}
		return nil
	})
	if err != nil {
		return BudgetLease{}, reservation, err
	}
	return lease, reservation, nil
}

func (s *Service) ReleaseBudgetLease(ctx context.Context, spaceID, leaseID, holderID string) (BudgetLease, error) {
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).Model(&BudgetLease{}).Where("id = ? AND space_id = ? AND holder_id = ? AND status = ?", leaseID, spaceID, holderID, "active").Updates(map[string]any{"status": "released", "released_at": now})
	if result.Error != nil {
		return BudgetLease{}, fmt.Errorf("释放预算占用: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return BudgetLease{}, errors.New("预算占用不存在或不属于当前调用方")
	}
	var lease BudgetLease
	if err := s.db.WithContext(ctx).Where("id = ?", leaseID).First(&lease).Error; err != nil {
		return BudgetLease{}, fmt.Errorf("读取预算占用: %w", err)
	}
	return lease, nil
}

func (s *Service) RecordBudgetUsage(ctx context.Context, spaceID, policyID string, input RecordBudgetUsageInput) (BudgetUsage, error) {
	var policy BudgetPolicy
	if err := s.db.WithContext(ctx).Where("id = ? AND space_id = ?", policyID, spaceID).First(&policy).Error; err != nil {
		return BudgetUsage{}, fmt.Errorf("读取预算策略: %w", err)
	}
	now := time.Now().UTC()
	usageID := newID("budgetusage")
	if strings.TrimSpace(input.RequestID) != "" {
		usageID = stableID("budgetusage", "tool:"+input.RequestID+":"+policyID)
	}
	usage := BudgetUsage{
		ID: usageID, SpaceID: spaceID, PolicyID: policyID,
		RequestID: input.RequestID, InputTokens: input.InputTokens, OutputTokens: input.OutputTokens,
		CostMicroUSD: input.CostMicroUSD, ToolCalls: input.ToolCalls,
		Provider: input.Provider, Model: input.Model, CreatedAt: now,
	}
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&usage).Error; err != nil {
		return BudgetUsage{}, fmt.Errorf("记录预算用量: %w", err)
	}
	return usage, nil
}

func (s *Service) budgetSnapshot(ctx context.Context, spaceID, scopeType, scopeID string) (*BudgetPolicy, BudgetUsage, error) {
	var policy BudgetPolicy
	err := s.db.WithContext(ctx).Where("space_id = ? AND scope_type = ? AND scope_id = ?", spaceID, scopeType, scopeID).Order("revised_at DESC").First(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, BudgetUsage{}, nil
	}
	if err != nil {
		return nil, BudgetUsage{}, fmt.Errorf("读取预算策略: %w", err)
	}
	var usage BudgetUsage
	row := map[string]any{}
	start := budgetPeriodStart(policy.Period, time.Now().UTC())
	if err := s.db.WithContext(ctx).Model(&BudgetUsage{}).Select("COALESCE(SUM(input_tokens),0) AS input_tokens, COALESCE(SUM(output_tokens),0) AS output_tokens, COALESCE(SUM(cost_micro_usd),0) AS cost_micro_usd, COALESCE(SUM(tool_calls),0) AS tool_calls").Where("policy_id = ? AND created_at >= ?", policy.ID, start).Scan(&row).Error; err != nil {
		return nil, BudgetUsage{}, fmt.Errorf("汇总预算用量: %w", err)
	}
	usage.InputTokens = numericValue(row["input_tokens"])
	usage.OutputTokens = numericValue(row["output_tokens"])
	usage.CostMicroUSD = numericValue(row["cost_micro_usd"])
	usage.ToolCalls = numericValue(row["tool_calls"])
	return &policy, usage, nil
}

func budgetPeriodStart(period string, now time.Time) time.Time {
	now = now.UTC()
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "daily":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	case "weekly":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		day := now.AddDate(0, 0, -(weekday - 1))
		return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
}

func scopeAllows(scopes []string, capability string) bool {
	for _, scope := range scopes {
		if scope == "*" || scope == capability {
			return true
		}
		if strings.HasSuffix(scope, "*") && strings.HasPrefix(capability, strings.TrimSuffix(scope, "*")) {
			return true
		}
	}
	return false
}

func normalizeDomains(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		value = strings.TrimPrefix(value, ".")
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func domainMatchesAny(host string, patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == host {
			return true
		}
		if strings.HasPrefix(pattern, "*.") && strings.HasSuffix(host, strings.TrimPrefix(pattern, "*")) {
			return true
		}
	}
	return false
}

func portForURL(value *url.URL) int {
	if value.Port() != "" {
		port, _ := strconv.Atoi(value.Port())
		return port
	}
	if value.Scheme == "http" {
		return 80
	}
	return 443
}

func containsInt(values []int, value int) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func numericValue(value any) int64 {
	switch current := value.(type) {
	case int64:
		return current
	case int:
		return int64(current)
	case float64:
		return int64(current)
	case []byte:
		parsed, _ := strconv.ParseInt(string(current), 10, 64)
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(current, 10, 64)
		return parsed
	default:
		return 0
	}
}

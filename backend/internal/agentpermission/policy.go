package agentpermission

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ModeAllow   = "allow"
	ModeObserve = "observe"
	ModeDeny    = "deny"
)

type SourcePolicy struct {
	ID                string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID           string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	ScopeType         string     `gorm:"column:scope_type;not null;index" json:"scopeType"`
	ChannelID         string     `gorm:"column:channel_id;not null;default:'';index" json:"channelId"`
	AccountID         string     `gorm:"column:account_id;not null;default:'';index" json:"accountId"`
	ConversationID    string     `gorm:"column:conversation_id;not null;default:'';index" json:"conversationId"`
	SenderID          string     `gorm:"column:sender_id;not null;default:'';index" json:"senderId"`
	ExplicitMode      string     `gorm:"column:explicit_mode;not null;default:'allow'" json:"explicitMode"`
	InferredMode      string     `gorm:"column:inferred_mode;not null;default:'observe'" json:"inferredMode"`
	TriggerMode       string     `gorm:"column:trigger_mode;not null;default:'direct_or_mention'" json:"triggerMode"`
	PermissionCeiling string     `gorm:"column:permission_ceiling;not null;default:'request_approval'" json:"permissionCeiling"`
	Enabled           bool       `gorm:"column:enabled;not null" json:"enabled"`
	ExpiresAt         *time.Time `gorm:"column:expires_at" json:"expiresAt,omitempty"`
	Revision          int64      `gorm:"column:revision;not null;default:1" json:"revision"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (SourcePolicy) TableName() string {
	return "agent_access_policies"
}

type DecisionRecord struct {
	ID             string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID        string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	RequestID      string    `gorm:"column:request_id;not null;default:'';index" json:"requestId"`
	ChannelID      string    `gorm:"column:channel_id;not null;default:'';index" json:"channelId"`
	AccountID      string    `gorm:"column:account_id;not null;default:''" json:"accountId"`
	ConversationID string    `gorm:"column:conversation_id;not null;default:'';index" json:"conversationId"`
	SenderID       string    `gorm:"column:sender_id;not null;default:'';index" json:"senderId"`
	Source         string    `gorm:"column:source;not null;default:'';index" json:"source"`
	TriggerMode    string    `gorm:"column:trigger_mode;not null;default:''" json:"triggerMode"`
	Strength       string    `gorm:"column:strength;not null;default:'';index" json:"strength"`
	Decision       string    `gorm:"column:decision;not null;default:'';index" json:"decision"`
	Allowed        bool      `gorm:"column:allowed;not null;default:false" json:"allowed"`
	Observed       bool      `gorm:"column:observed;not null;default:false" json:"observed"`
	PolicyID       string    `gorm:"column:policy_id;not null;default:''" json:"policyId"`
	PolicyRevision int64     `gorm:"column:policy_revision;not null;default:0" json:"policyRevision"`
	PermissionMode string    `gorm:"column:permission_mode;not null;default:''" json:"permissionMode"`
	Reason         string    `gorm:"column:reason;not null;default:''" json:"reason"`
	CreatedAt      time.Time `gorm:"column:created_at;not null;index" json:"createdAt"`
}

func (DecisionRecord) TableName() string {
	return "agent_access_decisions"
}

type AdmissionInput struct {
	SpaceID             string
	RequestID           string
	ChannelID           string
	AccountID           string
	ConversationID      string
	SenderID            string
	PeerKind            string
	Source              string
	TriggerMode         string
	Explicit            bool
	RequestedPermission string
}

type Decision struct {
	Allowed        bool           `json:"allowed"`
	Observed       bool           `json:"observed"`
	Decision       string         `json:"decision"`
	Reason         string         `json:"reason"`
	Strength       string         `json:"strength"`
	TriggerMode    string         `json:"triggerMode"`
	PermissionMode string         `json:"permissionMode"`
	Policy         *SourcePolicy  `json:"policy,omitempty"`
	Record         DecisionRecord `json:"record"`
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Ready() bool {
	return s != nil && s.db != nil
}

func normalizeMode(value, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ModeAllow, ModeObserve, ModeDeny:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return fallback
	}
}

func normalizeScopeType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "space", "channel", "account", "conversation", "sender":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "space"
	}
}

func policySpecificity(policy SourcePolicy) int {
	switch policy.ScopeType {
	case "sender":
		return 5
	case "conversation":
		return 4
	case "account":
		return 3
	case "channel":
		return 2
	default:
		return 1
	}
}

func policyMatches(policy SourcePolicy, input AdmissionInput, now time.Time) bool {
	if !policy.Enabled || policy.SpaceID != input.SpaceID {
		return false
	}
	if policy.ExpiresAt != nil && !policy.ExpiresAt.After(now) {
		return false
	}
	switch policy.ScopeType {
	case "channel":
		return policy.ChannelID == "" || policy.ChannelID == input.ChannelID
	case "account":
		return (policy.ChannelID == "" || policy.ChannelID == input.ChannelID) && (policy.AccountID == "" || policy.AccountID == input.AccountID)
	case "conversation":
		return (policy.ChannelID == "" || policy.ChannelID == input.ChannelID) && (policy.ConversationID == "" || policy.ConversationID == input.ConversationID)
	case "sender":
		return (policy.ChannelID == "" || policy.ChannelID == input.ChannelID) && (policy.ConversationID == "" || policy.ConversationID == input.ConversationID) && (policy.SenderID == "" || policy.SenderID == input.SenderID)
	default:
		return true
	}
}

func triggerAllowed(mode, trigger string, explicit bool) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "direct_or_mention":
		return explicit || trigger == "direct" || trigger == "mention"
	case "direct":
		return trigger == "direct"
	case "mention":
		return trigger == "mention"
	case "all":
		return true
	case "inferred_only":
		return !explicit
	default:
		return false
	}
}

func lowerPermission(current, ceiling string) string {
	current = Normalize(current)
	ceiling = Normalize(ceiling)
	if current == FullAccess && ceiling != FullAccess {
		return RequestApproval
	}
	return current
}

func (s *Service) Evaluate(ctx context.Context, input AdmissionInput) (Decision, error) {
	if s == nil || s.db == nil {
		return Decision{}, errors.New("来源访问策略服务不可用")
	}
	input.TriggerMode = strings.TrimSpace(input.TriggerMode)
	if input.TriggerMode == "" {
		input.TriggerMode = "direct"
	}
	strength := "inferred"
	if input.Explicit {
		strength = "explicit"
	}
	requestedPermission := Normalize(input.RequestedPermission)
	now := time.Now().UTC()
	var policies []SourcePolicy
	if err := s.db.WithContext(ctx).Where("space_id = ? AND enabled = 1", input.SpaceID).Find(&policies).Error; err != nil {
		return Decision{}, fmt.Errorf("读取来源访问策略: %w", err)
	}
	candidates := make([]SourcePolicy, 0, len(policies))
	for _, policy := range policies {
		if policyMatches(policy, input, now) && triggerAllowed(policy.TriggerMode, input.TriggerMode, input.Explicit) {
			candidates = append(candidates, policy)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := policySpecificity(candidates[i]), policySpecificity(candidates[j])
		if left == right {
			return candidates[i].UpdatedAt.After(candidates[j].UpdatedAt)
		}
		return left > right
	})
	decision := Decision{
		Allowed:        input.Explicit,
		Observed:       !input.Explicit,
		Decision:       ModeAllow,
		Reason:         "未配置更具体策略，明确请求放行，推断请求仅记录",
		Strength:       strength,
		TriggerMode:    input.TriggerMode,
		PermissionMode: requestedPermission,
	}
	var selected *SourcePolicy
	if len(candidates) > 0 {
		selected = &candidates[0]
		mode := selected.ExplicitMode
		if !input.Explicit {
			mode = selected.InferredMode
		}
		mode = normalizeMode(mode, ModeObserve)
		decision.Allowed = mode == ModeAllow
		decision.Observed = mode == ModeObserve
		decision.Decision = mode
		decision.PermissionMode = lowerPermission(requestedPermission, selected.PermissionCeiling)
		decision.Policy = selected
		decision.Reason = fmt.Sprintf("命中%s来源策略", selected.ScopeType)
		if decision.PermissionMode != requestedPermission {
			decision.Reason += "，权限已降至策略上限"
		}
	}
	if !decision.Allowed && !decision.Observed {
		decision.Reason = "来源策略拒绝本次请求"
	}
	policyID := ""
	var policyRevision int64
	if selected != nil {
		policyID, policyRevision = selected.ID, selected.Revision
	}
	record := DecisionRecord{
		ID:             uuid.NewString(),
		SpaceID:        input.SpaceID,
		RequestID:      input.RequestID,
		ChannelID:      input.ChannelID,
		AccountID:      input.AccountID,
		ConversationID: input.ConversationID,
		SenderID:       input.SenderID,
		Source:         input.Source,
		TriggerMode:    input.TriggerMode,
		Strength:       strength,
		Decision:       decision.Decision,
		Allowed:        decision.Allowed,
		Observed:       decision.Observed,
		PolicyID:       policyID,
		PolicyRevision: policyRevision,
		PermissionMode: decision.PermissionMode,
		Reason:         decision.Reason,
		CreatedAt:      now,
	}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return Decision{}, fmt.Errorf("记录来源访问决策: %w", err)
	}
	decision.Record = record
	return decision, nil
}

func (s *Service) List(ctx context.Context, spaceID string) ([]SourcePolicy, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("来源访问策略服务不可用")
	}
	var items []SourcePolicy
	err := s.db.WithContext(ctx).Where("space_id = ?", spaceID).Order("scope_type, channel_id, account_id, conversation_id, sender_id").Find(&items).Error
	return items, err
}

func (s *Service) Upsert(ctx context.Context, policy SourcePolicy) (SourcePolicy, error) {
	if s == nil || s.db == nil {
		return SourcePolicy{}, errors.New("来源访问策略服务不可用")
	}
	policy.SpaceID = strings.TrimSpace(policy.SpaceID)
	policy.ScopeType = normalizeScopeType(policy.ScopeType)
	policy.ChannelID = strings.TrimSpace(policy.ChannelID)
	policy.AccountID = strings.TrimSpace(policy.AccountID)
	policy.ConversationID = strings.TrimSpace(policy.ConversationID)
	policy.SenderID = strings.TrimSpace(policy.SenderID)
	policy.ExplicitMode = normalizeMode(policy.ExplicitMode, ModeAllow)
	policy.InferredMode = normalizeMode(policy.InferredMode, ModeObserve)
	policy.TriggerMode = strings.TrimSpace(policy.TriggerMode)
	if policy.TriggerMode == "" {
		policy.TriggerMode = "direct_or_mention"
	}
	policy.PermissionCeiling = Normalize(policy.PermissionCeiling)
	if policy.SpaceID == "" {
		return SourcePolicy{}, errors.New("来源访问策略缺少空间标识")
	}
	if policy.ScopeType != "space" && policy.ScopeType != "channel" && policy.ChannelID == "" {
		return SourcePolicy{}, errors.New("来源访问策略缺少渠道标识")
	}
	now := time.Now().UTC()
	if policy.ID == "" {
		policy.ID = uuid.NewString()
		policy.Revision = 1
		policy.CreatedAt = now
		policy.UpdatedAt = now
		if err := s.db.WithContext(ctx).Create(&policy).Error; err != nil {
			return SourcePolicy{}, fmt.Errorf("创建来源访问策略: %w", err)
		}
		return policy, nil
	}
	var existing SourcePolicy
	if err := s.db.WithContext(ctx).Where("id = ? AND space_id = ?", policy.ID, policy.SpaceID).First(&existing).Error; err != nil {
		return SourcePolicy{}, fmt.Errorf("读取来源访问策略: %w", err)
	}
	if policy.Revision != 0 && policy.Revision != existing.Revision {
		return SourcePolicy{}, fmt.Errorf("来源访问策略版本冲突: expected %d got %d", existing.Revision, policy.Revision)
	}
	policy.Revision = existing.Revision + 1
	policy.CreatedAt = existing.CreatedAt
	policy.UpdatedAt = now
	result := s.db.WithContext(ctx).Model(&SourcePolicy{}).Where("id = ? AND revision = ?", existing.ID, existing.Revision).Updates(map[string]any{
		"scope_type": policy.ScopeType, "channel_id": policy.ChannelID, "account_id": policy.AccountID,
		"conversation_id": policy.ConversationID, "sender_id": policy.SenderID,
		"explicit_mode": policy.ExplicitMode, "inferred_mode": policy.InferredMode,
		"trigger_mode": policy.TriggerMode, "permission_ceiling": policy.PermissionCeiling,
		"enabled": policy.Enabled, "expires_at": policy.ExpiresAt, "revision": policy.Revision, "updated_at": now,
	})
	if result.Error != nil {
		return SourcePolicy{}, fmt.Errorf("更新来源访问策略: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return SourcePolicy{}, errors.New("来源访问策略版本冲突")
	}
	return policy, nil
}

func (s *Service) Delete(ctx context.Context, spaceID, id string, revision int64) error {
	if s == nil || s.db == nil {
		return errors.New("来源访问策略服务不可用")
	}
	result := s.db.WithContext(ctx).Where("id = ? AND space_id = ? AND revision = ?", id, spaceID, revision).Delete(&SourcePolicy{})
	if result.Error != nil {
		return fmt.Errorf("删除来源访问策略: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return errors.New("来源访问策略版本冲突")
	}
	return nil
}

func (s *Service) EnsureCompatiblePolicies(ctx context.Context, spaceID string, channels []string) error {
	if s == nil || s.db == nil || len(channels) == 0 {
		return nil
	}
	for _, channelID := range channels {
		channelID = strings.TrimSpace(channelID)
		if channelID == "" {
			continue
		}
		policy := SourcePolicy{
			ID:                uuid.NewSHA1(uuid.NameSpaceOID, []byte(spaceID+"\x00agent-access\x00"+channelID)).String(),
			SpaceID:           spaceID,
			ScopeType:         "channel",
			ChannelID:         channelID,
			ExplicitMode:      ModeAllow,
			InferredMode:      ModeObserve,
			TriggerMode:       "direct_or_mention",
			PermissionCeiling: RequestApproval,
			Enabled:           true,
			Revision:          1,
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		}
		if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&policy).Error; err != nil {
			return fmt.Errorf("初始化来源访问策略 %s: %w", channelID, err)
		}
	}
	return nil
}

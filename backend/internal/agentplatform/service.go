package agentplatform

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/secret"
	"gorm.io/gorm"
)

type Service struct {
	db                   *gorm.DB
	dataDir              string
	runtime              *RuntimeManager
	marketplaceInstaller MarketplaceInstaller
	a2aExecutor          A2AExecutor
	secretBroker         *secret.Broker
	remoteMarketURL      string
	remoteMarketClient   *http.Client
	remoteMarketMu       sync.Mutex
	remoteMarketSynced   time.Time
	approvalBroker       *execution.ApprovalBroker
	workspaceRegistrar   RuntimeWorkspaceRegistrar
	evaluationExecutor   EvaluationExecutor
	a2aKeyMu             sync.Mutex
	a2aPrivateKey        ed25519.PrivateKey
}

type A2AExecutor interface {
	ExecuteA2ATask(ctx context.Context, spaceID, agentID string, input any) (any, error)
}

type ServiceConfig struct {
	DataDir         string
	RemoteMarketURL string
}

type platformActorContextKey struct{}

func WithPlatformActor(ctx context.Context, actorID, actorType string) context.Context {
	return context.WithValue(ctx, platformActorContextKey{}, map[string]string{
		"id": strings.TrimSpace(actorID), "type": strings.TrimSpace(actorType),
	})
}

func AllModels() []any {
	return platformModels()
}

func NewService(db *gorm.DB, config ServiceConfig) (*Service, error) {
	if db == nil {
		return nil, errors.New("agent platform: database is required")
	}
	service := &Service{
		db: db, dataDir: strings.TrimSpace(config.DataDir),
		remoteMarketURL:    strings.TrimSpace(config.RemoteMarketURL),
		remoteMarketClient: &http.Client{Timeout: 20 * time.Second},
	}
	service.runtime = NewRuntimeManager(db, service.dataDir)
	return service, nil
}

func (s *Service) Ready() bool {
	return s != nil && s.db != nil && s.runtime != nil
}

func (s *Service) Runtime() *RuntimeManager {
	if s == nil {
		return nil
	}
	return s.runtime
}

func (s *Service) SetSecretBroker(broker *secret.Broker) {
	if s != nil {
		s.secretBroker = broker
	}
}

func (s *Service) SetA2AExecutor(executor A2AExecutor) {
	if s != nil {
		s.a2aExecutor = executor
	}
}

func (s *Service) SetApprovalBroker(broker *execution.ApprovalBroker) {
	if s != nil {
		s.approvalBroker = broker
	}
}

func (s *Service) SetWorkspaceRegistrar(registrar RuntimeWorkspaceRegistrar) {
	if s == nil {
		return
	}
	s.workspaceRegistrar = registrar
	if s.runtime != nil {
		s.runtime.workspaceRegistrar = registrar
	}
}

func (s *Service) SetEvaluationExecutor(executor EvaluationExecutor) {
	if s != nil {
		s.evaluationExecutor = executor
	}
}

func (s *Service) EnsureLocalOwner(ctx context.Context, spaceID, displayName string) (PlatformUser, error) {
	if s == nil || s.db == nil {
		return PlatformUser{}, errors.New("agent platform: service unavailable")
	}
	spaceID = strings.TrimSpace(spaceID)
	if spaceID == "" {
		return PlatformUser{}, errors.New("agent platform: space id is required")
	}
	var user PlatformUser
	err := s.db.WithContext(ctx).Where("space_id = ? AND is_local_owner = ?", spaceID, true).First(&user).Error
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return PlatformUser{}, fmt.Errorf("读取本地所有者: %w", err)
	}
	now := time.Now().UTC()
	user = PlatformUser{
		ID:           newID("user"),
		SpaceID:      spaceID,
		DisplayName:  strings.TrimSpace(displayName),
		Status:       "active",
		IsLocalOwner: true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if user.DisplayName == "" {
		user.DisplayName = "本地用户"
	}
	permissions, marshalErr := json.Marshal([]string{"*"})
	if marshalErr != nil {
		return PlatformUser{}, fmt.Errorf("编码本地所有者权限: %w", marshalErr)
	}
	membership := PlatformMembership{
		ID:          newID("membership"),
		SpaceID:     spaceID,
		UserID:      user.ID,
		Role:        "owner",
		Status:      "active",
		Permissions: string(permissions),
		Revision:    1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return fmt.Errorf("创建本地所有者: %w", err)
		}
		if err := tx.Create(&membership).Error; err != nil {
			return fmt.Errorf("创建本地所有者权限: %w", err)
		}
		return nil
	})
	if err != nil {
		return PlatformUser{}, err
	}
	return user, nil
}

func (s *Service) Authorize(ctx context.Context, spaceID, userID, permission string) error {
	if s == nil || s.db == nil {
		return errors.New("agent platform: service unavailable")
	}
	spaceID = strings.TrimSpace(spaceID)
	userID = strings.TrimSpace(userID)
	if spaceID == "" || userID == "" {
		return errors.New("agent platform: authenticated user is required")
	}
	var user PlatformUser
	if err := s.db.WithContext(ctx).Where("id = ? AND space_id = ? AND status = ?", userID, spaceID, "active").First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("agent platform: user is not active in this space")
		}
		return fmt.Errorf("读取平台用户: %w", err)
	}
	if user.IsLocalOwner {
		return nil
	}
	var memberships []PlatformMembership
	if err := s.db.WithContext(ctx).Where("space_id = ? AND user_id = ? AND status = ?", spaceID, userID, "active").Find(&memberships).Error; err != nil {
		return fmt.Errorf("读取平台成员权限: %w", err)
	}
	for _, membership := range memberships {
		if membership.Role == "owner" || membership.Role == "admin" {
			return nil
		}
		var values []string
		if err := json.Unmarshal([]byte(membership.Permissions), &values); err != nil {
			return fmt.Errorf("解析平台成员权限: %w", err)
		}
		for _, value := range values {
			if value == "*" || value == permission {
				return nil
			}
		}
	}
	return fmt.Errorf("agent platform: permission %s is required", permission)
}

func (s *Service) recordActivity(ctx context.Context, event ActivityEvent) error {
	if s == nil || s.db == nil {
		return errors.New("agent platform: service unavailable")
	}
	if event.ID == "" {
		event.ID = newID("activity")
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if event.State == "" {
		event.State = "recorded"
	}
	if event.PayloadJSON == "" {
		event.PayloadJSON = "{}"
	}
	applyActivityActor(ctx, &event)
	if err := s.db.WithContext(ctx).Create(&event).Error; err != nil {
		return fmt.Errorf("记录 Agent 活动: %w", err)
	}
	return nil
}

func applyActivityActor(ctx context.Context, event *ActivityEvent) {
	if event == nil {
		return
	}
	if actor, ok := ctx.Value(platformActorContextKey{}).(map[string]string); ok {
		if event.ActorID == "" {
			event.ActorID = actor["id"]
		}
		if event.ActorType == "" {
			event.ActorType = actor["type"]
		}
	}
}

func newID(prefix string) string {
	return prefix + "_" + uuid.NewString()
}

func stableID(prefix, value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return prefix + "_" + hex.EncodeToString(sum[:12])
}

func hashJSON(value any) (string, string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", "", fmt.Errorf("编码 JSON: %w", err)
	}
	sum := sha256.Sum256(data)
	return string(data), hex.EncodeToString(sum[:]), nil
}

func normalizeJSON(raw string, fallback string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = fallback
	}
	if raw == "" {
		raw = "{}"
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return "", "", fmt.Errorf("解析 JSON: %w", err)
	}
	return hashJSON(value)
}

func trimLimit(value, fallback, maximum int) int {
	if value <= 0 {
		value = fallback
	}
	if value > maximum {
		value = maximum
	}
	return value
}

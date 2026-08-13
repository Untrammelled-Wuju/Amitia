package event

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type EventTypeID string

type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

type EventProducerPolicy struct {
	AllowedProducers      []string
	RequireSystemTrust    bool
	RequireNamespaceMatch bool
	MaxPayloadBytes       int64
	MaxMetadataBytes      int64
	RateLimitPerSecond    int
}

type EventSubscriberPolicy struct {
	AllowThirdParty     bool
	MaxSubscribers      int
	RequireApproval     bool
	AllowedFilterFields []string
	RequiredPermissions []string
	RequiredScope       string
}

type EventDeliveryPolicy struct {
	Timeout                time.Duration
	MaxAttempts            int
	InitialBackoff         time.Duration
	MaxBackoff             time.Duration
	BackoffMultiplier      float64
	JitterFactor           float64
	OrderingRequirement    EventOrderingRequirement
	MaxInFlight            int
	RetryableErrorCodes    []string
	NonRetryableErrorCodes []string
}

type EventOrderingRequirement string

const (
	OrderingNone         EventOrderingRequirement = "none"
	OrderingPerPartition EventOrderingRequirement = "per_partition"
	OrderingPerAggregate EventOrderingRequirement = "per_aggregate"
)

type EventRetentionPolicy struct {
	MaxAge                time.Duration
	MaxDeliveryCount      int
	DeleteAfterSuccess    bool
	DeleteAfterDeadLetter bool
	ArchiveDeadLetters    bool
}

type SensitiveFieldRule struct {
	Path               string
	Classification     string
	DefaultAction      SensitiveAction
	RequiredPermission []PermissionRequirement
}

type SensitiveAction string

const (
	SensitiveOmit                SensitiveAction = "omit"
	SensitiveMask                SensitiveAction = "mask"
	SensitiveHash                SensitiveAction = "hash"
	SensitiveSummary             SensitiveAction = "summary"
	SensitiveAllowWithPermission SensitiveAction = "allow_with_permission"
)

type EventProjectionRule struct {
	SourcePath         string
	TargetPath         string
	RequiredPermission string
	RequiresScope      string
}

type PermissionRequirement struct {
	Permission string
	Scope      string
	Reason     string
}

type EventTypeDefinition struct {
	EventTypeID      EventTypeID
	Version          int
	Description      string
	PayloadSchema    json.RawMessage
	MetadataSchema   json.RawMessage
	ProducerPolicy   EventProducerPolicy
	SubscriberPolicy EventSubscriberPolicy
	DeliveryPolicy   EventDeliveryPolicy
	OrderingPolicy   EventOrderingRequirement
	RetentionPolicy  EventRetentionPolicy
	SensitiveFields  []SensitiveFieldRule
	ProjectionRules  []EventProjectionRule
	MaxPayloadBytes  int64
	MaxMetadataBytes int64
	RiskLevel        RiskLevel
	DefinitionHash   string
}

func (d EventTypeDefinition) Hash() string {
	h := sha256.New()
	fmt.Fprintf(h, "type=%s\nversion=%d\n", d.EventTypeID, d.Version)
	if d.PayloadSchema != nil {
		h.Write(d.PayloadSchema)
	}
	if d.MetadataSchema != nil {
		h.Write(d.MetadataSchema)
	}
	pb, _ := json.Marshal(d.ProducerPolicy)
	h.Write(pb)
	sb, _ := json.Marshal(d.SubscriberPolicy)
	h.Write(sb)
	db, _ := json.Marshal(d.DeliveryPolicy)
	h.Write(db)
	rb, _ := json.Marshal(d.RetentionPolicy)
	h.Write(rb)
	sf, _ := json.Marshal(d.SensitiveFields)
	h.Write(sf)
	pr, _ := json.Marshal(d.ProjectionRules)
	h.Write(pr)
	fmt.Fprintf(h, "max_payload=%d\nmax_meta=%d\nrisk=%s\n", d.MaxPayloadBytes, d.MaxMetadataBytes, d.RiskLevel)
	return hex.EncodeToString(h.Sum(nil))
}

var reservedNamespaces = []string{
	"system.",
	"security.",
	"permission.",
	"scope.",
	"secret.",
}

func (d EventTypeDefinition) Validate() error {
	if d.EventTypeID == "" {
		return errors.New("event: event type id required")
	}
	if d.Version <= 0 {
		return errors.New("event: version must be positive")
	}
	if d.MaxPayloadBytes == 0 {
		return errors.New("event: max payload bytes required")
	}
	if d.MaxMetadataBytes == 0 {
		return errors.New("event: max metadata bytes required")
	}
	return nil
}

func (t EventTypeID) IsExtensionNamespace(extensionID string) bool {
	prefix := fmt.Sprintf("extension.%s.", extensionID)
	return strings.HasPrefix(string(t), prefix)
}

func (t EventTypeID) IsReservedNamespace() bool {
	for _, ns := range reservedNamespaces {
		if strings.HasPrefix(string(t), ns) {
			return true
		}
	}
	return false
}

func (t EventTypeID) IsHostNamespace() bool {
	return !strings.HasPrefix(string(t), "extension.")
}

type EventTypeRegistry interface {
	RegisterEventType(ctx context.Context, definition EventTypeDefinition) error
	GetEventType(ctx context.Context, typeID EventTypeID, version int) (EventTypeDefinition, error)
	ListEventTypes(ctx context.Context) ([]EventTypeDefinition, error)
	ListByNamespace(ctx context.Context, namespace string) ([]EventTypeDefinition, error)
	IsRegistered(ctx context.Context, typeID EventTypeID, version int) bool
	ValidatePayload(ctx context.Context, typeID EventTypeID, version int, payload []byte) error
}

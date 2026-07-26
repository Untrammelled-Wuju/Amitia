package resource

import "time"

type ReferenceType string

const (
	RefDependsOn       ReferenceType = "depends_on"
	RefContains        ReferenceType = "contains"
	RefUses            ReferenceType = "uses"
	RefGeneratedFrom   ReferenceType = "generated_from"
	RefInstalledBy     ReferenceType = "installed_by"
	RefOwnedBy         ReferenceType = "owned_by"
	RefScopedBy        ReferenceType = "scoped_by"
	RefSecuredBy       ReferenceType = "secured_by"
	RefScheduledBy     ReferenceType = "scheduled_by"
	RefRenderedBy      ReferenceType = "rendered_by"
	RefRuntimeManagedBy ReferenceType = "runtime_managed_by"
)

func (rt ReferenceType) IsValid() bool {
	switch rt {
	case RefDependsOn, RefContains, RefUses, RefGeneratedFrom,
		RefInstalledBy, RefOwnedBy, RefScopedBy, RefSecuredBy,
		RefScheduledBy, RefRenderedBy, RefRuntimeManagedBy:
		return true
	}
	return false
}

type OwnershipEffect string

const (
	EffectNone         OwnershipEffect = "none"
	EffectRetainTarget OwnershipEffect = "retain_target"
	EffectBlockDelete  OwnershipEffect = "block_delete"
	EffectCascadeDelete OwnershipEffect = "cascade_delete"
	EffectTransferOnDelete OwnershipEffect = "transfer_on_delete"
	EffectPromptUser   OwnershipEffect = "prompt_user"
)

func (oe OwnershipEffect) IsValid() bool {
	switch oe {
	case EffectNone, EffectRetainTarget, EffectBlockDelete,
		EffectCascadeDelete, EffectTransferOnDelete, EffectPromptUser:
		return true
	}
	return false
}

type DeleteStrategy string

const (
	StrategyCascade    DeleteStrategy = "cascade"
	StrategyRetain     DeleteStrategy = "retain"
	StrategyTransfer   DeleteStrategy = "transfer"
	StrategyPrompt     DeleteStrategy = "prompt"
	StrategyBlock      DeleteStrategy = "block"
	StrategyRebuildable DeleteStrategy = "rebuildable"
)

func (ds DeleteStrategy) IsValid() bool {
	switch ds {
	case StrategyCascade, StrategyRetain, StrategyTransfer, StrategyPrompt, StrategyBlock, StrategyRebuildable:
		return true
	}
	return false
}

type ResourceReference struct {
	ReferenceID      string          `json:"reference_id"`
	SourceResourceID string          `json:"source_resource_id"`
	TargetResourceID string          `json:"target_resource_id"`
	ReferenceType    ReferenceType   `json:"reference_type"`
	Required         bool            `json:"required"`
	OwnershipEffect  OwnershipEffect `json:"ownership_effect"`
	CreatedAt        time.Time       `json:"created_at"`
	Metadata         map[string]any  `json:"metadata,omitempty"`
}

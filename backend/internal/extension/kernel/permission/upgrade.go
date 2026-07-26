package permission

import (
	"context"
)

type PermissionUpgrade struct {
	PermissionID string `json:"permissionId"`
	Change       string `json:"change"`
	OldScope     string `json:"oldScope,omitempty"`
	NewScope     string `json:"newScope,omitempty"`
	RiskChange   string `json:"riskChange,omitempty"`
	RequiresReconfirmation bool `json:"requiresReconfirmation"`
}

type UpgradeDetector struct {
	registry *PermissionDefinitionRegistry
}

func NewUpgradeDetector(registry *PermissionDefinitionRegistry) *UpgradeDetector {
	return &UpgradeDetector{registry: registry}
}

func (d *UpgradeDetector) Detect(ctx context.Context, oldReqs, newReqs []PermissionRequirement) []PermissionUpgrade {
	upgrades := make([]PermissionUpgrade, 0)

	oldSet := make(map[string]PermissionRequirement)
	for _, req := range oldReqs {
		oldSet[req.PermissionID] = req
	}

	for _, newReq := range newReqs {
		oldReq, existed := oldSet[newReq.PermissionID]
		if !existed {
			upgrades = append(upgrades, PermissionUpgrade{
				PermissionID:           newReq.PermissionID,
				Change:                 "new_permission",
				RequiresReconfirmation: true,
			})
			continue
		}

		if newReq.Scope.Type != oldReq.Scope.Type {
			upgrades = append(upgrades, PermissionUpgrade{
				PermissionID:           newReq.PermissionID,
				Change:                 "scope_expanded",
				OldScope:               string(oldReq.Scope.Type),
				NewScope:               string(newReq.Scope.Type),
				RequiresReconfirmation: true,
			})
		}

		newDef, newOK := d.registry.Get(newReq.PermissionID)
		_, oldOK := d.registry.Get(oldReq.PermissionID)
		if newOK && oldOK && string(newDef.RiskLevel) != "" {
			upgrades = append(upgrades, PermissionUpgrade{
				PermissionID:           newReq.PermissionID,
				Change:                 "risk_changed",
				RiskChange:             string(newDef.RiskLevel),
				RequiresReconfirmation: riskIncreased(string(newDef.RiskLevel)),
			})
		}
	}

	return upgrades
}

func riskIncreased(risk string) bool {
	return risk == "high"
}

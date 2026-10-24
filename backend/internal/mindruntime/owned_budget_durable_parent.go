package mindruntime

import "context"

type ownedBudgetDurableParentKey struct{}

type OwnedBudgetParentLimits struct {
	ToolCalls          int
	Tokens             int64
	CostMicroUSD       int64
	ChildInteractionID string
}

func WithDurableOwnedBudgetParent(ctx context.Context, limits OwnedBudgetParentLimits) context.Context {
	return context.WithValue(ctx, ownedBudgetDurableParentKey{}, limits)
}

func durableOwnedBudgetParent(ctx context.Context) (OwnedBudgetParentLimits, bool) {
	limits, ok := ctx.Value(ownedBudgetDurableParentKey{}).(OwnedBudgetParentLimits)
	return limits, ok
}

const ownedBudgetParentCondition = `EXISTS (
    SELECT 1 FROM interaction_records AS parent
    WHERE parent.id=? AND parent.space_id=? AND parent.character_id=?
      AND parent.status NOT IN ('pausing','paused','cancelled','completed','failed','interrupted','archived','superseded')
      AND parent.status<>''
      AND (parent.cancel_requested_at IS NULL OR SUBSTR(CAST(parent.cancel_requested_at AS TEXT),1,4) IN ('','0000','0001'))
      AND json_valid(parent.recovery_descriptor_json)=1
      AND json_extract(parent.recovery_descriptor_json,'$.multiAgent.coordinationId')=?
      AND json_extract(parent.recovery_descriptor_json,'$.multiAgent.status') IN ('running','waiting')
      AND COALESCE(json_extract(parent.recovery_descriptor_json,'$.multiAgent.cancelRequested'),0)=0
      AND json_extract(parent.recovery_descriptor_json,'$.multiAgent.ownedScope.spaceId')=?
      AND json_extract(parent.recovery_descriptor_json,'$.multiAgent.ownedScope.roleId')=?
      AND json_extract(parent.recovery_descriptor_json,'$.multiAgent.ownedScope.resourceOwnerId')=?
      AND COALESCE(json_extract(parent.recovery_descriptor_json,'$.multiAgent.resourceOwnerId'),'') IN ('',?)
      AND COALESCE(json_extract(parent.recovery_descriptor_json,'$.multiAgent.maxToolCalls'),0)=?
      AND COALESCE(json_extract(parent.recovery_descriptor_json,'$.multiAgent.maxTotalTokens'),0)=?
      AND COALESCE(json_extract(parent.recovery_descriptor_json,'$.multiAgent.maxCostMicroUsd'),0)=?
      AND EXISTS (SELECT 1 FROM json_each(parent.recovery_descriptor_json,'$.multiAgent.assignmentRefs') AS assignment
          WHERE json_extract(assignment.value,'$.assignmentId')=?
            AND json_extract(assignment.value,'$.status')='running')
      AND EXISTS (SELECT 1 FROM interaction_records AS child
          WHERE child.id=? AND child.space_id=? AND child.character_id=?
            AND child.causation_id=parent.id AND child.correlation_id=?
            AND child.status='processing'
            AND (child.cancel_requested_at IS NULL OR SUBSTR(CAST(child.cancel_requested_at AS TEXT),1,4) IN ('','0000','0001')))
)`

func ownedBudgetParentArguments(spaceID, roleID, ownerID, parentID, coordinationID, assignmentID, childID string, toolLimit int, tokenLimit, costLimit int64) []any {
	return []any{
		parentID, spaceID, roleID, coordinationID,
		spaceID, roleID, ownerID, ownerID,
		toolLimit, tokenLimit, costLimit, assignmentID,
		childID, spaceID, roleID, coordinationID,
	}
}

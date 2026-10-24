package interaction

import (
	"context"
	"errors"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (c *MultiAgentCoordinator) validateStartParent(ctx context.Context, req StartCoordinationRequest, goal decision.Goal) error {
	if req.ParentInteractionID == "" {
		if req.OwnedScope != nil {
			return errors.New("multi_agent: owned delegation requires a durable parent interaction")
		}
		return nil
	}
	if c.tracker == nil {
		return ErrInteractionNotFound
	}
	parent, found, err := c.tracker.Get(ctx, req.ParentInteractionID)
	if err != nil {
		return err
	}
	if !found || parent == nil {
		return ErrInteractionNotFound
	}
	if parent.IsTerminal() || !parent.CancelRequestedAt.IsZero() {
		return errors.New("multi_agent: parent interaction is no longer allowed to dispatch workers")
	}
	scope := parent.Scope.Normalize()
	if goal.SpaceID != "" && goal.SpaceID != scope.SpaceID || goal.CharacterID != "" && goal.CharacterID != scope.CharacterID || goal.ConversationID != "" && goal.ConversationID != scope.ConversationID {
		return errors.New("multi_agent: parent goal and interaction ownership conflict")
	}
	if req.OwnedScope == nil {
		return nil
	}
	owned := *req.OwnedScope
	if owned.SpaceID == "" || owned.CoreID == "" || owned.RoleID == "" || owned.ResourceOwnerID == "" || owned.InitiatorDeviceID == "" || owned.TargetDeviceID == "" ||
		owned.SpaceID != scope.SpaceID || owned.RoleID != scope.CharacterID {
		return coordination.ErrWrongOwner
	}
	for _, worker := range req.WorkerRefs {
		if worker.CharacterID != owned.RoleID {
			return coordination.ErrRoleRequired
		}
	}
	return nil
}

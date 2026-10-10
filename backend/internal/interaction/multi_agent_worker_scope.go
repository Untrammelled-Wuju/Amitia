package interaction

import (
	"context"
	"fmt"
	"strings"
)

func (c *MultiAgentCoordinator) resolveWorkerScope(ctx context.Context, ac *activeCoordination) (string, string, error) {
	if c == nil || c.goals == nil {
		return "", "", fmt.Errorf("multi_agent: parent goal registry unavailable")
	}
	goal, exists := c.goals.Get(ac.parentGoalID)
	if !exists || goal.Revision != ac.parentGoalRev {
		return "", "", fmt.Errorf("multi_agent: parent goal identity is missing or stale")
	}
	spaceID := strings.TrimSpace(goal.SpaceID)
	conversationID := strings.TrimSpace(goal.ConversationID)
	if ac.parentInteractionID != "" {
		if c.tracker == nil {
			return "", "", fmt.Errorf("multi_agent: parent interaction tracker unavailable")
		}
		parent, found, err := c.tracker.Get(ctx, ac.parentInteractionID)
		if err != nil {
			return "", "", err
		}
		if !found || parent == nil {
			return "", "", fmt.Errorf("multi_agent: parent interaction not found")
		}
		scope := parent.Scope.Normalize()
		if spaceID != "" && scope.SpaceID != "" && spaceID != scope.SpaceID {
			return "", "", fmt.Errorf("multi_agent: parent goal and interaction space identities conflict")
		}
		if conversationID != "" && scope.ConversationID != "" && conversationID != scope.ConversationID {
			return "", "", fmt.Errorf("multi_agent: parent goal and interaction conversation identities conflict")
		}
		if scope.SpaceID != "" {
			spaceID = scope.SpaceID
		}
		if scope.ConversationID != "" {
			conversationID = scope.ConversationID
		}
	}
	if _, isUnifiedRunner := c.starter.(*unifiedEntryWorkerRunner); isUnifiedRunner {
		if spaceID == "" || conversationID == "" {
			return "", "", fmt.Errorf("multi_agent: unified worker requires a durable space and conversation scope")
		}
	}
	return spaceID, conversationID, nil
}

package kernel

import (
	"context"
	"database/sql"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/extension_page_host"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

type repositoryScopeRelationChecker struct {
	db         *sql.DB
	resources  sqlite.ResourceRepository
	operations sqlite.OperationRepository
	sessions   *extension_page_host.SessionManager
}

func newRepositoryScopeRelationChecker(db *sql.DB, resources sqlite.ResourceRepository, operations sqlite.OperationRepository) *repositoryScopeRelationChecker {
	return &repositoryScopeRelationChecker{db: db, resources: resources, operations: operations}
}

func (c *repositoryScopeRelationChecker) ConversationBelongsToCharacter(ctx context.Context, conversationID, characterID string) bool {
	if c == nil || c.db == nil || conversationID == "" || characterID == "" {
		return false
	}
	var count int
	return c.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM conversations WHERE id = ? AND character_id = ?`, conversationID, characterID).Scan(&count) == nil && count == 1
}

func (c *repositoryScopeRelationChecker) IsCharacterDeleted(ctx context.Context, characterID string) bool {
	if c == nil || c.db == nil || characterID == "" {
		return true
	}
	var count int
	return c.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM characters WHERE id = ?`, characterID).Scan(&count) != nil || count != 1
}

func (c *repositoryScopeRelationChecker) IsConversationDeleted(ctx context.Context, conversationID string) bool {
	if c == nil || c.db == nil || conversationID == "" {
		return true
	}
	var count int
	return c.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM conversations WHERE id = ?`, conversationID).Scan(&count) != nil || count != 1
}

func (c *repositoryScopeRelationChecker) ResourceOwnedBy(ctx context.Context, resourceID, resourceType, extensionID, moduleID string) bool {
	if c == nil || c.resources == nil || resourceID == "" || extensionID == "" {
		return false
	}
	owned, err := c.resources.GetResource(ctx, resourceID)
	if err != nil {
		return false
	}
	if owned.ExpiresAt != nil && time.Now().After(*owned.ExpiresAt) {
		return false
	}
	if resourceType == "" || owned.ResourceType != resourceType {
		return false
	}
	switch owned.OwnerType {
	case "extension":
		return owned.OwnerID == extensionID
	case "module":
		return moduleID != "" && owned.OwnerID == moduleID
	default:
		return false
	}
}

func (c *repositoryScopeRelationChecker) InvocationOwnedBy(ctx context.Context, invocationID, extensionID, moduleID string) bool {
	if c == nil || c.db == nil || c.operations == nil || invocationID == "" || extensionID == "" {
		return false
	}
	invocation, err := c.operations.GetInvocation(ctx, invocationID)
	if err != nil {
		return false
	}
	var count int
	return c.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM extension_contributions WHERE contribution_id = ? AND extension_id = ? AND (? = '' OR module_id = ?)`, invocation.ContributionID, extensionID, moduleID, moduleID).Scan(&count) == nil && count == 1
}

func (c *repositoryScopeRelationChecker) InvocationIsChildOf(ctx context.Context, invocationID, parentInvocationID string) bool {
	if c == nil || c.operations == nil || invocationID == "" || parentInvocationID == "" {
		return false
	}
	invocation, err := c.operations.GetInvocation(ctx, invocationID)
	return err == nil && invocation.ParentInvocationID == parentInvocationID
}

func (c *repositoryScopeRelationChecker) SessionValid(_ context.Context, sessionID, extensionID, moduleID string, generation int64) bool {
	if c == nil || c.sessions == nil || sessionID == "" || extensionID == "" || generation <= 0 {
		return false
	}
	session, err := c.sessions.GetSessionForExtension(extension_page_host.PageSessionID(sessionID), extension_page_host.ExtensionID(extensionID))
	if err != nil {
		return false
	}
	return session.ModuleID == moduleID && session.Generation == generation
}

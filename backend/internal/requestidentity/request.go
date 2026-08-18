package requestidentity

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	authctx "github.com/u-ai/backend/internal/auth"
)

const DefaultUserID = "default"

// ResolveGin resolves the authenticated account identity from the request
// context. Envelope/body user IDs are deliberately ignored for authenticated
// HTTP requests so ownership cannot be selected by the client payload.
func ResolveGin(c interface{}, _ string) string {
	if ginCtx, ok := c.(*gin.Context); ok && ginCtx != nil {
		if raw, exists := ginCtx.Get("actorContext"); exists && raw != nil {
			if actor, ok := raw.(*authctx.ActorContext); ok && actor != nil {
				if userID := strings.TrimSpace(string(actor.UserID)); userID != "" {
					return userID
				}
			}
		}
	}
	return DefaultUserID
}

// NormalizeUserID resolves local/default fallbacks through the configured local
// account identity while preserving authenticated cloud account IDs verbatim.
func NormalizeUserID(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID != "" && userID != DefaultUserID {
		return userID
	}
	if config.AppCfg != nil {
		if configured := strings.TrimSpace(config.AppCfg.Security.LocalUserID); configured != "" {
			return configured
		}
	}
	return "local_user"
}

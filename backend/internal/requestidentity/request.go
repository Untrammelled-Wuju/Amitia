package requestidentity

import (
	"strings"

	"github.com/gin-gonic/gin"
	authctx "github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/spaceidentity"
)

// LegacySpaceID is only a migration/read-compatibility sentinel for records
// created before the Space identity cutover. Runtime ownership must never be
// persisted with this value.
const LegacySpaceID = "default"

// ResolveGin returns the Space bound to the authenticated request. Request
// bodies, query parameters and headers are deliberately not accepted as an
// ownership source: a client cannot select another Space by supplying an ID.
// Local/system callers fall back to the process canonical Space, which is
// initialized before the HTTP server is exposed.
func ResolveGin(c interface{}) string {
	if ginCtx, ok := c.(*gin.Context); ok && ginCtx != nil {
		if raw, exists := ginCtx.Get("actorContext"); exists && raw != nil {
			if actor, ok := raw.(*authctx.ActorContext); ok && actor != nil {
				if id := strings.TrimSpace(string(actor.SpaceID)); id != "" {
					return id
				}
			}
		}
	}
	return CanonicalSpaceID()
}

func CanonicalSpaceID() string {
	return strings.TrimSpace(spaceidentity.DefaultSpaceID())
}

func NormalizeSpaceID(spaceID string) string {
	if v := strings.TrimSpace(spaceID); v != "" && v != LegacySpaceID {
		return v
	}
	return CanonicalSpaceID()
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

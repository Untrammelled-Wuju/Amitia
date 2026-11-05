package requestidentity

import (
	"strings"

	"github.com/gin-gonic/gin"
	authctx "github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/spaceidentity"
)

const fallbackSpaceID = "local-space"

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
	if id := strings.TrimSpace(spaceidentity.DefaultSpaceID()); id != "" {
		return id
	}
	return fallbackSpaceID
}

func NormalizeSpaceID(spaceID string) string {
	if v := strings.TrimSpace(spaceID); v != "" && v != "default" {
		return v
	}
	return CanonicalSpaceID()
}

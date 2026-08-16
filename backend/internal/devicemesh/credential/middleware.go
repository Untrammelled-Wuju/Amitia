package credential

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type TicketSnapshot struct {
	UserID    runtimeidentity.UserID
	DeviceID  runtimeidentity.DeviceID
	RuntimeID runtimeidentity.RuntimeID
	ExpiresAt time.Time
}

type BootstrapTicketValidator interface {
	Validate(ctx context.Context, rawTicket string) (*TicketSnapshot, error)
}

type DeviceRuntimePrincipal struct {
	CredentialID string
	UserID       runtimeidentity.UserID
	DeviceID     runtimeidentity.DeviceID
	RuntimeID    runtimeidentity.RuntimeID
	ExpiresAt    time.Time
}

type contextKey string

const (
	principalKey  contextKey = "device_runtime_principal"
	bootstrapTicketKey contextKey = "bootstrap_ticket_snapshot"
)

func DeviceAuthMiddleware(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		var rawCred string
		if strings.HasPrefix(header, "AmitiaDevice ") {
			rawCred = strings.TrimPrefix(header, "AmitiaDevice ")
		}
		if rawCred == "" {
			c.AbortWithStatusJSON(401, gin.H{"code": "mesh.credential_invalid", "message": "missing device credential"})
			return
		}

		cred, err := svc.Validate(c.Request.Context(), rawCred)
		if err != nil {
			status := 401
			code := "mesh.credential_invalid"
			if strings.Contains(err.Error(), "expired") {
				code = "mesh.credential_expired"
			} else if strings.Contains(err.Error(), "revoked") {
				code = "mesh.credential_revoked"
			}
			c.AbortWithStatusJSON(status, gin.H{"code": code, "message": err.Error()})
			return
		}

		principal := DeviceRuntimePrincipal{
			CredentialID: cred.ID,
			UserID:       cred.UserID,
			DeviceID:     cred.DeviceID,
			RuntimeID:    cred.RuntimeID,
			ExpiresAt:    cred.ExpiresAt,
		}

		if devHeader := c.GetHeader("X-Amitia-Device-ID"); devHeader != "" {
			if runtimeidentity.DeviceID(devHeader) != principal.DeviceID {
				c.AbortWithStatusJSON(401, gin.H{"code": "mesh.identity_mismatch", "message": "device id mismatch"})
				return
			}
		}
		if rtHeader := c.GetHeader("X-Amitia-Runtime-ID"); rtHeader != "" {
			if runtimeidentity.RuntimeID(rtHeader) != principal.RuntimeID {
				c.AbortWithStatusJSON(401, gin.H{"code": "mesh.identity_mismatch", "message": "runtime id mismatch"})
				return
			}
		}

		c.Set(string(principalKey), principal)
		c.Next()
	}
}

func BootstrapTicketAuthMiddleware(svc *Service, validator BootstrapTicketValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		var rawTicket string
		if strings.HasPrefix(header, "AmitiaBootstrap ") {
			rawTicket = strings.TrimPrefix(header, "AmitiaBootstrap ")
		}
		if rawTicket == "" {
			c.AbortWithStatusJSON(401, gin.H{"code": "mesh.bootstrap_invalid", "message": "missing bootstrap ticket"})
			return
		}

		snapshot, err := validator.Validate(c.Request.Context(), rawTicket)
		if err != nil {
			code := "mesh.bootstrap_invalid"
			if strings.Contains(err.Error(), "expired") {
				code = "mesh.bootstrap_expired"
			} else if strings.Contains(err.Error(), "consumed") {
				code = "mesh.bootstrap_consumed"
			} else if strings.Contains(err.Error(), "revoked") {
				code = "mesh.bootstrap_revoked"
			}
			c.AbortWithStatusJSON(401, gin.H{"code": code, "message": err.Error()})
			return
		}

		principal := DeviceRuntimePrincipal{
			UserID:    snapshot.UserID,
			DeviceID:  snapshot.DeviceID,
			RuntimeID: snapshot.RuntimeID,
			ExpiresAt: snapshot.ExpiresAt,
		}

		c.Set(string(principalKey), principal)
		c.Set(string(bootstrapTicketKey), snapshot)
		c.Next()
	}
}

func GinBootstrapTicket(c *gin.Context) (*TicketSnapshot, bool) {
	v, exists := c.Get(string(bootstrapTicketKey))
	if !exists {
		return nil, false
	}
	t, ok := v.(*TicketSnapshot)
	return t, ok
}

func PrincipalFromContext(ctx context.Context) (DeviceRuntimePrincipal, bool) {
	v, ok := ctx.Value(principalKey).(DeviceRuntimePrincipal)
	return v, ok
}

func GinPrincipal(c *gin.Context) (DeviceRuntimePrincipal, bool) {
	v, exists := c.Get(string(principalKey))
	if !exists {
		return DeviceRuntimePrincipal{}, false
	}
	p, ok := v.(DeviceRuntimePrincipal)
	return p, ok
}

func DeviceAuthCredentialWS(svc *Service, handler WSHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		var rawCred string
		if strings.HasPrefix(header, "AmitiaDevice ") {
			rawCred = strings.TrimPrefix(header, "AmitiaDevice ")
		}
		if rawCred == "" {
			c.AbortWithStatusJSON(401, gin.H{"code": "mesh.credential_invalid", "message": "missing device credential"})
			return
		}

		cred, err := svc.Validate(c.Request.Context(), rawCred)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"code": "mesh.credential_invalid", "message": err.Error()})
			return
		}

		principal := DeviceRuntimePrincipal{
			CredentialID: cred.ID,
			UserID:       cred.UserID,
			DeviceID:     cred.DeviceID,
			RuntimeID:    cred.RuntimeID,
		}

		c.Set(string(principalKey), principal)
		handler.HandleWS(c)
	}
}

type WSHandler interface {
	HandleWS(c *gin.Context)
}

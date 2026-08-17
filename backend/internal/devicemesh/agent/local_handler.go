package agent

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type LocalHandler struct {
	identity  *IdentityStore
	credStore *CredentialStore
	mesh      *MeshClient
	platform  runtimeidentity.Platform
	dataDir   string
}

func NewLocalHandler(dataDir string, platform runtimeidentity.Platform) *LocalHandler {
	return &LocalHandler{
		identity:  NewIdentityStore(dataDir),
		credStore: NewCredentialStore(dataDir),
		platform:  platform,
		dataDir:   dataDir,
	}
}

func (h *LocalHandler) SetMeshClient(c *MeshClient) {
	h.mesh = c
}

// R21: LoadCredential exposes credential loading for auto-recovery
func (h *LocalHandler) LoadCredential() (*StoredCredential, error) {
	return h.credStore.LoadCredential()
}

// R21: LoadIdentity exposes identity loading for auto-recovery
func (h *LocalHandler) LoadIdentity() (*LocalIdentity, error) {
	return h.identity.Load()
}

// R21: LoadCursor exposes cursor loading for auto-recovery
func (h *LocalHandler) LoadCursor() (*SessionCursor, error) {
	return h.credStore.LoadCursor()
}

func (h *LocalHandler) CredentialStore() *CredentialStore {
	return h.credStore
}

func (h *LocalHandler) RegisterRoutes(r *gin.Engine, authMW gin.HandlerFunc) {
	dm := r.Group("/internal/device-mesh")
	dm.Use(authMW)

	dm.GET("/identity", h.handleIdentity)
	dm.POST("/bootstrap", h.handleBootstrap)
	dm.GET("/status", h.handleStatus)
	dm.DELETE("/credential", h.handleDeleteCredential)
}

func (h *LocalHandler) handleIdentity(c *gin.Context) {
	id, err := h.identity.Load()
	if err != nil {
		c.JSON(500, gin.H{"code": "error", "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"deviceId":  id.DeviceID.String(),
		"runtimeId": id.RuntimeID.String(),
		"platform":  h.platform.String(),
	})
}

func (h *LocalHandler) handleBootstrap(c *gin.Context) {
	var req struct {
		CloudBaseURL    string `json:"cloudBaseUrl" binding:"required"`
		BootstrapTicket string `json:"bootstrapTicket" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "invalid_request", "message": err.Error()})
		return
	}

	id, err := h.identity.Load()
	if err != nil {
		c.JSON(500, gin.H{"code": "error", "message": err.Error()})
		return
	}

	client := NewBootstrapClient()
	resp, err := client.Exchange(c.Request.Context(), req.CloudBaseURL, req.BootstrapTicket,
		id.DeviceID.String(), id.RuntimeID.String(), h.platform.String(), "1.0.0")
	if err != nil {
		c.JSON(502, gin.H{"code": "bootstrap_failed", "message": err.Error()})
		return
	}

	expiresAt, err := time.Parse("2006-01-02T15:04:05.000Z", resp.ExpiresAt)
	if err != nil {
		expiresAt = time.Now().UTC().Add(30 * 24 * time.Hour)
	}

	cred := &StoredCredential{
		CloudBaseUrl: req.CloudBaseURL,
		CredentialID: resp.CredentialID,
		Credential:   resp.Credential,
		UserID:       runtimeidentity.UserID(resp.UserID),
		DeviceID:     runtimeidentity.DeviceID(resp.DeviceID),
		RuntimeID:    runtimeidentity.RuntimeID(resp.RuntimeID),
		ExpiresAt:    expiresAt,
		Protocol:     resp.Protocol,
	}

	if err := h.credStore.SaveCredential(cred); err != nil {
		c.JSON(500, gin.H{"code": "storage_error", "message": err.Error()})
		return
	}

	if err := h.credStore.DeleteCursor(); err != nil {
		log.Printf("devicemesh: agent: delete cursor failed: %v", err)
	}

	if h.mesh != nil {
		h.mesh.Stop()
	}

	cursor, err := h.credStore.LoadCursor()
	if err != nil {
		log.Printf("devicemesh: agent: load cursor failed: %v", err)
	}
	h.mesh = NewMeshClient(MeshClientConfig{
		CloudBaseURL: req.CloudBaseURL,
		Credential:   resp.Credential,
		UserID:       cred.UserID,
		Identity:     id,
		Cursor:       cursor,
	})
	h.mesh.SetCredentialStore(h.credStore)
	h.mesh.Start()

	c.JSON(200, gin.H{
		"ok":           true,
		"credentialId": resp.CredentialID,
		"deviceId":     resp.DeviceID,
		"runtimeId":    resp.RuntimeID,
		"expiresAt":    resp.ExpiresAt,
	})
}

func (h *LocalHandler) handleStatus(c *gin.Context) {
	state := StateUnprovisioned
	if h.mesh != nil {
		state = h.mesh.State()
	}

	cred, err := h.credStore.LoadCredential()
	if err != nil {
		log.Printf("devicemesh: agent: load credential failed: %v", err)
	}
	cursor, err := h.credStore.LoadCursor()
	if err != nil {
		log.Printf("devicemesh: agent: load cursor failed: %v", err)
	}

	resp := gin.H{
		"state":                string(state),
		"cloudBaseUrl":         "",
		"deviceId":             "",
		"runtimeId":            "",
		"runtimeSessionId":     "",
		"connectionGeneration": 0,
		"lastConnectedAt":      "",
		"lastHeartbeatAt":      "",
		"lastErrorCode":        "",
	}

	if cred != nil {
		resp["cloudBaseUrl"] = cred.CloudBaseUrl
		resp["deviceId"] = cred.DeviceID.String()
		resp["runtimeId"] = cred.RuntimeID.String()
	}

	if cursor != nil {
		resp["runtimeSessionId"] = cursor.RuntimeSessionID.String()
		resp["connectionGeneration"] = cursor.ConnectionGeneration
	}

	c.JSON(200, resp)
}

func (h *LocalHandler) handleDeleteCredential(c *gin.Context) {
	if h.mesh != nil {
		h.mesh.Stop()
		h.mesh = nil
	}

	if err := h.credStore.DeleteCredential(); err != nil {
		c.JSON(500, gin.H{"code": "delete_failed", "message": err.Error()})
		return
	}
	if err := h.credStore.DeleteCursor(); err != nil {
		c.JSON(500, gin.H{"code": "delete_failed", "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{"ok": true})
}

var _ = http.StatusOK

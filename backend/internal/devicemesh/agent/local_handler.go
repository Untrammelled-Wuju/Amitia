package agent

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/devicemesh/lan"
	"github.com/u-ai/backend/internal/devicemesh/proof"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type LocalHandler struct {
	mu                 sync.RWMutex
	identity           *IdentityStore
	credStore          *CredentialStore
	mesh               *MeshClient
	platform           runtimeidentity.Platform
	dataDir            string
	dispatcher         RuntimeDispatcher
	taskWorker         TaskWorkerIface
	taskRuntime        TaskRuntimeExecutor
	taskWorkerSet      bool
	credentialObserver func(*StoredCredential) error
	lanEndpoints       []lan.Endpoint
	providerMu         sync.Mutex
	providerContext    context.Context
	providerCancel     context.CancelFunc
	providerPaused     bool
	executionJournal   *executionjournal.Store
	executionGuard     RuntimeExecutionGuard
	localCoreID        string
	successorMu        sync.Mutex
	followMu           sync.Mutex
	followOperation    *providerFollowOperation
	followOnce         sync.Once
	followCancel       context.CancelFunc
	pendingCore        string
	bindingVersion     uint64
}

func NewLocalHandler(dataDir string, platform runtimeidentity.Platform) *LocalHandler {
	providerContext, providerCancel := context.WithCancel(context.Background())
	handler := &LocalHandler{
		providerContext: providerContext,
		providerCancel:  providerCancel,
		identity:        NewIdentityStore(dataDir),
		credStore:       NewCredentialStore(dataDir),
		platform:        platform,
		dataDir:         dataDir,
		dispatcher:      NewRuntimeDispatcher(),
	}
	if pending, err := handler.credStore.pendingUnpair(); err != nil || pending {
		handler.pauseProvider()
	}
	if pending, err := handler.credStore.LoadTransition(); err != nil {
		handler.pendingCore = "未确认的服务"
		handler.pauseProvider()
	} else if pending != nil {
		handler.pendingCore = pending.Successor.Endpoint.CoreID
		handler.pauseProvider()
	} else if candidate, err := handler.credStore.LoadCandidate(); err != nil {
		handler.pendingCore = "未确认的服务"
		handler.pauseProvider()
	} else if candidate != nil {
		handler.pendingCore = candidate.SpaceID.String()
		handler.pauseProvider()
	}
	return handler
}

func (h *LocalHandler) SetMeshClient(c *MeshClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.mesh = c
}

func (h *LocalHandler) SetDispatcher(d RuntimeDispatcher) {
	h.dispatcher = d
}

func (h *LocalHandler) SetExecutionJournal(journal *executionjournal.Store) {
	h.executionJournal = journal
}

func (h *LocalHandler) SetExecutionGuard(guard RuntimeExecutionGuard) {
	h.executionGuard = guard
}

func (h *LocalHandler) ExecutionJournal() *executionjournal.Store { return h.executionJournal }

func (h *LocalHandler) SetLocalCoreID(core string) {
	h.mu.Lock()
	h.localCoreID = core
	h.mu.Unlock()
	h.followOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		h.mu.Lock()
		h.followCancel = cancel
		h.mu.Unlock()
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					call, cancel := context.WithTimeout(ctx, 45*time.Second)
					_ = h.FollowSuccessor(call)
					cancel()
				}
			}
		}()
	})
}

func (h *LocalHandler) ProviderPath() ([]string, error) {
	h.mu.RLock()
	core := h.localCoreID
	h.mu.RUnlock()
	credential, err := h.credStore.LoadCredential()
	if err != nil {
		return nil, err
	}
	path := []string{core}
	if credential != nil {
		if len(credential.ProviderPath) > 0 {
			path = append(path, credential.ProviderPath...)
		} else {
			path = append(path, credential.SpaceID.String())
		}
	}
	return path, nil
}

func (h *LocalHandler) SetTaskWorker(w TaskWorkerIface) {
	h.taskWorker = w
	h.taskWorkerSet = w != nil
}

func (h *LocalHandler) SetTaskRuntime(tr TaskRuntimeExecutor) {
	h.taskRuntime = tr
	h.taskWorkerSet = tr != nil || h.taskWorker != nil
}

// SetCredentialObserver installs a device-local binding callback. It is used by
// subsystems that must persist canonical ownership when a Cloud credential is
// exchanged, without coupling Device Mesh to those subsystems.
func (h *LocalHandler) SetCredentialObserver(observer func(*StoredCredential) error) {
	h.credentialObserver = observer
}

func (h *LocalHandler) TaskWorkerSet() bool {
	return h.taskWorkerSet
}

// R21: LoadCredential exposes credential loading for auto-recovery
func (h *LocalHandler) LoadCredential() (*StoredCredential, error) {
	return h.credStore.LoadCredential()
}

// R21: LoadIdentity exposes identity loading for auto-recovery
func (h *LocalHandler) LoadIdentity() (*LocalIdentity, error) {
	return h.identity.Load()
}

func (h *LocalHandler) SignRequest(request *http.Request, core string) error {
	return h.identity.SignRequest(request, core)
}

// R21: LoadCursor exposes cursor loading for auto-recovery
func (h *LocalHandler) LoadCursor() (*SessionCursor, error) {
	return h.credStore.LoadCursor()
}

func (h *LocalHandler) CredentialStore() *CredentialStore {
	return h.credStore
}

func (h *LocalHandler) SetLANEndpoints(endpoints []lan.Endpoint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lanEndpoints = append([]lan.Endpoint(nil), endpoints...)
}

func (h *LocalHandler) LANEndpoints() []lan.Endpoint {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]lan.Endpoint(nil), h.lanEndpoints...)
}

func (h *LocalHandler) Stop() {
	h.pauseProvider()
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.followCancel != nil {
		h.followCancel()
	}
	if h.mesh != nil {
		h.mesh.Stop()
	}
}

func (h *LocalHandler) RegisterRoutes(r *gin.Engine, authMW gin.HandlerFunc, providerAuth ...gin.HandlerFunc) {
	dm := r.Group("/internal/device-mesh")
	dm.Use(authMW)

	dm.GET("/identity", h.handleIdentity)
	dm.GET("/lan", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.JSON(200, gin.H{"endpoints": h.LANEndpoints()})
	})
	dm.POST("/identity/sign-claim", h.handleSignClaim)
	dm.POST("/pairing/claim", h.handlePinnedPairing)
	dm.POST("/bootstrap", h.handleBootstrap)
	dm.GET("/status", h.handleStatus)
	dm.GET("/cloud-auth", h.handleCloudAuth)
	proxyAuth := authMW
	if len(providerAuth) > 0 {
		proxyAuth = providerAuth[0]
	}
	r.Any("/internal/device-mesh/provider/*path", proxyAuth, h.handleProviderProxy)
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
		"publicKey": id.PublicKey,
	})
}

func (h *LocalHandler) handleSignClaim(c *gin.Context) {
	var request struct {
		CoreID string          `json:"coreId" binding:"required"`
		Body   proof.ClaimBody `json:"body"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"message": "身份签名参数无效"})
		return
	}
	identity, err := h.identity.Load()
	if err != nil {
		c.JSON(503, gin.H{"message": err.Error()})
		return
	}
	if request.Body.DeviceID != identity.DeviceID.String() || request.Body.RuntimeID != identity.RuntimeID.String() {
		c.JSON(403, gin.H{"message": "只能为当前设备身份签名"})
		return
	}
	request.Body.Platform = strings.TrimSpace(request.Body.Platform)
	request.Body.Label = strings.TrimSpace(request.Body.Label)
	request.Body.OfferToken = strings.TrimSpace(request.Body.OfferToken)
	request.Body.SetupCode = strings.TrimSpace(request.Body.SetupCode)
	if request.Body.OfferToken == "" && request.Body.SetupCode == "" {
		c.JSON(400, gin.H{"message": "缺少配对码"})
		return
	}
	signed := proof.New(identity.PublicKey, request.CoreID, uuid.NewString(), request.Body, time.Now())
	signed.Signature, err = h.identity.Sign(signed.SigningBytes())
	if err != nil {
		c.JSON(503, gin.H{"message": err.Error()})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, signed)
}

type BindingRequest struct {
	expectedBindingVersion *uint64
	CloudBaseURL           string `json:"cloudBaseUrl" binding:"required"`
	BootstrapTicket        string `json:"bootstrapTicket" binding:"required"`
	Fingerprint            string `json:"fingerprint"`
	CoreID                 string `json:"coreId"`
}

type BindingError struct {
	Status  int
	Code    string
	Message string
}

func (e *BindingError) Error() string { return e.Message }

func bindingFailure(status int, payload gin.H) error {
	code, _ := payload["code"].(string)
	message, _ := payload["message"].(string)
	return &BindingError{Status: status, Code: code, Message: message}
}

func (h *LocalHandler) handleBootstrap(c *gin.Context) {
	var request BindingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"code": "invalid_request", "message": err.Error()})
		return
	}
	result, err := h.BindProvider(c.Request.Context(), request)
	if err != nil {
		if failure, ok := err.(*BindingError); ok {
			c.JSON(failure.Status, gin.H{"code": failure.Code, "message": failure.Message})
		} else {
			c.JSON(500, gin.H{"message": err.Error()})
		}
		return
	}
	c.JSON(200, result)
}

func (h *LocalHandler) BindProvider(ctx context.Context, req BindingRequest) (gin.H, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if req.expectedBindingVersion != nil && *req.expectedBindingVersion != h.bindingVersion {
		return nil, bindingFailure(409, gin.H{"code": "binding_changed", "message": "设备绑定已变化，旧服务切换请求已拦截"})
	}
	if h.credStore == nil || h.identity == nil {
		return nil, bindingFailure(503, gin.H{"code": "binding_unavailable", "message": "设备身份或配对凭证存储尚未就绪"})
	}
	if pending, err := h.credStore.pendingUnpair(); err != nil {
		return nil, err
	} else if pending {
		return nil, bindingFailure(409, gin.H{"code": "unpair_incomplete", "message": "解绑尚未完成，请先重试解绑，再添加设备"})
	}
	h.bindingVersion++
	if candidate, err := h.credStore.LoadCandidate(); err != nil {
		return nil, err
	} else if candidate != nil {
		if candidate.CloudBaseUrl != req.CloudBaseURL || candidate.Fingerprint != req.Fingerprint || req.CoreID != "" && candidate.SpaceID.String() != req.CoreID {
			return nil, bindingFailure(409, gin.H{"message": "已有尚未完成的服务切换，请先完成或撤销该配对"})
		}
		identity, err := h.identity.Load()
		if err != nil {
			return nil, err
		}
		if !time.Now().Before(candidate.ExpiresAt) || candidate.DeviceID != identity.DeviceID || candidate.RuntimeID != identity.RuntimeID {
			return nil, bindingFailure(403, gin.H{"message": "候选凭证已过期或设备身份已变化"})
		}
		configuration, err := lan.PinnedTLS(lan.Endpoint{URL: candidate.CloudBaseUrl, Fingerprint: candidate.Fingerprint, CoreID: candidate.SpaceID.String()})
		if err != nil {
			return nil, err
		}
		return h.activateProvider(ctx, candidate, identity, configuration)
	}
	id, err := h.identity.Load()
	if err != nil {
		return nil, bindingFailure(500, gin.H{"code": "error", "message": err.Error()})
	}

	client := NewBootstrapClient()
	var tlsConfig *tls.Config
	if req.Fingerprint != "" {
		endpoint := lan.Endpoint{URL: req.CloudBaseURL, Fingerprint: req.Fingerprint, CoreID: req.CoreID}
		tlsConfig, err = lan.PinnedTLS(endpoint)
		if err == nil {
			client, err = NewPinnedBootstrapClient(endpoint)
		}
		if err != nil {
			return nil, bindingFailure(400, gin.H{"code": "invalid_provider_identity", "message": err.Error()})
		}
	}
	client.SetIdentity(h.identity, req.CoreID)
	localCore := h.localCoreID
	if localCore == "" {
		localCore = id.DeviceID.String()
	}
	if req.CoreID != "" && (req.CoreID == localCore || req.CoreID == id.DeviceID.String()) {
		return nil, bindingFailure(409, gin.H{"code": "provider_topology_invalid", "message": "不能连接当前设备自身的云端服务"})
	}
	providerPath, err := client.ProviderPath(ctx, req.CloudBaseURL, req.CoreID, localCore)
	if err != nil {
		return nil, bindingFailure(409, gin.H{"code": "provider_topology_invalid", "message": err.Error()})
	}
	resp, err := client.Exchange(ctx, req.CloudBaseURL, req.BootstrapTicket,
		id.DeviceID.String(), id.RuntimeID.String(), h.platform.String(), "1.0.0")
	if err != nil {
		return nil, bindingFailure(502, gin.H{"code": "bootstrap_failed", "message": err.Error()})
	}

	expiresAt, err := time.Parse(time.RFC3339Nano, resp.ExpiresAt)
	if err != nil || !time.Now().Before(expiresAt) || resp.Credential == "" || resp.CredentialID == "" || resp.SpaceID == "" {
		return nil, bindingFailure(502, gin.H{"code": "invalid_credential_response", "message": "云端返回的绑定凭证无效"})
	}

	cred := &StoredCredential{
		CloudBaseUrl: req.CloudBaseURL,
		CredentialID: resp.CredentialID,
		Credential:   resp.Credential,
		SpaceID:      runtimeidentity.SpaceID(resp.SpaceID),
		DeviceID:     runtimeidentity.DeviceID(resp.DeviceID),
		RuntimeID:    runtimeidentity.RuntimeID(resp.RuntimeID),
		ExpiresAt:    expiresAt,
		Protocol:     resp.Protocol,
		Fingerprint:  req.Fingerprint,
		ProviderPath: providerPath,
	}
	if resp.DeviceID != id.DeviceID.String() || resp.RuntimeID != id.RuntimeID.String() || (req.Fingerprint != "" && resp.SpaceID != req.CoreID) {
		return nil, bindingFailure(403, gin.H{"code": "provider_identity_mismatch", "message": "配对响应与扫码身份不一致"})
	}

	if err := h.credStore.SaveCandidate(cred); err != nil {
		return nil, bindingFailure(500, gin.H{"code": "storage_error", "message": err.Error()})
	}
	return h.activateProvider(ctx, cred, id, tlsConfig)
}

func (h *LocalHandler) ResumeProviderBinding(ctx context.Context) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	credential, err := h.credStore.LoadCandidate()
	if err != nil || credential == nil {
		return false, err
	}
	if !time.Now().Before(credential.ExpiresAt) {
		return true, bindingFailure(401, gin.H{"message": "新服务的候选凭证已过期，请撤销该配对后重新连接"})
	}
	identity, err := h.identity.Load()
	if err != nil {
		return true, err
	}
	if credential.DeviceID != identity.DeviceID || credential.RuntimeID != identity.RuntimeID {
		return true, bindingFailure(403, gin.H{"message": "候选凭证与当前设备身份不一致"})
	}
	configuration, err := lan.PinnedTLS(lan.Endpoint{URL: credential.CloudBaseUrl, Fingerprint: credential.Fingerprint, CoreID: credential.SpaceID.String()})
	if err != nil {
		return true, err
	}
	h.pendingCore = credential.SpaceID.String()
	_, err = h.activateProvider(ctx, credential, identity, configuration)
	return true, err
}

func (h *LocalHandler) activateProvider(ctx context.Context, cred *StoredCredential, id *LocalIdentity, tlsConfig *tls.Config) (gin.H, error) {
	previous, err := h.credStore.LoadCredential()
	if err != nil {
		return nil, bindingFailure(500, gin.H{"code": "storage_error", "message": err.Error()})
	}
	h.pauseProvider()
	if h.mesh != nil {
		h.mesh.Stop()
		drainCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err = h.mesh.WaitStopped(drainCtx)
		cancel()
		if err != nil {
			return nil, bindingFailure(503, gin.H{"code": "provider_drain_failed", "message": "原设备通道尚未停止，服务切换已暂停"})
		}
	}
	gate := &bindingGate{dispatcher: h.dispatcher}
	newMesh := NewMeshClient(MeshClientConfig{
		CloudBaseURL:      cred.CloudBaseUrl,
		Credential:        cred.Credential,
		SpaceID:           cred.SpaceID,
		Identity:          id,
		RuntimeDispatcher: gate,
		TLSConfig:         tlsConfig,
		SignRequest:       h.identity.SignRequest,
		ExecutionJournal:  h.executionJournal,
		ExecutionGuard:    h.executionGuard,
	})
	var worker TaskWorkerIface
	if h.taskWorker != nil {
		worker = h.taskWorker
	} else {
		defaultWorker := NewTaskWorker(newMesh)
		if h.taskRuntime != nil {
			defaultWorker.SetTaskRuntime(h.taskRuntime)
		}
		worker = defaultWorker
	}
	gate.worker = worker
	newMesh.SetTaskWorker(gate)
	newMesh.Start()
	readyCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	err = newMesh.WaitReady(readyCtx)
	cancel()
	if err != nil {
		newMesh.Stop()
		return nil, bindingFailure(503, gin.H{"code": "provider_channel_not_ready", "message": "新服务通道尚未就绪，服务切换已暂停，请重试"})
	}
	if err := h.checkProviderBusiness(ctx, cred, tlsConfig); err != nil {
		newMesh.Stop()
		return nil, bindingFailure(503, gin.H{"code": "provider_business_not_ready", "message": err.Error()})
	}
	if previous != nil && previous.SpaceID != cred.SpaceID && cred.ProviderChangeID == "" {
		cred.PreviousCoreID = previous.SpaceID.String()
		cred.ProviderChangeID = uuid.NewString()
	} else if previous != nil && previous.SpaceID == cred.SpaceID && cred.ProviderChangeID == "" {
		cred.PreviousCoreID = previous.PreviousCoreID
		cred.ProviderChangeID = previous.ProviderChangeID
	}
	if err := h.credStore.SaveCandidate(cred); err != nil {
		newMesh.Stop()
		return nil, bindingFailure(500, gin.H{"code": "storage_error", "message": err.Error()})
	}
	if err := h.credStore.SaveCredential(cred); err != nil {
		newMesh.Stop()
		return nil, bindingFailure(500, gin.H{"code": "storage_error", "message": err.Error()})
	}
	if h.credentialObserver != nil {
		if err := h.credentialObserver(cred); err != nil {
			newMesh.Stop()
			var restoreErr error
			if previous != nil {
				restoreErr = h.credStore.SaveCredential(previous)
			} else {
				restoreErr = h.credStore.DeleteCredential()
			}
			return nil, bindingFailure(500, gin.H{"code": "owner_mapping_error", "message": errors.Join(err, restoreErr).Error()})
		}
	}
	if err := h.credStore.DeleteCursor(); err != nil {
		log.Printf("devicemesh: agent: delete cursor failed: %v", err)
	}
	newMesh.SetCredentialStore(h.credStore)
	h.mesh = newMesh
	if err := h.credStore.DeleteTransition(); err != nil {
		newMesh.Stop()
		return nil, bindingFailure(500, gin.H{"code": "storage_error", "message": err.Error()})
	}
	if err := h.credStore.DeleteCandidate(); err != nil {
		newMesh.Stop()
		return nil, bindingFailure(500, gin.H{"code": "storage_error", "message": err.Error()})
	}
	h.pendingCore = ""
	h.taskWorkerSet = worker != nil && h.taskRuntime != nil
	h.resumeProvider()
	gate.active.Store(true)

	return gin.H{
		"ok":           true,
		"credentialId": cred.CredentialID,
		"deviceId":     cred.DeviceID.String(),
		"runtimeId":    cred.RuntimeID.String(),
		"expiresAt":    cred.ExpiresAt.UTC().Format(time.RFC3339Nano),
	}, nil
}
func (h *LocalHandler) handleStatus(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()
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
		"state":                 string(state),
		"cloudBaseUrl":          "",
		"deviceId":              "",
		"runtimeId":             "",
		"runtimeSessionId":      "",
		"connectionGeneration":  0,
		"lastConnectedAt":       "",
		"lastHeartbeatAt":       "",
		"lastErrorCode":         "",
		"providerChangePending": h.pendingCore != "",
		"successorCoreId":       h.pendingCore,
	}

	if cred != nil {
		resp["cloudBaseUrl"] = cred.CloudBaseUrl
		resp["deviceId"] = cred.DeviceID.String()
		resp["runtimeId"] = cred.RuntimeID.String()
		resp["fingerprint"] = cred.Fingerprint
		resp["coreId"] = cred.SpaceID.String()
		resp["previousCoreId"] = cred.PreviousCoreID
		resp["providerChangeId"] = cred.ProviderChangeID
	}

	if cursor != nil {
		resp["runtimeSessionId"] = cursor.RuntimeSessionID.String()
		resp["connectionGeneration"] = cursor.ConnectionGeneration
	}

	c.JSON(200, resp)
}

func (h *LocalHandler) handleCloudAuth(c *gin.Context) {
	cred, err := h.credStore.LoadCredential()
	if err != nil {
		c.JSON(500, gin.H{"code": "credential_load_failed", "message": err.Error()})
		return
	}
	if cred == nil || cred.Credential == "" {
		c.JSON(404, gin.H{"code": "mesh.credential_missing", "message": "device is not paired with a cloud core"})
		return
	}
	if !cred.ExpiresAt.IsZero() && !time.Now().UTC().Before(cred.ExpiresAt) {
		c.JSON(401, gin.H{"code": "mesh.credential_expired", "message": "device credential has expired"})
		return
	}
	c.JSON(200, gin.H{
		"authorization": "AmitiaDevice " + cred.Credential,
		"cloudBaseUrl":  cred.CloudBaseUrl,
		"spaceId":       cred.SpaceID.String(),
		"deviceId":      cred.DeviceID.String(),
		"runtimeId":     cred.RuntimeID.String(),
		"expiresAt":     cred.ExpiresAt.UTC().Format(time.RFC3339Nano),
		"fingerprint":   cred.Fingerprint,
	})
}

func (h *LocalHandler) handleDeleteCredential(c *gin.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pauseProvider()
	if err := h.credStore.unpairIntent(true); err != nil {
		c.JSON(500, gin.H{"code": "delete_failed", "message": err.Error()})
		return
	}
	if err := h.finishUnpair(); err != nil {
		c.JSON(500, gin.H{"code": "local_authority_reset_failed", "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{"ok": true})
}

var _ = http.StatusOK

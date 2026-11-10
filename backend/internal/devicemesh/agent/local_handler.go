package agent

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
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
	lanRevision        uint64
	lanSubscribers     map[chan uint64]struct{}
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
	pendingCore        string
	bindingVersion     uint64
	stateMu            sync.Mutex
	stateSubscribers   map[chan AgentState]struct{}
	capabilityExposure *CapabilityExposureStore
	localExposureSync  func(context.Context, int64, []string) error
	coreMode           bool
}

var providerReadinessTimeout = 45 * time.Second

func NewLocalHandler(dataDir string, platform runtimeidentity.Platform) *LocalHandler {
	providerContext, providerCancel := context.WithCancel(context.Background())
	handler := &LocalHandler{
		providerContext:    providerContext,
		providerCancel:     providerCancel,
		identity:           NewIdentityStore(dataDir),
		credStore:          NewCredentialStore(dataDir),
		platform:           platform,
		dataDir:            dataDir,
		dispatcher:         NewRuntimeDispatcher(),
		stateSubscribers:   make(map[chan AgentState]struct{}),
		lanSubscribers:     make(map[chan uint64]struct{}),
		capabilityExposure: NewCapabilityExposureStore(dataDir),
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

func (h *LocalHandler) SetAgentMode(enabled bool) {
	h.mu.Lock()
	h.coreMode = !enabled
	h.mu.Unlock()
	if !enabled {
		h.pauseProvider()
	}
}

func (h *LocalHandler) AgentMode() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return !h.coreMode
}

func (h *LocalHandler) SetMeshClient(c *MeshClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.mesh = c
}

func (h *LocalHandler) SetLocalCapabilityExposureSync(sync func(context.Context, int64, []string) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.localExposureSync = sync
}

func (h *LocalHandler) PublishLocalCapabilityExposure(ctx context.Context) error {
	h.mu.RLock()
	syncExposure := h.localExposureSync
	h.mu.RUnlock()
	if syncExposure == nil {
		return nil
	}
	revision, exposed, err := h.capabilityExposure.Enabled()
	if err != nil {
		return err
	}
	return syncExposure(ctx, revision, exposed)
}

func (h *LocalHandler) PublishState(state AgentState) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	for subscriber := range h.stateSubscribers {
		select {
		case subscriber <- state:
		default:
		}
	}
	if state == StateReady {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			if err := h.FollowSuccessor(ctx); err != nil {
				log.Printf("[original-error] %v", err)
			}
		}()
	}
}

func (h *LocalHandler) subscribeState() (<-chan AgentState, func()) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	events := make(chan AgentState, 4)
	h.stateSubscribers[events] = struct{}{}
	return events, func() {
		h.stateMu.Lock()
		defer h.stateMu.Unlock()
		if _, ok := h.stateSubscribers[events]; ok {
			delete(h.stateSubscribers, events)
			close(events)
		}
	}
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

func (h *LocalHandler) PrepareProviderMesh(cred *StoredCredential, identity *LocalIdentity, tlsConfig *tls.Config) (*MeshClient, func()) {
	gate := &bindingGate{dispatcher: h.dispatcher}
	meshClient := NewMeshClient(MeshClientConfig{
		CloudBaseURL:       cred.CloudBaseUrl,
		Credential:         cred.Credential,
		SpaceID:            cred.SpaceID,
		Identity:           identity,
		RuntimeDispatcher:  gate,
		TLSConfig:          tlsConfig,
		SignRequest:        h.identity.SignRequest,
		ExecutionJournal:   h.executionJournal,
		ExecutionGuard:     h.executionGuard,
		CapabilityExposure: h.capabilityExposure,
		OnState:            h.PublishState,
	})
	var worker TaskWorkerIface
	if h.taskWorker != nil {
		worker = h.taskWorker
	} else {
		defaultWorker := NewTaskWorker(meshClient)
		if h.taskRuntime != nil {
			defaultWorker.SetTaskRuntime(h.taskRuntime)
		}
		worker = defaultWorker
	}
	gate.worker = worker
	meshClient.SetTaskWorker(gate)
	meshClient.SetCredentialStore(h.credStore)
	return meshClient, func() {
		gate.active.Store(true)
		h.taskWorkerSet = worker != nil && h.taskRuntime != nil
	}
}

func (h *LocalHandler) SetLocalCoreID(core string) {
	h.mu.Lock()
	h.localCoreID = core
	h.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if err := h.FollowSuccessor(ctx); err != nil {
			log.Printf("[original-error] %v", err)
		}
	}()
}

func (h *LocalHandler) ProviderPath() ([]string, error) {
	h.mu.RLock()
	core := h.localCoreID
	coreMode := h.coreMode
	h.mu.RUnlock()
	if coreMode {
		return []string{core}, nil
	}
	credential, err := h.credStore.LoadCredential()
	if err != nil {
		return nil, err
	}
	path := []string{core}
	if credential != nil {
		if len(credential.ProviderPath) > 0 {
			path = append(path, credential.ProviderPath...)
		} else if credential.SpaceID.String() != core {
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
	h.lanRevision++
	for subscriber := range h.lanSubscribers {
		select {
		case subscriber <- h.lanRevision:
		default:
		}
	}
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
	if h.mesh != nil {
		h.mesh.Stop()
	}
}

func (h *LocalHandler) RegisterRoutes(r *gin.Engine, authMW gin.HandlerFunc, providerAuth ...gin.HandlerFunc) {
	dm := r.Group("/internal/device-mesh")
	dm.Use(authMW)
	if len(providerAuth) > 0 {
		dm.Use(providerAuth[0])
	}

	dm.GET("/identity", h.handleIdentity)
	dm.GET("/lan", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.JSON(200, gin.H{"endpoints": h.LANEndpoints()})
	})
	dm.GET("/lan/events", h.handleLANEvents)
	dm.POST("/identity/sign-claim", h.handleSignClaim)
	dm.POST("/pairing/claim", h.handlePinnedPairing)
	dm.POST("/endpoint/update", h.handleUpdateEndpoint)
	dm.GET("/pairing/requests/:requestId/events", h.handlePinnedPairingEvents)
	dm.POST("/bootstrap", h.handleBootstrap)
	dm.GET("/status", h.handleStatus)
	dm.GET("/status/events", h.handleStatusEvents)
	dm.GET("/capabilities", h.handleCapabilities)
	dm.PUT("/capabilities/:capability", h.handleSetCapability)
	dm.GET("/capabilities/events", h.handleCapabilityEvents)
	dm.GET("/cloud-auth", h.handleCloudAuth)
	proxyMiddlewares := []gin.HandlerFunc{authMW}
	proxyMiddlewares = append(proxyMiddlewares, providerAuth...)
	proxyMiddlewares = append(proxyMiddlewares, h.handleProviderProxy)
	r.Any("/internal/device-mesh/provider/*path", proxyMiddlewares...)
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
		c.JSON(400, gin.H{"message": err.Error()})
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
	expectedCandidate      *StoredCredential
	expectedCanonical      *StoredCredential
	CloudBaseURL           string `json:"cloudBaseUrl" binding:"required"`
	BootstrapTicket        string `json:"bootstrapTicket" binding:"required"`
	Fingerprint            string `json:"fingerprint"`
	CoreID                 string `json:"coreId"`
	ReplaceCandidate       bool   `json:"replaceCandidate"`
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
	if message == "" {
		message = fmt.Sprint(payload)
	}
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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
	candidate, err := h.credStore.LoadCandidate()
	if err != nil {
		return nil, err
	}
	if req.expectedCandidate != nil {
		active, err := h.credStore.LoadCredential()
		if err != nil {
			return nil, err
		}
		if !sameOptionalBinding(active, req.expectedCanonical) || !sameCandidateBinding(candidate, req.expectedCandidate) {
			return nil, bindingFailure(409, gin.H{"code": "binding_changed", "message": "候选设备绑定已变化，旧服务切换请求已拦截"})
		}
	}
	h.bindingVersion++
	if candidate != nil && req.expectedCandidate == nil {
		matchesRequest := candidate.CloudBaseUrl == req.CloudBaseURL && candidate.Fingerprint == req.Fingerprint && (req.CoreID == "" || candidate.SpaceID.String() == req.CoreID)
		if !matchesRequest && !req.ReplaceCandidate {
			return nil, bindingFailure(409, gin.H{"message": "已有尚未完成的服务切换，请先完成或撤销该配对"})
		}
		if matchesRequest {
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := h.credStore.SaveCandidate(cred); err != nil {
		return nil, bindingFailure(500, gin.H{"code": "storage_error", "message": err.Error()})
	}
	return h.activateProvider(ctx, cred, id, tlsConfig)
}

func (h *LocalHandler) ResumeProviderBinding(ctx context.Context) (bool, error) {
	return h.resumeProviderBinding(ctx, nil, nil)
}

func (h *LocalHandler) resumeProviderBinding(ctx context.Context, expectedVersion *uint64, expectedCandidate *StoredCredential) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return true, err
	}
	credential, err := h.credStore.LoadCandidate()
	if err == nil && expectedVersion != nil && (*expectedVersion != h.bindingVersion || (credential == nil) != (expectedCandidate == nil) || credential != nil && !sameCandidateBinding(credential, expectedCandidate)) {
		return true, bindingFailure(409, gin.H{"code": "binding_changed", "message": "候选设备绑定已变化，旧服务切换请求已拦截"})
	}
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
	newMesh, activateWorker := h.PrepareProviderMesh(cred, id, tlsConfig)
	newMesh.Start()
	readyCtx, cancel := context.WithTimeout(ctx, providerReadinessTimeout)
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
	if err := ctx.Err(); err != nil {
		newMesh.Stop()
		return nil, err
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
	h.resumeProvider()
	activateWorker()

	return gin.H{
		"ok":           true,
		"credentialId": cred.CredentialID,
		"deviceId":     cred.DeviceID.String(),
		"runtimeId":    cred.RuntimeID.String(),
		"expiresAt":    cred.ExpiresAt.UTC().Format(time.RFC3339Nano),
	}, nil
}

func (h *LocalHandler) handleCapabilities(c *gin.Context) {
	state, err := h.capabilityExposure.Snapshot()
	if err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, state)
}

func (h *LocalHandler) handleSetCapability(c *gin.Context) {
	var request struct {
		Enabled          bool  `json:"enabled"`
		ExpectedRevision int64 `json:"expectedRevision"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	state, err := h.capabilityExposure.SetEnabled(c.Param("capability"), request.ExpectedRevision, request.Enabled)
	if err != nil {
		c.JSON(409, gin.H{"message": err.Error()})
		return
	}
	h.mu.RLock()
	mesh := h.mesh
	localSync := h.localExposureSync
	h.mu.RUnlock()
	synced := false
	var syncErrs []error
	if mesh != nil {
		syncCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		synced, err = mesh.PublishCapabilityExposure(syncCtx)
		cancel()
		if err != nil {
			syncErrs = append(syncErrs, err)
		}
	}
	if localSync != nil {
		syncCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		err = localSync(syncCtx, state.Revision, state.EnabledIDs)
		cancel()
		if err != nil {
			syncErrs = append(syncErrs, err)
		} else {
			synced = true
		}
	}
	syncError := ""
	if syncErr := errors.Join(syncErrs...); syncErr != nil {
		syncError = syncErr.Error()
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"state": state, "centerSynced": synced, "syncError": syncError})
}

func (h *LocalHandler) handleCapabilityEvents(c *gin.Context) {
	state, err := h.capabilityExposure.Snapshot()
	if err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}
	events, cancel := h.capabilityExposure.Subscribe()
	defer cancel()
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-store")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	send := func(value CapabilityExposureState) error {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if _, err := c.Writer.WriteString("event: capability_exposure\ndata: " + string(data) + "\n\n"); err != nil {
			return err
		}
		c.Writer.Flush()
		return nil
	}
	if err := send(state); err != nil {
		return
	}
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case value, ok := <-events:
			if !ok {
				return
			}
			if err := send(value); err != nil {
				return
			}
		case <-heartbeat.C:
			if _, err := c.Writer.WriteString(": keepalive\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}
func (h *LocalHandler) handleStatus(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.coreMode {
		c.JSON(200, gin.H{
			"state":                 string(StateUnprovisioned),
			"cloudBaseUrl":          "",
			"deviceId":              "",
			"runtimeId":             "",
			"runtimeSessionId":      "",
			"connectionGeneration":  0,
			"lastConnectedAt":       "",
			"lastHeartbeatAt":       "",
			"lastErrorCode":         "",
			"providerChangePending": false,
			"successorCoreId":       "",
			"agentMode":             false,
		})
		return
	}
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
		"agentMode":             true,
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

func (h *LocalHandler) handleStatusEvents(c *gin.Context) {
	events, cancel := h.subscribeState()
	defer cancel()
	h.mu.RLock()
	state := StateUnprovisioned
	if h.mesh != nil {
		state = h.mesh.State()
	}
	h.mu.RUnlock()
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-store")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	send := func(value AgentState) error {
		if _, err := c.Writer.WriteString("event: mesh_state\ndata: {\"state\":\"" + string(value) + "\"}\n\n"); err != nil {
			return err
		}
		c.Writer.Flush()
		return nil
	}
	if err := send(state); err != nil {
		return
	}
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case value, ok := <-events:
			if !ok {
				return
			}
			if err := send(value); err != nil {
				return
			}
		case <-heartbeat.C:
			if _, err := c.Writer.WriteString(": keepalive\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}

func (h *LocalHandler) handleCloudAuth(c *gin.Context) {
	h.mu.RLock()
	coreMode := h.coreMode
	h.mu.RUnlock()
	if coreMode {
		c.JSON(409, gin.H{"code": "mesh.agent_mode_required", "message": "当前 Runtime 是统筹中心节点，不作为子设备使用远程统筹凭据"})
		return
	}
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

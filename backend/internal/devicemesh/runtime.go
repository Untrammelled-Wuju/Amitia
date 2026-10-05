package devicemesh

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/bootstrap"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/credential"
	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/devicemesh/lan"
	"github.com/u-ai/backend/internal/devicemesh/server"
	"github.com/u-ai/backend/internal/deviceruntime"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type dispatcherResolveAdapter interface {
	Resolve(handlerName string) agent.RuntimeInvokeHandler
}

type Runtime struct {
	DB                        *sql.DB
	BootstrapSvc              *bootstrap.Service
	CredentialSvc             *credential.Service
	Coordination              *coordination.Service
	BusinessCoordinationReady bool
	CoreDataPort              coordination.DataPort
	LocalDeviceDataPort       coordination.DataPort
	LocalDeviceID             string
	Hub                       *server.ConnectionHub
	Handler                   *server.Handler
	Probe                     *server.ProbeService
	DeviceReg                 *host_registry.Registry
	LocalHandler              *agent.LocalHandler
	PendingInvocations        *capability.PendingInvocationManager
	PendingTasks              *task_runtime.PendingTaskManager
	sessions                  *deviceruntime.Service
	dispatcher                dispatcherResolveAdapter
	deviceDispatcher          dispatcherResolveAdapter
	taskRuntime               agent.TaskRuntimeExecutor
	deliveryMu                sync.Mutex
	deliveryCancel            context.CancelFunc
	deliveryDone              chan struct{}
}

func NewCloudRuntime(db *sql.DB, deviceReg *host_registry.Registry) (*Runtime, error) {
	hub := server.NewConnectionHub()
	return NewCloudRuntimeWithHub(db, deviceReg, hub)
}

func NewCloudRuntimeWithHub(db *sql.DB, deviceReg *host_registry.Registry, hub *server.ConnectionHub) (*Runtime, error) {
	return NewCloudRuntimeWithHubAndSessions(db, deviceReg, hub, nil)
}

// NewCloudRuntimeWithHubAndSessions constructs the cloud runtime around the
// caller-provided authoritative DeviceRuntime session service. Production
// wiring should pass the Kernel-owned service so Desktop Pet, DeviceMesh and
// the extension kernel share one in-process session authority. A nil service
// is accepted only for compatibility callers and tests, where this constructor
// creates an isolated service backed by the same database.
func NewCloudRuntimeWithHubAndSessions(
	db *sql.DB,
	deviceReg *host_registry.Registry,
	hub *server.ConnectionHub,
	sessions *deviceruntime.Service,
) (*Runtime, error) {
	if db == nil || deviceReg == nil || deviceReg.Database() != db {
		return nil, fmt.Errorf("devicemesh: credentials and device registry must share the authoritative database")
	}
	if err := EnsureSchema(context.Background(), db); err != nil {
		return nil, err
	}

	bootstrapRepo := bootstrap.NewRepository(db)
	credRepo := credential.NewRepository(db)
	credSvc := credential.NewService(credRepo, DeviceCredentialTTL)
	credSvc.EnableRequestProof()

	exchangeFn := func(ctx context.Context, tx *sql.Tx, spaceID runtimeidentity.SpaceID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID, now time.Time, expires time.Time) (string, string, error) {
		rawCred, err := credential.GenerateRawCredential()
		if err != nil {
			return "", "", err
		}
		credID := uuid.New().String()
		newCred := &credential.DeviceRuntimeCredential{
			ID:             credID,
			SpaceID:        spaceID,
			DeviceID:       deviceID,
			RuntimeID:      runtimeID,
			CredentialHash: credential.HashRawCredential(rawCred),
			Status:         credential.CredentialActive,
			CreatedAt:      now,
			ExpiresAt:      expires,
			LastUsedAt:     now,
			Revision:       1,
		}
		if err := credRepo.ExchangeAtomicTx(ctx, tx, spaceID, deviceID, runtimeID, now, newCred); err != nil {
			return "", "", err
		}
		return credID, rawCred, nil
	}

	trustFn := func(ctx context.Context, tx *sql.Tx, deviceID runtimeidentity.DeviceID) error {
		return deviceReg.MarkDeviceTrustedTx(ctx, tx, deviceID)
	}

	bootstrapSvc := bootstrap.NewServiceWithDependencies(bootstrapRepo, db, exchangeFn, trustFn, BootstrapTicketTTL, DeviceCredentialTTL)

	probe := server.NewProbeService(hub)

	if sessions == nil {
		sessionStore := deviceruntime.NewSQLiteSessionStore(db)
		if err := sessionStore.EnsureSchema(context.Background()); err != nil {
			return nil, err
		}
		var err error
		sessions, err = deviceruntime.NewService(sessionStore, deviceruntime.ServiceOptions{})
		if err != nil {
			return nil, err
		}
	}

	rt := &Runtime{
		DB:            db,
		BootstrapSvc:  bootstrapSvc,
		CredentialSvc: credSvc,
		Coordination:  coordination.NewService(db),
		Hub:           hub,
		Probe:         probe,
		DeviceReg:     deviceReg,
		sessions:      sessions,
	}

	rt.Handler = server.NewHandler(sessions, hub)
	rt.Coordination.SetAuthorityBarrier(rt.fenceRemoteAuthority)
	rt.Handler.AddOnReady(func(space runtimeidentity.SpaceID, device runtimeidentity.DeviceID) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			if err := rt.ReconcileRemoteDevice(ctx, space.String(), device.String()); err != nil {
				log.Printf("devicemesh: remote authority reconciliation failed: %v", err)
			}
		}()
	})

	return rt, nil
}

func (rt *Runtime) SetSessions(sessions *deviceruntime.Service) {
	rt.sessions = sessions
	if rt.Handler != nil {
		rt.Handler.SetSessions(sessions)
	}
}

func (rt *Runtime) SetDispatcher(d dispatcherResolveAdapter) {
	rt.dispatcher = d
	if rt.Handler != nil && d != nil {
		rt.Handler.SetDispatcher(d)
	}
}

func (rt *Runtime) SetTaskRuntime(tr agent.TaskRuntimeExecutor) {
	rt.taskRuntime = tr
}

func (rt *Runtime) SetPendingInvocations(mgr *capability.PendingInvocationManager) {
	rt.PendingInvocations = mgr
}

func (rt *Runtime) SetPendingTasks(mgr *task_runtime.PendingTaskManager) {
	rt.PendingTasks = mgr
}

func (rt *Runtime) GetSessions() *deviceruntime.Service {
	return rt.sessions
}

func (rt *Runtime) GetDispatcher() dispatcherResolveAdapter {
	return rt.dispatcher
}

func (rt *Runtime) GetTaskRuntime() agent.TaskRuntimeExecutor {
	return rt.taskRuntime
}

func (rt *Runtime) Start() error {
	rt.deliveryMu.Lock()
	defer rt.deliveryMu.Unlock()
	if rt.Hub == nil {
		rt.Hub = server.NewConnectionHub()
	}
	if rt.Probe == nil && rt.Hub != nil {
		rt.Probe = server.NewProbeService(rt.Hub)
	}
	if rt.Coordination != nil && rt.deliveryCancel == nil {
		ctx, cancel := context.WithCancel(context.Background())
		rt.deliveryCancel = cancel
		rt.deliveryDone = make(chan struct{})
		go rt.runDeviceDataDelivery(ctx, rt.deliveryDone)
	}
	return nil
}

func (rt *Runtime) Stop() error {
	rt.deliveryMu.Lock()
	defer rt.deliveryMu.Unlock()
	if rt.deliveryCancel != nil {
		rt.deliveryCancel()
		<-rt.deliveryDone
		rt.deliveryCancel = nil
		rt.deliveryDone = nil
	}
	if rt.Hub != nil {
		rt.Hub.CloseAll()
	}
	if rt.LocalHandler != nil {
		rt.LocalHandler.Stop()
	}
	return nil
}

func NewDeviceAgentRuntime(dataDir string, platform runtimeidentity.Platform, taskRuntime agent.TaskRuntimeExecutor, dispatcher dispatcherResolveAdapter, databases ...*sql.DB) (*Runtime, error) {
	if dispatcher == nil {
		return nil, fmt.Errorf("devicemesh: device-agent runtime dispatcher is required")
	}
	if taskRuntime == nil {
		return nil, fmt.Errorf("devicemesh: device-agent task runtime is required")
	}
	localHandler := agent.NewLocalHandler(dataDir, platform)
	localHandler.SetDispatcher(dispatcher)
	localHandler.SetTaskRuntime(taskRuntime)
	if len(databases) > 0 && databases[0] != nil {
		localHandler.SetExecutionJournal(executionjournal.NewStore(databases[0]))
		localHandler.SetExecutionGuard(agent.NewOwnedToolGuard(databases[0], dataDir))
	}

	rt := &Runtime{
		LocalHandler: localHandler,
		taskRuntime:  taskRuntime,
		dispatcher:   dispatcher,
	}
	if len(databases) > 0 && databases[0] != nil {
		rt.DB = databases[0]
		rt.Coordination = coordination.NewService(databases[0])
	}

	rt.autoRecoverCredential(localHandler)

	return rt, nil
}

func (rt *Runtime) autoRecoverCredential(handler *agent.LocalHandler) {
	cred, err := handler.LoadCredential()
	if err != nil || cred == nil {
		return
	}

	if cred.ExpiresAt.Before(time.Now()) {
		return
	}

	identity, err := handler.LoadIdentity()
	if err != nil || identity == nil {
		return
	}

	cursor, _ := handler.LoadCursor()
	var tlsConfig *tls.Config
	if cred.Fingerprint != "" {
		tlsConfig, err = lan.PinnedTLS(lan.Endpoint{URL: cred.CloudBaseUrl, Fingerprint: cred.Fingerprint, CoreID: cred.SpaceID.String()})
		if err != nil {
			return
		}
	}

	dispatcher := rt.dispatcher
	if rt.deviceDispatcher != nil {
		dispatcher = rt.deviceDispatcher
	}

	meshClient := agent.NewMeshClient(agent.MeshClientConfig{
		CloudBaseURL:      cred.CloudBaseUrl,
		Credential:        cred.Credential,
		SpaceID:           cred.SpaceID,
		Identity:          identity,
		Cursor:            cursor,
		RuntimeDispatcher: dispatcher,
		TLSConfig:         tlsConfig,
		SignRequest:       handler.SignRequest,
		ExecutionJournal:  handler.ExecutionJournal(),
	})
	taskWorker := agent.NewTaskWorker(meshClient)
	if rt.taskRuntime != nil {
		taskWorker.SetTaskRuntime(rt.taskRuntime)
	}
	meshClient.SetTaskWorker(taskWorker)
	meshClient.SetCredentialStore(handler.CredentialStore())
	handler.SetMeshClient(meshClient)
	meshClient.Start()
}

func (rt *Runtime) AttachDeviceAgent(dataDir string, platform runtimeidentity.Platform, dispatcher dispatcherResolveAdapter, observer func(*agent.StoredCredential) error) error {
	if rt.LocalHandler != nil || dispatcher == nil {
		return fmt.Errorf("设备 Agent 已存在或缺少能力路由")
	}
	handler := agent.NewLocalHandler(dataDir, platform)
	handler.SetDispatcher(dispatcher)
	handler.SetTaskRuntime(rt.taskRuntime)
	if rt.DB != nil {
		handler.SetExecutionJournal(executionjournal.NewStore(rt.DB))
		roleGuard, _ := rt.LocalDeviceDataPort.(coordination.SourceRoleExecutionGuard)
		handler.SetExecutionGuard(agent.NewOwnedToolGuard(rt.DB, dataDir, roleGuard))
	}
	handler.SetCredentialObserver(observer)
	if err := handler.RecoverUnpair(); err != nil {
		return err
	}
	cred, err := handler.LoadCredential()
	if err != nil {
		return err
	}
	if cred != nil && observer != nil {
		if err := observer(cred); err != nil {
			return err
		}
	}
	rt.LocalHandler = handler
	rt.deviceDispatcher = dispatcher
	rt.autoRecoverCredential(handler)
	return nil
}

// InvokeDeviceHandler sends a bounded management invocation to the active
// Device Agent for targetDeviceID and waits for the result. It is intended for
// cloud control-plane bridges such as Game Center; execution still occurs only
// on the device.
func (rt *Runtime) InvokeDeviceHandler(
	ctx context.Context,
	spaceID runtimeidentity.SpaceID,
	targetDeviceID runtimeidentity.DeviceID,
	handlerName string,
	input []byte,
	deadline time.Duration,
) (capability.UnifiedToolResult, error) {
	return rt.InvokeDeviceHandlerWithRuntimeType(ctx, spaceID, targetDeviceID, capability.RuntimeTypeGameHost, handlerName, input, deadline)
}

// InvokeDeviceHandlerWithRuntimeType is the generic cloud-to-device control
// plane primitive. It keeps GameHost compatibility while allowing internal
// subsystems such as Desktop Pet Behavior to use their own runtime identity.
func (rt *Runtime) InvokeDeviceHandlerWithRuntimeType(
	ctx context.Context,
	spaceID runtimeidentity.SpaceID,
	targetDeviceID runtimeidentity.DeviceID,
	runtimeType capability.RuntimeType,
	handlerName string,
	input []byte,
	deadline time.Duration,
) (capability.UnifiedToolResult, error) {
	if rt == nil || rt.Hub == nil || rt.PendingInvocations == nil {
		return capability.UnifiedToolResult{}, fmt.Errorf("devicemesh: invocation runtime unavailable")
	}
	conn, ok := rt.Hub.GetByDevice(spaceID, targetDeviceID)
	if !ok || conn == nil {
		return capability.UnifiedToolResult{}, fmt.Errorf("devicemesh: target device is offline")
	}
	if deadline <= 0 {
		deadline = 30 * time.Second
	}
	var authorityCall string
	if handlerName == "coordination.data" {
		var envelope struct {
			Operation   string                        `json:"operation"`
			Scope       coordination.ExecutionScope   `json:"scope"`
			Commit      coordination.Commit           `json:"commit"`
			Interrupted coordination.InterruptedReply `json:"interrupted"`
		}
		if err := json.Unmarshal(input, &envelope); err != nil {
			return capability.UnifiedToolResult{}, err
		}
		if envelope.Operation != "authority-fence" && envelope.Operation != "authority-reconcile" {
			scope := envelope.Scope
			if envelope.Operation == "apply" {
				scope = envelope.Commit.Scope
			}
			if envelope.Operation == "interrupted" {
				scope = envelope.Interrupted.Scope
			}
			if scope.SpaceID != spaceID.String() || scope.TargetDeviceID != targetDeviceID.String() || rt.Coordination == nil {
				return capability.UnifiedToolResult{}, coordination.ErrWrongOwner
			}
			var err error
			authorityCall, err = rt.Coordination.TrackRemoteAuthority(ctx, scope, conn.SessionID.String(), conn.Generation)
			if err != nil {
				return capability.UnifiedToolResult{}, err
			}
		}
	}
	invocationID := uuid.NewString()
	if authorityCall != "" {
		invocationID = authorityCall
	}
	port := capability.NewMeshDeviceRuntimeInvocationPort(&capability.MeshRuntimePorts{
		Hub:                rt.Hub,
		PendingInvocations: rt.PendingInvocations,
	})
	route := capability.RuntimeExecutionRoute{
		Binding: capability.RuntimeBinding{
			RuntimeType: runtimeType,
			HandlerName: handlerName,
		},
		Placement:    capability.ProviderPlacementDevice,
		SpaceID:      spaceID,
		DeviceID:     targetDeviceID,
		RuntimeID:    conn.RuntimeID,
		RemoteDevice: true,
	}
	request := capability.DeviceRuntimeInvocationRequest{
		Route:   route,
		Binding: route.Binding,
		Invocation: capability.ToolInvocationContext{
			InvocationID:     invocationID,
			SpaceID:          string(spaceID),
			DeadlineDuration: deadline,
		},
		Input: input,
	}
	result := port.Execute(ctx, request)
	if result.Status != capability.ToolResultStatusSuccess {
		if result.Error != nil {
			if rejected := coordination.ErrorFromProtocol(result.Error.Code); rejected != nil {
				if authorityCall != "" {
					confirmation, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					confirmErr := rt.Coordination.ConfirmRemoteAuthority(confirmation, authorityCall)
					cancel()
					if confirmErr != nil {
						return result, errors.Join(rejected, result.Error, confirmErr)
					}
				}
				return result, errors.Join(rejected, result.Error)
			}
			return result, result.Error
		}
		return result, fmt.Errorf("devicemesh: device invocation failed with status %s", result.Status)
	}
	if authorityCall != "" {
		confirmation, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := rt.Coordination.ConfirmRemoteAuthority(confirmation, authorityCall)
		cancel()
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

var _ = runtimeidentity.PlatformWindows

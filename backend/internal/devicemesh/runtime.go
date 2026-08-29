package devicemesh

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/bootstrap"
	"github.com/u-ai/backend/internal/devicemesh/credential"
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
	DB                 *sql.DB
	BootstrapSvc       *bootstrap.Service
	CredentialSvc      *credential.Service
	Hub                *server.ConnectionHub
	Handler            *server.Handler
	Probe              *server.ProbeService
	DeviceReg          *host_registry.Registry
	LocalHandler       *agent.LocalHandler
	PendingInvocations *capability.PendingInvocationManager
	PendingTasks       *task_runtime.PendingTaskManager
	sessions           *deviceruntime.Service
	dispatcher         dispatcherResolveAdapter
	taskRuntime        agent.TaskRuntimeExecutor
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
	if err := EnsureSchema(context.Background(), db); err != nil {
		return nil, err
	}

	bootstrapRepo := bootstrap.NewRepository(db)
	credRepo := credential.NewRepository(db)
	credSvc := credential.NewService(credRepo, DeviceCredentialTTL)

	exchangeFn := func(ctx context.Context, tx *sql.Tx, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID, now time.Time, credHash string, expires time.Time) (string, string, error) {
		credID := uuid.New().String()
		newCred := &credential.DeviceRuntimeCredential{
			ID:             credID,
			UserID:         userID,
			DeviceID:       deviceID,
			RuntimeID:      runtimeID,
			CredentialHash: credHash,
			Status:         credential.CredentialActive,
			CreatedAt:      now,
			ExpiresAt:      expires,
			LastUsedAt:     now,
			Revision:       1,
		}
		if err := credRepo.ExchangeAtomicTx(ctx, tx, userID, deviceID, runtimeID, now, newCred); err != nil {
			return "", "", err
		}
		rawCred := credential.HashRawCredential(credHash)
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
		Hub:           hub,
		Probe:         probe,
		DeviceReg:     deviceReg,
		sessions:      sessions,
	}

	rt.Handler = server.NewHandler(sessions, hub)

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
	if rt.Hub == nil {
		rt.Hub = server.NewConnectionHub()
	}
	if rt.Probe == nil && rt.Hub != nil {
		rt.Probe = server.NewProbeService(rt.Hub)
	}
	return nil
}

func (rt *Runtime) Stop() error {
	if rt.Hub != nil {
		rt.Hub.CloseAll()
	}
	return nil
}

func NewDeviceAgentRuntime(dataDir string, platform runtimeidentity.Platform, taskRuntime agent.TaskRuntimeExecutor, dispatcher dispatcherResolveAdapter) (*Runtime, error) {
	if dispatcher == nil {
		return nil, fmt.Errorf("devicemesh: device-agent runtime dispatcher is required")
	}
	if taskRuntime == nil {
		return nil, fmt.Errorf("devicemesh: device-agent task runtime is required")
	}
	localHandler := agent.NewLocalHandler(dataDir, platform)
	localHandler.SetDispatcher(dispatcher)
	localHandler.SetTaskRuntime(taskRuntime)

	rt := &Runtime{
		LocalHandler: localHandler,
		taskRuntime:  taskRuntime,
		dispatcher:   dispatcher,
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

	dispatcher := rt.dispatcher

	meshClient := agent.NewMeshClient(agent.MeshClientConfig{
		CloudBaseURL:      cred.CloudBaseUrl,
		Credential:        cred.Credential,
		UserID:            cred.UserID,
		Identity:          identity,
		Cursor:            cursor,
		RuntimeDispatcher: dispatcher,
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

// InvokeDeviceHandler sends a bounded management invocation to the active
// Device Agent for targetDeviceID and waits for the result. It is intended for
// cloud control-plane bridges such as Game Center; execution still occurs only
// on the device.
func (rt *Runtime) InvokeDeviceHandler(
	ctx context.Context,
	userID runtimeidentity.UserID,
	targetDeviceID runtimeidentity.DeviceID,
	handlerName string,
	input []byte,
	deadline time.Duration,
) (capability.UnifiedToolResult, error) {
	return rt.InvokeDeviceHandlerWithRuntimeType(ctx, userID, targetDeviceID, capability.RuntimeTypeGameHost, handlerName, input, deadline)
}

// InvokeDeviceHandlerWithRuntimeType is the generic cloud-to-device control
// plane primitive. It keeps GameHost compatibility while allowing internal
// subsystems such as Desktop Pet Behavior to use their own runtime identity.
func (rt *Runtime) InvokeDeviceHandlerWithRuntimeType(
	ctx context.Context,
	userID runtimeidentity.UserID,
	targetDeviceID runtimeidentity.DeviceID,
	runtimeType capability.RuntimeType,
	handlerName string,
	input []byte,
	deadline time.Duration,
) (capability.UnifiedToolResult, error) {
	if rt == nil || rt.Hub == nil || rt.PendingInvocations == nil {
		return capability.UnifiedToolResult{}, fmt.Errorf("devicemesh: invocation runtime unavailable")
	}
	conn, ok := rt.Hub.GetByDevice(userID, targetDeviceID)
	if !ok || conn == nil {
		return capability.UnifiedToolResult{}, fmt.Errorf("devicemesh: target device is offline")
	}
	if deadline <= 0 {
		deadline = 30 * time.Second
	}
	invocationID := uuid.NewString()
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
		UserID:       userID,
		DeviceID:     targetDeviceID,
		RuntimeID:    conn.RuntimeID,
		RemoteDevice: true,
	}
	request := capability.DeviceRuntimeInvocationRequest{
		Route:   route,
		Binding: route.Binding,
		Invocation: capability.ToolInvocationContext{
			InvocationID:     invocationID,
			UserID:           string(userID),
			DeadlineDuration: deadline,
		},
		Input: input,
	}
	result := port.Execute(ctx, request)
	if result.Status != capability.ToolResultStatusSuccess {
		if result.Error != nil {
			return result, result.Error
		}
		return result, fmt.Errorf("devicemesh: device invocation failed with status %s", result.Status)
	}
	return result, nil
}

var _ = runtimeidentity.PlatformWindows

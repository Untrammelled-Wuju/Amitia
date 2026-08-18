package devicemesh

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/deviceruntime"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/bootstrap"
	"github.com/u-ai/backend/internal/devicemesh/credential"
	"github.com/u-ai/backend/internal/devicemesh/server"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type dispatcherResolveAdapter interface {
	Resolve(handlerName string) agent.RuntimeInvokeHandler
}

type Runtime struct {
	DB            *sql.DB
	BootstrapSvc  *bootstrap.Service
	CredentialSvc *credential.Service
	Hub           *server.ConnectionHub
	Handler       *server.Handler
	Probe         *server.ProbeService
	DeviceReg     *host_registry.Registry
	LocalHandler  *agent.LocalHandler
	sessions      *deviceruntime.Service
	dispatcher    dispatcherResolveAdapter
	taskRuntime   agent.TaskRuntimeExecutor
}

func NewCloudRuntime(db *sql.DB, deviceReg *host_registry.Registry) (*Runtime, error) {
	hub := server.NewConnectionHub()
	return NewCloudRuntimeWithHub(db, deviceReg, hub)
}

func NewCloudRuntimeWithHub(db *sql.DB, deviceReg *host_registry.Registry, hub *server.ConnectionHub) (*Runtime, error) {
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

	sessionStore := deviceruntime.NewSQLiteSessionStore(db)
	if err := sessionStore.EnsureSchema(context.Background()); err != nil {
		return nil, err
	}
	sessions, err := deviceruntime.NewService(sessionStore, deviceruntime.ServiceOptions{})
	if err != nil {
		return nil, err
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

func (rt *Runtime) GetSessions() *deviceruntime.Service {
	return rt.sessions
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

func NewDeviceAgentRuntime(dataDir string, platform runtimeidentity.Platform) (*Runtime, error) {
	localHandler := agent.NewLocalHandler(dataDir, platform)

	rt := &Runtime{
		LocalHandler: localHandler,
	}

	// R21: Auto-recover credential on restart
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
	if dispatcher == nil {
		dispatcher = agent.NewRuntimeDispatcher()
	}

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

var _ = runtimeidentity.PlatformWindows

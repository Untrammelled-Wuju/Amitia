package devicemesh

import (
	"context"
	"database/sql"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/bootstrap"
	"github.com/u-ai/backend/internal/devicemesh/credential"
	"github.com/u-ai/backend/internal/devicemesh/server"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type Runtime struct {
	DB            *sql.DB
	BootstrapSvc  *bootstrap.Service
	CredentialSvc *credential.Service
	Hub           *server.ConnectionHub
	Handler       *server.Handler
	Probe         *server.ProbeService
	DeviceReg     *host_registry.Registry
	LocalHandler  *agent.LocalHandler
}

func NewCloudRuntime(db *sql.DB, deviceReg *host_registry.Registry) (*Runtime, error) {
	if err := EnsureSchema(context.Background(), db); err != nil {
		return nil, err
	}

	bootstrapRepo := bootstrap.NewRepository(db)
	bootstrapSvc := bootstrap.NewService(bootstrapRepo, BootstrapTicketTTL)

	credRepo := credential.NewRepository(db)
	credSvc := credential.NewService(credRepo, DeviceCredentialTTL)

	hub := server.NewConnectionHub()
	probe := server.NewProbeService(hub)

	return &Runtime{
		DB:            db,
		BootstrapSvc:  bootstrapSvc,
		CredentialSvc: credSvc,
		Hub:           hub,
		Probe:         probe,
		DeviceReg:     deviceReg,
	}, nil
}

func (rt *Runtime) SetSessions(sessions interface{}) {
}

func (rt *Runtime) Start() error {
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

// R21: autoRecoverCredential attempts to restore credential and auto-connect
func (rt *Runtime) autoRecoverCredential(handler *agent.LocalHandler) {
	cred, err := handler.LoadCredential()
	if err != nil || cred == nil {
		return
	}

	// Skip expired credentials
	if cred.ExpiresAt.Before(time.Now()) {
		return
	}

	identity, err := handler.LoadIdentity()
	if err != nil || identity == nil {
		return
	}

	cursor, _ := handler.LoadCursor()

	meshClient := agent.NewMeshClient(agent.MeshClientConfig{
		CloudBaseURL: cred.CloudBaseUrl,
		Credential:   cred.Credential,
		UserID:       cred.UserID,
		Identity:     identity,
		Cursor:       cursor,
	})
	handler.SetMeshClient(meshClient)
	meshClient.Start()
}

var _ = runtimeidentity.PlatformWindows

package devicemesh

import (
	"context"
	"database/sql"

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

	return &Runtime{
		LocalHandler: localHandler,
	}, nil
}

var _ = runtimeidentity.PlatformWindows

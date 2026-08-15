// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/config"

	"github.com/u-ai/backend/internal/belief"
	"github.com/u-ai/backend/internal/browser"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/chat"
	chatlocalmodel "github.com/u-ai/backend/internal/chat/localmodel"
	"github.com/u-ai/backend/internal/companion"
	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/behavior/adapters"
	"github.com/u-ai/backend/internal/desktoppet/behavior/events"
	"github.com/u-ai/backend/internal/desktoppet/behavior/wiring"
	"github.com/u-ai/backend/internal/desktoppet/device"
	"github.com/u-ai/backend/internal/desktoppet/editing"
	"github.com/u-ai/backend/internal/devicemesh"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/desktoppet/editing/baseline"
	"github.com/u-ai/backend/internal/desktoppet/editing/revisioncommit"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
	"github.com/u-ai/backend/internal/desktoppet/integration"
	"github.com/u-ai/backend/internal/desktoppet/maintenance"
	"github.com/u-ai/backend/internal/desktoppet/migration"
	migrationplans "github.com/u-ai/backend/internal/desktoppet/migration/plans"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/desktoppet/processing/application"
	processingcommit "github.com/u-ai/backend/internal/desktoppet/processing/commit"
	processingevents "github.com/u-ai/backend/internal/desktoppet/processing/events"
	processingworker "github.com/u-ai/backend/internal/desktoppet/processing/worker"
	processingworkspace "github.com/u-ai/backend/internal/desktoppet/processing/workspace"
	"github.com/u-ai/backend/internal/desktoppet/quality"
	"github.com/u-ai/backend/internal/desktoppet/quality/detectors"
	qualitygate "github.com/u-ai/backend/internal/desktoppet/quality/gate"
	qualityinput "github.com/u-ai/backend/internal/desktoppet/quality/input"
	qualitymeasurement "github.com/u-ai/backend/internal/desktoppet/quality/measurement"
	qualityrecovery "github.com/u-ai/backend/internal/desktoppet/quality/recovery"
	qualityworker "github.com/u-ai/backend/internal/desktoppet/quality/worker"
	qualitywriteback "github.com/u-ai/backend/internal/desktoppet/quality/writeback"
	"github.com/u-ai/backend/internal/desktoppet/readiness"
	"github.com/u-ai/backend/internal/desktoppet/release"
	releasebuild "github.com/u-ai/backend/internal/desktoppet/release/build"
	"github.com/u-ai/backend/internal/desktoppet/release/importer"
	releaserepo "github.com/u-ai/backend/internal/desktoppet/release/repository"
	releasestorage "github.com/u-ai/backend/internal/desktoppet/release/storage"
	releaseworker "github.com/u-ai/backend/internal/desktoppet/release/worker"
	"github.com/u-ai/backend/internal/desktoppet/runtime"
	runtimev2 "github.com/u-ai/backend/internal/desktoppet/runtime/protocol/v2"
	desktoppetsecurity "github.com/u-ai/backend/internal/desktoppet/security"
	"github.com/u-ai/backend/internal/desktoppet/worker"
	"github.com/u-ai/backend/internal/emote"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	extensionmcp "github.com/u-ai/backend/internal/extension/kernel/mcp"
	"github.com/u-ai/backend/internal/extension/kernel/script_host"
	"github.com/u-ai/backend/internal/extension/kernel/skill"
	"github.com/u-ai/backend/internal/gamehost/management"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/imagegen"
	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval/local"
	"github.com/u-ai/backend/internal/interaction"
	iosnative_background "github.com/u-ai/backend/internal/iosnative/background"
	"github.com/u-ai/backend/internal/localmodel/llamacpp"
	"github.com/u-ai/backend/internal/nativebridge"
	"github.com/u-ai/backend/internal/mcp"
	"github.com/u-ai/backend/internal/media"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/middleware/security"
	migrationcore "github.com/u-ai/backend/internal/migration"
	"github.com/u-ai/backend/internal/mindruntime"
	newoutbox "github.com/u-ai/backend/internal/outbox"
	"github.com/u-ai/backend/internal/personality"
	"github.com/u-ai/backend/internal/pipelinecheckpoint"
	"github.com/u-ai/backend/internal/profile"
	"github.com/u-ai/backend/internal/psyche"
	"github.com/u-ai/backend/internal/psyche/appraisal"
	"github.com/u-ai/backend/internal/psyche/budget"
	"github.com/u-ai/backend/internal/qdrant"
	"github.com/u-ai/backend/internal/queue"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/internal/safety"
	"github.com/u-ai/backend/internal/scriptruntime/commandenv"
	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/system/dataportability"
	"github.com/u-ai/backend/internal/temporal"
	"github.com/u-ai/backend/internal/vision"
	"github.com/u-ai/backend/internal/workspace"
	"github.com/u-ai/backend/internal/worldbook"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/app"
	"github.com/u-ai/backend/pkg/resourceuri"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

type AppServices struct {
	DB                           *gorm.DB
	RuntimeProfile               runtimeprofile.Profile
	RuntimePolicy                runtimeprofile.Policy
	DataPortability              *dataportability.Coordinator
	DeliveryStore                *delivery.SQLiteDeliveryStore
	ChatDeliveryAdapter          chat.DeliveryStore
	DeliveryWorker               *delivery.Worker
	Browser                      browser.BrowserProvider
	Graph                        graph.Service
	Memory                       memory.Service
	Profile                      profile.Service
	Episodic                     episodic.Service
	WorldBook                    worldbook.Service
	Vision                       vision.Service
	Companion                    companion.Service
	Chat                         chat.Service
	UnifiedEntry                 *interaction.UnifiedEntry
	DataLifecycle                *mindruntime.DataLifecycleCoordinator
	RuntimeQueue                 *queue.SQLiteRuntimeQueueStore
	NewOutbox                    *newoutbox.SQLiteOutboxStore
	OutboxWorker                 *newoutbox.Worker
	DesktopPetWorker             *worker.Worker
	ProcessingWorker             *processingworker.Worker
	QualityService               quality.QualityService
	QualityWorker                *qualityworker.Worker
	InstallationCoordinator      coordinator.InstallationCoordinator
	InstallationRepo             installation.Repository
	NewReleaseService            release.ReleaseService
	ReleaseRecoveryWorker        *release.ReleaseRecoveryWorker
	ReleaseBuildWorker           *releaseworker.ReleaseBuildWorker
	ReleaseEventPublisher        *release.ReleaseEventPublisher
	DesktopPetRuntimeV2          *runtimev2.RuntimeFacade
	EditingService               editing.Service
	RegenerationWorker           *editing.RegenerationWorker
	BridgeRecoveryWorker         *revisioncommit.RecoveryWorker
	BehaviorService              *behavior.BehaviorService
	AdapterManager               *adapters.AdapterManager
	Reconciliation               *mindruntime.ReconciliationEngine
	CircuitBreakers              *mindruntime.CircuitBreakerRegistry
	VoiceEntry                   *interaction.VoiceEntry
	Extension                    *extension.Runtime
	KernelContainer              *kernel.Container
	Emote                        *emote.Service
	Temporal                     *temporal.Service
	RelTimeCoordinator           *temporal.RelationshipTimeCoordinator
	OwnershipGuard               desktoppetsecurity.OwnershipGuard
	PathRegistry                 *desktoppetsecurity.PathRootRegistry
	ImportStagingRepo            desktoppetsecurity.ImportStagingRepository
	PackageImporter              *importer.PackageImporter
	Readiness                    *readiness.ReadinessService
	SafeMode                     *readiness.SafeModeController
	CanonicalStdioFactory        *extensionmcp.CanonicalStdioFactory
	CanonicalStdioRegistry       *extensionmcp.CanonicalStdioRegistry
	CanonicalStdioCaller         *CanonicalStdioCaller
	DesktopInstanceStore         *security.DesktopInstanceStore
	DeviceRepository             *device.Repository
	RuntimeOrchestrator          RuntimeOrchestrator
	RuntimeDomainEventConsumer   *runtimev2.OutboxConsumer
	DesktopPetMigrationRunner    *migration.Runner
	DesktopPetMaintenanceHandler *maintenance.Handler
	MigrationLock                *migrationcore.PersistentLock
	RecoveryDescriptor           *interaction.RecoveryDescriptorService
	PauseResumeService           *interaction.PauseResumeService
	BackgroundTaskCoordinator    *interaction.BackgroundTaskCoordinator
	MultiAgentCoordinator        *interaction.MultiAgentCoordinator
	GameCenterService            *management.GameCenterManagementService
	MediaService                 *media.Service
	WorkspaceRegistry            *workspace.Registry
	WorkspaceService             *workspace.Service
	ProductionCutover            *cutoverComposition
	ClosureGate                 *Stage2ClosureGate
	NativeBridgeRelay            *nativeBridgeRelay
	BackgroundTaskRuntimeWired  bool
	Artifact                    *ArtifactRuntime
	DeviceMesh                 *devicemesh.Runtime
}

type RuntimeOrchestrator interface {
	StartPhase(ctx context.Context, phase runtimeorchestrator.ComponentPhase) error
	StopAll(ctx context.Context) error
	Snapshot() runtimeorchestrator.RuntimeSnapshot
	GraphService() graph.Service
	SetGraphService(svc graph.Service)
}

type defaultCharacterProvider struct {
	repo character.Repository
}

func (p *defaultCharacterProvider) GetDefaultCharacterID(ctx context.Context) (string, error) {
	profile, err := p.repo.GetRuntimeProfile("")
	if err != nil {
		return "", err
	}
	return profile.CharacterID, nil
}

type reflectionMemoryServiceAdapter struct {
	memory memory.Service
}

func (a reflectionMemoryServiceAdapter) SubmitReflectionCandidate(req interaction.ReflectionCandidateSubmitRequest) error {
	memoryKey := fmt.Sprintf("reflection:%s:%s", strings.TrimSpace(req.CandidateID), req.Topic)
	memoryType := strings.TrimSpace(req.SuggestedMemoryType)
	if _, ok := memory.NormalizeMemoryType(memoryType); !ok {
		memoryType = string(memory.MemoryTypeFact)
	}
	_, err := a.memory.SubmitCandidate(&memory.SubmitCandidateRequest{
		Key:            memoryKey,
		Value:          req.Abstract,
		MemoryType:     memoryType,
		Importance:     req.Importance,
		SourceText:     req.Abstract,
		ConversationID: req.ConversationID,
		CharacterID:    req.CharacterID,
		CandidateKind:  string(memory.CandidateKindReflection),
	})
	return err
}

func NewAppServices(ctx *app.AppContext, graphSvc graph.Service, bootstrap *runtimeBootstrap, runtimeProfile runtimeprofile.Profile, policy runtimeprofile.Policy) (*AppServices, error) {
	if config.AppCfg == nil {
		config.AppCfg = &config.Config{}
	}
	if runtimeProfile == runtimeprofile.ProfileDeviceAgent {
		return newDeviceAgentServices(ctx, graphSvc, bootstrap, runtimeProfile, policy)
	}
	temporalSvc := temporal.NewService(temporal.NewRepository(ctx.DB), temporal.SystemClock{})
	relTimeRepo := temporal.NewRelationshipTimeRepository(ctx.DB, temporal.SystemClock{})
	relTimeCoordinator := temporal.NewRelationshipTimeCoordinator(relTimeRepo, temporal.SystemClock{})
	temporalSvc.SetRelationshipTimeProvider(relTimeCoordinator)
	memRepo := memory.NewRepository(ctx)
	memSvc := memory.NewService(memRepo, ctx, graphSvc)
	memory.SetTemporalRepo(memSvc, temporal.NewRepository(ctx.DB))
	profRepo := profile.NewRepository(ctx)
	profSvc := profile.NewService(profRepo, ctx, graphSvc)
	epiRepo := episodic.NewRepository(ctx)
	epiSvc := episodic.NewService(epiRepo, ctx, graphSvc)
	wbRepo := worldbook.NewRepository(ctx)
	wbSvc := worldbook.NewService(wbRepo, ctx, graphSvc)
	visionRepo := vision.NewRepository(ctx.DB)
	visionSvc := vision.NewService(visionRepo)
	compSvc := companion.NewService(ctx)
	compSvc.AttachTemporalResolver(temporalSvc)
	if temporalSvc.FeatureFlags().RelationshipTimeEnabled {
		compSvc.AttachAssistantContactRecorder(relTimeCoordinator)
	}
	compressor := chat.NewCompressor(ctx.DB)
	psycheStore := psyche.NewSQLitePsycheStore(ctx.DB)
	if err := psycheStore.InitSchema(); err != nil {
		log.Error("failed to init psyche store schema:", err)
		panic("failed to init psyche store schema")
	}
	if err := ctx.DB.AutoMigrate(&chat.RelationshipStateRecord{}, &chat.NeedStateRecord{}); err != nil {
		log.Error("failed to init relationship/need store schema:", err)
		panic("failed to init relationship/need store schema")
	}
	chatSvc := chat.NewService(chat.NewRepository(ctx), ctx, memSvc, profSvc, epiSvc, wbSvc, compressor, visionSvc, graphSvc, psycheStore)
	extensionRuntime, err := extension.NewRuntimeWithOptions(context.Background(), ctx.DB, "1.0.0", extension.RuntimeOptions{SkipPluginManagerStart: true})
	if err != nil {
		log.Error("failed to initialize skill runtime:", err)
		panic("failed to initialize skill runtime")
	}
	kernelRoot := filepath.Join(os.TempDir(), "amitia-extension-kernel")
	if config.AppCfg != nil && config.AppCfg.Storage.DataDir != "" {
		kernelRoot = filepath.Join(config.AppCfg.Storage.DataDir, "extensions-v2")
	}
	if err := extensionRuntime.AttachKernel(kernelRoot); err != nil {
		log.Error("failed to initialize extension kernel:", err)
		panic("failed to initialize extension kernel")
	}
	kernelDBPath := filepath.Join(kernelRoot, "kernel.db")
	charRepo := character.NewRepository(ctx)
	kernelCharReader := newKernelCharacterReader(charRepo)
	kernelConvReader := newKernelConversationReader(chatSvc)
	kernelMemQuerySvc := newKernelMemoryQueryService(memSvc)
	var nodeResolver script_host.NodeEnvironmentResolver
	var artifactResolver script_host.ArtifactResolver
	var commandResolver commandenv.Resolver
	if bootstrap != nil {
		nodeResolver = bootstrap.NodeEnvironmentResolver()
		host := bootstrap.RuntimeHost()
		if host != nil {
			chatlocalmodel.SetGlobalRuntimeHost(host)
			llamacpp.SetGlobalEmbeddingHost(host)
			var resolverErr error
			artifactResolver, resolverErr = script_host.NewArtifactResolver(script_host.ResolveContext{
				Host: host,
			})
			if resolverErr != nil {
				log.Error("failed to create artifact resolver:", resolverErr)
			}
			commandResolver, resolverErr = commandenv.NewResolver(commandenv.ResolveContext{
				NodeResolver: bootstrap.NodeEnvironmentResolver(),
			})
			if resolverErr != nil {
				log.Error("failed to create command resolver:", resolverErr)
			}
		}
	}

	imagegenRepo := imagegen.NewRepository(ctx.DB)
	imagegenSvc := imagegen.NewService(imagegenRepo)
	imageProviderRegistry := desktoppet.NewProviderRegistry()
	var resourceResolver *resourceuri.PhysicalResolver
	if config.AppCfg != nil && config.AppCfg.Storage.DataDir != "" {
		paths := util.DetectRuntimePaths(config.AppCfg.Storage.DataDir)
		resolver, resolverErr := resourceuri.NewPhysicalResolver(resourceuri.PhysicalRootsFromRuntimePaths(paths))
		if resolverErr != nil {
			log.Warn("failed to create resource resolver for image intelligence:", resolverErr)
		} else {
			resourceResolver = resolver
		}
	}

	var workspaceRegistry *workspace.Registry
	var workspaceService *workspace.Service
	if config.AppCfg != nil && config.AppCfg.Storage.DataDir != "" {
		var safBridge workspace.SAFBridge
		if bootstrap != nil {
			safBridge = newWorkspaceSAFBridge(bootstrap.AndroidNativeBridge())
		} else {
			safBridge = newWorkspaceSAFBridge(nil)
		}
		var remoteCredResolver workspace.RemoteCredentialResolver
		resolver, credErr := newWorkspaceRemoteCredentialResolver(config.AppCfg.Storage.DataDir)
		if credErr != nil {
			log.Warn("workspace remote credential resolver unavailable:", credErr)
			remoteCredResolver = &unavailableWorkspaceCredentialResolver{}
		} else {
			remoteCredResolver = resolver
		}
		workspaceRegistry, workspaceService, err = buildWorkspaceServices(config.AppCfg.Storage.DataDir, resourceResolver, ctx.DB, safBridge, remoteCredResolver)
		if err != nil {
			return nil, fmt.Errorf("initialize workspace services: %w", err)
		}
	}

	var mediaService *media.Service
	if bootstrap != nil {
		mediaService = buildMediaService(bootstrap.RuntimeHost(), config.AppCfg.Storage.DataDir, resourceResolver, workspaceService)
		if mediaService != nil {
			chatlocalmodel.SetGlobalMediaMaterializer(mediaService.Materializer())
		}
	}

	var browserProvider browser.BrowserProvider
	if config.AppCfg != nil && config.AppCfg.Providers.Browser.Enabled {
		browserProvider, err = buildProductionBrowserProvider(config.AppCfg, bootstrap)
		if err != nil {
			return nil, fmt.Errorf("browser provider init failed: %w", err)
		}
	} else {
		browserProvider = browser.NewDisabledProvider()
	}

	kernelBuilder := kernel.NewContainerBuilder().
		WithDBPath(kernelDBPath).
		WithExtensionRoot(kernelRoot).
		WithCharacterReader(kernelCharReader).
		WithConversationReader(kernelConvReader).
		WithMemoryQueryService(kernelMemQuerySvc).
		WithNodeEnvironmentResolver(nodeResolver).
		WithHostArtifactResolver(artifactResolver).
		WithSearchConfig(search.DefaultConfig()).
		WithDeepSearchTaskEntry("tasks/deep-search/index.js").
		WithVisionService(visionSvc).
		WithImageGenService(imagegenSvc).
		WithImageProviderRegistry(imageProviderRegistry).
		WithResourceResolver(resourceResolver).
		WithMediaService(mediaService).
		WithWorkspaceService(workspaceService).
		WithBrowserProvider(browserProvider).
		WithRuntimeProfile(runtimeProfile)

	installationRepo := installation.NewRepository(ctx.DB, ctx)
	runtimeConfig := runtime.DefaultRuntimeConfig()
	runtimeConfig.Enabled = config.AppCfg.DesktopPetRuntime.Enabled
	runtimeConfig.LoopbackOnly = config.AppCfg.DesktopPetRuntime.LoopbackOnly
	runtimeConfig.HeartbeatIntervalMs = config.AppCfg.DesktopPetRuntime.HeartbeatIntervalMs
	runtimeConfig.HeartbeatTimeoutMs = config.AppCfg.DesktopPetRuntime.HeartbeatTimeoutMs
	runtimeConfig.MaxMessageBytes = config.AppCfg.DesktopPetRuntime.MaxMessageBytes
	runtimeConfig.RegisterTimeoutSec = config.AppCfg.DesktopPetRuntime.RegisterTimeoutSec
	runtimeConfig.SendQueueSize = config.AppCfg.DesktopPetRuntime.SendQueueSize
	runtimeConfig.CommandTimeoutSec = config.AppCfg.DesktopPetRuntime.CommandTimeoutSec
	runtimeConfig.MaxRetryAttempts = config.AppCfg.DesktopPetRuntime.MaxRetryAttempts
	runtimeConfig.RetryBaseDelayMs = config.AppCfg.DesktopPetRuntime.RetryBaseDelayMs
	runtimeConfig.RetryMaxDelayMs = config.AppCfg.DesktopPetRuntime.RetryMaxDelayMs
	runtimeConfig.CommandRetentionHours = config.AppCfg.DesktopPetRuntime.CommandRetentionHours

	runtimeV2Facade := runtimev2.NewRuntimeFacade(ctx.DB, &runtimev2.FacadeConfig{
		Enabled:            runtimeConfig.Enabled,
		Path:               runtimeConfig.Path,
		LoopbackOnly:       runtimeConfig.LoopbackOnly,
		HeartbeatInterval:  time.Duration(runtimeConfig.HeartbeatIntervalMs) * time.Millisecond,
		HeartbeatTimeout:   time.Duration(runtimeConfig.HeartbeatTimeoutMs) * time.Millisecond,
		MaxMessageBytes:    int64(runtimeConfig.MaxMessageBytes),
		CommandTimeoutSec:  int64(runtimeConfig.CommandTimeoutSec),
		CommandRetentionHr: int64(runtimeConfig.CommandRetentionHours),
	})

	processingDataDir := mcpDataDirectory(ctx)

	releaseRepo := releaserepo.NewSQLiteRepository(ctx.DB)
	releaseStoragePort := releasestorage.NewFileSystemStorage(processingDataDir)
	gateReader := qualitygate.NewQualityGateReader(ctx.DB)
	releaseEventPublisher := release.NewReleaseEventPublisher(releaseRepo)
	newReleaseService := release.NewReleaseService(releaseRepo, gateReader, releaseStoragePort, releaseEventPublisher)

	kernelBuilder.WithDesktopPetPluginCapabilities(integration.NewProductionCapabilities(integration.ProductionCapabilitiesOptions{
		InstallationRepo: installationRepo,
		ReleaseService:   newReleaseService,
		RuntimeFacade:    runtimeV2Facade,
	}))

	if bootstrap != nil {
		kernelBuilder.WithRuntimeHost(bootstrap.RuntimeHost())
		kernelBuilder = applyAndroidLinuxProvider(kernelBuilder, bootstrap.RuntimeHost())

		var err error
		kernelBuilder, err = applyAndroidNativeProvider(kernelBuilder, bootstrap)
		if err != nil {
			return nil, fmt.Errorf("android native provider init failed: %w", err)
		}

		kernelBuilder = applyIOSNativeProvider(kernelBuilder, bootstrap.IOSNativeProvider())
	}

	kernelContainer, err := kernelBuilder.Build(context.Background())
	if err != nil {
		log.Error("failed to initialize kernel container:", err)
		panic("failed to initialize kernel container")
	}
	extensionRuntime.Kernel.SetContainer(kernelContainer)
	if err := extensionRuntime.Kernel.RecoverPackageOperations(context.Background()); err != nil {
		log.Error("package operation recovery failed: ", err)
		panic("failed to recover package operations")
	}
	if err := kernelContainer.Recover(context.Background()); err != nil {
		log.Warn("kernel recovery warning: ", err)
	}
	if kernelContainer.GameHost != nil {
		if err := kernelContainer.GameHost.Start(context.Background()); err != nil {
			log.Error("gamehost start failed: ", err)
			panic("failed to start gamehost")
		}
	}
	if kernelContainer.UpdateRecoveryManager != nil {
		recoveryActions, err := kernelContainer.UpdateRecoveryManager.ScanOnStartup(context.Background())
		if err != nil {
			log.Warn("update recovery scan warning: ", err)
		}
		for _, action := range recoveryActions {
			if err := kernelContainer.UpdateRecoveryManager.ExecuteRecovery(context.Background(), action); err != nil {
				log.Warn(fmt.Sprintf("update recovery execute failed for %s: %v", action.OperationID, err))
			} else {
				log.Info(fmt.Sprintf("update recovery completed for %s (strategy: %s)", action.OperationID, action.Strategy))
			}
		}
	}
	if err := extensionRuntime.AttachKernelFacade(extensionRuntime.Kernel); err != nil {
		log.Error("extension kernel facade attach failed: ", err)
		panic("failed to attach extension kernel facade")
	}
	artifactMaintenance, err := kernel.NewPackageArtifactMaintenanceForStore(kernelContainer.PackageRepository, kernelContainer.PackageArtifactStore, kernel.DefaultPackageArtifactMaintenanceConfig())
	if err != nil {
		log.Error("failed to initialize package artifact maintenance: ", err)
		panic("failed to initialize package artifact maintenance")
	}
	kernelContainer.ArtifactMaintenance = artifactMaintenance
	toolFacade := kernelContainer.ToolFacade
	if toolFacade == nil {
		panic("kernel container did not construct ToolFacade")
	}
	if extensionRuntime.AgentSkills != nil {
		toolFacade.SetAgentSkillBackend(extension.NewAgentSkillKernelAdapter(extensionRuntime.AgentSkills))
	}
	if extensionRuntime.AgentSkills != nil && bootstrap != nil {
		if host := bootstrap.RuntimeHost(); host != nil {
			nodeEnvResolver := bootstrap.NodeEnvironmentResolver()
			interpResolver, interpErr := skill.NewScriptInterpreterResolver(skill.InterpreterResolveContext{
				NodeResolver:    nodeEnvResolver,
				CommandResolver: commandResolver,
			})
			if interpErr == nil {
				scriptRuntime := skill.NewScriptRuntime(skill.ScriptRuntimeDeps{
					InterpreterResolver: interpResolver,
					ProcessSupervisor:   host.Processes(),
				})
				toolFacade.SetRunSkillScriptHandler(extension.NewSkillScriptHandler(extensionRuntime.AgentSkills, scriptRuntime))
			} else {
				log.Warn("failed to create skill script interpreter resolver: ", interpErr)
			}
		}
	}
	if extensionRuntime.AgentSkills != nil {
		baseURL := "http://" + config.AppCfg.Server.Addr()
		toolFacade.SetSkillResourceHandler(extension.NewSkillResourceAdapter(extensionRuntime.AgentSkills, baseURL))
	}
	chatSvc.SetToolRuntime(newChatToolRuntimeAdapter(toolFacade))
	actionMaterializer := interaction.NewActionMaterializer(toolFacade)
	actionDispatcher := interaction.NewActionDispatcher(toolFacade)
	observationBuilder := interaction.NewObservationBuilder()
	chatSvc.SetActionMaterializer(&actionMaterializer)
	chatSvc.SetActionDispatcher(actionDispatcher)
	chatSvc.SetObservationBuilder(observationBuilder)
	goalRegistry := decision.NewGoalRegistry()
	goalProgressService := interaction.NewGoalProgressService(goalRegistry)
	continuationService := interaction.NewContinuationService(goalRegistry)
	chatSvc.SetGoalProgressService(goalProgressService)
	chatSvc.SetContinuationService(continuationService)
	if kernelContainer.DevConsoleService != nil {
		kernelContainer.DevConsoleService.SetToolFacadeProvider(toolFacade.Counters())
		kernelContainer.DevConsoleService.SetLegacyCallProvider(kernel.GlobalLegacyCallCounter())
	}
	if kernelContainer.HookService != nil {
		toolFacade.SetHookService(kernelContainer.HookService)
		chatSvc.SetHookInvoker(chat.NewHookAdapter(kernelContainer.HookService))
	}
	extensionRuntime.Workshop.SetModelGenerator(chatSvc)
	orchCfg := interaction.DefaultOrchestratorConfig()
	tracker := interaction.NewSQLiteInteractionTracker(ctx.DB)
	if err := tracker.InitSchema(); err != nil {
		log.Error("failed to init interaction tracker schema:", err)
		panic("failed to init interaction tracker schema")
	}
	newOutboxStore := newoutbox.NewSQLiteOutboxStore(ctx.DB, newoutbox.DefaultOutboxStoreConfig())
	if err := ctx.DB.AutoMigrate(&newoutbox.OutboxRecordModel{}, &newoutbox.DeadLetterRecordModel{}); err != nil {
		log.Error("failed to init outbox store schema:", err)
		panic("failed to init outbox store schema")
	}
	runtimeQueue := queue.NewSQLiteRuntimeQueueStore(ctx.DB)
	deliveryStore := delivery.NewSQLiteDeliveryStore(ctx.DB)
	channelResolver := delivery.NewMapChannelResolverWith([]delivery.ChannelAdapter{
		delivery.NewWebChannelAdapter(),
		delivery.NewQQChannelAdapter("http://127.0.0.1:19877"),
		delivery.NewWechatChannelAdapter("http://127.0.0.1:19876"),
	})
	deliveryWorker := delivery.NewWorker(deliveryStore, channelResolver, delivery.DefaultWorkerConfig())
	deliveryAdapter := &chatDeliveryAdapter{store: deliveryStore}
	chatSvc.SetDeliveryStore(deliveryAdapter)
	emoteSvc := emote.NewService(ctx.DB, deliveryStore)
	emoteDecision := emote.NewDecisionService(emoteSvc)
	chat.RegisterMessagePlanningHook(emoteDecision.Plan)

	outboxAdapter := &chatOutboxAdapter{store: newOutboxStore}
	chatSvc.SetOutboxStore(outboxAdapter)
	if err := runtimeQueue.InitSchema(); err != nil {
		log.Error("failed to init runtime queue schema:", err)
		panic("failed to init runtime queue schema")
	}

	dispatchedPublisher := newoutbox.NewDispatchedPublisher(newoutbox.LogOnlyPublisher())
	postProcessAdapter := &postProcessPublisherAdapter{chatSvc: chatSvc}
	dispatchedPublisher.Register("postprocess.pipeline.execute", postProcessAdapter)
	dispatchedPublisher.Register("postprocess.context.trim", postProcessAdapter)
	dispatchedPublisher.Register("postprocess.mood.recovery", postProcessAdapter)
	dispatchedPublisher.Register("postprocess.compressor.maybe", postProcessAdapter)

	newOutboxWorker := newoutbox.NewWorker(newOutboxStore, dispatchedPublisher, newoutbox.DefaultWorkerConfig())

	interactionPublisher := &noopPublisher{}
	dispatchedPublisher.Register("interaction.completed", interactionPublisher)
	dispatchedPublisher.Register("interaction.state_changed", interactionPublisher)
	dispatchedPublisher.Register("interaction.runtime_assembled", interactionPublisher)

	reflectionMemoryAdapter := &reflectionMemoryServiceAdapter{memory: memSvc}
	reflectionPublisher := interaction.NewReflectionMemoryPublisher(reflectionMemoryAdapter, nil)
	dispatchedPublisher.Register(interaction.ReflectionCandidateApprovedEventType, reflectionPublisher)
	dispatchedPublisher.Register(interaction.ReflectionMemoryAbstractionEventType, reflectionPublisher)
	orch := interaction.NewOrchestratorWithStores(orchCfg, chatSvc.(interaction.MessageProcessor), tracker, newOutboxStore)
	if temporalSvc.FeatureFlags().RelationshipTimeEnabled {
		orch.SetRelationshipTimeCoordinator(relTimeCoordinator)
		chatSvc.SetRelationshipTimeCoordinator(relTimeCoordinator)
	}
	runtimeRegistry := newRuntimeContextLoaderRegistry(ctx, charRepo, temporalSvc)
	runtimePipeline := interaction.NewRuntimePipeline(runtimeRegistry, interaction.NewPathClassifier(), interaction.NewTokenBudgetManager(2400))
	runtimePipeline.SetPersonalityCompiler(personality.NewCompiler(personality.DefaultCompilerConfig()))
	runtimePipeline.SetSafetyGovernor(safety.NewGovernor(safety.DefaultGovernorConfig()))
	runtimePipeline.SetBeliefResolver(belief.ResolveBelief)
	runtimePipeline.SetAppraisalEngine(appraisal.NewEngine(appraisal.DefaultAppraisalConfig()))
	runtimePipeline.SetBudgetController(budget.NewBudgetController(0.5))
	runtimePipeline.SetGoalRegistry(goalRegistry)
	runtimePipeline.SetDecisionLayer(decision.DefaultCandidateRegistry(), decision.DefaultArbitrationLayer())
	orch.SetRuntimePipeline(runtimePipeline)
	chatSvc.SetReplanner(runtimePipeline)

	reflectionOutboxAdapter := &reflectionOutboxServiceAdapter{store: newOutboxStore}
	reflectionService := interaction.NewReflectionService(
		interaction.WithReflectionTriggerConfig(mindruntime.DefaultReflectionTriggerConfig()),
		interaction.WithReflectionRunConfig(mindruntime.DefaultReflectionRunConfig()),
		interaction.WithReflectionApprovalConfig(mindruntime.DefaultReflectionApprovalConfig()),
		interaction.WithReflectionSupervisorConfig(mindruntime.DefaultSupervisorConfig()),
		interaction.WithReflectionOutbox(reflectionOutboxAdapter),
	)
	chatSvc.SetReflectionProcessor(reflectionService)

	deadlineCfg := mindruntime.DefaultDeadlineConfig
	deadlineCfg.TotalTimeout = 180 * time.Second
	deadlineCfg.GenerationTimeout = 120 * time.Second
	dp := mindruntime.NewDeadlinePropagator(deadlineCfg)
	orch.SetDeadlineProvider(func(ctx context.Context, requestID string) (context.Context, context.CancelFunc) {
		return dp.ContextWithDeadline(ctx, requestID, mindruntime.DeadlineStageGeneration)
	})
	resolver := interaction.NewScopeResolverWithDefaultChar(interaction.NewConversationScopeBindingLookup(ctx.DB), &defaultCharacterProvider{repo: charRepo})
	dataLifecycle := mindruntime.NewDataLifecycleCoordinator(ctx.DB)
	if err := dataLifecycle.InitSchema(); err != nil {
		log.Error("failed to init data lifecycle schema:", err)
		panic("failed to init data lifecycle schema")
	}
	dataLifecycle.SetOutboxCleanupExecutor(mindruntime.NewDefaultOutboxCleanupExecutor(ctx.DB, graphSvc))
	if coordinatorSetter, ok := interface{}(memSvc).(interface {
		SetDataLifecycleCoordinator(*mindruntime.DataLifecycleCoordinator)
	}); ok {
		coordinatorSetter.SetDataLifecycleCoordinator(dataLifecycle)
	}
	if coordinatorSetter, ok := interface{}(profSvc).(interface {
		SetDataLifecycleCoordinator(*mindruntime.DataLifecycleCoordinator)
	}); ok {
		coordinatorSetter.SetDataLifecycleCoordinator(dataLifecycle)
	}
	if coordinatorSetter, ok := interface{}(epiSvc).(interface {
		SetDataLifecycleCoordinator(*mindruntime.DataLifecycleCoordinator)
	}); ok {
		coordinatorSetter.SetDataLifecycleCoordinator(dataLifecycle)
	}
	chatSvc.EnsureChannelConversation("wechat")
	chatSvc.EnsureChannelConversation("qq")

	entry := interaction.NewUnifiedEntry(orch, resolver, temporal.SystemClock{})
	compSvc.AttachUnifiedEntry(entry)
	compSvc.AttachDeliveryStore(deliveryStore)
	if coordinatorSetter, ok := interface{}(compSvc).(interface {
		SetDataLifecycleCoordinator(*mindruntime.DataLifecycleCoordinator)
	}); ok {
		coordinatorSetter.SetDataLifecycleCoordinator(dataLifecycle)
	}
	reconciliationEngine := mindruntime.NewReconciliationEngine(mindruntime.DefaultReconciliationConfig())
	graphReconAdapter := &graphReconciliationAdapter{graphSvc: graphSvc}
	qdrantReconAdapter := &qdrantReconciliationAdapter{qdrantClient: qdrant.NewQdrantClient()}
	if err := mindruntime.RegisterRuntimeReconciliationCheckers(reconciliationEngine, ctx.DB, graphReconAdapter, qdrantReconAdapter); err != nil {
		log.Warn("reconciliation checkers registration warning: ", err)
	}
	pipelineMgr := pipelinecheckpoint.New(ctx.DB)
	goalReader := interaction.NewGoalRecoveryReaderAdapter(goalRegistry)
	taskReader := interaction.NewTaskRecoveryReaderAdapter(kernelContainer.TaskRuntimeService)
	wfReader := interaction.NewWorkflowRecoveryReaderAdapter(kernelContainer.WorkflowExecutor)
	invReader := interaction.NewInvocationRecoveryReaderAdapter(kernelContainer.ObservabilityStore)
	pipeReader := interaction.NewPipelineRecoveryReaderAdapter(pipelineMgr)
	recoveryValidator := interaction.NewRecoveryDescriptorValidator(goalReader, taskReader, wfReader, invReader, pipeReader)
	recoveryBuilder := interaction.NewRecoveryDescriptorBuilder(goalReader, taskReader, wfReader, invReader, pipeReader)
	recoveryDescriptor := interaction.NewRecoveryDescriptorService(tracker, recoveryBuilder, recoveryValidator)
	pauseResumeService := interaction.NewPauseResumeService(tracker)
	backgroundTaskCoordinator := interaction.NewBackgroundTaskCoordinator(tracker, recoveryDescriptor, kernelContainer.TaskRuntimeService)
	_ = backgroundTaskCoordinator
	_ = pauseResumeService
	multiAgentCoordinator := interaction.NewMultiAgentCoordinator(tracker, goalRegistry, recoveryDescriptor, interaction.NewUnifiedEntryWorkerRunner(entry), pauseResumeService, interaction.DefaultMultiAgentPolicy())
	_ = multiAgentCoordinator
	registerAgentReconciliation(reconciliationEngine, goalRegistry, kernelContainer, recoveryDescriptor)
	cbRegistry := mindruntime.NewCircuitBreakerRegistry()
	cbRegistry.Register("qdrant", mindruntime.DefaultCircuitBreakerConfig())
	cbRegistry.Register("surrealdb", mindruntime.DefaultCircuitBreakerConfig())
	cbRegistry.Register("model_api", mindruntime.DefaultCircuitBreakerConfig())
	voiceEntry := interaction.NewVoiceEntry(entry)
	if err := deliveryStore.InitSchema(); err != nil {
		log.Error("failed to init delivery store schema:", err)
		panic("failed to init delivery store schema")
	}
	configureWorkflowHost(extensionRuntime, chatSvc, memSvc, deliveryStore, kernelContainer.HostEventEmitter)
	mcpDuplicateStore := mcp.NewDuplicateStore(ctx.DB)
	kernelContainer.MCPDuplicateProvider = &mcpDuplicateMetricAdapter{store: mcpDuplicateStore}
	canonicalStdioFactory := extensionmcp.NewCanonicalStdioFactory(commandResolver)
	canonicalStdioRegistry := extensionmcp.NewCanonicalStdioRegistry(canonicalStdioFactory)
	canonicalRemoteFactory := extensionmcp.NewCanonicalRemoteFactory()
	canonicalRemoteRegistry := extensionmcp.NewCanonicalRemoteRegistry(canonicalRemoteFactory)
	canonicalMCPCaller := NewCanonicalMCPCaller(canonicalStdioRegistry, canonicalRemoteRegistry)
	kernelContainer.WireMCPAdapter(makeKernelMCPCaller(canonicalMCPCaller), makeKernelMCPHealth(canonicalMCPCaller), nil)
	desktopPetRepo := desktoppet.NewRepository(ctx.DB, ctx)
	desktopPetWorker := worker.NewWorker(ctx.DB, desktopPetRepo, imageProviderRegistry)
	processingRepo := processing.NewRepository(ctx.DB, ctx)

	bgRegistry := kernelContainer.BackgroundRemovalRegistry
	if bgRegistry == nil {
		log.Error("background removal registry not initialized from kernel bootstrap")
		panic("background removal registry not initialized from kernel bootstrap")
	}

	processingPipeline := application.NewPipeline(bgRegistry, processingDataDir)
	artifactSourceAdapter := application.NewRepoArtifactSourceAdapter(processingRepo)
	processingSourceResolver := application.NewRepoSourceResolver(processingRepo, processingDataDir, artifactSourceAdapter)

	processingWSManager := processingworkspace.NewWorkspaceManager(processingDataDir)
	processingCommitJournalRepo := processingevents.NewCommitJournalRepository(ctx.DB)
	processingOutboxRepo := processingevents.NewOutboxRepository(ctx.DB)
	processingEventOutbox := processingevents.NewEventOutbox(processingOutboxRepo)
	processingManifestStore := processing.NewManifestStore(ctx.DB)
	processingCommitter := processingcommit.NewProcessingCommitter(
		ctx.DB, processingRepo, processingWSManager,
		processingCommitJournalRepo, processingEventOutbox,
		processingManifestStore, processingDataDir,
	)

	processingWorker := processingworker.NewWorker(ctx.DB, processingRepo, processingDataDir, processingPipeline, processingSourceResolver, processingCommitter)

	qualityRepo := quality.NewRepository(ctx.DB)
	qualityGateEvaluator := quality.NewGateEvaluator(qualityRepo)
	qualityWritebackSvc := qualitywriteback.NewQualityWritebackService(ctx.DB)
	qualityActiveBindingSvc := qualitywriteback.NewActiveBindingService(qualityRepo)
	qualityCommitter := qualitywriteback.NewCommitter(ctx.DB, qualityRepo, qualityWritebackSvc, qualityActiveBindingSvc)
	qualityTaskGateSvc := qualitygate.NewTaskGateService(qualityRepo, qualityGateEvaluator)
	qualityGateInvalidator := qualitygate.NewGateInvalidator(qualityRepo)
	qualityReviewDecisionSvc := qualitygate.NewReviewDecisionService(qualityRepo)
	qualityOutboxPublisher := qualityrecovery.NewOutboxPublisher(qualityRepo, quality.NewLogEventPublisher())
	qualityRecoveryWorker := qualityrecovery.NewRecoveryWorker(qualityRepo)
	qualityInputRepo := qualityinput.NewInputRepository(ctx.DB, processingDataDir)
	qualityMeasurementEngine := qualitymeasurement.NewImageMeasurementEngine(qualityRepo)
	qualitySvc, err := quality.NewQualityService(quality.ServiceConfig{
		DB:                ctx.DB,
		DataDir:           processingDataDir,
		Detectors:         detectors.NewDefaultDetectors(),
		EventPublisher:    quality.NewLogEventPublisher(),
		Repo:              qualityRepo,
		Committer:         qualityCommitter,
		TaskGateService:   qualityTaskGateSvc,
		GateInvalidator:   qualityGateInvalidator,
		ReviewDecisionSvc: qualityReviewDecisionSvc,
		RecoveryWorker:    qualityRecoveryWorker,
		OutboxPublisher:   qualityOutboxPublisher,
		InputRepository:   qualityInputRepo,
		MeasurementEngine: qualityMeasurementEngine,
	})
	if err != nil {
		return nil, fmt.Errorf("create quality service: %w", err)
	}
	if qualitySvc == nil {
		return nil, errors.New("quality service is nil")
	}
	qualityWorker := qualityworker.NewWorker(ctx.DB, qualitySvc, processingDataDir)

	runtimeSinkHolder := &runtimeEventSinkHolder{}
	runtimeOutboxSink := runtime.NewOutboxRuntimeEventSink(
		runtime.NewV2ActualStateEventOutbox(runtimeV2Facade.StateService().AppendDomainEvent),
	)
	runtimeSinkHolder.Set(runtimeOutboxSink)

	v2Notifier := runtimev2.NewV2RuntimeNotifier(runtimeV2Facade.StateService(), runtimeV2Facade.Events())
	_ = v2Notifier

	editingRepo := editing.NewRepository(ctx.DB)
	editingAssetStore := editing.NewAssetStore(processingDataDir, editingRepo)
	editingGenAdapter := newEditingGenerationPort(ctx)
	editingProcAdapter := newEditingProcessingPort(ctx)
	editingQualAdapter := newEditingQualityPort(ctx)
	editingSvc := editing.NewService(editingRepo, editingAssetStore, editingGenAdapter, editingProcAdapter, editingQualAdapter, ctx.DB, processingDataDir)
	if err := editingSvc.RecoverPendingJournals(context.Background()); err != nil {
		log.Warn("editing journal recovery warning: ", err)
	}
	if err := editingSvc.ExpireSessions(context.Background()); err != nil {
		log.Warn("editing session expiry warning: ", err)
	}

	regenerationWorker := editing.NewRegenerationWorker(editingRepo, editingGenAdapter, editingAssetStore, editingQualAdapter, editingProcAdapter)

	baselineCommitter := baseline.NewBaselineActionRevisionCommitter(ctx.DB)
	bridgeInboxRepo := revisioncommit.NewBridgeInboxRepository(ctx.DB)
	bridgeOutboxRepo := revisioncommit.NewOutboxRepository(ctx.DB)
	bridgeJournalRepo := revisioncommit.NewRepository(ctx.DB)
	procRevReader := &processingRevisionReaderAdapter{repo: processingRepo}
	bridgeProcessor := revisioncommit.NewBridgeProcessor(
		bridgeInboxRepo, bridgeJournalRepo, baselineCommitter, procRevReader, bridgeOutboxRepo, nil, "worker-main",
	)
	bridgeRecoveryWorker := revisioncommit.NewRecoveryWorker(bridgeProcessor, 30*time.Second)
	_ = bridgeRecoveryWorker

	if err := releaseStoragePort.Validate(); err != nil {
		return nil, fmt.Errorf("initialize release storage: %w", err)
	}

	pathRegistry := desktoppetsecurity.NewPathRootRegistry()
	if err := desktoppetsecurity.EnsureAllRequiredRoots(pathRegistry, config.AppCfg.Storage.DataDir); err != nil {
		return nil, fmt.Errorf("initialize required storage roots: %w", err)
	}
	importStagingRepo := desktoppetsecurity.NewImportStagingRepository(ctx.DB)

	coordRepo := &coordinatorRepoAdapter{installRepo: installationRepo}
	coordValidator := &coordinatorReleaseValidator{releases: releaseRepo}
	coordStager := &coordinatorReleaseStager{registry: pathRegistry, releases: releaseRepo}
	coordPublisher := &coordinatorRuntimePublisher{facade: runtimeV2Facade}
	coordProjection := &coordinatorProjectionService{installRepo: installationRepo}
	var installationCoordinator coordinator.InstallationCoordinator
	installationCoordinator = coordinator.NewCoordinator(coordRepo, coordValidator, coordStager, coordPublisher, coordProjection)
	deviceRepo := device.NewRepository(ctx.DB)

	var desktopInstanceStore *security.DesktopInstanceStore
	if config.AppCfg.Security.Mode == "local_single_user" {
		desktopInstanceStore, err = security.NewDesktopInstanceStore(config.AppCfg.Storage.DataDir)
		if err != nil {
			return nil, fmt.Errorf("initialize desktop instance store: %w", err)
		}
	}

	ownershipGuard := desktoppetsecurity.NewSQLiteOwnershipGuard(ctx.DB)

	bootstrapTicketRepo := runtime.NewBootstrapTicketRepository(ctx.DB)

	v2BehaviorRuntimePort := wiring.NewV2RuntimeActionAdapter(runtimeV2Facade)
	v2ActivePetPort := wiring.NewV2ActivePetAdapter(installationRepo, runtimeV2Facade, processingDataDir)

	behaviorAssembled, assembleErr := wiring.AssembleBehavior(wiring.AssemblyDeps{
		DB:                ctx.DB,
		ActivePetPort:     v2ActivePetPort,
		RuntimeActionPort: v2BehaviorRuntimePort,
		InstallRepo:       installationRepo,
		PsycheStore:       psycheStore,
		DataDir:           processingDataDir,
		ShadowMode:        false,
		RuntimeCmdOn:      true,
	})
	var behaviorSvc *behavior.BehaviorService
	if assembleErr != nil {
		log.Error("failed to assemble behavior engine: ", assembleErr)
	} else {
		behaviorSvc = behaviorAssembled.Service
	}

	var behaviorRuntimeSink *BehaviorRuntimeEventSink
	var runtimeDomainEventConsumer *runtimev2.OutboxConsumer
	if behaviorAssembled != nil && behaviorAssembled.Engine != nil {
		behaviorRuntimeSink = NewBehaviorRuntimeEventSink(installationRepo, behaviorAssembled.Engine)
		runtimeDomainEventConsumer = runtimev2.NewOutboxConsumer(ctx.DB, func(eventCtx context.Context, event runtimev2.DomainEventOutbox) error {
			domainEvent, err := runtime.DecodeV2OutboxEvent(event)
			if err != nil {
				return err
			}
			return behaviorRuntimeSink.OnRuntimeEvent(eventCtx, domainEvent)
		})
	}

	safeModeCtrl := readiness.NewSafeModeController()
	var readinessSvc *readiness.ReadinessService
	if runtimeV2Facade != nil {
		readinessSvc, err = readiness.NewFullStartupReadinessService(readiness.StartupReadinessDeps{
			DB:        ctx.DB,
			Extension: extensionRuntime,
			DesktopSessionReady: func() error {
				sqlDB, err := ctx.DB.DB()
				if err != nil {
					return fmt.Errorf("get underlying db: %w", err)
				}
				return sqlDB.PingContext(context.Background())
			},
			OwnershipReady: func() error {
				if ownershipGuard == nil {
					return fmt.Errorf("ownership guard is nil")
				}
				return nil
			},
			RuntimeTicketReady: func() error {
				return bootstrapTicketRepo.ReadinessCheck(context.Background())
			},
			RuntimeGatewayReady: func() error {
				if runtimeV2Facade == nil {
					return fmt.Errorf("runtime v2 facade is nil")
				}
				return nil
			},
			PathGuardReady: func() error {
				if pathRegistry == nil {
					return fmt.Errorf("path registry is nil")
				}
				return pathRegistry.Validate()
			},
			GenerationWorkerReady: func() error {
				if desktopPetWorker == nil {
					return fmt.Errorf("generation worker is nil")
				}
				return nil
			},
			ProcessingWorkerReady: func() error {
				if processingWorker == nil {
					return fmt.Errorf("processing worker is nil")
				}
				return nil
			},
			QualityWorkerReady: func() error {
				if qualityWorker == nil {
					return fmt.Errorf("quality worker is nil")
				}
				return nil
			},
			InstallationWorkerReady: func() error {
				if installationCoordinator == nil {
					return fmt.Errorf("installation coordinator is nil")
				}
				return nil
			},
			BehaviorWorkerReady: func() error {
				if behaviorSvc == nil {
					return fmt.Errorf("behavior service is nil")
				}
				return nil
			},
			MigrationReady: func() error {
				return checkMigrationState(ctx.DB)
			},
			LegacyChainReady: func() error {
				if !desktoppet.LegacyPackageWritesDisabled {
					return fmt.Errorf("legacy package writes not disabled")
				}
				return nil
			},
		})
		if err != nil {
			return nil, fmt.Errorf("initialize readiness service: %w", err)
		}
	}

	leaseManager := releasebuild.NewLeaseManager()
	journalManager := releasebuild.NewPublishJournalManager(releaseRepo)
	releaseRecoveryWorker := release.NewReleaseRecoveryWorker(releaseRepo, leaseManager, journalManager, releaseStoragePort, releaseEventPublisher)
	_ = releaseRecoveryWorker
	_ = newReleaseService
	_ = journalManager

	var adapterManager *adapters.AdapterManager
	if behaviorAssembled != nil && behaviorAssembled.Engine != nil {
		adapterManager = adapters.NewAdapterManager(
			adapters.NewEnginePublisher(behaviorAssembled.Engine),
			adapters.AdapterManagerOptions{
				Clock: behavior.NewRealClock(),
			},
		)
		if behaviorSvc != nil {
			behaviorSvc.SetAdapterManager(adapterManager)
		}
	}

	migrationRepo := migration.NewDBRepository(ctx.DB)
	migrationRunner := migration.NewRunner(migrationRepo)
	backupDir := filepath.Join(config.AppCfg.Storage.DataDir, "migration_backups")
	lockDir := filepath.Join(config.AppCfg.Storage.DataDir, "migration_locks")
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		return nil, fmt.Errorf("initialize migration lock directory: %w", err)
	}
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return nil, fmt.Errorf("initialize migration backup directory: %w", err)
	}
	migrationLock := migrationcore.NewPersistentLock(ctx.DB, lockDir)
	migrationRunner.SetLock(migrationLock)
	migrationRunner.SetBackupDir(backupDir)
	backupPort := newDomainMigrationBackupPort(ctx.DB, backupDir)
	migrationRunner.SetBackupPort(backupPort)
	migrationRunner.RegisterPlan(migrationplans.NewDesktopPetV2CutoverPlan(migrationplans.Dependencies{DB: ctx.DB}))

	maintenanceHandler := maintenance.NewHandler(migrationRunner, nil, nil, nil)

	dpCoord, err := buildDataPortabilityCoordinator(dataPortabilityDeps{
		DataDir:   config.AppCfg.Storage.DataDir,
		DB:        ctx.DB,
		MemSvc:    memSvc,
		EpicSvc:   epiSvc,
		ExtSvc:    extensionRuntime,
		Workspace: workspaceService,
	})
	if err != nil {
		return nil, fmt.Errorf("build data portability coordinator: %w", err)
	}

services := &AppServices{
	DB:                           ctx.DB,
	RuntimeProfile:               runtimeProfile,
	RuntimePolicy:                policy,
	DataPortability:              dpCoord,
		Graph:                        graphSvc,
		ChatDeliveryAdapter:          deliveryAdapter,
		Memory:                       memSvc,
		Profile:                      profSvc,
		Episodic:                     epiSvc,
		WorldBook:                    wbSvc,
		Vision:                       visionSvc,
		Companion:                    compSvc,
		Chat:                         chatSvc,
		UnifiedEntry:                 entry,
		DataLifecycle:                dataLifecycle,
		RuntimeQueue:                 runtimeQueue,
		NewOutbox:                    newOutboxStore,
		DeliveryStore:                deliveryStore,
		DeliveryWorker:               deliveryWorker,
		OutboxWorker:                 newOutboxWorker,
		DesktopPetWorker:             desktopPetWorker,
		ProcessingWorker:             processingWorker,
		QualityService:               qualitySvc,
		QualityWorker:                qualityWorker,
		InstallationCoordinator:      installationCoordinator,
		InstallationRepo:             installationRepo,
		NewReleaseService:            newReleaseService,
		ReleaseRecoveryWorker:        releaseRecoveryWorker,
		ReleaseEventPublisher:        releaseEventPublisher,
		DesktopPetRuntimeV2:          runtimeV2Facade,
		EditingService:               editingSvc,
		RegenerationWorker:           regenerationWorker,
		BridgeRecoveryWorker:         bridgeRecoveryWorker,
		BehaviorService:              behaviorSvc,
		AdapterManager:               adapterManager,
		Reconciliation:               reconciliationEngine,
		CircuitBreakers:              cbRegistry,
		VoiceEntry:                   voiceEntry,
		Extension:                    extensionRuntime,
		KernelContainer:              kernelContainer,
		Emote:                        emoteSvc,
		Temporal:                     temporalSvc,
		RelTimeCoordinator:           relTimeCoordinator,
		OwnershipGuard:               ownershipGuard,
		PathRegistry:                 pathRegistry,
		ImportStagingRepo:            importStagingRepo,
		PackageImporter:              importer.NewPackageImporterWithJournal(releaseRepo, releaseStoragePort, importer.NewDefaultPackageValidator(pathRegistry, importStagingRepo), pathRegistry, importStagingRepo, releasebuild.NewPublishJournalManager(releaseRepo)),
		Readiness:                    readinessSvc,
		SafeMode:                     safeModeCtrl,
		DesktopInstanceStore:         desktopInstanceStore,
		DeviceRepository:             deviceRepo,
		RuntimeDomainEventConsumer:   runtimeDomainEventConsumer,
		DesktopPetMigrationRunner:    migrationRunner,
		DesktopPetMaintenanceHandler: maintenanceHandler,
		MigrationLock:                migrationLock,
		RecoveryDescriptor:           recoveryDescriptor,
		PauseResumeService:           pauseResumeService,
		BackgroundTaskCoordinator:    backgroundTaskCoordinator,
		MultiAgentCoordinator:        multiAgentCoordinator,
		Browser:                      browserProvider,
		MediaService:                 mediaService,
		WorkspaceRegistry:            workspaceRegistry,
		WorkspaceService:             workspaceService,
		NativeBridgeRelay:            newNativeBridgeRelay(),
		Artifact:                     nil,
		DeviceMesh:                   nil,
	}
	if runtimeProfile == runtimeprofile.ProfileCloudCore && kernelContainer != nil {
		deviceMeshRuntime, err := devicemesh.NewCloudRuntime(kernelContainer.Store.DB(), kernelContainer.DeviceRegistry)
		if err != nil {
			log.Warn("device-mesh cloud runtime unavailable: ", err)
		} else {
			services.DeviceMesh = deviceMeshRuntime
		}
	}
	if services.Artifact == nil {
		artifactRuntime, err := BuildArtifactRuntime(ctx.DB, "")
		if err != nil {
			log.Warn("artifact runtime build deferred: ", err)
		} else {
			services.Artifact = artifactRuntime
			chatSvc.SetArtifactResolver(&chatArtifactAdapter{resolver: artifactRuntime.Resolver})
		}
	}
	if services.NativeBridgeRelay != nil && bootstrap != nil {
		if androidBridge, ok := bootstrap.AndroidNativeBridge().(*nativebridge.AndroidTransportBridge); ok {
			services.NativeBridgeRelay.RegisterAndroidBridge(androidBridge)
		}
		tryRegisterIOSBridge(services.NativeBridgeRelay, bootstrap)
	}
	if services.NativeBridgeRelay != nil && services.KernelContainer != nil && services.KernelContainer.EventService != nil {
		adapter := nativebridge.NewNativeEventSinkAdapter(services.KernelContainer.EventService)
		services.NativeBridgeRelay.Handler().SetEventSink("android", adapter)
		services.NativeBridgeRelay.Handler().SetEventSink("ios", adapter)
	}
	if bootstrap != nil && kernelContainer != nil && kernelContainer.TaskRuntimeService != nil {
		iosProv := bootstrap.IOSNativeProvider()
		if iosProv != nil {
			if setter, ok := iosProv.(interface {
				SetTaskRuntimePort(port iosnative_background.TaskRuntimePort)
			}); ok {
				setter.SetTaskRuntimePort(iosnative_background.NewTaskRuntimeServiceAdapter(kernelContainer.TaskRuntimeService))
				services.BackgroundTaskRuntimeWired = true
			}
		}
	}
	if err := runCanonicalBuildAssertions(services); err != nil {
		return nil, fmt.Errorf("canonical build assertion failed: %w", err)
	}
	return services, nil
}

func newDeviceAgentServices(ctx *app.AppContext, graphSvc graph.Service, bootstrap *runtimeBootstrap, runtimeProfile runtimeprofile.Profile, policy runtimeprofile.Policy) (*AppServices, error) {
	extensionRuntime, err := extension.NewRuntimeWithOptions(context.Background(), ctx.DB, "1.0.0", extension.RuntimeOptions{SkipPluginManagerStart: true})
	if err != nil {
		return nil, fmt.Errorf("initialize skill runtime: %w", err)
	}
	kernelRoot := filepath.Join(os.TempDir(), "amitia-extension-kernel")
	if config.AppCfg != nil && config.AppCfg.Storage.DataDir != "" {
		kernelRoot = filepath.Join(config.AppCfg.Storage.DataDir, "extensions-v2")
	}
	if err := extensionRuntime.AttachKernel(kernelRoot); err != nil {
		return nil, fmt.Errorf("initialize extension kernel: %w", err)
	}
	kernelDBPath := filepath.Join(kernelRoot, "kernel.db")
	var nodeResolver script_host.NodeEnvironmentResolver
	var artifactResolver script_host.ArtifactResolver
	if bootstrap != nil {
		nodeResolver = bootstrap.NodeEnvironmentResolver()
	}
	kernelBuilder := kernel.NewContainerBuilder().
		WithDBPath(kernelDBPath).
		WithExtensionRoot(kernelRoot).
		WithNodeEnvironmentResolver(nodeResolver).
		WithHostArtifactResolver(artifactResolver).
		WithRuntimeProfile(runtimeProfile).
		WithBackgroundBootstrapFunc(func() (backgroundremoval.Registry, error) {
			reg := backgroundremoval.NewRegistry()
			if err := reg.Register(local.NewLocalProvider(), local.LocalCapabilities()); err != nil {
				return nil, fmt.Errorf("register local background provider: %w", err)
			}
			return reg, nil
		})
	if bootstrap != nil {
		kernelBuilder.WithRuntimeHost(bootstrap.RuntimeHost())
	}
	kernelContainer, err := kernelBuilder.Build(context.Background())
	if err != nil {
		return nil, fmt.Errorf("initialize kernel container: %w", err)
	}
	extensionRuntime.Kernel.SetContainer(kernelContainer)
	if err := kernelContainer.Recover(context.Background()); err != nil {
		log.Warn("kernel recovery warning: ", err)
	}

	deviceAgentRuntime, err := devicemesh.NewDeviceAgentRuntime(config.AppCfg.Storage.DataDir, runtimeidentity.PlatformWindows)
	if err != nil {
		log.Warn("device-mesh agent runtime unavailable: ", err)
	}

	services := &AppServices{
		DB:               ctx.DB,
		RuntimeProfile:   runtimeProfile,
		RuntimePolicy:    policy,
		Graph:            nil,
		Chat:             nil,
		Extension:        extensionRuntime,
		KernelContainer:  kernelContainer,
		DeviceMesh:       deviceAgentRuntime,
	}
	return services, nil
}

func mcpDataDirectory(ctx *app.AppContext) string {
	if config.AppCfg != nil && strings.TrimSpace(config.AppCfg.Storage.DataDir) != "" {
		return config.AppCfg.Storage.DataDir
	}
	var databases []struct {
		Name string `gorm:"column:name"`
		File string `gorm:"column:file"`
	}
	if ctx != nil && ctx.DB != nil && ctx.DB.Raw("PRAGMA database_list").Scan(&databases).Error == nil {
		for _, database := range databases {
			if database.Name == "main" && strings.TrimSpace(database.File) != "" {
				return filepath.Dir(database.File)
			}
		}
	}
	return filepath.Join(".", "data")
}

func checkMigrationState(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM schema_migrations WHERE status = ?", "failed").Count(&count).Error; err != nil {
		return fmt.Errorf("check migration state: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("found %d failed migrations", count)
	}
	return nil
}

type characterOwnerPort struct {
	installRepo installation.Repository
}

func (p *characterOwnerPort) ResolveUserID(ctx context.Context, characterID string) string {
	if p.installRepo == nil {
		return ""
	}
	insts, err := p.installRepo.ListInstallationsByCharacter(characterID)
	if err != nil || len(insts) == 0 {
		return ""
	}
	return insts[0].UserID
}

type petInfoPort struct {
	installRepo installation.Repository
}

func (p *petInfoPort) ResolvePetInfo(ctx context.Context, petInstanceID string) (string, string) {
	if p.installRepo == nil {
		return "", ""
	}
	if inst, err := p.installRepo.GetInstallation(petInstanceID); err == nil && inst != nil {
		return inst.UserID, inst.CharacterID
	}
	return "", ""
}

func configureWorkflowHost(runtime *extension.Runtime, chatSvc chat.Service, memSvc memory.Service, deliveryStore *delivery.SQLiteDeliveryStore, hostEmitter event.HostEventEmitter) {
	runtime.WorkflowHost.Schedule = wrapWithWorkflowEvent(hostEmitter, "schedule", func(ctx context.Context, input json.RawMessage, scope extension.ExecutionScope) (json.RawMessage, []extension.SideEffectRecord, error) {
		var payload map[string]interface{}
		if err := json.Unmarshal(input, &payload); err != nil {
			return nil, nil, fmt.Errorf("日程参数无效: %w", err)
		}
		if payload["due_time"] == nil && payload["dueAt"] != nil {
			payload["due_time"] = payload["dueAt"]
		}
		idempotencyKey, _ := payload["idempotencyKey"].(string)
		normalized, _ := json.Marshal(payload)
		registered, err := runtime.Registry.Get(ctx, "dev.amitia.skill.create-schedule")
		if err != nil {
			return nil, nil, err
		}
		result, err := registered.Handler(ctx, extension.ExecuteSkillRequest{SkillID: registered.Definition.ID, Input: normalized, Scope: scope, IdempotencyKey: idempotencyKey})
		return result.Output, result.SideEffects, err
	})
	runtime.WorkflowHost.Notification = wrapWithWorkflowEvent(hostEmitter, "notification", func(_ context.Context, input json.RawMessage, scope extension.ExecutionScope) (json.RawMessage, []extension.SideEffectRecord, error) {
		var payload struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(input, &payload); err != nil {
			return nil, nil, fmt.Errorf("通知参数无效: %w", err)
		}
		payload.Content = strings.TrimSpace(payload.Content)
		if payload.Content == "" || len([]rune(payload.Content)) > 4000 {
			return nil, nil, fmt.Errorf("通知内容长度必须为 1 到 4000 个字符")
		}
		conversation, err := chatSvc.GetConversation(scope.ConversationID)
		if err != nil {
			return nil, nil, err
		}
		if conversation.CharacterID != scope.CharacterID || conversation.Channel != scope.Channel || conversation.PeerID == "" {
			return nil, nil, fmt.Errorf("通知只能发送到当前角色和会话绑定的渠道")
		}
		body, _ := json.Marshal(map[string]string{"content": payload.Content})
		interactionID := scope.RequestID
		if interactionID == "" {
			interactionID = uuid.New().String()
		}
		intent := delivery.NewDeliveryIntent(interactionID, conversation.Channel, conversation.PeerID, "text", body)
		if err := deliveryStore.CreateIntent(intent); err != nil {
			return nil, nil, err
		}
		output, _ := json.Marshal(map[string]string{"intentId": intent.ID, "status": string(intent.Status)})
		return output, []extension.SideEffectRecord{{Type: "notification_send", TargetID: intent.ID, Confirmed: true}}, nil
	})
	runtime.WorkflowHost.MemoryCandidate = wrapWithWorkflowEvent(hostEmitter, "memory_candidate", func(_ context.Context, input json.RawMessage, scope extension.ExecutionScope) (json.RawMessage, []extension.SideEffectRecord, error) {
		var payload struct {
			Key        string `json:"key"`
			Value      string `json:"value"`
			MemoryType string `json:"memoryType"`
			Importance int    `json:"importance"`
			Source     string `json:"source"`
		}
		if err := json.Unmarshal(input, &payload); err != nil {
			return nil, nil, fmt.Errorf("候选记忆参数无效: %w", err)
		}
		candidate, err := memSvc.SubmitCandidate(&memory.SubmitCandidateRequest{Key: payload.Key, Value: payload.Value, MemoryType: payload.MemoryType, Importance: payload.Importance, SourceText: payload.Source, ConversationID: scope.ConversationID, CharacterID: scope.CharacterID})
		if err != nil {
			return nil, nil, err
		}
		output, _ := json.Marshal(map[string]interface{}{"candidateId": candidate.ID, "status": "pending_review"})
		return output, []extension.SideEffectRecord{{Type: "memory_candidate_write", TargetID: candidate.ID, Confirmed: true}}, nil
	})
	runtime.WorkflowHost.ContextContribution = wrapWithWorkflowEvent(hostEmitter, "context_contribution", func(_ context.Context, input json.RawMessage, scope extension.ExecutionScope) (json.RawMessage, []extension.SideEffectRecord, error) {
		var payload struct {
			Content    string `json:"content"`
			TokenLimit int    `json:"tokenLimit"`
		}
		if err := json.Unmarshal(input, &payload); err != nil {
			return nil, nil, fmt.Errorf("上下文贡献参数无效: %w", err)
		}
		payload.Content = strings.TrimSpace(payload.Content)
		if payload.Content == "" || payload.TokenLimit < 1 || payload.TokenLimit > 1024 || len([]rune(payload.Content)) > payload.TokenLimit*8 {
			return nil, nil, fmt.Errorf("上下文贡献超出 1024 token 宿主限制")
		}
		output, _ := json.Marshal(map[string]interface{}{"content": payload.Content, "tokenLimit": payload.TokenLimit, "conversationId": scope.ConversationID})
		return output, []extension.SideEffectRecord{{Type: "context_injection", TargetID: scope.ConversationID, Confirmed: true}}, nil
	})
}

func wrapWithWorkflowEvent(hostEmitter event.HostEventEmitter, action string, handler func(context.Context, json.RawMessage, extension.ExecutionScope) (json.RawMessage, []extension.SideEffectRecord, error)) func(context.Context, json.RawMessage, extension.ExecutionScope) (json.RawMessage, []extension.SideEffectRecord, error) {
	return func(ctx context.Context, input json.RawMessage, scope extension.ExecutionScope) (json.RawMessage, []extension.SideEffectRecord, error) {
		output, effects, err := handler(ctx, input, scope)
		if err == nil && hostEmitter != nil {
			payload, _ := json.Marshal(map[string]interface{}{
				"action":         action,
				"characterId":    scope.CharacterID,
				"conversationId": scope.ConversationID,
				"channel":        scope.Channel,
				"requestId":      scope.RequestID,
			})
			opts := event.PublishOptions{
				TraceID:     scope.TraceID,
				OperationID: scope.RequestID,
			}
			_, _ = hostEmitter.EmitWorkflowCompleted(ctx, payload, opts)
		}
		return output, effects, err
	}
}

func newRuntimeContextLoaderRegistry(ctx *app.AppContext, charRepo character.Repository, temporalServices ...*temporal.Service) *interaction.ContextLoaderRegistry {
	runtimeRegistry := interaction.NewContextLoaderRegistry()
	runtimeRegistry.Register(interaction.NewRoleRuntimeProfileContextLoader(charRepo))
	runtimeRegistry.Register(interaction.NewChannelContextLoader())
	runtimeRegistry.Register(interaction.NewConversationContextLoader(ctx.DB))
	runtimeRegistry.Register(interaction.NewPsycheContextLoader(ctx.DB))
	runtimeRegistry.Register(interaction.NewRelationshipContextLoader(ctx.DB))
	runtimeRegistry.Register(interaction.NewBeliefContextLoader(ctx.DB))
	runtimeRegistry.Register(interaction.NewLifeContextLoader(ctx.DB))
	runtimeRegistry.Register(interaction.NewNeedContextLoader(ctx.DB))
	runtimeRegistry.Register(interaction.NewUnresolvedThreadContextLoader(ctx.DB))
	if len(temporalServices) > 0 && temporalServices[0] != nil {
		runtimeRegistry.Register(interaction.NewTemporalContextLoader(temporalServices[0]))
	}
	return runtimeRegistry
}

type chatOutboxAdapter struct {
	store *newoutbox.SQLiteOutboxStore
}

func (a *chatOutboxAdapter) AppendOutbox(aggregateID, eventType string, payload []byte) error {
	record := newoutbox.OutboxRecord{
		ID:          uuid.New().String(),
		AggregateID: aggregateID,
		EventType:   eventType,
		Payload:     payload,
		Status:      newoutbox.OutboxStatusPending,
		MaxRetries:  newoutbox.DefaultMaxRetries,
		AvailableAt: time.Now(),
		CreatedAt:   time.Now(),
	}
	return a.store.Append(record)
}

func (a *chatOutboxAdapter) AppendOutboxWithKey(aggregateID, eventType, idempotencyKey string, payload []byte) error {
	record := newoutbox.OutboxRecord{
		ID:             uuid.New().String(),
		AggregateID:    aggregateID,
		EventType:      eventType,
		Payload:        payload,
		Status:         newoutbox.OutboxStatusPending,
		MaxRetries:     newoutbox.DefaultMaxRetries,
		AvailableAt:    time.Now(),
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now(),
	}
	return a.store.Append(record)
}

type chatDeliveryAdapter struct {
	store *delivery.SQLiteDeliveryStore
}

func (a *chatDeliveryAdapter) CreateDeliveryIntent(interactionID, channel, peerID, contentType string, payload []byte) error {
	intent := delivery.NewDeliveryIntent(interactionID, channel, peerID, contentType, payload)
	return a.store.CreateIntent(intent)
}

func (a *chatDeliveryAdapter) CreateOutputLease(interactionID, characterID, userID, channel string) error {
	lease := delivery.NewOutputLease(interactionID, characterID, userID, channel)
	return a.store.CreateLease(lease)
}

func (a *chatDeliveryAdapter) AcquireOutputLease(interactionID, characterID, userID, channel string) (string, string, error) {
	lease := delivery.NewOutputLease(interactionID, characterID, userID, channel)
	if err := a.store.CreateLease(lease); err != nil {
		return "", "", err
	}
	return lease.ID, lease.OwnerToken, nil
}

func (a *chatDeliveryAdapter) ReleaseOutputLease(leaseID, ownerToken string) error {
	return a.store.ReleaseLease(leaseID, ownerToken)
}

func (a *chatDeliveryAdapter) PreemptActiveOutputLeases(characterID string) error {
	_, err := a.store.PreemptActiveLeasesByCharacter(characterID)
	return err
}

type reflectionOutboxServiceAdapter struct {
	store *newoutbox.SQLiteOutboxStore
}

func (a *reflectionOutboxServiceAdapter) Append(record newoutbox.OutboxRecord) error {
	return a.store.Append(record)
}

func registerAgentReconciliation(engine *mindruntime.ReconciliationEngine, goalRegistry *decision.GoalRegistry, kernelContainer *kernel.Container, recoveryDescriptor *interaction.RecoveryDescriptorService) {
	if engine == nil || goalRegistry == nil {
		return
	}
	goals := interaction.NewGoalReaderAdapter(goalRegistry)
	var tasks interaction.TaskReconciliationReader
	if kernelContainer != nil && kernelContainer.TaskRuntimeService != nil {
		tasks = interaction.NewTaskReaderAdapter(kernelContainer.TaskRuntimeService)
	}
	var workflows interaction.WorkflowReconciliationReader
	if kernelContainer != nil && kernelContainer.WorkflowExecutor != nil {
		workflows = interaction.NewWorkflowReaderAdapter(kernelContainer.WorkflowExecutor)
	}
	var invocations interaction.InvocationReconciliationReader
	if kernelContainer != nil && kernelContainer.ObservabilityStore != nil {
		invocations = interaction.NewInvocationReaderAdapter(kernelContainer.ObservabilityStore)
	} else {
		invocations = interaction.NewNoopInvocationReader()
	}
	processor := interaction.NewAgentReconciliationProcessor(goals, interaction.NewNoopAgentObservationReader(), tasks, workflows, invocations)
	settleDelay := mindruntime.DefaultAgentFactSettleDelay()
	engine.RegisterChecker(mindruntime.ReconciliationAgentGoalAction, interaction.NewGoalActionChecker(processor, settleDelay))
	engine.RegisterChecker(mindruntime.ReconciliationAgentActionObservation, interaction.NewActionObservationChecker(processor, settleDelay))
	engine.RegisterChecker(mindruntime.ReconciliationAgentObservationGoal, interaction.NewObservationGoalChecker(processor, settleDelay))
	engine.RegisterChecker(mindruntime.ReconciliationAgentTask, interaction.NewTaskConsistencyChecker(processor, settleDelay))
	engine.RegisterChecker(mindruntime.ReconciliationAgentWorkflow, interaction.NewWorkflowConsistencyChecker(processor, settleDelay))
	engine.RegisterChecker(mindruntime.ReconciliationAgentRuntime, interaction.NewInvocationConsistencyChecker(processor, settleDelay))
	if recoveryDescriptor != nil {
		engine.RegisterChecker(mindruntime.ReconciliationAgentDescriptorRefStale, interaction.NewDescriptorRefStaleChecker(recoveryDescriptor, settleDelay))
		engine.RegisterChecker(mindruntime.ReconciliationAgentDescriptorSchema, interaction.NewDescriptorRefStaleChecker(recoveryDescriptor, settleDelay))
	}
}

type graphReconciliationAdapter struct {
	graphSvc graph.Service
}

func (a *graphReconciliationAdapter) Name() string { return "graph" }

func (a *graphReconciliationAdapter) CheckSideEffectExists(ctx context.Context, aggregateID, eventType string) (bool, error) {
	if a.graphSvc == nil {
		return false, nil
	}
	nodes, err := a.graphSvc.GetAllNodes(aggregateID)
	if err != nil {
		return false, err
	}
	return len(nodes) > 0, nil
}

type qdrantReconciliationAdapter struct {
	qdrantClient *qdrant.QdrantClient
}

func (a *qdrantReconciliationAdapter) Name() string { return "qdrant" }

func (a *qdrantReconciliationAdapter) CheckSideEffectExists(ctx context.Context, aggregateID, eventType string) (bool, error) {
	if a.qdrantClient == nil {
		return false, nil
	}
	_, err := a.qdrantClient.SearchWithFilter(ctx, "memory_embeddings", nil, qdrant.QdrantFilter{CharacterID: aggregateID}, 1)
	if err != nil {
		return false, nil
	}
	return true, nil
}

type postProcessPublisherAdapter struct {
	chatSvc chat.Service
}

func (a *postProcessPublisherAdapter) Publish(record newoutbox.OutboxRecord) error {
	return a.chatSvc.ReplayPostProcess(record.EventType, record.Payload)
}

type noopPublisher struct{}

func (p *noopPublisher) Publish(record newoutbox.OutboxRecord) error {
	return nil
}

type runtimeEventSinkHolder struct {
	mu   sync.Mutex
	sink runtime.RuntimeEventSink
}

func (h *runtimeEventSinkHolder) OnRuntimeEvent(ctx context.Context, event runtime.RuntimeDomainEvent) error {
	h.mu.Lock()
	sink := h.sink
	h.mu.Unlock()
	if sink == nil {
		return errors.New("runtime event sink is not configured")
	}
	return sink.OnRuntimeEvent(ctx, event)
}

func (h *runtimeEventSinkHolder) Set(sink runtime.RuntimeEventSink) {
	h.mu.Lock()
	h.sink = sink
	h.mu.Unlock()
}

type BehaviorRuntimeEventSink struct {
	installRepo installation.Repository
	engine      *behavior.BehaviorEngine
}

func NewBehaviorRuntimeEventSink(installRepo installation.Repository, engine *behavior.BehaviorEngine) *BehaviorRuntimeEventSink {
	return &BehaviorRuntimeEventSink{installRepo: installRepo, engine: engine}
}

func (s *BehaviorRuntimeEventSink) OnRuntimeEvent(ctx context.Context, event runtime.RuntimeDomainEvent) error {
	if s.engine == nil {
		return nil
	}
	characterID, petInstanceID := s.resolveCharacterAndPet(event)
	now := time.Now()

	switch event.EventType {
	case "clicked":
		builder := events.NewEnvelope("runtime.pointer.clicked", behavior.OriginDesktop).
			UserID(event.UserID).
			CharacterID(characterID).
			PetInstanceID(petInstanceID).
			InstallationID(event.InstallationID).
			OccurredAt(event.Timestamp).
			DedupKey(events.BuildDedupKey(petInstanceID, event.EventType, event.Timestamp.Format(time.RFC3339Nano)))
		if len(event.Payload) > 0 {
			builder.PayloadRaw(event.Payload)
		}
		return s.engine.SubmitEvent(ctx, builder.Build(now))

	case "dragged":
		builder := events.NewEnvelope("runtime.drag.completed", behavior.OriginDesktop).
			UserID(event.UserID).
			CharacterID(characterID).
			PetInstanceID(petInstanceID).
			InstallationID(event.InstallationID).
			OccurredAt(event.Timestamp).
			DedupKey(events.BuildDedupKey(petInstanceID, "drag_completed", event.Timestamp.Format(time.RFC3339Nano)))
		if len(event.Payload) > 0 {
			builder.PayloadRaw(event.Payload)
		}
		return s.engine.SubmitEvent(ctx, builder.Build(now))

	case "playback_completed":
		return s.submitPlaybackEvent(ctx, event, characterID, petInstanceID, "runtime.playback.action_completed", now)

	case "playback_interrupted":
		return s.submitPlaybackEvent(ctx, event, characterID, petInstanceID, "runtime.playback.action_interrupted", now)
	}

	return nil
}

func (s *BehaviorRuntimeEventSink) submitPlaybackEvent(ctx context.Context, event runtime.RuntimeDomainEvent, characterID, petInstanceID, eventType string, now time.Time) error {
	commandID := ""
	decisionID := ""
	actionKey := ""
	if len(event.Payload) > 0 {
		var payload map[string]interface{}
		if json.Unmarshal(event.Payload, &payload) == nil {
			if v, ok := payload["commandId"].(string); ok {
				commandID = v
			}
			if v, ok := payload["decisionId"].(string); ok {
				decisionID = v
			}
			if v, ok := payload["actionKey"].(string); ok {
				actionKey = v
			}
		}
	}

	builder := events.NewEnvelope(eventType, behavior.OriginPlayback).
		UserID(event.UserID).
		CharacterID(characterID).
		PetInstanceID(petInstanceID).
		InstallationID(event.InstallationID).
		OccurredAt(event.Timestamp).
		DedupKey(events.BuildDedupKey(commandID, eventType))

	if commandID != "" {
		builder.PayloadField("commandId", commandID)
	}
	if decisionID != "" {
		builder.PayloadField("decisionId", decisionID)
	}
	if actionKey != "" {
		builder.PayloadField("actionKey", actionKey)
	}

	return s.engine.SubmitEvent(ctx, builder.Build(now))
}

func (s *BehaviorRuntimeEventSink) resolveCharacterAndPet(event runtime.RuntimeDomainEvent) (string, string) {
	characterID := event.CharacterID
	petInstanceID := ""

	if len(event.Payload) > 0 {
		var payload map[string]interface{}
		if json.Unmarshal(event.Payload, &payload) == nil {
			if v, ok := payload["petInstanceId"].(string); ok && v != "" {
				petInstanceID = v
			}
		}
	}

	if characterID == "" && event.InstallationID != "" && s.installRepo != nil {
		inst, err := s.installRepo.GetInstallation(event.InstallationID)
		if err == nil && inst != nil {
			characterID = inst.CharacterID
		}
	}

	return characterID, petInstanceID
}

var _ runtime.RuntimeEventSink = (*runtimeEventSinkHolder)(nil)
var _ runtime.RuntimeEventSink = (*BehaviorRuntimeEventSink)(nil)

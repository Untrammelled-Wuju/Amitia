package management

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/gamehost/control"
	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/handshake"
	"github.com/u-ai/backend/internal/gamehost/readiness"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type KernelManagementReader interface {
	ListGameCenterExtensions(ctx context.Context) ([]kerneldomain.ExtensionDefinition, []kerneldomain.ExtensionInstallation, error)
	GetGameCenterExtension(ctx context.Context, extensionID string) (*kerneldomain.ExtensionDefinition, *kerneldomain.ExtensionInstallation, error)
}

type PluginRegistryReader interface {
	Get(ctx context.Context, pluginID string) (ghdomain.PluginDescriptor, error)
	ListByExtension(ctx context.Context, extensionID string) ([]ghdomain.PluginDescriptor, error)
	List(ctx context.Context) ([]ghdomain.PluginDescriptor, error)
}

type RuntimeManagerReader interface {
	ListRuntimes() []*runtime.RuntimeInstanceRef
	GetRuntime(runtimeID string) (*runtime.RuntimeInstanceRef, error)
}

type RuntimeTopologyReader interface {
	GetTopologySnapshot(runtimeID string) (runtime.RuntimeTopologySnapshot, error)
	ResolveDefinitionID(runtimeID, serviceID string) (string, error)
	ResolveModuleID(runtimeID, serviceID string) (string, error)
}

type HandshakeReader interface {
	GetState(connectionID string) (handshake.HandshakeState, bool)
	GetSnapshot(connectionID string) *handshake.HandshakeSnapshot
	IsReady(connectionID string) bool
}

type ControlAuthorityReader interface {
	GetSnapshot(ctx context.Context, runtimeID string) (control.ControlAuthoritySnapshot, bool)
	List(ctx context.Context) ([]control.ControlAuthoritySnapshot, error)
}

type HealthReader interface {
	GetServiceHealth(runtimeID string, serviceID string) (runtime.ServiceHealthSnapshot, bool)
	ListServiceHealth(runtimeID string) []runtime.ServiceHealthSnapshot
}

// AgentContextBinder is implemented by the GameHost container/bridge. It lets
// the normal Game Center session activation flow bind a runtime+service to the
// currently selected Agent before any tool invocation occurs.
type AgentContextBinder interface {
	BindAgentContext(context.Context, capability.RuntimeBinding, capability.ToolInvocationContext) error
}

type GameCenterManagementService struct {
	kernel      KernelManagementReader
	registry    PluginRegistryReader
	runtimes    RuntimeManagerReader
	topology    RuntimeTopologyReader
	handshake   HandshakeReader
	authority   ControlAuthorityReader
	health      HealthReader
	readiness   readiness.Reader
	agentBinder AgentContextBinder
}

type GameCenterManagementServiceOptions struct {
	Kernel      KernelManagementReader
	Registry    PluginRegistryReader
	Runtimes    RuntimeManagerReader
	Topology    RuntimeTopologyReader
	Handshake   HandshakeReader
	Authority   ControlAuthorityReader
	Health      HealthReader
	Readiness   readiness.Reader
	AgentBinder AgentContextBinder
}

func NewGameCenterManagementService(opts GameCenterManagementServiceOptions) *GameCenterManagementService {
	return &GameCenterManagementService{
		kernel:      opts.Kernel,
		registry:    opts.Registry,
		runtimes:    opts.Runtimes,
		topology:    opts.Topology,
		handshake:   opts.Handshake,
		authority:   opts.Authority,
		health:      opts.Health,
		readiness:   opts.Readiness,
		agentBinder: opts.AgentBinder,
	}
}

func (s *GameCenterManagementService) ListPlugins(ctx context.Context, filter PluginFilter) (*GameCenterPluginList, error) {
	if s.kernel == nil {
		return &GameCenterPluginList{Items: []GamePluginSummaryDTO{}, Total: 0, Page: filter.Page, PageSize: filter.PageSize}, nil
	}
	if s.registry == nil {
		return nil, fmt.Errorf("plugin registry not available")
	}

	defs, insts, err := s.kernel.ListGameCenterExtensions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list game center extensions: %w", err)
	}

	instMap := make(map[kerneldomain.ExtensionID]kerneldomain.ExtensionInstallation)
	for _, inst := range insts {
		instMap[inst.ExtensionID] = inst
	}

	var items []GamePluginSummaryDTO
	for _, def := range defs {
		inst, installed := instMap[def.ID]
		if !installed {
			continue
		}

		plugins, err := s.registry.ListByExtension(ctx, string(def.ID))
		if err != nil || len(plugins) == 0 {
			continue
		}

		for i := range plugins {
			summary := s.aggregatePluginSummary(ctx, def, inst, plugins[i])
			if s.matchesPluginFilter(summary, filter) {
				items = append(items, summary)
			}
		}
	}

	total := len(items)
	paged := paginate(items, filter.Page, filter.PageSize)

	return &GameCenterPluginList{
		Items:    paged,
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}, nil
}

func (s *GameCenterManagementService) GetPlugin(ctx context.Context, extensionID, pluginID string) (*GamePluginDetailDTO, error) {
	if s.kernel == nil {
		return nil, fmt.Errorf("kernel reader not available")
	}
	if s.registry == nil {
		return nil, fmt.Errorf("plugin registry not available")
	}

	def, inst, err := s.kernel.GetGameCenterExtension(ctx, extensionID)
	if err != nil {
		return nil, fmt.Errorf("get extension: %w", err)
	}
	if def == nil {
		return nil, fmt.Errorf("extension not found")
	}

	if inst == nil {
		return nil, fmt.Errorf("extension not installed")
	}

	plugins, err := s.registry.ListByExtension(ctx, extensionID)
	if err != nil {
		return nil, fmt.Errorf("list plugins: %w", err)
	}

	var target *ghdomain.PluginDescriptor
	for i := range plugins {
		if string(plugins[i].ID) == pluginID {
			target = &plugins[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("plugin not found")
	}

	detail := s.aggregatePluginDetail(ctx, *def, *inst, *target)
	return &detail, nil
}

func (s *GameCenterManagementService) ListRuntimes(ctx context.Context, filter RuntimeFilter) (*GameCenterRuntimeList, error) {
	if s.runtimes == nil || s.registry == nil {
		return &GameCenterRuntimeList{Items: []GameRuntimeSummaryDTO{}, Total: 0}, nil
	}

	allRuntimes := s.runtimes.ListRuntimes()
	var items []GameRuntimeSummaryDTO

	for _, rt := range allRuntimes {
		plugin, err := s.registry.Get(ctx, string(rt.PluginID))
		if err != nil {
			continue
		}

		if s.pluginBelongsToGameCenter(plugin) {
			summary := s.aggregateRuntimeSummary(ctx, rt, plugin)
			if s.matchesRuntimeFilter(summary, filter) {
				items = append(items, summary)
			}
		}
	}

	total := len(items)
	paged := paginate(items, filter.Page, filter.PageSize)

	return &GameCenterRuntimeList{
		Items: paged,
		Total: total,
	}, nil
}

func (s *GameCenterManagementService) GetRuntime(ctx context.Context, runtimeID, pluginID string) (*GameRuntimeDetailDTO, error) {
	if s.runtimes == nil || s.registry == nil {
		return nil, fmt.Errorf("runtime manager not available")
	}

	rtRef, err := s.runtimes.GetRuntime(runtimeID)
	if err != nil {
		return nil, fmt.Errorf("runtime not found: %w", err)
	}

	if pluginID != "" && string(rtRef.PluginID) != pluginID {
		return nil, fmt.Errorf("runtime does not belong to specified plugin")
	}

	plugin, err := s.registry.Get(ctx, string(rtRef.PluginID))
	if err != nil {
		return nil, fmt.Errorf("plugin not found: %w", err)
	}

	if !s.pluginBelongsToGameCenter(plugin) {
		return nil, fmt.Errorf("runtime does not belong to game center")
	}

	detail := s.aggregateRuntimeDetail(ctx, rtRef, plugin)
	return &detail, nil
}

func (s *GameCenterManagementService) ListServices(ctx context.Context, runtimeID string) (*GameCenterServiceList, error) {
	if s.topology == nil {
		return &GameCenterServiceList{Items: []GameServiceDTO{}, Total: 0}, nil
	}

	snap, err := s.topology.GetTopologySnapshot(runtimeID)
	if err != nil {
		return nil, fmt.Errorf("topology snapshot: %w", err)
	}

	readinessSnapshot, _ := s.resolveRuntimeReadiness(ctx, runtimeID)
	services := make([]GameServiceDTO, 0, len(snap.Services))
	for _, svc := range snap.Services {
		definitionID, _ := s.topology.ResolveDefinitionID(runtimeID, string(svc.ServiceID))
		moduleID, _ := s.topology.ResolveModuleID(runtimeID, string(svc.ServiceID))
		connected, ready := false, false
		if serviceReadiness, found := readinessSnapshot.Service(svc.ServiceID); found {
			connected = serviceReadiness.Connected
			ready = serviceReadiness.Ready
		}
		dto := GameServiceDTO{
			ServiceID:    string(svc.ServiceID),
			RuntimeID:    string(svc.RuntimeID),
			DefinitionID: definitionID,
			ModuleID:     moduleID,
			State:        string(svc.State),
			Health:       s.serviceHealth(string(svc.RuntimeID), string(svc.ServiceID)),
			Connected:    connected,
			Ready:        ready,
		}
		services = append(services, dto)
	}

	return &GameCenterServiceList{
		Items: services,
		Total: len(services),
	}, nil
}

func (s *GameCenterManagementService) GetRuntimeHealth(ctx context.Context, runtimeID string) (*HealthSummaryDTO, error) {
	if s.health == nil || s.topology == nil {
		return &HealthSummaryDTO{Status: "unknown"}, nil
	}
	topology, err := s.topology.GetTopologySnapshot(runtimeID)
	if err != nil {
		return &HealthSummaryDTO{Status: "unknown"}, nil
	}
	services := s.health.ListServiceHealth(runtimeID)
	result := runtime.AggregateRuntimeHealth(topology, services)
	var latest time.Time
	for _, svc := range services {
		if svc.LastChangedAt.After(latest) {
			latest = svc.LastChangedAt
		}
	}
	summary := &HealthSummaryDTO{Status: string(result.Health), Message: result.Reason}
	if !latest.IsZero() {
		summary.UpdatedAt = &latest
	}
	return summary, nil
}

func (s *GameCenterManagementService) GetHandshakeStatus(ctx context.Context, connectionID string) (*HandshakeSummaryDTO, error) {
	if s.handshake == nil {
		return &HandshakeSummaryDTO{HandshakeState: "unavailable", Ready: false}, nil
	}

	state, found := s.handshake.GetState(connectionID)
	if !found {
		return &HandshakeSummaryDTO{HandshakeState: "not_found", Ready: false}, nil
	}

	snap := s.handshake.GetSnapshot(connectionID)
	result := &HandshakeSummaryDTO{
		HandshakeState: string(state),
		Ready:          state == handshake.HandshakeStateReady,
	}

	if snap != nil {
		result.Protocol = snap.Protocol
		result.SDKName = snap.SDKName
		result.SDKVersion = snap.SDKVersion
	}

	return result, nil
}

func (s *GameCenterManagementService) GetRuntimeHandshakeStatus(ctx context.Context, runtimeID string) (*HandshakeSummaryDTO, error) {
	if s.readiness == nil || s.handshake == nil {
		return &HandshakeSummaryDTO{HandshakeState: "unavailable", Ready: false}, nil
	}
	runtimeReadiness, err := s.resolveRuntimeReadiness(ctx, runtimeID)
	if err != nil {
		return &HandshakeSummaryDTO{HandshakeState: "not_found", Ready: false}, nil
	}

	result := &HandshakeSummaryDTO{Ready: runtimeReadiness.Ready}
	switch runtimeReadiness.Reason {
	case readiness.ReasonReady:
		result.HandshakeState = string(handshake.HandshakeStateReady)
	case readiness.ReasonRequiredServiceStale:
		result.HandshakeState = "stale_generation"
	case readiness.ReasonRequiredServiceDisconnected:
		result.HandshakeState = "not_found"
	case readiness.ReasonRequiredServiceNotRunning, readiness.ReasonRuntimeNotOperational:
		result.HandshakeState = "blocked"
	case readiness.ReasonNoActiveGeneration, readiness.ReasonTopologyEmpty, readiness.ReasonNoServiceReady:
		result.HandshakeState = "pending"
	default:
		result.HandshakeState = "pending"
	}

	for _, service := range runtimeReadiness.Services {
		if !service.Connected || service.ConnectionID == "" {
			continue
		}
		if snap := s.handshake.GetSnapshot(string(service.ConnectionID)); snap != nil && result.Protocol == "" {
			result.Protocol = snap.Protocol
			result.SDKName = snap.SDKName
			result.SDKVersion = snap.SDKVersion
		}
		// Only surface the concrete connection handshake state when handshake
		// readiness itself is the runtime blocker. Lifecycle/topology blockers
		// must never be presented as a misleading runtime-level "ready" state.
		if runtimeReadiness.Reason != readiness.ReasonRequiredServiceNotReady || !service.Required || service.Ready {
			continue
		}
		if state, found := s.handshake.GetState(string(service.ConnectionID)); found {
			result.HandshakeState = string(state)
		}
	}
	return result, nil
}

func (s *GameCenterManagementService) GetControlAuthority(ctx context.Context, runtimeID string) (*ControlAuthorityDTO, error) {
	if s.runtimes != nil && s.registry != nil {
		if rt, err := s.runtimes.GetRuntime(runtimeID); err == nil && rt != nil {
			if plugin, pluginErr := s.registry.Get(ctx, string(rt.PluginID)); pluginErr == nil && !pluginHasCapability(plugin, ghdomain.HostFeatureRealtimeControl) {
				return &ControlAuthorityDTO{RuntimeID: runtimeID, Mode: "unsupported", Epoch: 0}, nil
			}
		}
	}
	if s.authority == nil {
		return &ControlAuthorityDTO{Mode: "unavailable", Epoch: 0}, nil
	}

	snap, found := s.authority.GetSnapshot(ctx, runtimeID)
	if !found {
		return &ControlAuthorityDTO{Mode: "observe_only", Epoch: 1}, nil
	}

	var updatedAt time.Time
	if !snap.UpdatedAt.IsZero() {
		updatedAt = snap.UpdatedAt
	}

	return &ControlAuthorityDTO{
		RuntimeID: runtimeID,
		Mode:      string(snap.Mode),
		Epoch:     snap.Epoch,
		UpdatedAt: &updatedAt,
	}, nil
}

// BindAgentContext binds an active UI/host Agent scope to one or all process
// services of a GameHost runtime. The runtime identity is validated through
// the runtime manager and topology; callers cannot bind an arbitrary
// plugin/service tuple. Omitting serviceId intentionally applies the same host
// context to every process service so multi-service plugins are safe by
// default and the UI does not need to understand plugin-internal topology.
func (s *GameCenterManagementService) BindAgentContext(ctx context.Context, runtimeID string, req AgentContextBindRequest) error {
	runtimeID = strings.TrimSpace(runtimeID)
	if runtimeID == "" {
		return fmt.Errorf("runtimeId required")
	}
	if s.agentBinder == nil || s.runtimes == nil {
		return fmt.Errorf("game-center: Agent context binding is unavailable")
	}
	rtRef, err := s.runtimes.GetRuntime(runtimeID)
	if err != nil || rtRef == nil {
		return fmt.Errorf("runtime not found")
	}
	if s.topology == nil {
		return fmt.Errorf("runtime topology unavailable")
	}
	snapshot, err := s.topology.GetTopologySnapshot(runtimeID)
	if err != nil {
		return fmt.Errorf("runtime topology unavailable: %w", err)
	}
	requestedServiceID := strings.TrimSpace(req.ServiceID)
	serviceIDs := make([]string, 0, len(snapshot.Services))
	for _, svc := range snapshot.Services {
		if svc.ServiceKind != ghdomain.ServiceKindProcess {
			continue
		}
		serviceID := strings.TrimSpace(string(svc.ServiceID))
		if serviceID == "" {
			continue
		}
		if requestedServiceID != "" && serviceID != requestedServiceID {
			continue
		}
		serviceIDs = append(serviceIDs, serviceID)
	}
	if len(serviceIDs) == 0 {
		return fmt.Errorf("service not found")
	}
	if strings.TrimSpace(req.UserID) == "" && strings.TrimSpace(req.CharacterID) == "" && strings.TrimSpace(req.ConversationID) == "" {
		return fmt.Errorf("Agent target required")
	}
	invocation := capability.ToolInvocationContext{
		InvocationID:   "gamehost-context-bind",
		UserID:         strings.TrimSpace(req.UserID),
		CharacterID:    strings.TrimSpace(req.CharacterID),
		ConversationID: strings.TrimSpace(req.ConversationID),
		Channel:        strings.TrimSpace(req.Channel),
		SessionID:      strings.TrimSpace(req.SessionID),
		Source:         capability.InvocationSourceUser,
	}
	for _, serviceID := range serviceIDs {
		binding := capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeGameHost,
			RuntimeID:   runtimeID,
			Metadata: map[string]any{
				"pluginId":  string(rtRef.PluginID),
				"serviceId": serviceID,
			},
		}
		if err := s.agentBinder.BindAgentContext(ctx, binding, invocation); err != nil {
			return fmt.Errorf("bind Agent context for service %s: %w", serviceID, err)
		}
	}
	return nil
}

func (s *GameCenterManagementService) aggregatePluginSummary(
	ctx context.Context,
	def kerneldomain.ExtensionDefinition,
	inst kerneldomain.ExtensionInstallation,
	plugin ghdomain.PluginDescriptor,
) GamePluginSummaryDTO {
	runtimeCount := s.countRuntimesForPlugin(string(plugin.ID))
	health := s.pluginHealth(string(plugin.ID))
	enabled := inst.EnablementState == kerneldomain.EnablementEnabled
	installState := string(inst.InstallationState)
	caps := pluginCapabilityStrings(plugin)

	return GamePluginSummaryDTO{
		ExtensionID:      string(def.ID),
		PluginID:         string(plugin.ID),
		Name:             plugin.Name,
		Version:          plugin.Version,
		Description:      def.Name.Default,
		Enabled:          enabled,
		InstallState:     installState,
		Health:           health,
		RuntimeCount:     runtimeCount,
		ManagementTarget: string(kerneldomain.ManagementTargetGameCenter),
		Capabilities:     caps,
	}
}

func (s *GameCenterManagementService) aggregatePluginDetail(
	ctx context.Context,
	def kerneldomain.ExtensionDefinition,
	inst kerneldomain.ExtensionInstallation,
	plugin ghdomain.PluginDescriptor,
) GamePluginDetailDTO {
	caps := pluginCapabilityStrings(plugin)
	permissions := append([]string(nil), plugin.RequiredPermissions...)

	runtimes := s.listRuntimeSummariesForPlugin(ctx, string(plugin.ID))
	health := s.pluginHealth(string(plugin.ID))

	return GamePluginDetailDTO{
		ExtensionID:      string(def.ID),
		PluginID:         string(plugin.ID),
		Name:             plugin.Name,
		Version:          plugin.Version,
		Description:      def.Name.Default,
		Enabled:          inst.EnablementState == kerneldomain.EnablementEnabled,
		InstallState:     string(inst.InstallationState),
		PackageRevision:  inst.PackageID,
		ManagementTarget: string(kerneldomain.ManagementTargetGameCenter),
		Capabilities:     caps,
		Permissions:      permissions,
		Provider:         def.Publisher.DisplayName,
		Runtimes:         runtimes,
		HealthSummary:    &HealthSummaryDTO{Status: health},
	}
}

func (s *GameCenterManagementService) aggregateRuntimeSummary(
	ctx context.Context,
	rt *runtime.RuntimeInstanceRef,
	plugin ghdomain.PluginDescriptor,
) GameRuntimeSummaryDTO {
	serviceCount := s.countServices(string(rt.ID))
	connected, ready := s.runtimeConnectionState(ctx, string(rt.ID))
	controlMode, epoch := "", uint64(0)
	if pluginHasCapability(plugin, ghdomain.HostFeatureRealtimeControl) {
		controlMode, epoch = s.runtimeAuthority(ctx, string(rt.ID))
	}
	health := s.runtimeHealth(string(rt.ID))

	return GameRuntimeSummaryDTO{
		RuntimeID:      string(rt.ID),
		PluginID:       string(rt.PluginID),
		ExtensionID:    plugin.ExtensionID,
		State:          string(rt.State),
		Health:         health,
		ServiceCount:   serviceCount,
		Connected:      connected,
		Ready:          ready,
		ControlMode:    controlMode,
		AuthorityEpoch: epoch,
	}
}

func (s *GameCenterManagementService) aggregateRuntimeDetail(
	ctx context.Context,
	rt *runtime.RuntimeInstanceRef,
	plugin ghdomain.PluginDescriptor,
) GameRuntimeDetailDTO {
	services, serviceErr := s.ListServices(ctx, string(rt.ID))
	if serviceErr != nil || services == nil {
		services = &GameCenterServiceList{Items: []GameServiceDTO{}, Total: 0}
	}
	hs, _ := s.GetRuntimeHandshakeStatus(ctx, string(rt.ID))
	if hs == nil {
		hs = &HandshakeSummaryDTO{HandshakeState: "unavailable", Ready: false}
	}
	var authority *ControlAuthorityDTO
	if pluginHasCapability(plugin, ghdomain.HostFeatureRealtimeControl) {
		authority, _ = s.GetControlAuthority(ctx, string(rt.ID))
	}
	health, _ := s.GetRuntimeHealth(ctx, string(rt.ID))
	if health == nil {
		health = &HealthSummaryDTO{Status: "unknown"}
	}

	processRunning := readiness.IsOperationalRuntimeState(rt.State)
	processGeneration := uint64(0)
	if runtimeReadiness, err := s.resolveRuntimeReadiness(ctx, string(rt.ID)); err == nil {
		processRunning = runtimeReadiness.Operational
		if runtimeReadiness.Generation > 0 {
			processGeneration = uint64(runtimeReadiness.Generation)
		}
	}

	return GameRuntimeDetailDTO{
		RuntimeID:        string(rt.ID),
		PluginID:         string(rt.PluginID),
		ExtensionID:      plugin.ExtensionID,
		RuntimeState:     string(rt.State),
		Process:          &ProcessSummaryDTO{Managed: true, Running: processRunning, ProcessGeneration: processGeneration},
		Connection:       s.getConnectionSummary(ctx, string(rt.ID)),
		Handshake:        hs,
		Services:         services.Items,
		ControlAuthority: authority,
		HealthSummary:    health,
	}
}

func (s *GameCenterManagementService) pluginBelongsToGameCenter(plugin ghdomain.PluginDescriptor) bool {
	return plugin.ExtensionID != ""
}

func (s *GameCenterManagementService) countRuntimesForPlugin(pluginID string) int {
	if s.runtimes == nil {
		return 0
	}
	all := s.runtimes.ListRuntimes()
	count := 0
	for _, rt := range all {
		if string(rt.PluginID) == pluginID {
			count++
		}
	}
	return count
}

func (s *GameCenterManagementService) listRuntimeSummariesForPlugin(ctx context.Context, pluginID string) []GameRuntimeSummaryDTO {
	if s.runtimes == nil || s.registry == nil {
		return nil
	}
	all := s.runtimes.ListRuntimes()
	var result []GameRuntimeSummaryDTO
	for _, rt := range all {
		if string(rt.PluginID) != pluginID {
			continue
		}
		plugin, err := s.registry.Get(ctx, pluginID)
		if err != nil {
			continue
		}
		result = append(result, s.aggregateRuntimeSummary(ctx, rt, plugin))
	}
	return result
}

func (s *GameCenterManagementService) countServices(runtimeID string) int {
	if s.topology == nil {
		return 0
	}
	snap, err := s.topology.GetTopologySnapshot(runtimeID)
	if err != nil {
		return 0
	}
	return len(snap.Services)
}

func (s *GameCenterManagementService) pluginHealth(pluginID string) string {
	if s.runtimes == nil || s.health == nil || s.topology == nil {
		return "unknown"
	}
	all := s.runtimes.ListRuntimes()
	hasRuntime := false
	hasUnknown := false
	worst := "healthy"
	for _, rt := range all {
		if string(rt.PluginID) != pluginID {
			continue
		}
		hasRuntime = true
		health := s.runtimeHealth(string(rt.ID))
		switch health {
		case "unhealthy":
			return "unhealthy"
		case "degraded":
			worst = "degraded"
		case "unknown":
			hasUnknown = true
		}
	}
	if !hasRuntime {
		return "unknown"
	}
	if worst == "healthy" && hasUnknown {
		return "unknown"
	}
	return worst
}

func (s *GameCenterManagementService) runtimeHealth(runtimeID string) string {
	if s.health == nil || s.topology == nil {
		return "unknown"
	}
	topology, err := s.topology.GetTopologySnapshot(runtimeID)
	if err != nil {
		return "unknown"
	}
	result := runtime.AggregateRuntimeHealth(topology, s.health.ListServiceHealth(runtimeID))
	return string(result.Health)
}

func (s *GameCenterManagementService) serviceHealth(runtimeID, serviceID string) string {
	if s.health == nil {
		return "unknown"
	}
	snap, found := s.health.GetServiceHealth(runtimeID, serviceID)
	if !found {
		return "unknown"
	}
	return string(snap.Health)
}

func (s *GameCenterManagementService) runtimeConnectionState(ctx context.Context, runtimeID string) (connected, ready bool) {
	runtimeReadiness, err := s.resolveRuntimeReadiness(ctx, runtimeID)
	if err != nil {
		return false, false
	}
	return runtimeReadiness.Connected, runtimeReadiness.Ready
}

func (s *GameCenterManagementService) runtimeAuthority(ctx context.Context, runtimeID string) (mode string, epoch uint64) {
	if s.authority == nil {
		return "unavailable", 0
	}
	snap, found := s.authority.GetSnapshot(ctx, runtimeID)
	if !found {
		return "observe_only", 1
	}
	return string(snap.Mode), snap.Epoch
}

func pluginCapabilityStrings(plugin ghdomain.PluginDescriptor) []string {
	result := make([]string, 0, len(plugin.Capabilities))
	for _, capability := range plugin.Capabilities {
		result = append(result, string(capability))
	}
	return result
}

func pluginHasCapability(plugin ghdomain.PluginDescriptor, capability ghdomain.HostFeature) bool {
	for _, declared := range plugin.Capabilities {
		if ghdomain.HostFeature(declared) == capability {
			return true
		}
	}
	return false
}

func (s *GameCenterManagementService) matchesPluginFilter(summary GamePluginSummaryDTO, filter PluginFilter) bool {
	if filter.Search != "" {
		search := strings.ToLower(filter.Search)
		if !strings.Contains(strings.ToLower(summary.Name), search) &&
			!strings.Contains(strings.ToLower(summary.PluginID), search) &&
			!strings.Contains(strings.ToLower(summary.ExtensionID), search) {
			return false
		}
	}
	if filter.Status != "" && summary.InstallState != filter.Status {
		return false
	}
	if filter.Enabled != nil && summary.Enabled != *filter.Enabled {
		return false
	}
	return true
}

func (s *GameCenterManagementService) matchesRuntimeFilter(summary GameRuntimeSummaryDTO, filter RuntimeFilter) bool {
	if filter.PluginID != "" && summary.PluginID != filter.PluginID {
		return false
	}
	if filter.Status != "" && summary.State != filter.Status {
		return false
	}
	return true
}

func (s *GameCenterManagementService) getConnectionSummary(ctx context.Context, runtimeID string) *ConnectionSummaryDTO {
	runtimeReadiness, err := s.resolveRuntimeReadiness(ctx, runtimeID)
	if err != nil {
		return &ConnectionSummaryDTO{Connected: false}
	}
	result := &ConnectionSummaryDTO{Connected: runtimeReadiness.Connected}
	if !runtimeReadiness.Connected {
		return result
	}
	if runtimeReadiness.Generation > 0 {
		result.PeerGeneration = uint64(runtimeReadiness.Generation)
	}
	if s.handshake != nil {
		for _, service := range runtimeReadiness.Services {
			if !service.Connected || service.ConnectionID == "" {
				continue
			}
			if snap := s.handshake.GetSnapshot(string(service.ConnectionID)); snap != nil {
				result.ProtocolVersion = snap.Protocol
				break
			}
		}
	}
	return result
}

func (s *GameCenterManagementService) resolveRuntimeReadiness(ctx context.Context, runtimeID string) (readiness.Snapshot, error) {
	if s == nil || s.readiness == nil {
		return readiness.Snapshot{}, fmt.Errorf("runtime readiness resolver not available")
	}
	return s.readiness.Resolve(ctx, ghdomain.RuntimeInstanceID(runtimeID))
}

func paginate[T any](items []T, page, pageSize int) []T {
	if pageSize <= 0 {
		return items
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

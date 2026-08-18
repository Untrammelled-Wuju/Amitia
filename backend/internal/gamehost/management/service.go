package management

import (
	"context"
	"fmt"
	"strings"
	"time"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/gamehost/control"
	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/handshake"
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
}

type ConnectionRegistryReader interface {
	ListByRuntime(runtimeID string) []*ConnectionSnapshot
	FindByPeer(runtimeID, serviceID string) (*ConnectionSnapshot, bool)
}

type ConnectionSnapshot struct {
	ConnectionID string
	RuntimeID    string
	ServiceID    string
	Connected    bool
	Protocol     string
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

type GameCenterManagementService struct {
	kernel      KernelManagementReader
	registry    PluginRegistryReader
	runtimes    RuntimeManagerReader
	topology    RuntimeTopologyReader
	connections ConnectionRegistryReader
	handshake   HandshakeReader
	authority   ControlAuthorityReader
	health      HealthReader
}

type GameCenterManagementServiceOptions struct {
	Kernel      KernelManagementReader
	Registry    PluginRegistryReader
	Runtimes    RuntimeManagerReader
	Topology    RuntimeTopologyReader
	Connections ConnectionRegistryReader
	Handshake   HandshakeReader
	Authority   ControlAuthorityReader
	Health      HealthReader
}

func NewGameCenterManagementService(opts GameCenterManagementServiceOptions) *GameCenterManagementService {
	return &GameCenterManagementService{
		kernel:      opts.Kernel,
		registry:    opts.Registry,
		runtimes:    opts.Runtimes,
		topology:    opts.Topology,
		connections: opts.Connections,
		handshake:   opts.Handshake,
		authority:   opts.Authority,
		health:      opts.Health,
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

	services := make([]GameServiceDTO, 0, len(snap.Services))
	for _, svc := range snap.Services {
		dto := GameServiceDTO{
			ServiceID:    string(svc.ID),
			RuntimeID:    string(svc.RuntimeID),
			DefinitionID: string(svc.ServiceID),
			State:        string(svc.State),
			Health:       s.serviceHealth(string(svc.RuntimeID), string(svc.ServiceID)),
			Connected:    s.serviceConnected(string(svc.RuntimeID), string(svc.ServiceID)),
			Ready:        s.serviceReady(string(svc.RuntimeID), string(svc.ServiceID)),
		}
		services = append(services, dto)
	}

	return &GameCenterServiceList{
		Items: services,
		Total: len(services),
	}, nil
}

func (s *GameCenterManagementService) GetRuntimeHealth(ctx context.Context, runtimeID string) (*HealthSummaryDTO, error) {
	if s.health == nil {
		return &HealthSummaryDTO{Status: "unknown"}, nil
	}

	services := s.health.ListServiceHealth(runtimeID)
	if len(services) == 0 {
		return &HealthSummaryDTO{Status: "unknown"}, nil
	}

	worst := "healthy"
	var latest time.Time
	for _, svc := range services {
		if svc.Health == "unhealthy" {
			worst = "unhealthy"
		} else if svc.Health == "degraded" && worst != "unhealthy" {
			worst = "degraded"
		}
		if svc.LastChangedAt.After(latest) {
			latest = svc.LastChangedAt
		}
	}

	return &HealthSummaryDTO{
		Status:    worst,
		UpdatedAt: &latest,
	}, nil
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
	if s.connections == nil || s.handshake == nil {
		return &HandshakeSummaryDTO{HandshakeState: "unavailable", Ready: false}, nil
	}
	connections := s.connections.ListByRuntime(runtimeID)
	if len(connections) == 0 {
		return &HandshakeSummaryDTO{HandshakeState: "not_found", Ready: false}, nil
	}

	result := &HandshakeSummaryDTO{HandshakeState: "ready", Ready: true}
	for _, conn := range connections {
		if conn == nil || !conn.Connected {
			result.HandshakeState = "pending"
			result.Ready = false
			continue
		}
		state, found := s.handshake.GetState(conn.ConnectionID)
		if !found || state != handshake.HandshakeStateReady {
			if found {
				result.HandshakeState = string(state)
			} else {
				result.HandshakeState = "not_found"
			}
			result.Ready = false
		}
		if snap := s.handshake.GetSnapshot(conn.ConnectionID); snap != nil && result.Protocol == "" {
			result.Protocol = snap.Protocol
			result.SDKName = snap.SDKName
			result.SDKVersion = snap.SDKVersion
		}
	}
	return result, nil
}

func (s *GameCenterManagementService) GetControlAuthority(ctx context.Context, runtimeID string) (*ControlAuthorityDTO, error) {
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
	}
}

func (s *GameCenterManagementService) aggregatePluginDetail(
	ctx context.Context,
	def kerneldomain.ExtensionDefinition,
	inst kerneldomain.ExtensionInstallation,
	plugin ghdomain.PluginDescriptor,
) GamePluginDetailDTO {
	caps := make([]string, 0, len(plugin.Capabilities))
	for _, c := range plugin.Capabilities {
		caps = append(caps, string(c))
	}

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
	connected, ready := s.runtimeConnectionState(string(rt.ID))
	controlMode, epoch := s.runtimeAuthority(ctx, string(rt.ID))
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
	services, _ := s.ListServices(ctx, string(rt.ID))
	hs, _ := s.GetHandshakeStatus(ctx, string(rt.ID))
	authority, _ := s.GetControlAuthority(ctx, string(rt.ID))
	health, _ := s.GetRuntimeHealth(ctx, string(rt.ID))

	return GameRuntimeDetailDTO{
		RuntimeID:        string(rt.ID),
		PluginID:         string(rt.PluginID),
		ExtensionID:      plugin.ExtensionID,
		RuntimeState:     string(rt.State),
		Process:          &ProcessSummaryDTO{Managed: true, Running: rt.State == ghdomain.RuntimeStateRunning},
		Connection:       s.getConnectionSummary(string(rt.ID)),
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
	if s.runtimes == nil || s.health == nil {
		return "unknown"
	}
	all := s.runtimes.ListRuntimes()
	hasRuntime := false
	worst := "healthy"
	for _, rt := range all {
		if string(rt.PluginID) != pluginID {
			continue
		}
		hasRuntime = true
		health := s.runtimeHealth(string(rt.ID))
		if health == "unhealthy" {
			return "unhealthy"
		}
		if health == "degraded" {
			worst = "degraded"
		}
	}
	if !hasRuntime {
		return "unknown"
	}
	return worst
}

func (s *GameCenterManagementService) runtimeHealth(runtimeID string) string {
	if s.health == nil {
		return "unknown"
	}
	services := s.health.ListServiceHealth(runtimeID)
	if len(services) == 0 {
		return "unknown"
	}
	worst := "healthy"
	for _, svc := range services {
		if svc.Health == "unhealthy" {
			return "unhealthy"
		}
		if svc.Health == "degraded" && worst != "unhealthy" {
			worst = "degraded"
		}
	}
	return worst
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

func (s *GameCenterManagementService) serviceConnected(runtimeID, serviceID string) bool {
	if s.connections == nil {
		return false
	}
	_, found := s.connections.FindByPeer(runtimeID, serviceID)
	return found
}

func (s *GameCenterManagementService) serviceReady(runtimeID, serviceID string) bool {
	if s.connections == nil || s.handshake == nil {
		return false
	}
	conn, found := s.connections.FindByPeer(runtimeID, serviceID)
	if !found {
		return false
	}
	return s.handshake.IsReady(conn.ConnectionID)
}

func (s *GameCenterManagementService) runtimeConnectionState(runtimeID string) (connected, ready bool) {
	if s.connections == nil {
		return false, false
	}
	conns := s.connections.ListByRuntime(runtimeID)
	if len(conns) == 0 {
		return false, false
	}
	connected = true
	ready = true
	for _, conn := range conns {
		if s.handshake != nil {
			if s.handshake.IsReady(conn.ConnectionID) {
				continue
			}
		}
		ready = false
	}
	return connected, ready
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

func (s *GameCenterManagementService) getConnectionSummary(runtimeID string) *ConnectionSummaryDTO {
	if s.connections == nil {
		return &ConnectionSummaryDTO{Connected: false}
	}
	conns := s.connections.ListByRuntime(runtimeID)
	if len(conns) == 0 {
		return &ConnectionSummaryDTO{Connected: false}
	}
	result := &ConnectionSummaryDTO{Connected: true}
	if s.handshake != nil && len(conns) > 0 {
		snap := s.handshake.GetSnapshot(conns[0].ConnectionID)
		if snap != nil {
			result.ProtocolVersion = snap.Protocol
		}
	}
	return result
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

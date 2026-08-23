package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/script_host"
	kernelsecret "github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/integration/service_definition"
	"github.com/u-ai/backend/internal/gamehost/registry"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
	gamehostsecret "github.com/u-ai/backend/internal/gamehost/secret"
	gameprotocol "github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type InstalledGeneration struct {
	Path         string
	GenerationID string
	TreeHash     string
	Version      string
}

// InstalledGenerationResolver resolves the package generation selected by the
// extension package lifecycle. GameHost never reconstructs package paths when
// this authoritative resolver is available.
type InstalledGenerationResolver interface {
	ResolveInstalledGeneration(ctx context.Context, extensionID string) (InstalledGeneration, error)
}

type StartupSecretManifestRegistrar interface {
	RegisterStartupManifest(runtimeID, serviceID string, startup []gamehostsecret.ServiceSecretManifest)
	UnregisterStartupManifest(runtimeID, serviceID string)
	RemoveRuntimeStartupManifests(runtimeID string)
}

type RuntimeGraphProvisioner struct {
	source             KernelContributionSource
	mapper             GamePluginContributionMapper
	pluginRegistry     *registry.Registry
	runtimeManager     *ghruntime.Manager
	topologyStore      *ghruntime.TopologyStore
	supervisor         *trusted_service.ProcessSupervisor
	definitionMapper   *service_definition.DefinitionMapper
	secretRegistrar    StartupSecretManifestRegistrar
	extensionRoot      string
	nodeResolver       script_host.NodeEnvironmentResolver
	generationResolver InstalledGenerationResolver
	runtimeExecutor    ghruntime.RuntimeExecutor
}

type RuntimeGraphProvisionerOptions struct {
	Source             KernelContributionSource
	Mapper             GamePluginContributionMapper
	PluginRegistry     *registry.Registry
	RuntimeManager     *ghruntime.Manager
	TopologyStore      *ghruntime.TopologyStore
	Supervisor         *trusted_service.ProcessSupervisor
	DefinitionMapper   *service_definition.DefinitionMapper
	SecretRegistrar    StartupSecretManifestRegistrar
	ExtensionRoot      string
	NodeResolver       script_host.NodeEnvironmentResolver
	GenerationResolver InstalledGenerationResolver
}

func NewRuntimeGraphProvisioner(opts RuntimeGraphProvisionerOptions) (*RuntimeGraphProvisioner, error) {
	if opts.Source == nil {
		return nil, fmt.Errorf("runtime graph provisioner: source is required")
	}
	if opts.Mapper == nil {
		return nil, fmt.Errorf("runtime graph provisioner: mapper is required")
	}
	if opts.PluginRegistry == nil {
		return nil, fmt.Errorf("runtime graph provisioner: plugin registry is required")
	}
	if opts.RuntimeManager == nil {
		return nil, fmt.Errorf("runtime graph provisioner: runtime manager is required")
	}
	if opts.TopologyStore == nil {
		return nil, fmt.Errorf("runtime graph provisioner: topology store is required")
	}
	if opts.Supervisor == nil {
		return nil, fmt.Errorf("runtime graph provisioner: supervisor is required")
	}
	if opts.DefinitionMapper == nil {
		opts.DefinitionMapper = service_definition.NewDefinitionMapper()
	}
	return &RuntimeGraphProvisioner{
		source:             opts.Source,
		mapper:             opts.Mapper,
		pluginRegistry:     opts.PluginRegistry,
		runtimeManager:     opts.RuntimeManager,
		topologyStore:      opts.TopologyStore,
		supervisor:         opts.Supervisor,
		definitionMapper:   opts.DefinitionMapper,
		secretRegistrar:    opts.SecretRegistrar,
		extensionRoot:      opts.ExtensionRoot,
		nodeResolver:       opts.NodeResolver,
		generationResolver: opts.GenerationResolver,
	}, nil
}

func (p *RuntimeGraphProvisioner) SetRuntimeExecutor(executor ghruntime.RuntimeExecutor) {
	if p == nil {
		return
	}
	p.runtimeExecutor = executor
}

func (p *RuntimeGraphProvisioner) SetSecretRegistrar(registrar StartupSecretManifestRegistrar) {
	if p == nil {
		return
	}
	p.secretRegistrar = registrar
}

func (p *RuntimeGraphProvisioner) Reconcile(ctx context.Context) error {
	plugins, err := p.source.ListEnabledGamePlugins(ctx)
	if err != nil {
		return fmt.Errorf("list enabled game plugins: %w", err)
	}
	desired := make(map[ghdomain.PluginID]struct{}, len(plugins))
	for _, kp := range plugins {
		descriptor, mapErr := p.mapper.ToDescriptor(ctx, kp.Extension, kp.Contribution)
		if mapErr != nil {
			return fmt.Errorf("map enabled game plugin %s/%s: %w", kp.Extension.ID, kp.Contribution.ID, mapErr)
		}
		desired[descriptor.ID] = struct{}{}
		if err := p.reconcilePlugin(ctx, kp); err != nil {
			return fmt.Errorf("reconcile plugin %s/%s: %w", kp.Extension.ID, kp.Contribution.ID, err)
		}
	}
	if err := p.pruneOrphanRuntimes(ctx, desired); err != nil {
		return fmt.Errorf("prune orphan game runtimes: %w", err)
	}
	return nil
}

func (p *RuntimeGraphProvisioner) pruneOrphanRuntimes(ctx context.Context, desired map[ghdomain.PluginID]struct{}) error {
	for _, runtimeRef := range p.runtimeManager.ListRuntimes() {
		if runtimeRef == nil {
			continue
		}
		if _, keep := desired[runtimeRef.PluginID]; keep {
			continue
		}
		if ghdomain.IsActiveRuntimeState(runtimeRef.State) {
			if p.runtimeExecutor == nil {
				return fmt.Errorf("runtime %s for removed plugin %s is active but runtime executor is unavailable", runtimeRef.ID, runtimeRef.PluginID)
			}
			if err := p.runtimeExecutor.StopRuntime(ctx, runtimeRef.ID); err != nil {
				return fmt.Errorf("stop orphan runtime %s: %w", runtimeRef.ID, err)
			}
		}

		definitionIDs := make([]string, 0)
		if snapshot, err := p.topologyStore.GetTopologySnapshot(runtimeRef.ID); err == nil {
			for _, service := range snapshot.Services {
				if definitionID, resolveErr := p.topologyStore.ResolveDefinitionID(runtimeRef.ID, service.ServiceID); resolveErr == nil && definitionID != "" {
					definitionIDs = append(definitionIDs, definitionID)
				}
			}
		}
		if p.runtimeExecutor != nil {
			if err := p.runtimeExecutor.CleanupRuntime(ctx, runtimeRef.ID); err != nil {
				return fmt.Errorf("cleanup orphan runtime %s: %w", runtimeRef.ID, err)
			}
		}
		if p.secretRegistrar != nil {
			p.secretRegistrar.RemoveRuntimeStartupManifests(string(runtimeRef.ID))
		}
		if err := p.topologyStore.RemoveRuntime(runtimeRef.ID); err != nil && !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("remove orphan topology %s: %w", runtimeRef.ID, err)
		}
		if err := p.runtimeManager.RemoveRuntime(runtimeRef.ID); err != nil && !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("remove orphan runtime %s: %w", runtimeRef.ID, err)
		}
		for _, definitionID := range definitionIDs {
			if p.supervisor.HasDefinition(definitionID) {
				if err := p.supervisor.Unregister(definitionID); err != nil {
					return fmt.Errorf("unregister orphan service definition %s: %w", definitionID, err)
				}
			}
		}
	}
	return nil
}

func (p *RuntimeGraphProvisioner) reconcilePlugin(ctx context.Context, kp KernelGamePlugin) error {
	bootService, err := p.buildBootService(ctx, kp)
	if err != nil {
		return fmt.Errorf("build boot service: %w", err)
	}

	descriptor, err := p.mapper.ToDescriptor(ctx, kp.Extension, kp.Contribution)
	if err != nil {
		return fmt.Errorf("map to descriptor: %w", err)
	}

	runtime, _, err := p.runtimeManager.EnsurePrimaryRuntime(ctx, descriptor.ID)
	if err != nil {
		return fmt.Errorf("ensure primary runtime: %w", err)
	}

	metadata := map[string]string{"protocol": bootService.Protocol}
	svcView := service_definition.ServiceRuntimeView{
		ExtensionID:      string(kp.Extension.ID),
		ModuleID:         bootService.ModuleID,
		RuntimeType:      bootService.RuntimeType,
		Name:             bootService.Name,
		Description:      bootService.Name,
		PublisherID:      kp.Extension.Publisher.PublisherID,
		PublisherTrust:   kp.Extension.Publisher.TrustLevel,
		EntryPoint:       bootService.EntryPoint,
		ExecutablePath:   bootService.ExecutablePath,
		ExecutableSHA256: bootService.ExecutableSHA256,
		Arguments:        bootService.Arguments,
		IntegrityValue:   bootService.IntegrityValue,
		Dependencies:     bootService.Dependencies,
		Env:              bootService.Env,
		Metadata:         metadata,
		Network:          bootService.Network,
		Enabled:          true,
	}

	definitionID := svcView.ToDefinitionID()
	def, err := p.definitionMapper.MapToDefinition(svcView)
	if err != nil {
		return fmt.Errorf("map to definition: %w", err)
	}
	if p.supervisor.HasDefinition(definitionID) {
		existing, getErr := p.supervisor.GetDefinition(definitionID)
		if getErr != nil {
			return fmt.Errorf("read existing definition: %w", getErr)
		}
		if existing.ManifestHash != def.ManifestHash {
			if err := p.supervisor.Unregister(definitionID); err != nil {
				return fmt.Errorf("replace stale definition: %w", err)
			}
			if err := p.supervisor.Register(def); err != nil {
				return fmt.Errorf("register replacement definition: %w", err)
			}
		}
	} else if err := p.supervisor.Register(def); err != nil {
		return fmt.Errorf("register definition: %w", err)
	}

	bootServiceID := bootService.ID
	descriptor.Services = append(descriptor.Services, ghdomain.ServiceDescriptor{
		ID:       bootServiceID,
		Name:     bootService.Name,
		Kind:     ghdomain.ServiceKindProcess,
		Required: true,
	})
	definitionIDs := map[ghdomain.ServiceID]string{bootServiceID: definitionID}
	if err := p.topologyStore.PutRuntimeGraph(runtime, descriptor, definitionIDs); err != nil {
		return fmt.Errorf("put runtime graph: %w", err)
	}
	if err := p.topologyStore.BindModuleID(runtime.ID, bootServiceID, bootService.ModuleID); err != nil {
		return fmt.Errorf("bind runtime module: %w", err)
	}

	if p.secretRegistrar != nil {
		snapshot, err := p.topologyStore.GetTopologySnapshot(runtime.ID)
		if err != nil {
			return fmt.Errorf("get topology snapshot for binding validation: %w", err)
		}
		view := newTopologyServiceView(snapshot)
		grouped, errs := p.extractSecretManifestGrouped(kp, view)
		if len(errs) > 0 {
			return fmt.Errorf("secret binding validation failed: %v", errs[0])
		}
		for serviceID, manifest := range grouped {
			p.secretRegistrar.RegisterStartupManifest(string(runtime.ID), serviceID, manifest)
		}
	}
	return nil
}

type topologyServiceView struct {
	snapshot ghruntime.RuntimeTopologySnapshot
}

func newTopologyServiceView(snapshot ghruntime.RuntimeTopologySnapshot) *topologyServiceView {
	return &topologyServiceView{snapshot: snapshot}
}
func (v *topologyServiceView) HasService(serviceID string) bool {
	for _, svc := range v.snapshot.Services {
		if string(svc.ServiceID) == serviceID {
			return true
		}
	}
	return false
}
func (v *topologyServiceView) ExecutableServiceCount() int {
	count := 0
	for _, svc := range v.snapshot.Services {
		if svc.ServiceKind == ghdomain.ServiceKindProcess {
			count++
		}
	}
	return count
}
func (v *topologyServiceView) DefaultExecutableService() (string, bool) {
	for _, svc := range v.snapshot.Services {
		if svc.ServiceKind == ghdomain.ServiceKindProcess {
			return string(svc.ServiceID), true
		}
	}
	return "", false
}

func (p *RuntimeGraphProvisioner) extractSecretManifestGrouped(kp KernelGamePlugin, view gamehostsecret.TopologyServiceView) (map[string][]gamehostsecret.ServiceSecretManifest, []error) {
	grouped := make(map[string][]gamehostsecret.ServiceSecretManifest)
	var errs []error
	if len(kp.Extension.SecretRefs) == 0 {
		return grouped, nil
	}
	for _, sr := range kp.Extension.SecretRefs {
		if sr.Ref == "" {
			continue
		}
		manifest := gamehostsecret.ServiceSecretManifest{Ref: kernelsecret.SecretRef(sr.Ref), Purpose: gamehostsecret.Purpose(sr.Purpose), Required: sr.Required, ServiceID: sr.ServiceID}
		binded, err := gamehostsecret.ValidateAndBindSecretRef(manifest, view)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		grouped[binded.ServiceID] = append(grouped[binded.ServiceID], *binded)
	}
	return grouped, errs
}

func buildGameNetworkPolicy(spec *gameprotocol.GameNetworkPolicy, permissions []string) (trusted_service.ServiceNetworkPolicy, error) {
	if spec == nil || strings.TrimSpace(spec.Mode) == "" {
		return trusted_service.ServiceNetworkPolicy{}, nil
	}
	mode := strings.ToLower(strings.TrimSpace(spec.Mode))
	policy := trusted_service.ServiceNetworkPolicy{Mode: mode, Enforce: true, AuditAll: spec.AuditAll, RequireProxy: spec.RequireProxy}
	switch mode {
	case "none":
		return policy, nil
	case "loopback":
		policy.AllowOutbound = true
		policy.LoopbackOnly = true
		return policy, nil
	case "unrestricted":
		if !containsString(permissions, "service.network.request") {
			return trusted_service.ServiceNetworkPolicy{}, fmt.Errorf("unrestricted outbound network requires service.network.request")
		}
		policy.AllowOutbound = true
		return policy, nil
	case "restricted":
		if !containsString(permissions, "service.network.request") {
			return trusted_service.ServiceNetworkPolicy{}, fmt.Errorf("restricted outbound network requires service.network.request")
		}
		policy.AllowOutbound = true
		policy.AllowedDomains = append([]string(nil), spec.AllowedDomains...)
		policy.AllowedPorts = append([]int(nil), spec.AllowedPorts...)
		return policy, nil
	default:
		return trusted_service.ServiceNetworkPolicy{}, fmt.Errorf("unsupported mode %q", mode)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

type bootServiceInfo struct {
	ID               ghdomain.ServiceID
	ModuleID         string
	Name             string
	RuntimeType      string
	EntryPoint       string
	ExecutablePath   string
	ExecutableSHA256 string
	Arguments        []string
	IntegrityValue   string
	Dependencies     []trusted_service.LibraryDep
	Protocol         string
	Env              map[string]string
	Network          trusted_service.ServiceNetworkPolicy
}

func (p *RuntimeGraphProvisioner) buildBootService(ctx context.Context, kp KernelGamePlugin) (bootServiceInfo, error) {
	gameSpec, err := gameprotocol.ParseGamePluginSpec(kp.Contribution.Definition)
	if err != nil {
		return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: parse game plugin spec: %w", kp.Extension.ID, kp.Contribution.ID, err)
	}
	if err := gameSpec.Validate(); err != nil {
		return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: validate game plugin spec: %w", kp.Extension.ID, kp.Contribution.ID, err)
	}
	runtimeModuleID, ok := kp.Contribution.Definition["runtimeModuleId"].(string)
	if !ok || strings.TrimSpace(runtimeModuleID) == "" {
		return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: runtimeModuleId is required", kp.Extension.ID, kp.Contribution.ID)
	}
	module, found := kp.Extension.FindModule(kerneldomain.ModuleID(runtimeModuleID))
	if !found || module.Runtime == nil {
		return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: runtime module %s is unavailable", kp.Extension.ID, kp.Contribution.ID, runtimeModuleID)
	}
	if strings.TrimSpace(module.Runtime.EntryPoint) == "" {
		return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: entry point is required", kp.Extension.ID, kp.Contribution.ID)
	}

	name := kp.Contribution.Name.Default
	if name == "" {
		name = kp.Extension.Name.Default
	}
	if name == "" {
		name = string(kp.Contribution.ID)
	}
	serviceID := module.Runtime.ServiceID
	if serviceID == "" {
		serviceID = string(module.ID)
	}
	protocolVersion := gameprotocol.ProtocolVersion
	if raw, ok := kp.Contribution.Definition["protocolVersion"].(string); ok && strings.TrimSpace(raw) != "" {
		protocolVersion = strings.TrimSpace(raw)
	}
	if protocolVersion != gameprotocol.ProtocolVersion {
		return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: unsupported protocolVersion %q", kp.Extension.ID, kp.Contribution.ID, protocolVersion)
	}

	networkPolicy, err := buildGameNetworkPolicy(gameSpec.Network, kp.Contribution.RequiredPermissions)
	if err != nil {
		return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: network policy: %w", kp.Extension.ID, kp.Contribution.ID, err)
	}

	info := bootServiceInfo{
		ID:          ghdomain.ServiceID(serviceID),
		ModuleID:    runtimeModuleID,
		Name:        name,
		RuntimeType: string(module.Runtime.Type),
		EntryPoint:  module.Runtime.EntryPoint,
		Protocol:    protocolVersion,
		Env:         module.Runtime.Env,
		Network:     networkPolicy,
	}
	if info.RuntimeType == "" {
		info.RuntimeType = service_definition.ServiceRuntimeType
	}
	if !service_definition.IsValidServiceRuntimeType(info.RuntimeType) {
		return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: unsupported game runtime type %q", kp.Extension.ID, kp.Contribution.ID, info.RuntimeType)
	}

	entryPath := module.Runtime.EntryPoint
	if p.generationResolver != nil {
		generation, resolveErr := p.generationResolver.ResolveInstalledGeneration(ctx, string(kp.Extension.ID))
		if resolveErr != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: resolve authoritative package generation: %w", kp.Extension.ID, kp.Contribution.ID, resolveErr)
		}
		if generation.Version != "" && generation.Version != fmt.Sprintf("%v", kp.Extension.Version) {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: package generation version %q does not match enabled definition version %q", kp.Extension.ID, kp.Contribution.ID, generation.Version, kp.Extension.Version)
		}
		entryPath, resolveErr = resolveGamePluginEntry(generation.Path, entryPath)
		if resolveErr != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: entry point: %w", kp.Extension.ID, kp.Contribution.ID, resolveErr)
		}
		if err := ensureRegularFile(entryPath); err != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: entry point: %w", kp.Extension.ID, kp.Contribution.ID, err)
		}
	} else if p.extensionRoot != "" {
		// Compatibility path for isolated tests/legacy embedders only. Production
		// wiring supplies generationResolver and therefore never guesses paths.
		definitionHash, hashErr := hashExtensionDefinition(kp.Extension)
		if hashErr != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: hash installed definition: %w", kp.Extension.ID, kp.Contribution.ID, hashErr)
		}
		bundlePath := resolveGamePluginBundlePath(p.extensionRoot, string(kp.Extension.ID), fmt.Sprintf("%v", kp.Extension.Version), definitionHash)
		if bundlePath == "" {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: installed bundle path not found", kp.Extension.ID, kp.Contribution.ID)
		}
		var resolveErr error
		entryPath, resolveErr = resolveGamePluginEntry(bundlePath, entryPath)
		if resolveErr != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: entry point: %w", kp.Extension.ID, kp.Contribution.ID, resolveErr)
		}
		if err := ensureRegularFile(entryPath); err != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: entry point: %w", kp.Extension.ID, kp.Contribution.ID, err)
		}
	}

	switch info.RuntimeType {
	case service_definition.JavaScriptRuntimeType:
		if p.nodeResolver == nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: managed Node resolver is unavailable", kp.Extension.ID, kp.Contribution.ID)
		}
		nodeEnv, err := p.nodeResolver.Resolve(ctx)
		if err != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: resolve managed Node: %w", kp.Extension.ID, kp.Contribution.ID, err)
		}
		if err := ensureRegularFile(nodeEnv.NodeBinary); err != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: managed Node: %w", kp.Extension.ID, kp.Contribution.ID, err)
		}
		sha, err := hashRuntimeFile(nodeEnv.NodeBinary)
		if err != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: hash managed Node: %w", kp.Extension.ID, kp.Contribution.ID, err)
		}
		info.ExecutablePath = nodeEnv.NodeBinary
		info.ExecutableSHA256 = sha
		info.IntegrityValue = "managed-node:sha256:" + sha
		entrySHA, hashErr := hashRuntimeFile(entryPath)
		if hashErr != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: hash JavaScript entry point: %w", kp.Extension.ID, kp.Contribution.ID, hashErr)
		}
		info.Dependencies = []trusted_service.LibraryDep{{
			Name:     "game-plugin-entrypoint",
			Path:     entryPath,
			Sha256:   entrySHA,
			Required: true,
		}}
		info.Arguments = []string{entryPath}
	default:
		info.ExecutablePath = entryPath
		if _, err := os.Stat(entryPath); err == nil {
			sha, hashErr := hashRuntimeFile(entryPath)
			if hashErr != nil {
				return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: hash runtime executable: %w", kp.Extension.ID, kp.Contribution.ID, hashErr)
			}
			info.ExecutableSHA256 = sha
			info.IntegrityValue = "package-runtime:sha256:" + sha
		}
	}
	return info, nil
}

func resolveGamePluginBundlePath(extRoot, extensionID, version, definitionHash string) string {
	if extRoot == "" || extensionID == "" {
		return ""
	}
	safeID := strings.NewReplacer("/", "__", "\\", "__", ":", "_", "..", "_").Replace(extensionID)
	installedRoot := filepath.Join(extRoot, "installed", safeID)

	if version != "" && definitionHash != "" {
		candidate := filepath.Join(installedRoot, version, definitionHash)
		if _, err := os.Stat(filepath.Join(candidate, "manifest.json")); err == nil {
			return candidate
		}
	}

	if version != "" {
		return newestInstalledGeneration(filepath.Join(installedRoot, version))
	}

	versions, err := os.ReadDir(installedRoot)
	if err != nil {
		return ""
	}
	for i := len(versions) - 1; i >= 0; i-- {
		if !versions[i].IsDir() {
			continue
		}
		if candidate := newestInstalledGeneration(filepath.Join(installedRoot, versions[i].Name())); candidate != "" {
			return candidate
		}
	}
	return ""
}

func newestInstalledGeneration(versionDir string) string {
	artifacts, err := os.ReadDir(versionDir)
	if err != nil {
		return ""
	}
	for i := len(artifacts) - 1; i >= 0; i-- {
		if !artifacts[i].IsDir() {
			continue
		}
		candidate := filepath.Join(versionDir, artifacts[i].Name())
		if _, err := os.Stat(filepath.Join(candidate, "manifest.json")); err == nil {
			return candidate
		}
	}
	return ""
}

func hashExtensionDefinition(def kerneldomain.ExtensionDefinition) (string, error) {
	data, err := json.Marshal(def)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func resolveGamePluginEntry(bundlePath, entryPoint string) (string, error) {
	if filepath.IsAbs(entryPoint) {
		return "", fmt.Errorf("absolute entry point is not allowed")
	}
	bundleAbs, err := filepath.Abs(bundlePath)
	if err != nil {
		return "", err
	}
	entryAbs, err := filepath.Abs(filepath.Join(bundleAbs, filepath.Clean(entryPoint)))
	if err != nil {
		return "", err
	}
	if entryAbs == bundleAbs || !strings.HasPrefix(entryAbs, bundleAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("entry point escapes installed bundle")
	}
	return entryAbs, nil
}

func ensureRegularFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	return nil
}

func hashRuntimeFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

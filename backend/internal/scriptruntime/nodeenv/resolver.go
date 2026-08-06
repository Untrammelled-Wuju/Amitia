// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package nodeenv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/resourceuri"
	"github.com/u-ai/backend/pkg/util"
)

type Resolver interface {
	Resolve(context.Context) (Environment, error)
	Snapshot() DetectionSnapshot
}

type ResolveContext struct {
	Config    *config.Config
	Host      runtimehost.RuntimeHost
	Inspector FileInspector
}

func NewResolver(ctx ResolveContext) (Resolver, error) {
	if ctx.Config == nil {
		return nil, errors.New("nodeenv: config is nil")
	}
	if ctx.Host == nil {
		return nil, errors.New("nodeenv: host is nil")
	}
	inspector := ctx.Inspector
	if inspector == nil {
		inspector = newDefaultFileInspector()
	}
	return &nodeEnvResolver{
		config:    ctx.Config,
		host:      ctx.Host,
		inspector: inspector,
	}, nil
}

type nodeEnvResolver struct {
	config    *config.Config
	host      runtimehost.RuntimeHost
	inspector FileInspector

	mu          sync.Mutex
	once        sync.Once
	result      Environment
	lastErr     error
	snapshot    DetectionSnapshot
	resolved    bool
	diagnostics []CandidateDiagnostic
}

func (r *nodeEnvResolver) Resolve(ctx context.Context) (Environment, error) {
	r.mu.Lock()
	if r.resolved {
		r.mu.Unlock()
		return r.result, r.lastErr
	}
	r.mu.Unlock()

	r.once.Do(func() {
		r.diagnostics = nil
		r.result, r.lastErr = r.detect(ctx)
		r.snapshot = r.buildSnapshot(r.result, r.lastErr)
		r.mu.Lock()
		r.resolved = true
		r.mu.Unlock()
	})

	return r.result, r.lastErr
}

func (r *nodeEnvResolver) Snapshot() DetectionSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.resolved {
		return DetectionSnapshot{
			State:      DetectionStateNotStarted,
			DetectedAt: time.Time{},
		}
	}
	return r.snapshot.clone()
}

func (r *nodeEnvResolver) buildSnapshot(env Environment, err error) DetectionSnapshot {
	snap := DetectionSnapshot{
		Diagnostics: r.cloneDiagnostics(),
		Environment: env.Clone(),
		DetectedAt:  time.Now().UTC(),
	}
	if err != nil {
		snap.State = DetectionStateFailed
		snap.LastError = err.Error()
		return snap
	}
	if env.PackageManagementAvailable {
		snap.State = DetectionStateReady
	} else {
		snap.State = DetectionStatePartial
	}
	return snap
}

func (r *nodeEnvResolver) cloneDiagnostics() []CandidateDiagnostic {
	if len(r.diagnostics) == 0 {
		return nil
	}
	out := make([]CandidateDiagnostic, len(r.diagnostics))
	copy(out, r.diagnostics)
	return out
}

func (r *nodeEnvResolver) addDiagnostic(kind CandidateKind, source Source, path string, result CandidateResult, errMsg string) {
	r.diagnostics = append(r.diagnostics, CandidateDiagnostic{
		Kind:   kind,
		Source: source,
		Path:   path,
		Result: result,
		Error:  errMsg,
	})
}

func (r *nodeEnvResolver) detect(ctx context.Context) (Environment, error) {
	if err := ctx.Err(); err != nil {
		return Environment{}, err
	}

	providerCfg := r.config.Providers.ScriptRuntime
	if !providerCfg.Enabled {
		return Environment{}, ErrScriptRuntimeDisabled
	}
	if providerCfg.Provider != "builtin.node-process" {
		return Environment{}, ErrNodeProviderNotSelected
	}

	caps := r.host.Capabilities()
	if !caps.RequirementSatisfied(runtimehost.CapabilityRequirement{ID: runtimehost.CapProcessSpawn, Minimum: runtimehost.SupportSupported}) {
		return Environment{}, newHostCapabilityError(string(runtimehost.CapProcessSpawn))
	}
	if !caps.RequirementSatisfied(runtimehost.CapabilityRequirement{ID: runtimehost.CapFilesystemLocal, Minimum: runtimehost.SupportSupported}) {
		return Environment{}, newHostCapabilityError(string(runtimehost.CapFilesystemLocal))
	}
	if !caps.RequirementSatisfied(runtimehost.CapabilityRequirement{ID: runtimehost.CapFilesystemExecutable, Minimum: runtimehost.SupportSupported}) {
		return Environment{}, newHostCapabilityError(string(runtimehost.CapFilesystemExecutable))
	}

	guest := r.host.Descriptor().Guest
	architecture := r.host.Descriptor().Architecture

	if !isSupportedGuest(guest) {
		return Environment{}, newUnsupportedGuest(guest)
	}

	env := Environment{
		Guest:        guest,
		Architecture: architecture,
	}

	paths := r.host.Paths()

	if err := r.detectNode(ctx, &env, providerCfg, paths); err != nil {
		return env, err
	}

	nodeBinary := env.NodeBinary
	distributionRoot := deriveDistributionRoot(nodeBinary, guest)
	env.DistributionRoot = distributionRoot

	if err := r.detectPackageManagers(ctx, &env, providerCfg, distributionRoot); err != nil {
		return env, err
	}

	if err := r.detectWorkDir(&env, providerCfg, distributionRoot); err != nil {
		return env, err
	}

	return env, nil
}

func (r *nodeEnvResolver) detectNode(ctx context.Context, env *Environment, providerCfg config.ScriptRuntimeProviderConfig, paths util.RuntimePaths) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	explicitPath := strings.TrimSpace(providerCfg.Node.BinaryPath)
	if explicitPath != "" {
		resolved, err := r.normalizeExplicitPath(explicitPath, paths.Root)
		if err != nil {
			return err
		}
		return r.resolveNodeFromExplicitPath(env, resolved)
	}

	if err := r.resolveNodeFromRuntimePackage(ctx, env, paths); err == nil {
		return nil
	} else if errors.Is(err, ErrNodeNotFound) {
	} else {
		return err
	}

	return r.resolveNodeFromLegacy(ctx, env, paths)
}

func (r *nodeEnvResolver) resolveNodeFromExplicitPath(env *Environment, path string) error {
	info, err := r.inspector.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			r.addDiagnostic(CandidateKindNode, SourceExplicit, path, CandidateResultNotFound, "path not found")
			return newNodeNotFound(SourceExplicit)
		}
		return err
	}
	if info.IsDir() {
		r.addDiagnostic(CandidateKindNode, SourceExplicit, path, CandidateResultInvalidFile, "path is a directory")
		return newInvalidNodeBinary(path, "path is a directory")
	}
	if env.Guest != platform.GuestPlatformWindows {
		if !hasExecPerm(info.Mode()) {
			r.addDiagnostic(CandidateKindNode, SourceExplicit, path, CandidateResultNotExecutable, "missing execute permission")
			return newNodeNotExecutable(path)
		}
	}
	env.NodeBinary = path
	env.Source = SourceExplicit
	r.addDiagnostic(CandidateKindNode, SourceExplicit, path, CandidateResultSelected, "")
	return nil
}

func (r *nodeEnvResolver) resolveNodeFromRuntimePackage(ctx context.Context, env *Environment, paths util.RuntimePaths) error {
	physical := resourceuri.PhysicalRootsFromRuntimePaths(paths)
	resolver, err := resourceuri.NewPhysicalResolver(physical)
	if err != nil {
		return err
	}

	candidates := runtimePackageNodeCandidates(env.Guest)
	for _, cand := range candidates {
		if err := ctx.Err(); err != nil {
			return err
		}
		uri, err := resourceuri.Parse(cand.uri)
		if err != nil {
			continue
		}
		resolved, err := resolver.Resolve(uri)
		if err != nil {
			r.addDiagnostic(CandidateKindNode, SourceRuntimePackage, cand.uri, CandidateResultNotFound, "resource not resolvable")
			continue
		}
		if resolved.Kind == resourceuri.ResourceKindVirtual {
			r.addDiagnostic(CandidateKindNode, SourceRuntimePackage, cand.uri, CandidateResultInvalidFile, "resolved to virtual resource")
			return newNativeResource(cand.uri)
		}
		localPath := resolved.LocalPath
		info, statErr := r.inspector.Stat(localPath)
		if statErr != nil {
			r.addDiagnostic(CandidateKindNode, SourceRuntimePackage, localPath, CandidateResultNotFound, "not found")
			continue
		}
		if info.IsDir() {
			r.addDiagnostic(CandidateKindNode, SourceRuntimePackage, localPath, CandidateResultInvalidFile, "path is a directory")
			continue
		}
		if env.Guest != platform.GuestPlatformWindows {
			if !hasExecPerm(info.Mode()) {
				r.addDiagnostic(CandidateKindNode, SourceRuntimePackage, localPath, CandidateResultNotExecutable, "missing execute permission")
				continue
			}
		}
		env.NodeBinary = localPath
		env.Source = SourceRuntimePackage
		r.addDiagnostic(CandidateKindNode, SourceRuntimePackage, localPath, CandidateResultSelected, "")
		return nil
	}
	return newNodeNotFound(SourceRuntimePackage)
}

func (r *nodeEnvResolver) resolveNodeFromLegacy(ctx context.Context, env *Environment, paths util.RuntimePaths) error {
	candidates := legacyNodeCandidates(env.Guest, paths.Root, paths.WorkspaceDir)
	for _, cand := range candidates {
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := r.inspector.Stat(cand.path)
		if err != nil {
			r.addDiagnostic(CandidateKindNode, SourceLegacyBundled, cand.path, CandidateResultNotFound, "not found")
			continue
		}
		if info.IsDir() {
			r.addDiagnostic(CandidateKindNode, SourceLegacyBundled, cand.path, CandidateResultInvalidFile, "path is a directory")
			continue
		}
		if env.Guest != platform.GuestPlatformWindows {
			if !hasExecPerm(info.Mode()) {
				r.addDiagnostic(CandidateKindNode, SourceLegacyBundled, cand.path, CandidateResultNotExecutable, "missing execute permission")
				continue
			}
		}
		env.NodeBinary = cand.path
		env.Source = SourceLegacyBundled
		r.addDiagnostic(CandidateKindNode, SourceLegacyBundled, cand.path, CandidateResultSelected, "")
		return nil
	}
	return newNodeNotFound(SourceLegacyBundled)
}

func (r *nodeEnvResolver) detectPackageManagers(ctx context.Context, env *Environment, providerCfg config.ScriptRuntimeProviderConfig, distributionRoot string) error {
	distroRoot := distributionRoot
	if env.DistributionRoot != "" {
		distroRoot = env.DistributionRoot
	}

	explicitNpm := strings.TrimSpace(providerCfg.Node.NPMPath)
	if explicitNpm != "" {
		npmPath, err := r.normalizeExplicitPath(explicitNpm, r.host.Paths().Root)
		if err != nil {
			return err
		}
		if err := r.validatePackageManagerScript(npmPath, CandidateKindNPMCLI, SourceExplicit); err != nil {
			return err
		}
		env.NPMCLI = npmPath
	} else {
		found := false
		for _, cand := range runtimePackageNPMCandidates(env.Guest) {
			if err := ctx.Err(); err != nil {
				return err
			}
			if local, ok := r.resolveResource(cand.uri); ok {
				if info, err := r.inspector.Stat(local); err == nil && !info.IsDir() && isPackageManagerExtension(local) {
					env.NPMCLI = local
					r.addDiagnostic(CandidateKindNPMCLI, SourceRuntimePackage, local, CandidateResultSelected, "")
					found = true
					break
				}
			}
		}
		if !found {
			for _, cand := range legacyNpmCandidates(distroRoot, env.Guest) {
				if err := ctx.Err(); err != nil {
					return err
				}
				if info, err := r.inspector.Stat(cand.path); err == nil && !info.IsDir() {
					env.NPMCLI = cand.path
					r.addDiagnostic(CandidateKindNPMCLI, SourceLegacyBundled, cand.path, CandidateResultSelected, "")
					found = true
					break
				}
			}
		}
		if !found {
			r.addDiagnostic(CandidateKindNPMCLI, env.Source, "", CandidateResultNotFound, "npm CLI not found")
		}
	}

	explicitNpx := strings.TrimSpace(providerCfg.Node.NPXPath)
	if explicitNpx != "" {
		npxPath, err := r.normalizeExplicitPath(explicitNpx, r.host.Paths().Root)
		if err != nil {
			return err
		}
		if err := r.validatePackageManagerScript(npxPath, CandidateKindNPXCLI, SourceExplicit); err != nil {
			return err
		}
		env.NPXCLI = npxPath
	} else {
		found := false
		for _, cand := range runtimePackageNPXCandidates(env.Guest) {
			if err := ctx.Err(); err != nil {
				return err
			}
			if local, ok := r.resolveResource(cand.uri); ok {
				if info, err := r.inspector.Stat(local); err == nil && !info.IsDir() && isPackageManagerExtension(local) {
					env.NPXCLI = local
					r.addDiagnostic(CandidateKindNPXCLI, SourceRuntimePackage, local, CandidateResultSelected, "")
					found = true
					break
				}
			}
		}
		if !found {
			for _, cand := range legacyNpxCandidates(distroRoot, env.Guest) {
				if err := ctx.Err(); err != nil {
					return err
				}
				if info, err := r.inspector.Stat(cand.path); err == nil && !info.IsDir() {
					env.NPXCLI = cand.path
					r.addDiagnostic(CandidateKindNPXCLI, SourceLegacyBundled, cand.path, CandidateResultSelected, "")
					found = true
					break
				}
			}
		}
		if !found {
			r.addDiagnostic(CandidateKindNPXCLI, env.Source, "", CandidateResultNotFound, "npx CLI not found")
		}
	}

	env.PackageManagementAvailable = env.NPMCLI != "" && env.NPXCLI != ""
	return nil
}

func (r *nodeEnvResolver) detectWorkDir(env *Environment, providerCfg config.ScriptRuntimeProviderConfig, distributionRoot string) error {
	explicit := strings.TrimSpace(providerCfg.Node.WorkDir)
	if explicit != "" {
		resolved, err := r.normalizeExplicitPath(explicit, r.host.Paths().Root)
		if err != nil {
			return err
		}
		info, err := r.inspector.Stat(resolved)
		if err != nil {
			r.addDiagnostic(CandidateKindWorkDir, SourceExplicit, resolved, CandidateResultNotFound, "work directory not found")
			return &invalidWorkDirError{reason: "work directory not found: " + resolved}
		}
		if !info.IsDir() {
			r.addDiagnostic(CandidateKindWorkDir, SourceExplicit, resolved, CandidateResultInvalidFile, "work directory is not a directory")
			return &invalidWorkDirError{reason: "work directory is not a directory: " + resolved}
		}
		env.WorkDir = resolved
		r.addDiagnostic(CandidateKindWorkDir, SourceExplicit, resolved, CandidateResultSelected, "")
		return nil
	}

	if distributionRoot != "" {
		info, err := r.inspector.Stat(distributionRoot)
		if err == nil && info.IsDir() {
			env.WorkDir = distributionRoot
			r.addDiagnostic(CandidateKindWorkDir, env.Source, distributionRoot, CandidateResultSelected, "default to distribution root")
			return nil
		}
	}
	r.addDiagnostic(CandidateKindWorkDir, env.Source, distributionRoot, CandidateResultNotFound, "default work directory unavailable")
	return &invalidWorkDirError{reason: "distribution root unavailable for default work directory"}
}

func (r *nodeEnvResolver) validatePackageManagerScript(path string, kind CandidateKind, source Source) error {
	if isShellWrapperExtension(path) {
		r.addDiagnostic(kind, source, path, CandidateResultUnsupportedWrapper, "shell wrapper not allowed")
		return newShellWrapper(path)
	}
	if !isPackageManagerExtension(path) {
		r.addDiagnostic(kind, source, path, CandidateResultInvalidFile, "invalid extension")
		return &invalidPackageManagerCLIError{path: path, reason: "invalid extension"}
	}
	info, err := r.inspector.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			r.addDiagnostic(kind, source, path, CandidateResultNotFound, "path not found")
			return &invalidPackageManagerCLIError{path: path, reason: "not found"}
		}
		return err
	}
	if info.IsDir() {
		r.addDiagnostic(kind, source, path, CandidateResultInvalidFile, "path is a directory")
		return &invalidPackageManagerCLIError{path: path, reason: "path is a directory"}
	}
	r.addDiagnostic(kind, source, path, CandidateResultSelected, "")
	return nil
}

func (r *nodeEnvResolver) normalizeExplicitPath(raw, runtimeRoot string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw), nil
	}
	if runtimeRoot == "" {
		return "", newRuntimePathError("cannot resolve relative path: runtime root is empty")
	}
	joined := filepath.Join(runtimeRoot, raw)
	return filepath.Clean(joined), nil
}

func (r *nodeEnvResolver) resolveResource(uriStr string) (string, bool) {
	physical := resourceuri.PhysicalRootsFromRuntimePaths(r.host.Paths())
	resolver, err := resourceuri.NewPhysicalResolver(physical)
	if err != nil {
		return "", false
	}
	uri, err := resourceuri.Parse(uriStr)
	if err != nil {
		return "", false
	}
	resolved, err := resolver.Resolve(uri)
	if err != nil {
		return "", false
	}
	if resolved.Kind == resourceuri.ResourceKindVirtual {
		return "", false
	}
	return resolved.LocalPath, true
}

func isSupportedGuest(guest platform.GuestPlatform) bool {
	switch guest {
	case platform.GuestPlatformWindows, platform.GuestPlatformLinux, platform.GuestPlatformMacOS:
		return true
	default:
		return false
	}
}

func hasExecPerm(mode os.FileMode) bool {
	return mode.Perm()&0111 != 0
}

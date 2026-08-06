// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantenv

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/resourceuri"
)

const qdrantProviderID = "builtin.qdrant-process"

type Resolver interface {
	Resolve(context.Context) (Environment, error)
	Snapshot() DetectionSnapshot
	Invalidate()
}

type ResolveContext struct {
	Config    *config.Config
	Host      runtimehost.RuntimeHost
	Inspector FileInspector
}

type resolver struct {
	config    *config.Config
	host      runtimehost.RuntimeHost
	inspector FileInspector

	mu              sync.Mutex
	cachedEnv       Environment
	cachedErr       error
	cacheValid      bool
	lastSnapshot    DetectionSnapshot
	detecting       bool
	detectWaiters   []chan struct{}
}

func NewResolver(ctx ResolveContext) (Resolver, error) {
	if ctx.Config == nil {
		return nil, fmt.Errorf("qdrantenv: config is required")
	}
	if ctx.Host == nil {
		return nil, fmt.Errorf("qdrantenv: host is required")
	}
	inspector := ctx.Inspector
	if inspector == nil {
		inspector = newDefaultFileInspector()
	}
	return &resolver{
		config:    ctx.Config,
		host:      ctx.Host,
		inspector: inspector,
	}, nil
}

func (r *resolver) Resolve(ctx context.Context) (Environment, error) {
	r.mu.Lock()
	if r.cacheValid && (r.cachedErr == nil || !isNotInstalledErr(r.cachedErr)) {
		env, err := r.cachedEnv, r.cachedErr
		r.mu.Unlock()
		return env, err
	}
	if r.detecting {
		waiter := make(chan struct{})
		r.detectWaiters = append(r.detectWaiters, waiter)
		r.mu.Unlock()
		select {
		case <-waiter:
			r.mu.Lock()
			env, err := r.cachedEnv, r.cachedErr
			r.mu.Unlock()
			return env, err
		case <-ctx.Done():
			return Environment{}, ctx.Err()
		}
	}
	r.detecting = true
	r.mu.Unlock()

	env, err := r.performDetection(ctx)

	r.mu.Lock()
	r.cachedEnv = env
	r.cachedErr = err
	if err == nil || isNotInstalledErr(err) {
		r.cacheValid = true
	}
	r.lastSnapshot = DetectionSnapshot{
		State:       snapshotState(env, err),
		Environment: env,
		DetectedAt:  time.Now().UTC(),
	}
	if err != nil {
		r.lastSnapshot.LastError = err.Error()
	}
	waiters := r.detectWaiters
	r.detectWaiters = nil
	r.detecting = false
	r.mu.Unlock()
	for _, w := range waiters {
		close(w)
	}

	return env, err
}

func (r *resolver) Snapshot() DetectionSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastSnapshot.clone()
}

func (r *resolver) Invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cacheValid = false
	r.cachedEnv = Environment{}
	r.cachedErr = nil
}

func (r *resolver) performDetection(ctx context.Context) (Environment, error) {
	if err := ctx.Err(); err != nil {
		return Environment{}, err
	}

	vectorCfg := &r.config.Providers.VectorStore
	if !vectorCfg.Enabled {
		return Environment{}, ErrVectorStoreDisabled
	}
	if vectorCfg.Provider != qdrantProviderID {
		return Environment{}, ErrQdrantProviderNotSelected
	}

	caps := r.host.Capabilities()
	if !caps.Supports(runtimehost.CapProcessSpawn) {
		return Environment{}, newHostCapabilityError(string(runtimehost.CapProcessSpawn))
	}
	if !caps.Supports(runtimehost.CapFilesystemLocal) {
		return Environment{}, newHostCapabilityError(string(runtimehost.CapFilesystemLocal))
	}
	if !caps.Supports(runtimehost.CapFilesystemExecutable) {
		return Environment{}, newHostCapabilityError(string(runtimehost.CapFilesystemExecutable))
	}

	descriptor := r.host.Descriptor()
	guest := descriptor.Guest
	architecture := descriptor.Architecture

	env := Environment{
		Guest:        guest,
		Architecture: architecture,
	}

	rawExplicit := vectorCfg.Qdrant.BinaryPath
	explicitPath := normalizeConfiguredPath(rawExplicit, r.host.Paths().Root)
	if explicitPath != "" {
		return r.resolveExplicit(explicitPath, env)
	}

	runtimeRoot, err := r.resolveRuntimeResourceRoot()
	if err != nil {
		return Environment{}, err
	}

	for _, uri := range runtimePackageCandidates(guest) {
		resolved, ok := r.resolveRuntimePackage(uri, runtimeRoot)
		if !ok {
			continue
		}
		env.BinaryPath = resolved
		env.DistributionRoot = deriveDistributionRoot(resolved)
		env.Source = SourceRuntimePackage
		env.Installed = true
		env.Explicit = false
		return env, nil
	}

	paths := r.host.Paths()
	for _, candidate := range legacyCandidates(guest, architecture, paths.Root, paths.WorkspaceDir) {
		info, err := r.inspector.Stat(candidate)
		if err != nil {
			continue
		}
		valid, reason := isValidBinaryPath(info, guest)
		if !valid {
			continue
		}
		abs, err := r.inspector.Abs(candidate)
		if err != nil {
			abs = candidate
		}
		env.BinaryPath = abs
		env.DistributionRoot = deriveDistributionRoot(abs)
		env.Source = SourceLegacyBundled
		env.Installed = true
		env.Explicit = false
		return env, nil
	}

	if runtimeRoot == "" {
		return Environment{}, newRuntimeRootUnavailable("runtime resource root unavailable")
	}

	target := standardInstallTarget(guest, runtimeRoot)
	env.BinaryPath = target
	env.DistributionRoot = runtimeRoot
	env.Source = SourceRuntimePackage
	env.Installed = false
	env.Explicit = false
	return env, newQdrantBinaryNotInstalled(SourceRuntimePackage)
}

func (r *resolver) resolveExplicit(raw string, env Environment) (Environment, error) {
	info, err := r.inspector.Stat(raw)
	if err != nil {
		return Environment{}, newExplicitBinaryNotFound(raw)
	}
	if info.IsDir() {
		return Environment{}, newInvalidQdrantBinary(raw, "path is directory")
	}
	if env.Guest != platform.GuestPlatformWindows {
		if info.Mode().Perm()&0111 == 0 {
			return Environment{}, newQdrantBinaryNotExecutable(raw)
		}
	}
	abs, err := r.inspector.Abs(raw)
	if err != nil {
		abs = raw
	}
	env.BinaryPath = abs
	env.DistributionRoot = deriveDistributionRoot(abs)
	env.Source = SourceExplicit
	env.Installed = true
	env.Explicit = true
	return env, nil
}

func (r *resolver) resolveRuntimeResourceRoot() (string, error) {
	uri, err := resourceuri.Parse("amitia://runtime/")
	if err != nil {
		return "", err
	}
	roots := resourceuri.PhysicalRootsFromRuntimePaths(r.host.Paths())
	presolver, perr := resourceuri.NewPhysicalResolver(roots)
	if perr != nil {
		return "", perr
	}
	resolved, rerr := presolver.Resolve(uri)
	if rerr != nil {
		return "", nil
	}
	return resolved.LocalPath, nil
}

func (r *resolver) resolveRuntimePackage(uriRaw string, resourceRoot string) (string, bool) {
	uri, err := resourceuri.Parse(uriRaw)
	if err != nil {
		return "", false
	}
	roots := resourceuri.PhysicalRootsFromRuntimePaths(r.host.Paths())
	presolver, perr := resourceuri.NewPhysicalResolver(roots)
	if perr != nil {
		return "", false
	}
	if resourceRoot == "" {
		return "", false
	}
	resolved, rerr := presolver.Resolve(uri)
	if rerr != nil {
		return "", false
	}
	info, serr := r.inspector.Stat(resolved.LocalPath)
	if serr != nil {
		return "", false
	}
	if info.IsDir() {
		return "", false
	}
	if !hasExecutableBit(info) && r.host.Descriptor().Guest != platform.GuestPlatformWindows {
		return "", false
	}
	return resolved.LocalPath, true
}

func normalizeConfiguredPath(raw, runtimeRoot string) string {
	raw = trimSpaceOnly(raw)
	if raw == "" {
		return ""
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	if runtimeRoot == "" {
		return ""
	}
	return filepath.Clean(filepath.Join(runtimeRoot, raw))
}

func trimSpaceOnly(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func isNotInstalledErr(err error) bool {
	if err == nil {
		return false
	}
	return err == ErrQdrantBinaryNotInstalled
}

func snapshotState(env Environment, err error) DetectionState {
	if err != nil {
		if isNotInstalledErr(err) {
			return DetectionStateNotInstalled
		}
		return DetectionStateFailed
	}
	if env.Installed {
		return DetectionStateReady
	}
	return DetectionStateNotInstalled
}

func deriveDistributionRoot(binaryPath string) string {
	if binaryPath == "" {
		return ""
	}
	dir := filepath.Dir(binaryPath)
	base := filepath.Base(dir)
	if base == "bin" {
		return filepath.Clean(filepath.Dir(dir))
	}
	return filepath.Clean(dir)
}

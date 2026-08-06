// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package script_host

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/resourceuri"
)

type ResolveContext struct {
	Host               runtimehost.RuntimeHost
	PluginHostEntryURI string
	TaskHostEntryURI   string
	Inspector          FileInspector
}

func NewArtifactResolver(ctx ResolveContext) (ArtifactResolver, error) {
	if ctx.Host == nil {
		return nil, errors.New("script_host: host is nil")
	}
	inspector := ctx.Inspector
	if inspector == nil {
		inspector = newDefaultFileInspector()
	}
	return &artifactResolver{
		host:               ctx.Host,
		pluginHostEntryURI: strings.TrimSpace(ctx.PluginHostEntryURI),
		taskHostEntryURI:   strings.TrimSpace(ctx.TaskHostEntryURI),
		inspector:          inspector,
	}, nil
}

type artifactResolver struct {
	host               runtimehost.RuntimeHost
	pluginHostEntryURI string
	taskHostEntryURI   string
	inspector          FileInspector
}

func (r *artifactResolver) Resolve(ctx context.Context, kind Kind) (Artifact, error) {
	if !knownKind(kind) {
		return Artifact{Kind: kind}, &unknownHostKindError{kind: kind}
	}

	if err := ctx.Err(); err != nil {
		return Artifact{Kind: kind}, err
	}

	explicitURI := r.explicitURI(kind)
	if explicitURI != "" {
		return r.resolveFromExplicit(ctx, kind, explicitURI)
	}

	if err := ctx.Err(); err != nil {
		return Artifact{Kind: kind}, err
	}

	artifact, err := r.resolveFromRuntimePackage(ctx, kind)
	if err == nil {
		return artifact, nil
	}
	if errors.Is(err, ErrNativeResourceNotAllowedEquivalent) || errors.Is(err, ErrRuntimeResourceUnavailable) {
		return Artifact{Kind: kind}, err
	}

	if err := ctx.Err(); err != nil {
		return Artifact{Kind: kind}, err
	}

	return r.resolveFromLegacy(ctx, kind)
}

func (r *artifactResolver) explicitURI(kind Kind) string {
	switch kind {
	case KindPluginHost:
		return r.pluginHostEntryURI
	case KindTaskHost:
		return r.taskHostEntryURI
	}
	return ""
}

func (r *artifactResolver) resolveFromExplicit(ctx context.Context, kind Kind, uriStr string) (Artifact, error) {
	localPath, err := r.resolveURIToLocal(uriStr)
	if err != nil {
		return Artifact{Kind: kind}, err
	}

	info, err := r.inspector.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Artifact{Kind: kind}, newHostArtifactNotFound(kind, SourceExplicit, localPath)
		}
		return Artifact{Kind: kind}, err
	}
	if info.IsDir() {
		return Artifact{Kind: kind}, newInvalidHostArtifact(kind, localPath, "path is a directory")
	}
	if !isHostEntryExtension(localPath) {
		return Artifact{Kind: kind}, newUnsupportedHostEntry(kind, localPath, filepath.Ext(localPath))
	}

	return Artifact{
		Kind:             kind,
		EntryPath:        localPath,
		DistributionRoot: deriveDistributionRoot(localPath),
		Source:           SourceExplicit,
	}, nil
}

func (r *artifactResolver) resolveFromRuntimePackage(ctx context.Context, kind Kind) (Artifact, error) {
	uri := defaultRuntimeURI(kind)
	physical := resourceuri.PhysicalRootsFromRuntimePaths(r.host.Paths())
	resolver, err := resourceuri.NewPhysicalResolver(physical)
	if err != nil {
		return Artifact{Kind: kind}, err
	}

	resourceURI, err := resourceuri.Parse(uri)
	if err != nil {
		return Artifact{Kind: kind}, err
	}

	resolved, err := resolver.Resolve(resourceURI)
	if err != nil {
		return Artifact{Kind: kind}, newRuntimeResourceUnavailable(kind, uri)
	}

	if resolved.Kind == resourceuri.ResourceKindVirtual {
		return Artifact{Kind: kind}, &nativeResourceError{kind: kind, uri: uri}
	}

	localPath := resolved.LocalPath
	info, err := r.inspector.Stat(localPath)
	if err != nil {
		return Artifact{Kind: kind}, newHostArtifactNotFound(kind, SourceRuntimePackage, localPath)
	}
	if info.IsDir() {
		return Artifact{Kind: kind}, newInvalidHostArtifact(kind, localPath, "path is a directory")
	}
	if !isHostEntryExtension(localPath) {
		return Artifact{Kind: kind}, newUnsupportedHostEntry(kind, localPath, filepath.Ext(localPath))
	}

	return Artifact{
		Kind:             kind,
		EntryPath:        localPath,
		DistributionRoot: deriveDistributionRoot(localPath),
		Source:           SourceRuntimePackage,
	}, nil
}

func (r *artifactResolver) resolveFromLegacy(ctx context.Context, kind Kind) (Artifact, error) {
	workspaceDir := r.host.Paths().WorkspaceDir
	if workspaceDir == "" {
		return Artifact{Kind: kind}, newWorkspaceUnavailable(kind)
	}

	relative := defaultLegacyRelative(kind)
	candidates := []string{
		filepath.Join(workspaceDir, relative),
	}

	for _, cand := range candidates {
		if err := ctx.Err(); err != nil {
			return Artifact{Kind: kind}, err
		}
		info, err := r.inspector.Stat(cand)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		if !isHostEntryExtension(cand) {
			return Artifact{Kind: kind}, newUnsupportedHostEntry(kind, cand, filepath.Ext(cand))
		}
		return Artifact{
			Kind:             kind,
			EntryPath:        filepath.Clean(cand),
			DistributionRoot: deriveDistributionRoot(cand),
			Source:           SourceLegacyWorkspace,
		}, nil
	}

	return Artifact{Kind: kind}, newHostArtifactNotFound(kind, SourceLegacyWorkspace, filepath.Join(workspaceDir, relative))
}

func (r *artifactResolver) resolveURIToLocal(uriStr string) (string, error) {
	physical := resourceuri.PhysicalRootsFromRuntimePaths(r.host.Paths())
	resolver, err := resourceuri.NewPhysicalResolver(physical)
	if err != nil {
		return "", err
	}
	uri, err := resourceuri.Parse(uriStr)
	if err != nil {
		return "", err
	}
	resolved, err := resolver.Resolve(uri)
	if err != nil {
		return "", newRuntimeResourceUnavailable(KindPluginHost, uriStr)
	}
	if resolved.Kind == resourceuri.ResourceKindVirtual {
		return "", fmt.Errorf("%w: %s", errVirtualResource, uriStr)
	}
	return resolved.LocalPath, nil
}

func defaultRuntimeURI(kind Kind) string {
	switch kind {
	case KindPluginHost:
		return "amitia://runtime/plugin-host/dist/index.js"
	case KindTaskHost:
		return "amitia://runtime/task-host/dist/index.js"
	}
	return ""
}

func defaultLegacyRelative(kind Kind) string {
	switch kind {
	case KindPluginHost:
		return "runtime/plugin-host/dist/index.js"
	case KindTaskHost:
		return "runtime/task-host/dist/index.js"
	}
	return ""
}

var errVirtualResource = errors.New("script_host: cannot use virtual resource")

type nativeResourceError struct {
	kind Kind
	uri  string
}

func (e *nativeResourceError) Error() string {
	return "script_host: native resource not allowed: kind=" + string(e.kind) + " uri=" + e.uri
}

func (e *nativeResourceError) Is(target error) bool {
	return target == ErrNativeResourceNotAllowedEquivalent
}

var ErrNativeResourceNotAllowedEquivalent = errors.New("script_host: native resource not allowed")

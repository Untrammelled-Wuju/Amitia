// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sidecar

import (
	"context"
	"path/filepath"

	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/resourceuri"
)

type ArtifactResolver interface {
	Resolve(ctx context.Context, kind Kind) (Artifact, error)
}

type ResolveContext struct {
	Host           runtimehost.RuntimeHost
	WeChatEntryURI string
	QQEntryURI     string
	FileInspector  FileInspector
}

type sidecarResolver struct {
	host           runtimehost.RuntimeHost
	wechatEntryURI string
	qqEntryURI     string
	inspector      FileInspector
}

func NewArtifactResolver(ctx ResolveContext) (ArtifactResolver, error) {
	if ctx.Host == nil {
		return nil, ErrWorkspaceUnavailable
	}
	inspector := ctx.FileInspector
	if inspector == nil {
		inspector = defaultFileInspector{}
	}
	return &sidecarResolver{
		host:           ctx.Host,
		wechatEntryURI: ctx.WeChatEntryURI,
		qqEntryURI:     ctx.QQEntryURI,
		inspector:      inspector,
	}, nil
}

func (r *sidecarResolver) Resolve(ctx context.Context, kind Kind) (Artifact, error) {
	if !knownKind(kind) {
		return Artifact{}, newSidecarError(kind, "", ErrUnknownSidecarKind)
	}

	artifact, err := r.resolveExplicit(ctx, kind)
	if err == nil {
		return artifact, nil
	}

	artifact, err = r.resolveRuntimePackage(ctx, kind)
	if err == nil {
		return artifact, nil
	}

	artifact, err = r.resolveWorkspaceBundle(ctx, kind)
	if err == nil {
		return artifact, nil
	}

	artifact, err = r.resolveWorkspaceSource(ctx, kind)
	if err == nil {
		return artifact, nil
	}

	return Artifact{}, newSidecarError(kind, "", ErrSidecarArtifactNotFound)
}

func (r *sidecarResolver) resolver() (*resourceuri.PhysicalResolver, error) {
	paths := r.host.Paths()
	roots := resourceuri.PhysicalRootsFromRuntimePaths(paths)
	return resourceuri.NewPhysicalResolver(roots)
}

func (r *sidecarResolver) resolveExplicit(ctx context.Context, kind Kind) (Artifact, error) {
	var uri string
	switch kind {
	case KindWeChat:
		uri = r.wechatEntryURI
	case KindQQ:
		uri = r.qqEntryURI
	}
	if uri == "" {
		return Artifact{}, newSidecarError(kind, SourceExplicit, ErrSidecarArtifactNotFound)
	}

	parsed, err := resourceuri.Parse(uri)
	if err != nil {
		return Artifact{}, newSidecarError(kind, SourceExplicit, err)
	}

	resolver, err := r.resolver()
	if err != nil {
		return Artifact{}, newSidecarError(kind, SourceExplicit, err)
	}

	resolved, err := resolver.Resolve(parsed)
	if err != nil {
		return Artifact{}, newSidecarError(kind, SourceExplicit, err)
	}

	if !isJSFile(resolved.LocalPath) {
		return Artifact{}, newSidecarError(kind, SourceExplicit, ErrSidecarArtifactInvalid)
	}

	if err := r.statFile(resolved.LocalPath); err != nil {
		return Artifact{}, newSidecarError(kind, SourceExplicit, err)
	}

	bundleName := "bundle.mjs"
	bundlePath := filepath.Join(filepath.Dir(resolved.LocalPath), bundleName)
	if err := r.statFile(bundlePath); err != nil {
		return Artifact{}, newSidecarError(kind, SourceExplicit, ErrSidecarBundleIncomplete)
	}

	return Artifact{
		Kind:       kind,
		EntryPath:  resolved.LocalPath,
		ArgsPrefix: nil,
		WorkingDir: filepath.Dir(resolved.LocalPath),
		Source:     SourceExplicit,
	}, nil
}

func (r *sidecarResolver) resolveRuntimePackage(ctx context.Context, kind Kind) (Artifact, error) {
	uri := sidecarRuntimeURI(kind)
	if uri == "" {
		return Artifact{}, newSidecarError(kind, SourceRuntimePackage, ErrSidecarArtifactNotFound)
	}

	parsed, err := resourceuri.Parse(uri)
	if err != nil {
		return Artifact{}, newSidecarError(kind, SourceRuntimePackage, err)
	}

	resolver, err := r.resolver()
	if err != nil {
		return Artifact{}, newSidecarError(kind, SourceRuntimePackage, err)
	}

	resolved, err := resolver.Resolve(parsed)
	if err != nil {
		return Artifact{}, newSidecarError(kind, SourceRuntimePackage, err)
	}

	launcherPath := resolved.LocalPath
	if err := r.statFile(launcherPath); err != nil {
		return Artifact{}, newSidecarError(kind, SourceRuntimePackage, err)
	}

	dir := filepath.Dir(launcherPath)
	bundlePath := filepath.Join(dir, "bundle.mjs")
	if err := r.statFile(bundlePath); err != nil {
		return Artifact{}, newSidecarError(kind, SourceRuntimePackage, ErrSidecarBundleIncomplete)
	}

	return Artifact{
		Kind:       kind,
		EntryPath:  launcherPath,
		ArgsPrefix: nil,
		WorkingDir: dir,
		Source:     SourceRuntimePackage,
	}, nil
}

func (r *sidecarResolver) resolveWorkspaceBundle(ctx context.Context, kind Kind) (Artifact, error) {
	workspace := r.host.Paths().WorkspaceDir
	if workspace == "" {
		return Artifact{}, newSidecarError(kind, SourceWorkspaceBundle, ErrWorkspaceUnavailable)
	}

	subdir := sidecarSubdir(kind)
	if subdir == "" {
		return Artifact{}, newSidecarError(kind, SourceWorkspaceBundle, ErrUnknownSidecarKind)
	}

	launcherPath := filepath.Join(workspace, "backend", subdir, "launcher.mjs")
	bundlePath := filepath.Join(workspace, "backend", subdir, "bundle.mjs")

	if err := r.statFile(launcherPath); err != nil {
		return Artifact{}, newSidecarError(kind, SourceWorkspaceBundle, err)
	}
	if err := r.statFile(bundlePath); err != nil {
		return Artifact{}, newSidecarError(kind, SourceWorkspaceBundle, err)
	}

	return Artifact{
		Kind:       kind,
		EntryPath:  launcherPath,
		ArgsPrefix: nil,
		WorkingDir: filepath.Dir(launcherPath),
		Source:     SourceWorkspaceBundle,
	}, nil
}

func (r *sidecarResolver) resolveWorkspaceSource(ctx context.Context, kind Kind) (Artifact, error) {
	workspace := r.host.Paths().WorkspaceDir
	if workspace == "" {
		return Artifact{}, newSidecarError(kind, SourceWorkspaceSource, ErrWorkspaceUnavailable)
	}

	subdir := sidecarSourceSubdir(kind)
	tsxPath := filepath.Join(workspace, "backend", subdir, "node_modules", "tsx", "dist", "cli.mjs")
	indexTs := filepath.Join(workspace, "backend", subdir, "src", "index.ts")
	pkgJson := filepath.Join(workspace, "backend", subdir, "package.json")

	if err := r.statFile(tsxPath); err != nil {
		return Artifact{}, newSidecarError(kind, SourceWorkspaceSource, ErrSidecarSourceIncomplete)
	}
	if err := r.statFile(indexTs); err != nil {
		return Artifact{}, newSidecarError(kind, SourceWorkspaceSource, ErrSidecarSourceIncomplete)
	}
	if err := r.statFile(pkgJson); err != nil {
		return Artifact{}, newSidecarError(kind, SourceWorkspaceSource, ErrSidecarSourceIncomplete)
	}

	absTsx, _ := filepath.Abs(tsxPath)
	absIndex, _ := filepath.Abs(indexTs)
	absDir, _ := filepath.Abs(filepath.Dir(tsxPath))
	_ = absDir

	return Artifact{
		Kind:       kind,
		EntryPath:  absTsx,
		ArgsPrefix: []string{absIndex},
		WorkingDir: filepath.Dir(absTsx),
		Source:     SourceWorkspaceSource,
	}, nil
}

func (r *sidecarResolver) statFile(path string) error {
	_, err := r.inspector.Stat(path)
	return err
}

func sidecarSourceSubdir(kind Kind) string {
	return sidecarSubdir(kind)
}

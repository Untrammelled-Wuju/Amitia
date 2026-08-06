// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package commandenv

import (
	"context"
	"os"
	"path/filepath"

	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
)

type Request struct {
	Command string
	Args    []string
}

type Invocation struct {
	Executable string
	Args       []string
	Kind       Kind
	Source     Source
}

type NodeEnvironmentResolver interface {
	Resolve(context.Context) (nodeenv.Environment, error)
}

type Resolver interface {
	Resolve(context.Context, Request) (Invocation, error)
}

type FileInspector interface {
	Stat(path string) (os.FileInfo, error)
	Abs(path string) (string, error)
}

type defaultFileInspector struct{}

func (defaultFileInspector) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (defaultFileInspector) Abs(path string) (string, error) {
	return filepath.Abs(path)
}

type ResolveContext struct {
	NodeResolver      NodeEnvironmentResolver
	ExecutableLocator ExecutableLocator
	FileInspector     FileInspector
}

type commandResolver struct {
	nodeResolver  NodeEnvironmentResolver
	locator       ExecutableLocator
	fileInspector FileInspector
}

func NewResolver(ctx ResolveContext) (Resolver, error) {
	nodeResolver := ctx.NodeResolver
	locator := ctx.ExecutableLocator
	fileInspector := ctx.FileInspector

	if nodeResolver == nil {
		nodeResolver = UnavailableNodeResolver()
	}
	if locator == nil {
		locator = newDefaultLocator()
	}
	if fileInspector == nil {
		fileInspector = defaultFileInspector{}
	}

	return &commandResolver{
		nodeResolver:  nodeResolver,
		locator:       locator,
		fileInspector: fileInspector,
	}, nil
}

func (r *commandResolver) Resolve(ctx context.Context, req Request) (Invocation, error) {
	if err := validateCommandString(req.Command); err != nil {
		return Invocation{}, err
	}
	if err := validateArgs(req.Args); err != nil {
		return Invocation{}, err
	}

	trimmed := normalizeCommand(req.Command)
	if isShellCommand(trimmed) {
		return Invocation{}, newCommandError(ErrShellCommandForbidden, KindNative, baseName(trimmed))
	}

	kind, isManaged := classifyCommand(trimmed)
	if isManaged {
		return r.resolveManaged(ctx, req, kind, trimmed)
	}

	if filepath_IsAbs(trimmed) {
		return r.resolveAbsoluteNative(ctx, req, trimmed)
	}

	return r.resolveLookupNative(req, trimmed)
}

func (r *commandResolver) resolveManaged(ctx context.Context, req Request, kind Kind, trimmed string) (Invocation, error) {
	absManaged := false
	if filepath_IsAbs(trimmed) {
		absManaged = true
	}

	env, err := r.nodeResolver.Resolve(ctx)
	if err != nil {
		return Invocation{}, wrapNodeError(err, kind, baseName(trimmed))
	}

	if absManaged {
		managedNode := env.NodeBinary
		if managedNode == "" {
			return Invocation{}, newCommandError(ErrNodeEnvironmentUnavailable, kind, baseName(trimmed))
		}
		if !samePath(trimmed, managedNode) {
			return Invocation{}, newCommandError(ErrUnmanagedNodeCommand, kind, baseName(trimmed))
		}
		inv := buildManagedInvocation(env, kind, req.Args)
		return inv, nil
	}

	inv := buildManagedInvocation(env, kind, req.Args)
	return inv, nil
}

func buildManagedInvocation(env nodeenv.Environment, kind Kind, args []string) Invocation {
	switch kind {
	case KindNPM:
		return Invocation{
			Executable: env.NodeBinary,
			Args:       append([]string{env.NPMCLI}, copyArgs(args)...),
			Kind:       KindNPM,
			Source:     SourceManagedNode,
		}
	case KindNPX:
		return Invocation{
			Executable: env.NodeBinary,
			Args:       append([]string{env.NPXCLI}, copyArgs(args)...),
			Kind:       KindNPX,
			Source:     SourceManagedNode,
		}
	default:
		return Invocation{
			Executable: env.NodeBinary,
			Args:       copyArgs(args),
			Kind:       KindNode,
			Source:     SourceManagedNode,
		}
	}
}

func (r *commandResolver) resolveAbsoluteNative(ctx context.Context, req Request, trimmed string) (Invocation, error) {
	abs, err := r.fileInspector.Abs(trimmed)
	if err != nil {
		return Invocation{}, newCommandError(ErrExecutableInvalid, KindNative, trimmed)
	}
	info, err := r.fileInspector.Stat(abs)
	if err != nil {
		return Invocation{}, newCommandError(ErrExecutableInvalid, KindNative, abs)
	}
	if info.IsDir() {
		return Invocation{}, newCommandError(ErrExecutableInvalid, KindNative, abs)
	}
	return Invocation{
		Executable: abs,
		Args:       copyArgs(req.Args),
		Kind:       KindNative,
		Source:     SourceNativePath,
	}, nil
}

func (r *commandResolver) resolveLookupNative(req Request, trimmed string) (Invocation, error) {
	resolved, err := r.locator.LookPath(trimmed)
	if err != nil {
		return Invocation{}, newCommandError(ErrCommandNotFound, KindNative, trimmed)
	}
	abs, err := r.fileInspector.Abs(resolved)
	if err != nil {
		abs = resolved
	}
	return Invocation{
		Executable: abs,
		Args:       copyArgs(req.Args),
		Kind:       KindNative,
		Source:     SourceNativeLookUp,
	}, nil
}

// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package skill

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/u-ai/backend/internal/scriptruntime/commandenv"
	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
)

type InterpreterResolveContext struct {
	NodeResolver      nodeenv.Resolver
	CommandResolver   commandenv.Resolver
	ExecutableLocator commandenv.ExecutableLocator
	FileInspector     commandenv.FileInspector
}

type scriptInterpreterResolver struct {
	nodeResolver    nodeenv.Resolver
	commandResolver commandenv.Resolver
}

func NewScriptInterpreterResolver(ctx InterpreterResolveContext) (ScriptInterpreterResolver, error) {
	nr := ctx.NodeResolver
	if nr == nil {
		nr = &unavailableNodeResolver{}
	}
	cr := ctx.CommandResolver
	if cr == nil {
		var err error
		cr, err = commandenv.NewResolver(commandenv.ResolveContext{
			NodeResolver:      nr,
			ExecutableLocator: ctx.ExecutableLocator,
			FileInspector:     ctx.FileInspector,
		})
		if err != nil {
			return nil, err
		}
	}
	return &scriptInterpreterResolver{
		nodeResolver:    nr,
		commandResolver: cr,
	}, nil
}

type unavailableNodeResolver struct{}

func (r *unavailableNodeResolver) Resolve(_ context.Context) (nodeenv.Environment, error) {
	return nodeenv.Environment{}, ErrScriptInterpreterUnavailable
}

func (r *unavailableNodeResolver) Snapshot() nodeenv.DetectionSnapshot {
	return nodeenv.DetectionSnapshot{}
}

func (r *scriptInterpreterResolver) Resolve(ctx context.Context, runtime string, extensionID string) (ScriptInterpreter, error) {
	switch runtime {
	case ScriptRuntimeNode:
		return r.resolveNode(ctx)
	case ScriptRuntimeNative:
		return ScriptInterpreter{
			Kind:   InterpreterKindNative,
			Source: "native",
		}, nil
	default:
		return ScriptInterpreter{}, ErrScriptInterpreterUnavailable
	}
}

func (r *scriptInterpreterResolver) ResolveFromDescriptor(ctx context.Context, desc SkillScriptDescriptor) (ScriptInterpreter, error) {
	if desc.Runtime == "" {
		return ScriptInterpreter{}, ErrScriptInterpreterUnavailable
	}
	if desc.Runtime == ScriptRuntimeNode {
		interp, err := r.resolveNode(ctx)
		if err != nil {
			return ScriptInterpreter{}, err
		}
		if desc.EntryName != "" {
			interp.ArgsPrefix = append([]string{desc.EntryName}, interp.ArgsPrefix...)
		}
		return interp, nil
	}
	if desc.Runtime == ScriptRuntimeNative {
		if desc.EntryName == "" {
			return ScriptInterpreter{}, ErrScriptInterpreterUnavailable
		}
		abs, err := filepath.Abs(desc.EntryName)
		if err != nil {
			return ScriptInterpreter{}, ErrScriptInterpreterUnavailable
		}
		return ScriptInterpreter{
			Kind:       InterpreterKindNative,
			Executable: abs,
			Source:     "native",
		}, nil
	}
	return ScriptInterpreter{}, ErrScriptInterpreterUnavailable
}

func (r *scriptInterpreterResolver) resolveNode(ctx context.Context) (ScriptInterpreter, error) {
	env, err := r.nodeResolver.Resolve(ctx)
	if err != nil {
		if errors.Is(err, nodeenv.ErrNodeNotFound) {
			return ScriptInterpreter{}, ErrScriptInterpreterUnavailable
		}
		return ScriptInterpreter{}, ErrScriptInterpreterUnavailable
	}
	return ScriptInterpreter{
		Kind:       InterpreterKindNode,
		Executable: env.NodeBinary,
		Source:     "managed-node",
		Version:    env.Architecture,
	}, nil
}

func (r *scriptInterpreterResolver) ResolveCommand(ctx context.Context, command string, args []string) (commandenv.Invocation, error) {
	if r.commandResolver == nil {
		return commandenv.Invocation{}, ErrScriptInterpreterUnavailable
	}
	return r.commandResolver.Resolve(ctx, commandenv.Request{
		Command: command,
		Args:    args,
	})
}

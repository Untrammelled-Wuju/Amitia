// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/scriptruntime/commandenv"
	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
)

type ProcessLauncher interface {
	Resolve(ctx context.Context, spec MCPStdioSpec) (commandenv.Invocation, error)
}

type NPXLauncher struct {
	nodeResolver    nodeenv.Resolver
	commandResolver commandenv.Resolver
}

func NewNPXLauncher(nodeResolver nodeenv.Resolver, commandResolver commandenv.Resolver) *NPXLauncher {
	return &NPXLauncher{nodeResolver: nodeResolver, commandResolver: commandResolver}
}

func (l *NPXLauncher) Resolve(ctx context.Context, spec MCPStdioSpec) (commandenv.Invocation, error) {
	if l.commandResolver == nil {
		return commandenv.Invocation{}, fmt.Errorf("MCP_NPX_UNAVAILABLE: command resolver is not configured")
	}
	return l.commandResolver.Resolve(ctx, commandenv.Request{Command: spec.Command, Args: spec.Args})
}

func BuildNPXInvocation(
	ctx context.Context,
	spec MCPNPXSpec,
	policy NPXPolicy,
	resolver commandenv.Resolver,
) (commandenv.Invocation, map[string]string, error) {
	if err := ValidateNPXPackageName(spec.Package); err != nil {
		return commandenv.Invocation{}, nil, err
	}

	if err := ValidateNPXBinaryName(spec.Binary); err != nil {
		return commandenv.Invocation{}, nil, err
	}

	if spec.AllowFloatingVersion {
		if !policy.AllowFloatingVersion {
			return commandenv.Invocation{}, nil, fmt.Errorf("MCP_NPX_FLOATING_VERSION_DENIED: floating version requires explicit policy")
		}
	} else {
		if err := validateExactVersion(spec.Package); err != nil {
			return commandenv.Invocation{}, nil, err
		}
	}

	npxArgs := buildNPXControlArgs(spec, policy)

	fullArgs := append(npxArgs, spec.Package)
	if spec.Binary != "" {
		fullArgs = append(fullArgs, "--", spec.Binary)
	}
	fullArgs = append(fullArgs, spec.Args...)

	req := commandenv.Request{
		Command: "npx",
		Args:    fullArgs,
	}

	inv, err := resolver.Resolve(ctx, req)
	if err != nil {
		return commandenv.Invocation{}, nil, fmt.Errorf("MCP_NPX_UNAVAILABLE: %w", err)
	}

	env := buildNPXEnvironment(spec, policy)

	return inv, env, nil
}

func buildNPXControlArgs(spec MCPNPXSpec, policy NPXPolicy) []string {
	args := []string{"--yes", "--ignore-scripts"}

	if spec.FetchPolicyOrDefault() == NPXFetchDeny {
		args = append(args, "--no")
	} else {
		args = append(args, "--package", spec.Package)
	}

	return args
}

func buildNPXEnvironment(spec MCPNPXSpec, policy NPXPolicy) map[string]string {
	env := map[string]string{
		"NPM_CONFIG_AUDIT":           "false",
		"NPM_CONFIG_FUND":            "false",
		"NPM_CONFIG_UPDATE_NOTIFIER": "false",
		"NPM_CONFIG_IGNORE_SCRIPTS":  "true",
		"NPM_CONFIG_YES":             "true",
	}

	if policy.CacheDir != "" {
		env["NPM_CONFIG_CACHE"] = policy.CacheDir
	}

	if policy.UserConfigFile != "" {
		env["NPM_CONFIG_USERCONFIG"] = policy.UserConfigFile
	}

	if policy.Registry != "" {
		env["NPM_CONFIG_REGISTRY"] = policy.Registry
	}

	for k, v := range spec.Environment {
		if IsNPXReservedEnvVar(k) {
			continue
		}
		if strings.ContainsAny(k+v, "\r\n") {
			continue
		}
		env[k] = v
	}

	return env
}

func validateExactVersion(packageName string) error {
	atIndex := strings.LastIndex(packageName, "@")
	if atIndex <= 0 {
		if strings.HasPrefix(packageName, "@") {
			return nil
		}
		return fmt.Errorf("MCP_NPX_VERSION_REQUIRED: exact version is required")
	}

	versionPart := packageName[atIndex+1:]
	if versionPart == "" {
		return fmt.Errorf("MCP_NPX_VERSION_REQUIRED: empty version")
	}

	if versionPart == "latest" || versionPart == "next" || versionPart == "*" {
		return fmt.Errorf("MCP_NPX_FLOATING_VERSION_DENIED: floating version is not allowed")
	}

	if strings.HasPrefix(versionPart, "^") || strings.HasPrefix(versionPart, "~") {
		return fmt.Errorf("MCP_NPX_FLOATING_VERSION_DENIED: semver range is not allowed")
	}

	return nil
}

type NPXPolicy struct {
	AllowFloatingVersion bool
	CacheDir             string
	UserConfigFile       string
	Registry             string
}

func ResolveNPXNodeEnvironment(ctx context.Context, resolver nodeenv.Resolver) (nodeenv.Environment, error) {
	if resolver == nil {
		return nodeenv.Environment{}, fmt.Errorf("MCP_NPX_UNAVAILABLE: node environment resolver is not configured")
	}

	env, err := resolver.Resolve(ctx)
	if err != nil {
		return nodeenv.Environment{}, fmt.Errorf("MCP_NPX_UNAVAILABLE: %w", err)
	}

	if env.NPXCLI == "" {
		return nodeenv.Environment{}, fmt.Errorf("MCP_NPX_UNAVAILABLE: NPXCLI is not available")
	}

	if env.NodeBinary == "" {
		return nodeenv.Environment{}, fmt.Errorf("MCP_NPX_UNAVAILABLE: NodeBinary is not available")
	}

	return env, nil
}

func BuildNPXPath(nodeEnv nodeenv.Environment) string {
	if nodeEnv.NodeBinary == "" {
		return ""
	}
	return filepath.Dir(nodeEnv.NodeBinary)
}

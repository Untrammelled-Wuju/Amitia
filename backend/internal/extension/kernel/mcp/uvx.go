// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/scriptruntime/commandenv"
)

type UVXLauncher struct {
	commandResolver commandenv.Resolver
}

func NewUVXLauncher(commandResolver commandenv.Resolver) *UVXLauncher {
	return &UVXLauncher{commandResolver: commandResolver}
}

func (l *UVXLauncher) Resolve(ctx context.Context, spec MCPStdioSpec) (commandenv.Invocation, error) {
	if l.commandResolver == nil {
		return commandenv.Invocation{}, fmt.Errorf("MCP_UVX_NOT_AVAILABLE: command resolver is not configured")
	}
	return l.commandResolver.Resolve(ctx, commandenv.Request{Command: spec.Command, Args: spec.Args})
}

func BuildUvxInvocation(
	ctx context.Context,
	spec UvxLaunchSpec,
	policy UvxPolicy,
	resolver commandenv.Resolver,
) (commandenv.Invocation, map[string]string, error) {
	if resolver == nil {
		return commandenv.Invocation{}, nil, fmt.Errorf("MCP_UVX_NOT_AVAILABLE: command resolver is not configured")
	}

	requirement, err := buildCanonicalRequirement(spec)
	if err != nil {
		return commandenv.Invocation{}, nil, err
	}

	if err := validatePackageRequirement(requirement, policy); err != nil {
		return commandenv.Invocation{}, nil, err
	}

	if err := ValidateUvxCommand(spec.Command); err != nil {
		return commandenv.Invocation{}, nil, err
	}

	maxArgs := policy.MaxArgs
	if maxArgs <= 0 {
		maxArgs = UVXMaxArgs
	}
	maxArgBytes := policy.MaxArgBytes
	if maxArgBytes <= 0 {
		maxArgBytes = UVXMaxArgBytes
	}
	maxTotalBytes := UVXMaxArgsTotalBytes

	if err := ValidateUvxArgs(spec.Args, maxArgs, maxArgBytes, maxTotalBytes); err != nil {
		return commandenv.Invocation{}, nil, err
	}

	if err := ValidateUvxPythonSelector(spec.Python); err != nil {
		return commandenv.Invocation{}, nil, err
	}

	if err := ValidateUvxWorkDir(spec.WorkingDirectory); err != nil {
		return commandenv.Invocation{}, nil, err
	}

	uvArgs := buildUvxArgs(spec, requirement, policy)

	req := commandenv.Request{
		Command: "uv",
		Args:    uvArgs,
	}

	inv, err := resolver.Resolve(ctx, req)
	if err != nil {
		return commandenv.Invocation{}, nil, fmt.Errorf("MCP_UVX_NOT_AVAILABLE: %w", err)
	}

	env := buildUvxEnvironment(spec, policy)

	return inv, env, nil
}

func buildCanonicalRequirement(spec UvxLaunchSpec) (PythonToolRequirement, error) {
	if spec.Package != "" {
		return ParsePythonToolRequirement(spec.Package)
	}

	if spec.Command == "" {
		return PythonToolRequirement{}, ErrUvxPackageInvalid
	}

	req := PythonToolRequirement{Name: spec.Command}

	if spec.Version != "" {
		ver, err := parseVersionSpec(spec.Version)
		if err != nil {
			return PythonToolRequirement{}, err
		}
		req.VersionSpec = ver
	}

	req.Extras = spec.Extras

	return req, nil
}

func validatePackageRequirement(req PythonToolRequirement, policy UvxPolicy) error {
	if req.VersionSpec == "" && policy.RequireExactVersion {
		return ErrUvxPackageVersionUnlucky
	}
	return nil
}

func buildUvxArgs(spec UvxLaunchSpec, req PythonToolRequirement, policy UvxPolicy) []string {
	args := []string{"tool", "run"}

	args = append(args, "--no-config")
	args = append(args, "--no-python-downloads")
	args = append(args, "--isolated")
	args = append(args, "--no-progress")

	if spec.Python != "" {
		args = append(args, "--python", spec.Python)
	}

	if spec.Offline {
		args = append(args, "--offline")
	}

	if spec.Index != nil && spec.Index.DefaultIndex != "" && policy.AllowCustomIndex {
		args = append(args, "--index-url", spec.Index.DefaultIndex)
	}

	args = append(args, "--from", req.Canonical())

	args = append(args, spec.Command)

	args = append(args, spec.Args...)

	return args
}

func buildUvxEnvironment(spec UvxLaunchSpec, policy UvxPolicy) map[string]string {
	env := map[string]string{}

	if policy.cacheDir() != "" {
		env["UV_CACHE_DIR"] = policy.cacheDir()
	}

	for k, v := range spec.Environment {
		if isUvxReservedEnvVar(k) {
			continue
		}
		if strings.ContainsAny(k+v, "\r\n\x00") {
			continue
		}
		env[k] = v
	}

	return env
}

func (p UvxPolicy) cacheDir() string {
	return ""
}

func isUvxReservedEnvVar(name string) bool {
	switch name {
	case "UV_CACHE_DIR", "UV_TOOL_DIR", "UV_INDEX_URL", "UV_NO_CONFIG", "UV_PYTHON", "UV_OFFLINE", "UV_ISOLATED", "UV_NO_PROGRESS", "UV_NO_PYTHON_DOWNLOADS", "UV_ALLOW_INSECURE_HOST", "PATH", "HOME", "USERPROFILE", "SYSTEMROOT", "TEMP", "TMP":
		return true
	}
	return false
}

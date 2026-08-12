//go:build linux && !android

package packages

import (
	"context"
)

type PackagesHandler interface {
	Handle(ctx context.Context, operation string, payload map[string]any) (map[string]any, error)
	Status(ctx context.Context) (RuntimePackagesStatus, error)
}

type packagesHandler struct {
	service PackageService
}

func NewPackagesHandler(service PackageService) PackagesHandler {
	return &packagesHandler{service: service}
}

func (h *packagesHandler) Handle(ctx context.Context, operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case OpStatus:
		return h.handleStatus(ctx, payload)
	case OpAptUpdate:
		return h.handleAptUpdate(ctx, payload)
	case OpAptInstall:
		return h.handleAptInstall(ctx, payload)
	case OpAptQuery:
		return h.handleAptQuery(ctx, payload)
	case OpPythonInvoke:
		return h.handlePythonInvoke(ctx, payload)
	case OpPythonInstall:
		return h.handlePythonInstall(ctx, payload)
	case OpNodeInvoke:
		return h.handleNodeInvoke(ctx, payload)
	case OpNodeInstall:
		return h.handleNodeInstall(ctx, payload)
	case OpNodeNpx:
		return h.handleNodeNpx(ctx, payload)
	default:
		return nil, &Error{Code: "packages.invalid_operation", Message: "unknown operation: " + operation}
	}
}

func (h *packagesHandler) Status(ctx context.Context) (RuntimePackagesStatus, error) {
	return h.service.Status(ctx)
}

func (h *packagesHandler) handleStatus(ctx context.Context, payload map[string]any) (map[string]any, error) {
	status, err := h.service.Status(ctx)
	if err != nil {
		return nil, err
	}
	return statusToMap(status), nil
}

func (h *packagesHandler) handleAptUpdate(ctx context.Context, payload map[string]any) (map[string]any, error) {
	timeoutMs := parseInt64Field(payload, "timeoutMs", 5*60*1000)
	result, err := h.service.AptUpdate(ctx, timeoutMs)
	if err != nil {
		return nil, err
	}
	return installResultToMap(result), nil
}

func (h *packagesHandler) handleAptInstall(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := parseAptInstallRequest(payload)
	timeoutMs := parseInt64Field(payload, "timeoutMs", 5*60*1000)
	result, err := h.service.AptInstall(ctx, req, timeoutMs)
	if err != nil {
		return resultMapWithError(result, err)
	}
	return installResultToMap(result), nil
}

func (h *packagesHandler) handleAptQuery(ctx context.Context, payload map[string]any) (map[string]any, error) {
	packages := parseStringArray(payload, "packages")
	result, err := h.service.AptQuery(ctx, packages)
	if err != nil {
		return nil, err
	}
	return queryResultToMap(result), nil
}

func (h *packagesHandler) handlePythonInvoke(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := parsePythonInvokeRequest(payload)
	result, err := h.service.PythonInvoke(ctx, req)
	if err != nil {
		return nil, err
	}
	return invokeResultToMap(result), nil
}

func (h *packagesHandler) handlePythonInstall(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := parsePythonInstallRequest(payload)
	timeoutMs := parseInt64Field(payload, "timeoutMs", 5*60*1000)
	result, err := h.service.PythonInstall(ctx, req, timeoutMs)
	if err != nil {
		return resultMapWithError(result, err)
	}
	return installResultToMap(result), nil
}

func (h *packagesHandler) handleNodeInvoke(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := parseNodeInvokeRequest(payload)
	result, err := h.service.NodeInvoke(ctx, req)
	if err != nil {
		return nil, err
	}
	return invokeResultToMap(result), nil
}

func (h *packagesHandler) handleNodeInstall(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := parseNodeInstallRequest(payload)
	timeoutMs := parseInt64Field(payload, "timeoutMs", 5*60*1000)
	result, err := h.service.NodeInstall(ctx, req, timeoutMs)
	if err != nil {
		return resultMapWithError(result, err)
	}
	return installResultToMap(result), nil
}

func (h *packagesHandler) handleNodeNpx(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := parseNodeInvokeRequest(payload)
	result, err := h.service.NpxInvoke(ctx, req)
	if err != nil {
		return nil, err
	}
	return invokeResultToMap(result), nil
}

const (
	OpStatus       = "packages.status"
	OpAptUpdate    = "packages.apt.update"
	OpAptInstall   = "packages.apt.install"
	OpAptQuery     = "packages.apt.query"
	OpPythonInvoke = "packages.python.invoke"
	OpPythonInstall = "packages.python.install"
	OpNodeInvoke   = "packages.node.invoke"
	OpNodeInstall  = "packages.node.install"
	OpNodeNpx      = "packages.node.npx"
)

func parseInt64Field(m map[string]any, key string, defaultVal int64) int64 {
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	return defaultVal
}

func parseStringArray(m map[string]any, key string) []string {
	if v, ok := m[key].([]any); ok {
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func parseAptInstallRequest(m map[string]any) AptInstallRequest {
	return AptInstallRequest{Packages: parseStringArray(m, "packages")}
}

func parsePythonInvokeRequest(m map[string]any) PythonInvokeRequest {
	req := PythonInvokeRequest{}
	req.Args = parseStringArray(m, "args")
	req.WorkingDir, _ = m["workingDir"].(string)
	req.Stdin, _ = m["stdin"].(string)
	return req
}

func parsePythonInstallRequest(m map[string]any) PythonPackageInstallRequest {
	return PythonPackageInstallRequest{Packages: parseStringArray(m, "packages")}
}

func parseNodeInvokeRequest(m map[string]any) NodeInvokeRequest {
	req := NodeInvokeRequest{}
	req.Args = parseStringArray(m, "args")
	req.WorkingDir, _ = m["workingDir"].(string)
	req.Stdin, _ = m["stdin"].(string)
	return req
}

func parseNodeInstallRequest(m map[string]any) NodePackageInstallRequest {
	return NodePackageInstallRequest{Packages: parseStringArray(m, "packages")}
}

func statusToMap(s RuntimePackagesStatus) map[string]any {
	return map[string]any{
		"supported": s.Supported,
		"apt":       aptStatusToMap(s.Apt),
		"python":    pythonStatusToMap(s.Python),
		"node":      nodeStatusToMap(s.Node),
	}
}

func aptStatusToMap(s AptStatus) map[string]any {
	return map[string]any{
		"available":         s.Available,
		"executable":        s.Executable,
		"version":           s.Version,
		"architecture":      s.Architecture,
		"privilegeState":    s.PrivilegeState,
		"packageIndexState": s.PackageIndexState,
	}
}

func pythonStatusToMap(s PythonStatus) map[string]any {
	return map[string]any{
		"available":                    s.Available,
		"version":                      s.Version,
		"implementation":               s.Implementation,
		"pipAvailable":                 s.PipAvailable,
		"pipVersion":                   s.PipVersion,
		"venvAvailable":                s.VenvAvailable,
		"managedEnvironmentAvailable":  s.ManagedEnvironmentAvailable,
	}
}

func nodeStatusToMap(s NodeStatus) map[string]any {
	return map[string]any{
		"available":                  s.Available,
		"version":                    s.Version,
		"npmAvailable":               s.NPMAvailable,
		"npmVersion":                 s.NPMVersion,
		"npxAvailable":               s.NPXAvailable,
		"npxVersion":                 s.NPXVersion,
		"packageManagementAvailable": s.PackageManagementAvailable,
		"source":                     s.Source,
		"architecture":               s.Architecture,
	}
}

func installResultToMap(r *PackageInstallResult) map[string]any {
	if r == nil {
		return map[string]any{"exitCode": 1}
	}
	installed := make([]map[string]any, 0, len(r.Installed))
	for _, p := range r.Installed {
		installed = append(installed, map[string]any{"name": p.Name, "version": p.Version})
	}
	return map[string]any{
		"manager":    r.Manager,
		"requested":  r.Requested,
		"installed":  installed,
		"exitCode":   r.ExitCode,
		"durationMs": r.DurationMs,
	}
}

func queryResultToMap(r *PackageInstallResult) map[string]any {
	installed := make([]map[string]any, 0, len(r.Installed))
	for _, p := range r.Installed {
		installed = append(installed, map[string]any{"name": p.Name, "version": p.Version})
	}
	return map[string]any{
		"queried": r.Requested,
		"installed": installed,
	}
}

func invokeResultToMap(r *InvokeResult) map[string]any {
	return map[string]any{
		"exitCode":        r.ExitCode,
		"stdout":          r.Stdout,
		"stderr":          r.Stderr,
		"durationMs":      r.DurationMs,
		"timedOut":        r.TimedOut,
		"signal":          r.Signal,
		"stdoutTruncated": r.StdoutTruncated,
		"stderrTruncated": r.StderrTruncated,
	}
}

func resultMapWithError(result *PackageInstallResult, err error) (map[string]any, error) {
	if result != nil {
		m := installResultToMap(result)
		if err != nil {
			m["error"] = err.Error()
		}
		return m, err
	}
	return nil, err
}

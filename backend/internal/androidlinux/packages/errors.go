package packages

const (
	ErrCodeLinuxPackagesUnavailable    = "packages.unavailable"
	ErrCodePackageManagerNotFound      = "packages.apt.manager_not_found"
	ErrCodePackageManagerBusy          = "packages.apt.busy"
	ErrCodePackageManagerPermission    = "packages.apt.permission_denied"
	ErrCodePackageManagerIndexRequired = "packages.apt.index_required"
	ErrCodePackageManagerRecovery      = "packages.apt.recovery_required"
	ErrCodePackageNameInvalid          = "packages.name.invalid"
	ErrCodePackageInstallFailed        = "packages.install.failed"
	ErrCodePackageQueryFailed          = "packages.query.failed"
	ErrCodePythonNotFound              = "python.not_found"
	ErrCodePythonVersionFailed         = "python.version_failed"
	ErrCodePythonPipUnavailable        = "python.pip_unavailable"
	ErrCodePythonVenvUnavailable       = "python.venv_unavailable"
	ErrCodePythonVenvCreateFailed      = "python.venv_create_failed"
	ErrCodePythonPackageSpecInvalid    = "python.package_spec_invalid"
	ErrCodePythonPackageInstallFailed  = "python.package_install_failed"
	ErrCodeNodeEnvironmentUnavailable  = "node.environment_unavailable"
	ErrCodeNodeVersionFailed           = "node.version_failed"
	ErrCodeNodeNpmUnavailable          = "node.npm_unavailable"
	ErrCodeNodeNpxUnavailable          = "node.npx_unavailable"
	ErrCodeNpmPackageSpecInvalid       = "npm.package_spec_invalid"
	ErrCodeNpmPackageInstallFailed     = "npm.package_install_failed"
	ErrCodeNpxPackageNotInstalled      = "npx.package_not_installed"
	ErrCodeRuntimeInvokeFailed         = "packages.invoke.failed"
	ErrCodePackageOperationTimeout     = "packages.operation.timeout"
	ErrCodePackageOperationCancelled   = "packages.operation.cancelled"
)

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}

func (e *Error) IsShellExecutionError() bool {
	switch e.Code {
	case ErrCodeRuntimeInvokeFailed,
		ErrCodePackageOperationTimeout,
		ErrCodePackageOperationCancelled:
		return true
	}
	return false
}

func ErrUnsupported() *Error {
	return &Error{Code: ErrCodeLinuxPackagesUnavailable, Message: "packages provider not available"}
}

func ErrManagerNotFound(manager string) *Error {
	return &Error{Code: ErrCodePackageManagerNotFound, Message: "package manager not found: " + manager}
}

func ErrManagerBusy(manager string) *Error {
	return &Error{Code: ErrCodePackageManagerBusy, Message: "package manager busy: " + manager}
}

func ErrManagerPermission(manager string) *Error {
	return &Error{Code: ErrCodePackageManagerPermission, Message: "package manager permission denied: " + manager}
}

func ErrIndexRequired() *Error {
	return &Error{Code: ErrCodePackageManagerIndexRequired, Message: "package index update required before install"}
}

func ErrManagerRecovery() *Error {
	return &Error{Code: ErrCodePackageManagerRecovery, Message: "package manager requires recovery"}
}

func ErrInvalidPackageName(name string) *Error {
	return &Error{Code: ErrCodePackageNameInvalid, Message: "invalid package name: " + name}
}

func ErrPackageInstallFailed(pkg string, reason string) *Error {
	return &Error{Code: ErrCodePackageInstallFailed, Message: "install failed for " + pkg + ": " + reason}
}

func ErrPackageQueryFailed(pkg string, reason string) *Error {
	return &Error{Code: ErrCodePackageQueryFailed, Message: "query failed for " + pkg + ": " + reason}
}

func ErrPythonNotFound() *Error {
	return &Error{Code: ErrCodePythonNotFound, Message: "python3 not found"}
}

func ErrPythonVersionFailed(reason string) *Error {
	return &Error{Code: ErrCodePythonVersionFailed, Message: "python version detection failed: " + reason}
}

func ErrPythonPipUnavailable() *Error {
	return &Error{Code: ErrCodePythonPipUnavailable, Message: "pip not available"}
}

func ErrPythonVenvUnavailable() *Error {
	return &Error{Code: ErrCodePythonVenvUnavailable, Message: "python3-venv not available"}
}

func ErrPythonVenvCreateFailed(reason string) *Error {
	return &Error{Code: ErrCodePythonVenvCreateFailed, Message: "venv creation failed: " + reason}
}

func ErrInvalidPythonPackageSpec(spec string) *Error {
	return &Error{Code: ErrCodePythonPackageSpecInvalid, Message: "invalid python package spec: " + spec}
}

func ErrPythonPackageInstallFailed(pkg string, reason string) *Error {
	return &Error{Code: ErrCodePythonPackageInstallFailed, Message: "pip install failed for " + pkg + ": " + reason}
}

func ErrNodeEnvironmentUnavailable() *Error {
	return &Error{Code: ErrCodeNodeEnvironmentUnavailable, Message: "node environment unavailable"}
}

func ErrNodeUnavailable() *Error {
	return &Error{Code: ErrCodeNodeEnvironmentUnavailable, Message: "node environment unavailable - no resolver available"}
}

func ErrNodeVersionFailed(reason string) *Error {
	return &Error{Code: ErrCodeNodeVersionFailed, Message: "node version detection failed: " + reason}
}

func ErrNpmUnavailable() *Error {
	return &Error{Code: ErrCodeNodeNpmUnavailable, Message: "npm not available"}
}

func ErrNpxUnavailable() *Error {
	return &Error{Code: ErrCodeNodeNpxUnavailable, Message: "npx not available"}
}

func ErrInvalidNpmPackageSpec(spec string) *Error {
	return &Error{Code: ErrCodeNpmPackageSpecInvalid, Message: "invalid npm package spec: " + spec}
}

func ErrNpmPackageInstallFailed(pkg string, reason string) *Error {
	return &Error{Code: ErrCodeNpmPackageInstallFailed, Message: "npm install failed for " + pkg + ": " + reason}
}

func ErrNpxPackageNotInstalled(pkg string) *Error {
	return &Error{Code: ErrCodeNpxPackageNotInstalled, Message: "npx package not installed: " + pkg}
}

func ErrInvokeFailed(reason string) *Error {
	return &Error{Code: ErrCodeRuntimeInvokeFailed, Message: "invoke failed: " + reason}
}

func ErrTimeout(op string) *Error {
	return &Error{Code: ErrCodePackageOperationTimeout, Message: op + " timed out"}
}

func ErrCancelled(op string) *Error {
	return &Error{Code: ErrCodePackageOperationCancelled, Message: op + " cancelled"}
}

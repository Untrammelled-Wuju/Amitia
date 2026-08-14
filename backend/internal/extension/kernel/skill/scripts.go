// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package skill

import (
	"context"
	"errors"
	"time"
)

const (
	ScriptRuntimeNode   = "node"
	ScriptRuntimePython = "python"
	ScriptRuntimeNative = "native"
	ScriptRuntimeShell  = "shell"
)

const (
	ScriptKindExec    = "exec"
	ScriptKindQuery   = "query"
	ScriptKindRender  = "render"
	ScriptKindService = "service"
)

const (
	ArgTypeString = "string"
	ArgTypeInt    = "int"
	ArgTypeFloat  = "float"
	ArgTypeBool   = "bool"
	ArgTypeEnum   = "enum"
	ArgTypePath   = "path"
)

const (
	InputSourceLiteral  = "literal"
	InputSourceResource = "resource"
	InputSourceSecret   = "secret"
	InputSourceArtifact = "artifact"
)

const (
	OutputModeStdout   = "stdout"
	OutputModeFile     = "file"
	OutputModeResource = "resource"
)

const (
	WorkingDirPolicySkillRoot = "skillRoot"
	WorkingDirPolicyTemp      = "temp"
	WorkingDirPolicyExplicit  = "explicit"
)

const (
	StatusSuccess   = "success"
	StatusFailed    = "failed"
	StatusTimeout   = "timeout"
	StatusCancelled = "cancelled"
	StatusDenied    = "denied"
	StatusError     = "error"
)

const (
	InterpreterKindNode   = "node"
	InterpreterKindPython = "python"
	InterpreterKindNative = "native"
)

const (
	DefaultScriptTimeout  = 30 * time.Second
	MaxScriptTimeout      = 10 * time.Minute
	MaxStdoutBytes        = 1 << 20
	MaxStderrBytes        = 256 << 10
	MaxDiskBytes          = 256 << 20
	MaxOutputFiles        = 1000
	MaxConcurrentPerUser  = 2
	MaxConcurrentPerSkill = 1
)

var (
	ErrScriptNotFound                = errors.New("SKILL_SCRIPT_NOT_FOUND")
	ErrScriptNotActive               = errors.New("SKILL_SCRIPT_NOT_ACTIVE")
	ErrScriptPermissionDenied        = errors.New("SKILL_SCRIPT_PERMISSION_DENIED")
	ErrScriptInvalidDescriptor       = errors.New("SKILL_SCRIPT_INVALID_DESCRIPTOR")
	ErrScriptHashMismatch            = errors.New("SKILL_SCRIPT_HASH_MISMATCH")
	ErrScriptPathEscape              = errors.New("SKILL_SCRIPT_PATH_ESCAPE")
	ErrScriptSymlinkForbidden        = errors.New("SKILL_SCRIPT_SYMLINK_FORBIDDEN")
	ErrScriptHardlinkForbidden       = errors.New("SKILL_SCRIPT_HARDLINK_FORBIDDEN")
	ErrScriptInterpreterUnavailable  = errors.New("SKILL_SCRIPT_INTERPRETER_UNAVAILABLE")
	ErrScriptInterpreterForbidden    = errors.New("SKILL_SCRIPT_INTERPRETER_FORBIDDEN")
	ErrScriptShellForbidden          = errors.New("SKILL_SCRIPT_SHELL_FORBIDDEN")
	ErrScriptShebangForbidden        = errors.New("SKILL_SCRIPT_SHEBANG_FORBIDDEN")
	ErrScriptDependencyForbidden     = errors.New("SKILL_SCRIPT_DEPENDENCY_FORBIDDEN")
	ErrScriptAutoInstallForbidden    = errors.New("SKILL_SCRIPT_AUTO_INSTALL_FORBIDDEN")
	ErrScriptArgInvalid              = errors.New("SKILL_SCRIPT_ARG_INVALID")
	ErrScriptArgMissing              = errors.New("SKILL_SCRIPT_ARG_MISSING")
	ErrScriptArgTypeMismatch         = errors.New("SKILL_SCRIPT_ARG_TYPE_MISMATCH")
	ErrScriptInputInvalid            = errors.New("SKILL_SCRIPT_INPUT_INVALID")
	ErrScriptInputResolutionFailed   = errors.New("SKILL_SCRIPT_INPUT_RESOLUTION_FAILED")
	ErrScriptOutputInvalid           = errors.New("SKILL_SCRIPT_OUTPUT_INVALID")
	ErrScriptOutputCommitFailed      = errors.New("SKILL_SCRIPT_OUTPUT_COMMIT_FAILED")
	ErrScriptResourceUnresolved      = errors.New("SKILL_SCRIPT_RESOURCE_UNRESOLVED")
	ErrScriptResourceCommitFailed    = errors.New("SKILL_SCRIPT_RESOURCE_COMMIT_FAILED")
	ErrScriptSecretBindingFailed     = errors.New("SKILL_SCRIPT_SECRET_BINDING_FAILED")
	ErrScriptTimeoutExceeded         = errors.New("SKILL_SCRIPT_TIMEOUT_EXCEEDED")
	ErrScriptStdoutOverflow          = errors.New("SKILL_SCRIPT_STDOUT_OVERFLOW")
	ErrScriptStderrOverflow          = errors.New("SKILL_SCRIPT_STDERR_OVERFLOW")
	ErrScriptDiskQuotaExceeded       = errors.New("SKILL_SCRIPT_DISK_QUOTA_EXCEEDED")
	ErrScriptFileQuotaExceeded       = errors.New("SKILL_SCRIPT_FILE_QUOTA_EXCEEDED")
	ErrScriptConcurrencyExceeded     = errors.New("SKILL_SCRIPT_CONCURRENCY_EXCEEDED")
	ErrScriptProcessFailed           = errors.New("SKILL_SCRIPT_PROCESS_FAILED")
	ErrScriptProcessRegisterFailed   = errors.New("SKILL_SCRIPT_PROCESS_REGISTER_FAILED")
	ErrScriptProcessStartFailed      = errors.New("SKILL_SCRIPT_PROCESS_START_FAILED")
	ErrScriptProcessSupervisorFailed = errors.New("SKILL_SCRIPT_PROCESS_SUPERVISOR_FAILED")
	ErrScriptInvalidWorkingDir       = errors.New("SKILL_SCRIPT_INVALID_WORKING_DIR")
	ErrScriptInvalidOutputMode       = errors.New("SKILL_SCRIPT_INVALID_OUTPUT_MODE")
	ErrScriptRestartForbidden        = errors.New("SKILL_SCRIPT_RESTART_FORBIDDEN")
	ErrScriptDaemonForbidden         = errors.New("SKILL_SCRIPT_DAEMON_FORBIDDEN")
	ErrScriptInheritEnvForbidden     = errors.New("SKILL_SCRIPT_INHERIT_ENV_FORBIDDEN")
	ErrScriptRawExecutableForbidden  = errors.New("SKILL_SCRIPT_RAW_EXECUTABLE_FORBIDDEN")
	ErrScriptInvalidTimeout          = errors.New("SKILL_SCRIPT_INVALID_TIMEOUT")
	ErrScriptExecutionCancelled      = errors.New("SKILL_SCRIPT_EXECUTION_CANCELLED")
	ErrScriptInternalError           = errors.New("SKILL_SCRIPT_INTERNAL_ERROR")
)

type SkillScriptDescriptor struct {
	SkillName        string
	ExtensionID      string
	ArtifactID       string
	SkillContentHash string
	RelativePath     string
	FileHash         string
	Kind             string
	Runtime          string
	EntryName        string
	DeclaredArgs     []SkillScriptArgSpec
	DeclaredInputs   []SkillScriptInputSpec
	DeclaredOutputs  []SkillScriptOutputSpec
	Dependencies     []SkillScriptDependency
	Permissions      []string
	Timeout          time.Duration
	WorkingDirPolicy string
	OutputMode       string
	Description      string
	Metadata         map[string]any
}

type SkillScriptArgSpec struct {
	Name       string
	Type       string
	Required   bool
	Repeatable bool
	Enum       []string
	MinInt     *int64
	MaxInt     *int64
	MinFloat   *float64
	MaxFloat   *float64
	MaxLength  int
	CLIFlag    string
	Position   int
	Default    string
}

type SkillScriptInputSpec struct {
	Name     string
	Type     string
	Source   string
	Required bool
	Resource string
	Secret   string
	Literal  string
}

type SkillScriptOutputSpec struct {
	Name     string
	Type     string
	Mode     string
	Path     string
	Resource string
}

type SkillScriptDependency struct {
	Kind     string
	Name     string
	Version  string
	Optional bool
}

type SkillScriptResult struct {
	ExecutionID     string
	ExitCode        int
	Status          string
	Output          string
	Resources       map[string]string
	StdoutTruncated bool
	StderrTruncated bool
	Duration        time.Duration
	FileCount       int
	ErrorMessage    string
}

type SkillScriptExecutionPlan struct {
	Descriptor      SkillScriptDescriptor
	Interpreter     ScriptInterpreter
	ResolvedArgs    map[string]any
	ResolvedInputs  map[string]string
	ResolvedOutputs map[string]string
	Permissions     []string
	Timeout         time.Duration
	WorkingDir      string
	Executable      string
	Args            []string
	EnvPolicy       string
	Env             map[string]string
}

type ScriptInterpreter struct {
	Kind       string
	Executable string
	ArgsPrefix []string
	Source     string
	Version    string
}

type ScriptInterpreterResolver interface {
	Resolve(ctx context.Context, runtime string, extensionID string) (ScriptInterpreter, error)
	ResolveFromDescriptor(ctx context.Context, desc SkillScriptDescriptor) (ScriptInterpreter, error)
}

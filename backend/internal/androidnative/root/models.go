package root

import "context"

type AuthorizationState string

const (
	AuthorizationUnknown  AuthorizationState = "unknown"
	AuthorizationRequired AuthorizationState = "required"
	AuthorizationGranted  AuthorizationState = "granted"
	AuthorizationDenied   AuthorizationState = "denied"
)

type RootStatus struct {
	PlatformSupported   bool               `json:"platformSupported"`
	RootFramework       string             `json:"rootFramework,omitempty"`
	RootManagerDetected bool               `json:"rootManagerDetected"`
	SUBinaryDetected    bool               `json:"suBinaryDetected"`
	Authorization       AuthorizationState `json:"authorizationState"`
	RootAvailable       bool               `json:"rootAvailable"`
	Backend             string             `json:"backend"`
	State               string             `json:"state"`
}

type ExecuteRequest struct {
	Executable string            `json:"executable"`
	Args       []string          `json:"args,omitempty"`
	Stdin      string            `json:"stdin,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	WorkDir    string            `json:"workDir,omitempty"`
	TimeoutMS  int               `json:"timeoutMs,omitempty"`
	Mode       string            `json:"mode,omitempty"`
}

type ExecuteResult struct {
	ExitCode          int    `json:"exitCode"`
	ExitCodeAvailable bool   `json:"exitCodeAvailable"`
	Stdout            string `json:"stdout"`
	Stderr            string `json:"stderr"`
	DurationMS        int64  `json:"durationMs"`
	TimedOut          bool   `json:"timedOut"`
}

type InternalExecuteOptions struct {
	Timeout      int64
	MaxOutput    int64
	StdinEnabled bool
}

type InternalRootExecutor interface {
	ExecuteRoot(
		ctx context.Context,
		req ExecuteRequest,
		opts InternalExecuteOptions,
	) (ExecuteResult, error)
}

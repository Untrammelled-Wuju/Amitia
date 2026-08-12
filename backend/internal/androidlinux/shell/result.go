//go:build linux && !android

package shell

type ShellExecuteResult struct {
	ExitCode        int               `json:"exitCode"`
	Stdout          string            `json:"stdout"`
	Stderr          string            `json:"stderr"`
	StdoutTruncated bool              `json:"stdoutTruncated"`
	StderrTruncated bool              `json:"stderrTruncated"`
	StdoutBytes     int64             `json:"stdoutBytes"`
	StderrBytes     int64             `json:"stderrBytes"`
	DurationMs      int64             `json:"durationMs"`
	TimedOut        bool              `json:"timedOut"`
	Signal          string            `json:"signal,omitempty"`
	WorkingDir      string            `json:"workingDir"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type ShellExecOutput struct {
	ExitCode        int
	Stdout          []byte
	Stderr          []byte
	StdoutTruncated bool
	StderrTruncated bool
	DurationMs      int64
	TimedOut        bool
	Signal          string
	ProcessPID      int
}

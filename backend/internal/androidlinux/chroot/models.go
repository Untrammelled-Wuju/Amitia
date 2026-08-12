//go:build linux && !android

package chroot

type ChrootStatus struct {
	Enabled          bool     `json:"enabled"`
	DefaultRootFSP   string   `json:"defaultRootfsPath,omitempty"`
	KnownFSPs        []string `json:"knownRootfsPaths"`
	MaxFSBytes       int64    `json:"maxFsBytes"`
	MaxEnvironments  int      `json:"maxEnvironments"`
	AvailableEnvironments []string `json:"availableEnvironments"`
	CurrentEnv       string   `json:"currentEnvironment"`
	ExecBackends     []string `json:"execBackends"`
}

type ChrootInspectRequest struct {
	RootFSPath string `json:"rootfsPath"`
}

type ChrootInspectResult struct {
	RootFSPath  string `json:"rootfsPath"`
	Exists      bool   `json:"exists"`
	Valid       bool   `json:"valid"`
	TotalBytes  int64  `json:"totalBytes"`
	FileCount   int    `json:"fileCount"`
	HasBinSH    bool   `json:"hasBinSh"`
	HasBinBash  bool   `json:"hasBinBash"`
	FSType      string `json:"fsType"`
	Error       string `json:"error,omitempty"`
}

type ChrootExecRequest struct {
	RootFSPath     string            `json:"rootfsPath"`
	Command        string            `json:"command"`
	Args           []string          `json:"args,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
	Stdin          string            `json:"stdin,omitempty"`
	TimeoutMs      int64             `json:"timeoutMs,omitempty"`
	MaxOutputBytes int64             `json:"maxOutputBytes,omitempty"`
	WorkingDir     string            `json:"workingDir,omitempty"`
	User           string            `json:"user,omitempty"`
}

type ChrootExecResult struct {
	RootFSPath      string `json:"rootfsPath"`
	ExitCode        int    `json:"exitCode"`
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	StdoutTruncated bool   `json:"stdoutTruncated"`
	StderrTruncated bool   `json:"stderrTruncated"`
	StdoutBytes     int64  `json:"stdoutBytes"`
	StderrBytes     int64  `json:"stderrBytes"`
	DurationMs      int64  `json:"durationMs"`
	Environment     string `json:"environment"`
}

type RootFSEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	TotalBytes int64  `json:"totalBytes"`
	FileCount  int    `json:"fileCount"`
	Valid      bool   `json:"valid"`
}

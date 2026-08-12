//go:build linux && !android

package packages

type RuntimePackagesStatus struct {
	Supported bool       `json:"supported"`
	Apt       AptStatus  `json:"apt"`
	Python    PythonStatus `json:"python"`
	Node      NodeStatus `json:"node"`
}

type AptStatus struct {
	Available         bool   `json:"available"`
	Executable        string `json:"executable"`
	Version           string `json:"version"`
	Architecture      string `json:"architecture"`
	PrivilegeState    string `json:"privilegeState"`
	PackageIndexState string `json:"packageIndexState"`
}

type PythonStatus struct {
	Available                  bool   `json:"available"`
	Version                    string `json:"version"`
	Implementation             string `json:"implementation"`
	PipAvailable               bool   `json:"pipAvailable"`
	PipVersion                 string `json:"pipVersion"`
	VenvAvailable              bool   `json:"venvAvailable"`
	ManagedEnvironmentAvailable bool `json:"managedEnvironmentAvailable"`
}

type NodeStatus struct {
	Available                  bool   `json:"available"`
	Version                    string `json:"version"`
	NPMAvailable               bool   `json:"npmAvailable"`
	NPMVersion                 string `json:"npmVersion"`
	NPXAvailable               bool   `json:"npxAvailable"`
	NPXVersion                 string `json:"npxVersion"`
	PackageManagementAvailable bool   `json:"packageManagementAvailable"`
	Source                     string `json:"source"`
	Architecture               string `json:"architecture"`
}

type AptInstallRequest struct {
	Packages []string `json:"packages"`
}

type PythonInvokeRequest struct {
	Args       []string `json:"args"`
	WorkingDir string   `json:"workingDir,omitempty"`
	Stdin      string   `json:"stdin,omitempty"`
	TimeoutMs  int64    `json:"timeoutMs,omitempty"`
}

type PythonPackageInstallRequest struct {
	Packages []string `json:"packages"`
}

type NodeInvokeRequest struct {
	Args       []string `json:"args"`
	WorkingDir string   `json:"workingDir,omitempty"`
	Stdin      string   `json:"stdin,omitempty"`
	TimeoutMs  int64    `json:"timeoutMs,omitempty"`
}

type NodePackageInstallRequest struct {
	Packages []string `json:"packages"`
}

type InstalledPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type PackageInstallResult struct {
	Manager    string            `json:"manager"`
	Requested  []string          `json:"requested"`
	Installed  []InstalledPackage `json:"installed"`
	ExitCode   int               `json:"exitCode"`
	DurationMs int64             `json:"durationMs"`
}

type InvokeResult struct {
	ExitCode        int    `json:"exitCode"`
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	DurationMs      int64  `json:"durationMs"`
	TimedOut        bool   `json:"timedOut"`
	Signal          string `json:"signal,omitempty"`
	StdoutTruncated bool   `json:"stdoutTruncated"`
	StderrTruncated bool   `json:"stderrTruncated"`
}

type PackageNameVersion struct {
	Name    string
	Version string
}

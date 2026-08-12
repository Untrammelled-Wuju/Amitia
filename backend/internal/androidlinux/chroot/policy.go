//go:build linux && !android

package chroot

type Policy struct {
	Enabled            bool     `json:"enabled"`
	MaxFSBytes         int64    `json:"maxFsBytes"`
	MaxEnvironments    int      `json:"maxEnvironments"`
	AllowedRootFSDirs  []string `json:"allowedRootfsDirs"`
	RequireBinSH       bool     `json:"requireBinSH"`
	AllowProotExec     bool     `json:"allowProotExec"`
	DefaultExecBackend string   `json:"defaultExecBackend"`
	MaxOutputBytes     int64    `json:"maxOutputBytes"`
	DefaultTimeoutSec  int      `json:"defaultTimeoutSec"`
	MaxTimeoutSec      int      `json:"maxTimeoutSec"`
	MaxStdinBytes      int64    `json:"maxStdinBytes"`
}

func DefaultPolicy() Policy {
	return Policy{
		Enabled:            true,
		MaxFSBytes:         10 * 1024 * 1024 * 1024,
		MaxEnvironments:    4,
		RequireBinSH:       true,
		AllowProotExec:     true,
		DefaultExecBackend: "proot",
		MaxOutputBytes:     100 * 1024 * 1024,
		DefaultTimeoutSec:  30,
		MaxTimeoutSec:      600,
		MaxStdinBytes:      10 * 1024 * 1024,
	}
}

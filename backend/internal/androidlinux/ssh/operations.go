//go:build linux && !android

package ssh

const (
	OpSSHStatus     = "ssh.status"
	OpSSHExec       = "ssh.exec"
	OpSSHHostKeyScan = "ssh.hostkey.scan"
)

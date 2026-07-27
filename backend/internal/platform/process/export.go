package process

import "os/exec"

func ConfigureProcess(cmd *exec.Cmd) {
	configureProcess(cmd)
}

func AttachProcessTree(cmd *exec.Cmd) (ProcessTreeHandle, error) {
	return attachProcessTree(cmd)
}

func TerminateProcessTree(pid int, handle ProcessTreeHandle) error {
	return terminateProcessTree(pid, handle)
}

func CloseProcessTree(handle ProcessTreeHandle) {
	closeProcessTree(handle)
}

func IsProcessAlive(pid int) bool {
	return defaultIsAlive(pid)
}

func defaultIsAlive(pid int) bool {
	return NewDefaultProcessManager().IsAlive(pid)
}

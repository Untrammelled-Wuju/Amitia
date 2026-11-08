//go:build !darwin || ios

package process

import (
	"errors"
	"io"
	"os/exec"
)

func startParentBoundCommand(cmd *exec.Cmd) (io.Closer, error) {
	return nil, cmd.Start()
}

func RunParentBoundChild([]string) error {
	return errors.New("process: parent-bound helper requires macOS")
}

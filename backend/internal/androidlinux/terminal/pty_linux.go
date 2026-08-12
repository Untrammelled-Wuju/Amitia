//go:build linux && !android

package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

type ptyHandle struct {
	master *os.File
}

type ptySize struct {
	Rows uint16
	Cols uint16
}

func startPTY(shell string, size ptySize, env []string, cwd string) (*os.File, *exec.Cmd, int, error) {
	cmd := exec.Command(shell, "-i")
	cmd.Env = env
	cmd.Dir = cwd

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
	}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: size.Rows,
		Cols: size.Cols,
	})
	if err != nil {
		return nil, nil, 0, fmt.Errorf("start pty: %w", err)
	}

	return ptmx, cmd, cmd.Process.Pid, nil
}

func resizePTY(ptmx *os.File, size ptySize) error {
	if ptmx == nil {
		return fmt.Errorf("ptmx is nil")
	}
	return pty.Setsize(ptmx, &pty.Winsize{
		Rows: size.Rows,
		Cols: size.Cols,
	})
}

func closePTY(ptmx *os.File) error {
	if ptmx == nil {
		return nil
	}
	return ptmx.Close()
}

func probePTY() error {
	cmd := exec.Command("/bin/sh", "-i")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
	}
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("pty probe failed: %w", err)
	}
	_ = ptmx.Close()
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	return nil
}

//go:build linux || darwin

package workbenchterminal

import (
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
)

type unixProcess struct {
	master    *os.File
	command   *exec.Cmd
	closeOnce sync.Once
}

func launchProcess(cwd string, rows, cols uint16) (process, error) {
	command := exec.Command(shellExecutable(), "-i")
	command.Dir = cwd
	command.Env = append(os.Environ(), "TERM=xterm-256color")
	master, err := pty.StartWithSize(command, &pty.Winsize{Rows: rows, Cols: cols})
	if err != nil {
		return nil, err
	}
	return &unixProcess{master: master, command: command}, nil
}
func (p *unixProcess) Read(b []byte) (int, error)  { return p.master.Read(b) }
func (p *unixProcess) Write(b []byte) (int, error) { return p.master.Write(b) }
func (p *unixProcess) Wait() error                 { return p.command.Wait() }
func (p *unixProcess) Mode() string                { return "pty" }
func (p *unixProcess) Resize(rows, cols uint16) error {
	return pty.Setsize(p.master, &pty.Winsize{Rows: rows, Cols: cols})
}
func (p *unixProcess) Close() error {
	p.closeOnce.Do(func() {
		if p.command.Process != nil {
			_ = syscall.Kill(-p.command.Process.Pid, syscall.SIGKILL)
			_ = p.command.Process.Kill()
		}
		_ = p.master.Close()
	})
	return nil
}

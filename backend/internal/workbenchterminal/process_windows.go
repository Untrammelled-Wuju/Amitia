//go:build windows

package workbenchterminal

import (
	"errors"
	"io"
	amitialog "log"
	"os"
	"os/exec"
	"sync"
)

type windowsProcess struct {
	command   *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	closeOnce sync.Once
}

func launchProcess(cwd string, rows, cols uint16) (process, error) {
	p, err := launchConPTY(cwd, rows, cols)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, ErrConPTYUnavailable) {
		return nil, err
	}
	return launchPipeProcess(cwd, rows, cols)
}

func launchPipeProcess(cwd string, rows, cols uint16) (process, error) {
	command := exec.Command(shellExecutable())
	command.Dir = cwd
	command.Env = os.Environ()
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		if err := stdin.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return nil, err
	}
	command.Stderr = command.Stdout
	if err := command.Start(); err != nil {
		if err := stdin.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if err := stdout.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return nil, err
	}
	return &windowsProcess{command: command, stdin: stdin, stdout: stdout}, nil
}
func (p *windowsProcess) Read(b []byte) (int, error)  { return p.stdout.Read(b) }
func (p *windowsProcess) Write(b []byte) (int, error) { return p.stdin.Write(b) }
func (p *windowsProcess) Resize(rows, cols uint16) error {
	return errors.New("ConPTY is not available in pipe mode")
}
func (p *windowsProcess) Mode() string { return "pipe" }
func (p *windowsProcess) Wait() error  { return p.command.Wait() }
func (p *windowsProcess) Close() error {
	p.closeOnce.Do(func() {
		if err := p.stdin.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if err := p.stdout.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if p.command.Process != nil {
			terminateTerminalProcessTree(p.command.Process.Pid)
			if err := p.command.Process.Kill(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
		}
	})
	return nil
}

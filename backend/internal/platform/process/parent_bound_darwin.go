//go:build darwin && !ios

package process

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const parentBoundStopGrace = time.Second

func startParentBoundCommand(cmd *exec.Cmd) (io.Closer, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	parent, lifetime, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	cmd.Args = append([]string{executable, ParentBoundChildFlag, cmd.Path}, cmd.Args[1:]...)
	cmd.Path = executable
	cmd.ExtraFiles = []*os.File{parent}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	startErr := cmd.Start()
	closeErr := parent.Close()
	if startErr != nil {
		return nil, errors.Join(startErr, closeErr, lifetime.Close())
	}
	if closeErr != nil {
		return nil, errors.Join(closeErr, lifetime.Close(), cmd.Wait())
	}
	return lifetime, nil
}

func RunParentBoundChild(args []string) (resultErr error) {
	if len(args) == 0 || !filepath.IsAbs(args[0]) {
		return errors.New("process: parent-bound child requires an absolute executable")
	}
	if syscall.Getpgrp() != os.Getpid() {
		return errors.New("process: parent-bound helper requires its own process group")
	}
	parent := os.NewFile(3, "parent-lifetime")
	defer func() { resultErr = errors.Join(resultErr, parent.Close()) }()
	info, err := parent.Stat()
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		return errors.New("process: parent lifetime descriptor is not a pipe")
	}
	syscall.CloseOnExec(3)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, os.Interrupt)
	defer signal.Stop(signals)
	parentDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, parent)
		parentDone <- err
	}()

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("process: start parent-bound child: %w", err)
	}
	childDone := make(chan error, 1)
	go func() { childDone <- cmd.Wait() }()
	var parentErr error
	select {
	case err := <-childDone:
		if err == nil {
			return errors.New("process: parent-bound child exited unexpectedly")
		}
		return err
	case parentErr = <-parentDone:
	case <-signals:
	}

	stopErr := cmd.Process.Signal(syscall.SIGTERM)
	if errors.Is(stopErr, os.ErrProcessDone) {
		stopErr = nil
	}
	timer := time.NewTimer(parentBoundStopGrace)
	defer timer.Stop()
	select {
	case err := <-childDone:
		return errors.Join(parentErr, stopErr, err)
	case <-timer.C:
		killErr := cmd.Process.Kill()
		if errors.Is(killErr, os.ErrProcessDone) {
			killErr = nil
		}
		if killErr != nil {
			return errors.Join(parentErr, stopErr, killErr)
		}
		return errors.Join(parentErr, stopErr, errors.New("process: parent-bound child exceeded shutdown grace period"), <-childDone)
	}
}

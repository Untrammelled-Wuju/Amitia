//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

const supervisorFlag = "--supervise"

func hasSupervisorFlag(args []string) bool {
	for _, arg := range args {
		if arg == supervisorFlag {
			return true
		}
	}
	return false
}

func supervisorChildArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == supervisorFlag {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func runSupervisor(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	command := exec.Command(exe, supervisorChildArgs(args)...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return err
	}
	fmt.Printf("AMITIA_CORE_PID=%d\n", command.Process.Pid)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		<-signals
		_ = command.Process.Signal(syscall.SIGTERM)
	}()

	return command.Wait()
}

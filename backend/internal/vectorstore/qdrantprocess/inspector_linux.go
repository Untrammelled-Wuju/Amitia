// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build linux

package qdrantprocess

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type linuxInspector struct {
	procRoot string
}

func newPlatformInspector() ProcessInspector {
	return linuxInspector{procRoot: "/proc"}
}

func newLinuxInspector(procRoot string) ProcessInspector {
	return linuxInspector{procRoot: procRoot}
}

func (l linuxInspector) Inspect(ctx context.Context, pid int) (ProcessIdentity, error) {
	if pid <= 0 {
		return ProcessIdentity{}, fmt.Errorf("%w: invalid PID %d", ErrProcessInspectionFailed, pid)
	}

	exePath, err := l.readLink(fmt.Sprintf("%s/%d/exe", l.procRoot, pid))
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("%w: exe: %v", ErrProcessInspectionFailed, err)
	}
	exePath = cleanExePath(exePath)

	startMarker, err := l.readStartMarker(pid)
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("%w: start marker: %v", ErrProcessInspectionFailed, err)
	}

	var cmdline []string
	cmdline, err = l.readCmdline(pid)
	if err != nil && !errors.Is(err, io.EOF) {
		cmdline = nil
	}

	return ProcessIdentity{
		PID:            pid,
		ExecutablePath: exePath,
		StartMarker:    startMarker,
		CommandLine:    cmdline,
	}, nil
}

func (l linuxInspector) IsAlive(ctx context.Context, id ProcessIdentity) (bool, error) {
	actual, err := l.Inspect(ctx, id.PID)
	if err != nil {
		return false, err
	}
	if !SameProcessIdentity(id, actual) {
		return false, nil
	}
	return true, nil
}

func (l linuxInspector) readStartMarker(pid int) (string, error) {
	statPath := fmt.Sprintf("%s/%d/stat", l.procRoot, pid)
	data, err := os.ReadFile(statPath)
	if err != nil {
		return "", err
	}
	line := strings.TrimRight(string(data), "\n")
	if len(line) == 0 {
		return "", errors.New("empty stat")
	}

	return parseStatStartMarker(line, pid)
}

func parseStatStartMarker(line string, pid int) (string, error) {
	idx := strings.LastIndex(line, ")")
	if idx < 0 || idx+2 >= len(line) {
		return "", errors.New("malformed stat")
	}
	fields := strings.Fields(line[idx+2:])
	if len(fields) < 20 {
		return "", errors.New("truncated stat")
	}
	starttime := fields[19]
	return strconv.Itoa(pid) + ":" + starttime, nil
}

func (l linuxInspector) readCmdline(pid int) ([]string, error) {
	cmdlinePath := fmt.Sprintf("%s/%d/cmdline", l.procRoot, pid)
	f, err := os.Open(cmdlinePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	const maxCmdline = 64 * 1024
	data := make([]byte, maxCmdline)
	n, err := f.Read(data)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, ErrProcessInspectionFailed
	}
	data = data[:n]

	if len(data) == 0 {
		return nil, nil
	}

	var args []string
	start := 0
	for idx := 0; idx < len(data); idx++ {
		if data[idx] == 0 {
			if idx > start {
				args = append(args, string(data[start:idx]))
			}
			start = idx + 1
		}
	}
	if start < len(data) {
		args = append(args, string(data[start:]))
	}
	return args, nil
}

func (l linuxInspector) readLink(path string) (string, error) {
	return os.Readlink(path)
}

func cleanExePath(path string) string {
	cleaned := filepath.Clean(path)
	cleaned = strings.TrimSuffix(cleaned, " (deleted)")
	return cleaned
}

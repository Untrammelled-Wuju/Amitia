package main

import (
	"runtime"
	"strings"
	"testing"
)

func skipUnsupportedDeviceIsolation(t *testing.T, err error) {
	t.Helper()
	if runtime.GOOS == "darwin" && err != nil && strings.Contains(err.Error(), "task_dependency_unavailable") {
		t.Skipf("verified device isolation is unavailable on this host: %v", err)
	}
}

func skipUnsupportedDeviceIsolationBody(t *testing.T, body string) {
	t.Helper()
	if runtime.GOOS == "darwin" && strings.Contains(body, "task_dependency_unavailable") {
		t.Skip("verified device isolation is unavailable on this host")
	}
}

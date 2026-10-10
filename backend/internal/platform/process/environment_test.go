package process

import (
	"runtime"
	"testing"
)

func TestWindowsEnvironmentBuilderRecoversMissingTMP(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only PowerShell sandbox prerequisite")
	}
	validTemp := t.TempDir()
	t.Setenv("TMP", "")
	t.Setenv("TEMP", validTemp)
	b := NewEnvironmentBuilder()
	if b.vars["TMP"] != validTemp || b.vars["TEMP"] != validTemp {
		t.Fatalf("child PowerShell would inherit invalid temporary path: TMP=%q TEMP=%q", b.vars["TMP"], b.vars["TEMP"])
	}
	if b.vars["ProgramFiles(x86)"] != "" {
		t.Fatal("Windows temporary directory repair must not expand child environment privileges")
	}
}

func TestWindowsEnvironmentBuilderRetainsExistingTMP(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only PowerShell sandbox prerequisite")
	}
	tmp, temp := t.TempDir(), t.TempDir()
	t.Setenv("TMP", tmp)
	t.Setenv("TEMP", temp)
	b := NewEnvironmentBuilder()
	if b.vars["TMP"] != tmp || b.vars["TEMP"] != temp {
		t.Fatalf("explicit temporary paths were overwritten: TMP=%q TEMP=%q", b.vars["TMP"], b.vars["TEMP"])
	}
}

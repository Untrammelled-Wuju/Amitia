package packageformat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizePackagePathPortableCanonical(t *testing.T) {
	valid := []string{
		"actions/idle/action.json",
		"actions/idle/frame 01.png",
		"actions/待机/帧#01.png",
		"actions/idle/100%.png",
		"actions/idle/frame..png",
		"actions/idle/..frame.png",
	}
	for _, input := range valid {
		got, err := NormalizePackagePath(input)
		if err != nil || got != input {
			t.Fatalf("expected valid canonical path %q, got %q err=%v", input, got, err)
		}
	}
}

func TestNormalizePackagePathRejectsPortableHazards(t *testing.T) {
	invalid := []string{
		"", "/absolute/file.png", "../escape.png", "actions/../escape.png",
		"actions/./idle.png", "actions//idle.png", "actions/idle/",
		"actions\\idle\\frame.png", "C:/frame.png", "actions/idle/a?.png",
		"actions/idle/a*.png", "actions/idle/a|b.png", "actions/idle/a:b.png",
		"actions/idle/a<.png", "actions/idle/a>.png", "actions/idle/a\"b.png",
		"actions/idle/trailing. ", "actions/idle/trailing.", "actions/idle/CON",
		"actions/idle/con.png", "actions/idle/LPT1.json", "actions/idle/NUL.asset.bin",
		"actions/idle/control\x00.png", "actions/idle/control\x1f.png",
		"actions/e\u0301/frame.png", strings.Repeat("a", maxPackageSegmentBytes+1),
		strings.Repeat("a", maxPackagePathBytes+1),
	}
	for _, input := range invalid {
		if got, err := NormalizePackagePath(input); err == nil {
			t.Fatalf("expected invalid path %q, got %q", input, got)
		}
	}
}

func TestSecureJoinUnderRoot(t *testing.T) {
	root := t.TempDir()
	got, err := SecureJoinUnderRoot(root, "actions/idle/frame.png")
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(root, "actions", "idle", "frame.png")
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
	for _, input := range []string{"../escape", "actions/../escape", "/absolute"} {
		if _, err := SecureJoinUnderRoot(root, input); err == nil {
			t.Fatalf("expected secure join to reject %q", input)
		}
	}
}

func TestSecureResolveExistingUnderRootRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "actions", "idle"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "actions", "idle", "frame.png"), []byte("frame"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := SecureResolveExistingUnderRoot(root, "actions/idle/frame.png"); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "actions", "idle", "link.png")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := SecureResolveExistingUnderRoot(root, "actions/idle/link.png"); err == nil {
		t.Fatal("expected symlink rejection")
	}
}

func TestCaseFoldPathUsesUnicodeSimpleLowercase(t *testing.T) {
	if got := CaseFoldPath("actions/İ/Frame.PNG"); got != "actions/i/frame.png" {
		t.Fatalf("unexpected case-fold key: %q", got)
	}
}

// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package resourceuri

import (
	"errors"
	"strings"
	"testing"
)

func TestParseAllResourceRoots(t *testing.T) {
	roots := []struct {
		input string
		root  ResourceRoot
	}{
		{"amitia://workspace/", ResourceRootWorkspace},
		{"amitia://attachments/", ResourceRootAttachments},
		{"amitia://data/", ResourceRootData},
		{"amitia://cache/", ResourceRootCache},
		{"amitia://runtime/", ResourceRootRuntime},
		{"amitia://config/", ResourceRootConfig},
		{"amitia://extensions/", ResourceRootExtensions},
		{"amitia://logs/", ResourceRootLogs},
		{"amitia://temp/", ResourceRootTemp},
		{"amitia://native/", ResourceRootNative},
	}
	for _, tc := range roots {
		u, err := Parse(tc.input)
		if err != nil {
			t.Fatalf("Parse(%q) failed: %v", tc.input, err)
		}
		if u.Root() != tc.root {
			t.Fatalf("Parse(%q).Root()=%s, want %s", tc.input, u.Root(), tc.root)
		}
		if u.RelativePath() != "" {
			t.Fatalf("Parse(%q).RelativePath()=%q, want empty", tc.input, u.RelativePath())
		}
		if !u.IsRoot() {
			t.Fatalf("Parse(%q).IsRoot()=false, want true", tc.input)
		}
		expected := strings.ToLower(tc.input)
		if u.String() != expected {
			t.Fatalf("Parse(%q).String()=%q, want %q", tc.input, u.String(), expected)
		}
	}
}

func TestParseNormalizesResourcePath(t *testing.T) {
	cases := []struct {
		input   string
		wantStr string
	}{
		{"amitia://workspace/", "amitia://workspace/"},
		{"amitia://workspace/a//b", "amitia://workspace/a/b"},
		{"amitia://workspace/a/./b", "amitia://workspace/a/b"},
		{"amitia://workspace/projects/demo", "amitia://workspace/projects/demo"},
		{"amitia://attachments/image/avatar.png", "amitia://attachments/image/avatar.png"},
		{"amitia://config/文件.txt", "amitia://config/%E6%96%87%E4%BB%B6.txt"},
		{"amitia://data/my file.txt", "amitia://data/my%20file.txt"},
	}
	for _, tc := range cases {
		u, err := Parse(tc.input)
		if err != nil {
			t.Fatalf("Parse(%q) failed: %v", tc.input, err)
		}
		if u.String() != tc.wantStr {
			t.Fatalf("Parse(%q).String()=%q, want %q", tc.input, u.String(), tc.wantStr)
		}
	}
}

func TestParseRejectsInvalidScheme(t *testing.T) {
	bad := []string{
		"file:///etc/passwd",
		"http://example.com/",
		"https://example.com/",
		"://workspace/",
		"ws://workspace/",
	}
	for _, input := range bad {
		err := catchParse(input)
		if err == nil {
			t.Fatalf("Parse(%q) should have failed", input)
		}
		if !errors.Is(err, ErrInvalidScheme) {
			t.Fatalf("Parse(%q) error=%v, want ErrInvalidScheme", input, err)
		}
	}
}

func TestParseRejectsUnknownRoot(t *testing.T) {
	bad := []string{
		"amitia://android/",
		"amitia://ios/",
		"amitia://qdrant/",
		"amitia://unknown/",
		"amitia://proot/",
		"amitia://server/",
	}
	for _, input := range bad {
		err := catchParse(input)
		if err == nil {
			t.Fatalf("Parse(%q) should have failed", input)
		}
		if !errors.Is(err, ErrInvalidRoot) {
			t.Fatalf("Parse(%q) error=%v, want ErrInvalidRoot", input, err)
		}
	}
}

func TestParseRejectsQueryAndFragment(t *testing.T) {
	bad := []string{
		"amitia://workspace/file?token=abc",
		"amitia://workspace/file?",
		"amitia://workspace/file#section",
		"amitia://workspace/file?x=1&y=2",
	}
	for _, input := range bad {
		err := catchParse(input)
		if err == nil {
			t.Fatalf("Parse(%q) should have failed", input)
		}
	}
}

func TestParseRejectsUserInfoAndPort(t *testing.T) {
	bad := []string{
		"amitia://user@workspace/file",
		"amitia://user:pass@workspace/file",
		"amitia://workspace:1234/file",
		"amitia://workspace:80/",
	}
	for _, input := range bad {
		err := catchParse(input)
		if err == nil {
			t.Fatalf("Parse(%q) should have failed", input)
		}
	}
}

func TestParseRejectsPathTraversal(t *testing.T) {
	bad := []string{
		"amitia://workspace/../data",
		"amitia://workspace/a/../../data",
		"amitia://workspace/%2e%2e/data",
		"amitia://workspace/%2E%2E/data",
		"amitia://workspace/a/%2e%2e/%2e%2e/data",
		"amitia://workspace/a\\..\\data",
		"amitia://workspace/../../../etc/passwd",
		"amitia://data/..%2f..%2fetc/passwd",
	}
	for _, input := range bad {
		err := catchParse(input)
		if err == nil {
			t.Fatalf("Parse(%q) should have failed", input)
		}
		if !errors.Is(err, ErrPathTraversal) {
			t.Fatalf("Parse(%q) error=%v, want ErrPathTraversal", input, err)
		}
	}
}

func TestParseAllowsDoubleDotsInsideFilename(t *testing.T) {
	ok := []string{
		"amitia://data/report..txt",
		"amitia://data/version..backup",
		"amitia://data/..hidden",
		"amitia://workspace/notes..bak/file",
	}
	for _, input := range ok {
		_, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse(%q) should succeed, got: %v", input, err)
		}
	}
}

func TestMustParse(t *testing.T) {
	u := MustParse("amitia://workspace/projects/demo")
	if u.Root() != ResourceRootWorkspace {
		t.Fatalf("MustParse().Root()=%s", u.Root())
	}
	if u.RelativePath() != "projects/demo" {
		t.Fatalf("MustParse().RelativePath()=%q", u.RelativePath())
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("MustParse should have panicked on invalid input")
		}
	}()
	MustParse("invalid://uri")
}

func TestResourceURIStringRoundTrip(t *testing.T) {
	inputs := []string{
		"amitia://workspace/",
		"amitia://workspace/projects/demo",
		"amitia://attachments/image/avatar.png",
		"amitia://extensions/example.skill/config.json",
		"amitia://data/./cleaned",
	}
	for _, input := range inputs {
		u1, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse(%q) failed: %v", input, err)
		}
		s := u1.String()
		u2, err := Parse(s)
		if err != nil {
			t.Fatalf("Parse roundtrip(%q) from %q failed: %v", input, s, err)
		}
		if u1.Root() != u2.Root() || u1.RelativePath() != u2.RelativePath() {
			t.Fatalf("Roundtrip mismatch: %+v vs %+v", u1, u2)
		}
	}
}

func catchParse(input string) error {
	_, err := Parse(input)
	return err
}

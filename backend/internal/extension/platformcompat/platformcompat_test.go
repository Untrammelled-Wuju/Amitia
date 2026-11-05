package platformcompat

import (
	"reflect"
	"testing"
)

func TestNormalizeListUsesCanonicalPlatformNames(t *testing.T) {
	got, err := NormalizeList([]string{"win32", "darwin", "macos", "linux-arm64", "android", "iOS", "web"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{Windows, MacOS, Linux, Android, IOS, Web}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeList() = %v, want %v", got, want)
	}
}

func TestSupportsMacOSDarwinAliasAndUnsupportedPlatform(t *testing.T) {
	if !Supports([]string{"macos"}, "darwin") {
		t.Fatal("macOS package must support the darwin host")
	}
	if Supports([]string{"windows", "linux", "macos"}, "android") {
		t.Fatal("desktop-only package must not support Android")
	}
	if !Supports(nil, "android") {
		t.Fatal("missing platform declaration must retain all-platform semantics")
	}
}

func TestNormalizeListRejectsUnknownPlatform(t *testing.T) {
	if _, err := NormalizeList([]string{"windows", "plan9"}); err == nil {
		t.Fatal("unknown platform must be rejected")
	}
}

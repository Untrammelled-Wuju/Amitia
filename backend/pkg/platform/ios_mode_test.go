package platform

import "testing"

func TestIOSISHModeNormalization(t *testing.T) {
	for _, value := range []string{"ios-ish", "IOS-ISH", " ios-ish "} {
		if !IsIOSISHMode(value) || IsAndroidPRootMode(value) {
			t.Fatalf("incorrect iOS guest mode: %q", value)
		}
	}
	for _, value := range []string{"ios", "android-proot", "desktop", ""} {
		if IsIOSISHMode(value) {
			t.Fatalf("incorrect iOS guest mode accepted: %q", value)
		}
	}
}

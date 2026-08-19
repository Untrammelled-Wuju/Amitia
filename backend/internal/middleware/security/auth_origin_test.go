package security

import "testing"

func TestIsDesktopDevelopmentOrigin(t *testing.T) {
	for _, origin := range []string{"http://localhost:5178", "http://127.0.0.1:5178", "http://[::1]:5178"} {
		if !isDesktopDevelopmentOrigin(origin) {
			t.Fatalf("origin %s should be trusted", origin)
		}
	}
	for _, origin := range []string{"https://localhost:5178", "http://localhost:3000", "http://example.com:5178"} {
		if isDesktopDevelopmentOrigin(origin) {
			t.Fatalf("origin %s should not be trusted", origin)
		}
	}
}

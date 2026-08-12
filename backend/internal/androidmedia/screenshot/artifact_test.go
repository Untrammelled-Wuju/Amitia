package screenshot

import (
	"testing"
	"time"
)

func TestArtifact_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		input  Artifact
		expect bool
	}{
		{
			name: "valid",
			input: Artifact{
				ResourceURI: "amitia://temp/android-media/screenshots/x.png",
				MIMEType:    "image/png",
				Width:       1080,
				Height:      2400,
				SizeBytes:   1000,
			},
			expect: true,
		},
		{
			name: "empty uri",
			input: Artifact{
				Width:     1080,
				Height:    2400,
				SizeBytes: 1000,
			},
			expect: false,
		},
		{
			name: "empty mime",
			input: Artifact{
				ResourceURI: "amitia://temp/x.png",
				Width:       1080,
				Height:      2400,
				SizeBytes:   1000,
			},
			expect: false,
		},
		{
			name: "zero width",
			input: Artifact{
				ResourceURI: "amitia://temp/x.png",
				MIMEType:    "image/png",
				Width:       0,
				Height:      2400,
				SizeBytes:   1000,
			},
			expect: false,
		},
		{
			name: "zero size",
			input: Artifact{
				ResourceURI: "amitia://temp/x.png",
				MIMEType:    "image/png",
				Width:       1080,
				Height:      2400,
				SizeBytes:   0,
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.input.IsValid(); got != tt.expect {
				t.Errorf("expected %v, got %v", tt.expect, got)
			}
		})
	}
}

func TestArtifact_IsExpired(t *testing.T) {
	now := time.Now()

	artifact := Artifact{
		ExpiresAt: now.Add(-1 * time.Minute),
	}
	if !artifact.IsExpired(now) {
		t.Error("expected artifact to be expired")
	}

	artifact2 := Artifact{
		ExpiresAt: now.Add(1 * time.Minute),
	}
	if artifact2.IsExpired(now) {
		t.Error("expected artifact to not be expired")
	}

	artifact3 := Artifact{}
	if artifact3.IsExpired(now) {
		t.Error("artifact without expiry should not be considered expired")
	}
}

func TestSafeResourceName(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
		ext       string
		expected  string
	}{
		{
			name:      "simple",
			requestID: "req-123",
			ext:       ".png",
			expected:  "req-123.png",
		},
		{
			name:      "with slash",
			requestID: "req/abc",
			ext:       ".jpg",
			expected:  "req_abc.jpg",
		},
		{
			name:      "no ext prefix",
			requestID: "req-456",
			ext:       "webp",
			expected:  "req-456.webp",
		},
		{
			name:      "empty ext",
			requestID: "req-789",
			ext:       "",
			expected:  "req-789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SafeResourceName(tt.requestID, tt.ext)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

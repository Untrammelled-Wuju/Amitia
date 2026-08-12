package mediaread

import (
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func createTestPNGFile(t *testing.T, path string, width, height int) {
	t.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{128, 128, 128, 255})
		}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Fatalf("failed to encode PNG: %v", err)
	}
}

func TestImageNormalizer_NeedsNormalization(t *testing.T) {
	tests := []struct {
		name     string
		policy   Policy
		info     ImageInfo
		expected bool
	}{
		{"no normalization needed", Policy{NormalizeOrientation: false, StripSensitiveMetadata: false}, ImageInfo{Orientation: 0}, false},
		{"needs orientation", Policy{NormalizeOrientation: true, StripSensitiveMetadata: false}, ImageInfo{Orientation: 90}, true},
		{"needs metadata strip", Policy{NormalizeOrientation: false, StripSensitiveMetadata: true}, ImageInfo{Orientation: 0}, true},
		{"default policy no orientation", DefaultPolicy(), ImageInfo{Orientation: 0}, true},
		{"default policy with orientation", DefaultPolicy(), ImageInfo{Orientation: 90}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			norm := NewImageNormalizer(tt.policy, NewImageDecoder(tt.policy))
			if got := norm.NeedsNormalization(tt.info); got != tt.expected {
				t.Errorf("NeedsNormalization() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestImageNormalizer_ResourceURIFromTemp(t *testing.T) {
	policy := DefaultPolicy()
	norm := NewImageNormalizer(policy, NewImageDecoder(policy))

	uri := norm.ResourceURIFromTemp("req-abc", "jpeg")
	if uri != "amitia://temp/android-media/mediaread/req-abc.jpg" {
		t.Fatalf("unexpected URI: %s", uri)
	}

	uri2 := norm.ResourceURIFromTemp("req-xyz", "png")
	if uri2 != "amitia://temp/android-media/mediaread/req-xyz.png" {
		t.Fatalf("unexpected URI: %s", uri2)
	}
}

func TestImageNormalizer_DecodeFull_SmallImage(t *testing.T) {
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "test.png")
	createTestPNGFile(t, pngPath, 10, 10)

	policy := DefaultPolicy()
	decoder := NewImageDecoder(policy)
	norm := NewImageNormalizer(policy, decoder)

	data, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	info := ImageInfo{
		Format: FormatPNG,
		Width:  10,
		Height: 10,
	}

	img, err := norm.DecodeFull(t.Context(), &mockReader{data: data}, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if img == nil {
		t.Fatal("expected decoded image")
	}

	bounds := img.Bounds()
	if bounds.Dx() != 10 || bounds.Dy() != 10 {
		t.Fatalf("expected 10x10, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

type mockReader struct {
	data []byte
	pos  int
}

func (r *mockReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	if r.pos >= len(r.data) {
		return n, io.EOF
	}
	return n, nil
}

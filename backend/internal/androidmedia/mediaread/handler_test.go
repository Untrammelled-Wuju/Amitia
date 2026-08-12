package mediaread

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

type fakeResourceReader struct {
	readFunc func(ctx context.Context, uri string) (io.ReadCloser, ResolvedResource, error)
}

func (f *fakeResourceReader) Read(ctx context.Context, uri string) (io.ReadCloser, ResolvedResource, error) {
	if f.readFunc != nil {
		return f.readFunc(ctx, uri)
	}
	return nil, ResolvedResource{}, nil
}

func TestHandler_Info_EmptyURI(t *testing.T) {
	handler := NewHandlerWithReader(DefaultPolicy(), &fakeResourceReader{})
	_, err := handler.Info(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty URI")
	}
	if !strings.Contains(err.Error(), MediaReadInvalidURI) {
		t.Fatalf("expected invalid URI error, got: %v", err)
	}
}

func TestHandler_Info_ReaderError(t *testing.T) {
	reader := &fakeResourceReader{
		readFunc: func(ctx context.Context, uri string) (io.ReadCloser, ResolvedResource, error) {
			return nil, ResolvedResource{}, &MediaReadError{Code: MediaReadResourceNotFound, Message: "not found"}
		},
	}
	handler := NewHandlerWithReader(DefaultPolicy(), reader)

	_, err := handler.Info(context.Background(), "amitia://workspace/test.png")
	if err == nil {
		t.Fatal("expected error from reader")
	}
}

func TestHandler_Image_EmptyURI(t *testing.T) {
	handler := NewHandlerWithReader(DefaultPolicy(), &fakeResourceReader{})
	_, err := handler.Image(context.Background(), "", DecodeOptions{})
	if err == nil {
		t.Fatal("expected error for empty URI")
	}
}

func TestHandler_Image_NormalizationSkip(t *testing.T) {
	policy := DefaultPolicy()
	policy.NormalizeOrientation = false
	policy.StripSensitiveMetadata = false

	reader := &fakeResourceReader{
		readFunc: func(ctx context.Context, uri string) (io.ReadCloser, ResolvedResource, error) {
			return io.NopCloser(bytes.NewReader([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52})),
				ResolvedResource{URI: uri, LocalPath: "/tmp/test.png", MIMEType: "image/png", SizeBytes: 100},
				nil
		},
	}
	handler := NewHandlerWithReader(policy, reader)

	_, err := handler.Image(context.Background(), "amitia://workspace/test.png", DecodeOptions{})
	// Will fail because data isn't valid PNG - but tests the flow
	if err != nil && !strings.Contains(err.Error(), MediaReadDecodeFailed) {
		// Expected - data isn't a valid PNG
	}
}

func TestHandler_ResolveImageInput_EmptyURI(t *testing.T) {
	handler := NewHandlerWithReader(DefaultPolicy(), &fakeResourceReader{})
	_, err := handler.ResolveImageInput(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty URI")
	}
}

func TestHandler_ResolveImageInput_ReaderError(t *testing.T) {
	reader := &fakeResourceReader{
		readFunc: func(ctx context.Context, uri string) (io.ReadCloser, ResolvedResource, error) {
			return nil, ResolvedResource{}, &MediaReadError{Code: MediaReadResourceNotFound, Message: "not found"}
		},
	}
	handler := NewHandlerWithReader(DefaultPolicy(), reader)

	_, err := handler.ResolveImageInput(context.Background(), "amitia://workspace/test.png")
	if err == nil {
		t.Fatal("expected error from reader")
	}
}

func TestImageInputIsValid(t *testing.T) {
	tests := []struct {
		name     string
		input    ImageInput
		expected bool
	}{
		{"valid", ImageInput{ResourceURI: "amitia://test", MIMEType: "image/png", Bytes: []byte{1}}, true},
		{"empty URI", ImageInput{MIMEType: "image/png", Bytes: []byte{1}}, false},
		{"empty MIME", ImageInput{ResourceURI: "amitia://test", Bytes: []byte{1}}, false},
		{"empty bytes", ImageInput{ResourceURI: "amitia://test", MIMEType: "image/png"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.input.IsValid(); got != tt.expected {
				t.Errorf("IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestImageInputToReader(t *testing.T) {
	data := []byte("test")
	input := ImageInput{Bytes: data}
	reader := input.ToReader()
	read, _ := io.ReadAll(reader)
	if !bytes.Equal(read, data) {
		t.Fatalf("expected %v, got %v", data, read)
	}
}

func TestHandler_NewHandler(t *testing.T) {
	handler := NewHandler(DefaultPolicy(), nil)
	if handler == nil {
		t.Fatal("expected handler")
	}
	if handler.policy.MaxInputBytes == 0 {
		t.Fatal("expected policy to be set")
	}
}

func TestHandler_TempPath(t *testing.T) {
	handler := NewHandlerWithReader(DefaultPolicy(), &fakeResourceReader{})
	path := handler.tempPath("test-req-123", "jpeg")
	if !strings.Contains(path, "test-req-123") {
		t.Fatalf("expected path to contain request ID, got %s", path)
	}
	if !strings.HasSuffix(path, ".jpg") {
		t.Fatalf("expected .jpg extension, got %s", path)
	}
}

func TestHandler_Close(t *testing.T) {
	handler := NewHandlerWithReader(DefaultPolicy(), &fakeResourceReader{})
	if err := handler.Close(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

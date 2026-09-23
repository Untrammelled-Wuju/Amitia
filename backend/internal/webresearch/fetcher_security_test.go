package webresearch

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

func TestReadBoundedHTTPBodyRejectsRawOversize(t *testing.T) {
	_, err := readBoundedHTTPBody(strings.NewReader(strings.Repeat("x", 33)), "identity", 32)
	if err == nil || err.Code != ErrFetchFailed {
		t.Fatalf("expected raw size rejection, got %#v", err)
	}
}

func TestReadBoundedHTTPBodyRejectsDecodedOversize(t *testing.T) {
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	_, _ = zw.Write([]byte(strings.Repeat("x", 4096)))
	_ = zw.Close()
	_, err := readBoundedHTTPBody(bytes.NewReader(compressed.Bytes()), "gzip", 1024)
	if err == nil || err.Code != ErrFetchFailed {
		t.Fatalf("expected decoded size rejection, got %#v", err)
	}
}

func TestReadBoundedHTTPBodyRejectsCompressionBombRatio(t *testing.T) {
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	_, _ = zw.Write([]byte(strings.Repeat("A", 200000)))
	_ = zw.Close()
	_, err := readBoundedHTTPBody(bytes.NewReader(compressed.Bytes()), "gzip", 300000)
	if err == nil || !strings.Contains(strings.ToLower(err.Message), "ratio") {
		t.Fatalf("expected decompression ratio rejection, got %#v", err)
	}
}

func TestReadBoundedHTTPBodyRejectsUnsupportedEncoding(t *testing.T) {
	_, err := readBoundedHTTPBody(strings.NewReader("content"), "br", 1024)
	if err == nil || err.Code != ErrUnsupportedContent {
		t.Fatalf("expected unsupported encoding, got %#v", err)
	}
}

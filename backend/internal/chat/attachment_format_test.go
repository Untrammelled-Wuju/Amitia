package chat

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func ownedPresentationFixture(t *testing.T, slides []string) []byte {
	t.Helper()
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	presentation, err := writer.Create("ppt/presentation.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := presentation.Write([]byte(`<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`)); err != nil {
		t.Fatal(err)
	}
	for index, slide := range slides {
		entry, err := writer.Create("ppt/slides/slide" + strconv.Itoa(index+1) + ".xml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(slide)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestAttachmentFormatIgnoresMisleadingDeclaredMIME(t *testing.T) {
	slides := []string{
		`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>答辩标题</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
		`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>记忆顺序调整</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
	}
	path := filepath.Join(t.TempDir(), "答辩稿.pptx")
	if err := os.WriteFile(path, ownedPresentationFixture(t, slides), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, declared := range []string{"application/zip", "application/octet-stream", "", "binary/octet-stream"} {
		format, err := ResolveAttachmentFormat(path, declared)
		if err != nil || format != AttachmentFormatPresentation {
			t.Fatalf("declared=%q format=%q error=%v", declared, format, err)
		}
		text, err := ExtractAttachmentText(t.Context(), path, declared)
		if err != nil || !strings.Contains(text, "答辩标题") || !strings.Contains(text, "记忆顺序调整") {
			t.Fatalf("declared=%q text=%q error=%v", declared, text, err)
		}
	}
}

func TestAttachmentFormatRejectsNonDocumentContainers(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entry, err := writer.Create("notes/readme.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("打包内容")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "bundle.zip")
	if err := os.WriteFile(zipPath, archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractAttachmentText(t.Context(), zipPath, "application/zip"); !errors.Is(err, ErrUnsupportedAttachmentFormat) || !strings.Contains(err.Error(), "ZIP") {
		t.Fatalf("archive was not reported as unsupported: %v", err)
	}
	legacyPath := filepath.Join(t.TempDir(), "legacy.ppt")
	if err := os.WriteFile(legacyPath, append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, bytes.Repeat([]byte{0}, 512)...), 0o600); err != nil {
		t.Fatal(err)
	}
	if format, err := ResolveAttachmentFormat(legacyPath, "application/vnd.ms-powerpoint"); err != nil || format != AttachmentFormatLegacyOffice {
		t.Fatalf("legacy format=%q error=%v", format, err)
	}
	if _, err := ExtractAttachmentText(t.Context(), legacyPath, "application/vnd.ms-powerpoint"); !errors.Is(err, ErrUnsupportedAttachmentFormat) {
		t.Fatalf("legacy office was parsed as text: %v", err)
	}
	binaryPath := filepath.Join(t.TempDir(), "blob.bin")
	if err := os.WriteFile(binaryPath, []byte{0x00, 0x01, 0x02, 0x03, 0xFF}, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractAttachmentText(t.Context(), binaryPath, "application/octet-stream"); !errors.Is(err, ErrUnsupportedAttachmentFormat) || !strings.Contains(err.Error(), "二进制") {
		t.Fatalf("binary attachment was not reported as unsupported: %v", err)
	}
}

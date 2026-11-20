package chat

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeOwnedTestFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func ownedWordFixture(t *testing.T, document string, duplicate bool) []byte {
	t.Helper()
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	count := 1
	if duplicate {
		count = 2
	}
	for range count {
		file, err := writer.Create("word/document.xml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(document)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestOwnedDocumentExtractionUsesMaterializedFilesAndPreservesErrors(t *testing.T) {
	textPath := writeOwnedTestFile(t, "content.txt", []byte("独立设备里的正文"))
	text, err := ExtractAttachmentText(t.Context(), textPath, "text/plain")
	if err != nil || text != "独立设备里的正文" {
		t.Fatalf("text=%q error=%v", text, err)
	}
	document := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>设备正文</w:t><w:tab/><w:t>下一段</w:t></w:r></w:p></w:body></w:document>`
	wordPath := writeOwnedTestFile(t, "document.docx", ownedWordFixture(t, document, false))
	word, err := ExtractAttachmentText(t.Context(), wordPath, "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	if err != nil || word != "设备正文\t下一段\n" {
		t.Fatalf("text=%q error=%v", word, err)
	}
	for name, data := range map[string][]byte{
		"duplicate.docx": ownedWordFixture(t, document, true),
		"invalid.docx":   ownedWordFixture(t, "<invalid>", false),
		"large.docx":     ownedWordFixture(t, strings.Repeat("x", (2<<20)+1), false),
		"not-a-zip.docx": []byte("not a zip"),
	} {
		path := writeOwnedTestFile(t, name, data)
		if _, err := ExtractAttachmentText(t.Context(), path, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"); err == nil {
			t.Fatalf("invalid Word document %s was parsed", name)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := ExtractAttachmentText(ctx, wordPath, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"); err == nil {
		t.Fatal("cancelled work continued parsing")
	}
}

func TestOwnedDocumentOutputIsBounded(t *testing.T) {
	var output ownedTextBuffer
	if _, err := output.Write([]byte(strings.Repeat("a", 1<<20))); err != nil {
		t.Fatal(err)
	}
	if _, err := output.Write([]byte("excess")); err == nil || output.Len() != 1<<20 {
		t.Fatal("document parser exceeded its output budget")
	}
}

func TestOwnedPDFParsesRealDocumentUsingPathCommand(t *testing.T) {
	command := os.Getenv("AMITIA_TEST_PDF_TOOL")
	if command == "" {
		t.Skip("actual PDF command not supplied")
	}
	t.Setenv("PATH", filepath.Dir(command)+string(os.PathListSeparator)+os.Getenv("PATH"))
	content := "BT /F1 12 Tf 72 720 Td (Hello Core) Tj ET"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}
	var data bytes.Buffer
	data.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, object := range objects {
		offsets = append(offsets, data.Len())
		fmt.Fprintf(&data, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := data.Len()
	fmt.Fprintf(&data, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&data, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&data, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	path := writeOwnedTestFile(t, "document.pdf", data.Bytes())
	text, err := ExtractAttachmentText(t.Context(), path, "application/pdf")
	if err != nil || !strings.Contains(text, "Hello Core") {
		t.Fatalf("actual PDF text not extracted: %q %v", text, err)
	}
}

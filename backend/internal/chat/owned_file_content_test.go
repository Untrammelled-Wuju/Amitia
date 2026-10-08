package chat

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/devicemesh/business"
)

func ownedFileFixture(data []byte, mime string) business.Attachment {
	hash := sha256.Sum256(data)
	return business.Attachment{Kind: "file", Name: "用户文件", MIME: mime, Data: base64.StdEncoding.EncodeToString(data), Hash: hex.EncodeToString(hash[:])}
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

func TestOwnedFileContentUsesActualBytesAndRejectsInvalidDocuments(t *testing.T) {
	item := ownedFileFixture([]byte("独立设备里的正文"), "text/plain")
	parts, err := ownedMessageContentContext(t.Context(), "请分析", []business.Attachment{item})
	if err != nil || !strings.Contains(parts.([]map[string]any)[1]["text"].(string), "独立设备里的正文") {
		t.Fatalf("file content missing: %v", err)
	}
	item.Hash = "changed"
	if _, err := ownedMessageContentContext(t.Context(), "请分析", []business.Attachment{item}); err == nil {
		t.Fatal("unverified file reached model prompt")
	}
	document := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>设备正文</w:t><w:tab/><w:t>下一段</w:t></w:r></w:p></w:body></w:document>`
	word := ownedWordFixture(t, document, false)
	text, err := ownedWordText(t.Context(), word)
	if err != nil || text != "设备正文\t下一段\n" {
		t.Fatalf("text=%q error=%v", text, err)
	}
	for _, invalid := range [][]byte{ownedWordFixture(t, document, true), ownedWordFixture(t, "<invalid>", false), ownedWordFixture(t, strings.Repeat("x", (2<<20)+1), false), []byte("not a zip")} {
		if _, err := ownedWordText(t.Context(), invalid); err == nil {
			t.Fatal("invalid or oversized Word document was parsed")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := ownedWordText(ctx, word); err == nil {
		t.Fatal("cancelled Core work continued parsing")
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

func TestOwnedPDFParsesRealDocumentUsingCoreConfiguredCommand(t *testing.T) {
	command := os.Getenv("AMITIA_TEST_PDF_TOOL")
	if command == "" {
		t.Skip("actual PDF command not supplied")
	}
	previous := config.AppCfg
	current := config.Config{}
	if previous != nil {
		current = *previous
	}
	current.Providers.Search.Research.PDFTextCommand = command
	config.AppCfg = &current
	t.Cleanup(func() { config.AppCfg = previous })
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
	item := ownedFileFixture(data.Bytes(), "application/pdf")
	parts, err := ownedMessageContentContext(t.Context(), "阅读PDF", []business.Attachment{item})
	if err != nil || !strings.Contains(parts.([]map[string]any)[1]["text"].(string), "Hello Core") {
		t.Fatalf("actual PDF text not delivered: %v", err)
	}
}

package webresearch

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestParsePDFTextPages(t *testing.T) {
	pages := parsePDFTextPages("first page\fsecond page\f")
	if len(pages) != 2 || pages[0] != "first page" || pages[1] != "second page" {
		t.Fatalf("unexpected pages: %#v", pages)
	}
}

func TestParsePDFInfo(t *testing.T) {
	title, author, pages := parsePDFInfo("Title: Example\nAuthor: Tester\nPages: 12\n")
	if title != "Example" || author != "Tester" || pages != 12 {
		t.Fatalf("unexpected info title=%q author=%q pages=%d", title, author, pages)
	}
}

func TestPDFEvidenceLocatorUsesPageNumber(t *testing.T) {
	page := Page{ContentType: "application/pdf", Content: "--- PDF Page 1 ---\nalpha architecture text\n\n--- PDF Page 2 ---\nbeta provider routing text"}
	items := buildEvidenceForPage(page, "provider routing", nil, 4, 1000)
	if len(items) == 0 {
		t.Fatal("expected PDF evidence")
	}
	if items[0].Locator.Kind != "pdf_page" || items[0].Locator.Page != 1 {
		t.Fatalf("unexpected locator: %#v", items[0].Locator)
	}
}

type fakePDFParser struct {
	doc PDFDocument
	err *Error
}

func (p fakePDFParser) Available(context.Context) bool { return true }
func (p fakePDFParser) Parse(context.Context, []byte) (PDFDocument, *Error) {
	return p.doc, p.err
}

func TestFetcherReadResponseParsesPDFBeforeBrowserFallback(t *testing.T) {
	fetcher := NewFetcher(Config{PDFEnabled: false, MaxFetchBytes: 1024, MaxPageChars: 4096})
	fetcher.pdfParser = fakePDFParser{doc: PDFDocument{
		Title: "Spec",
		Pages: []string{"first evidence", "second evidence"},
	}}
	resp := &http.Response{
		Header: http.Header{"Content-Type": []string{"application/pdf"}},
		Body:   io.NopCloser(strings.NewReader("%PDF-fake")),
	}
	page, err := fetcher.readResponse(context.Background(), resp, "https://example.com/spec.pdf", "evidence")
	if err != nil {
		t.Fatalf("readResponse: %v", err)
	}
	if page.Title != "Spec" || page.ContentType != "application/pdf" {
		t.Fatalf("unexpected page: %#v", page)
	}
	if !strings.Contains(page.Content, "--- PDF Page 1 ---") || !strings.Contains(page.Content, "--- PDF Page 2 ---") {
		t.Fatalf("page markers missing: %q", page.Content)
	}
	if page.Hash == "" {
		t.Fatal("expected stable PDF content hash")
	}
}

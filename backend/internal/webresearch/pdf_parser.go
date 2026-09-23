package webresearch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// PDFDocument is the normalized output of a PDF text provider. Page numbers in
// the public evidence locator are zero-based to match screenshot/open commands.
type PDFDocument struct {
	Title     string
	Author    string
	PageCount int
	Pages     []string
}

// PDFParser is intentionally provider-neutral. Implementations may use a local
// parser, a sandboxed service, or a plugin, but the web_run contract never
// exposes the concrete provider to the model.
type PDFParser interface {
	Available(ctx context.Context) bool
	Parse(ctx context.Context, raw []byte) (PDFDocument, *Error)
}

type popplerPDFParser struct {
	textCommand string
	infoCommand string
	timeout     time.Duration
	maxChars    int
}

func newPopplerPDFParser(config Config) PDFParser {
	if !config.PDFEnabled {
		return nil
	}
	return &popplerPDFParser{
		textCommand: strings.TrimSpace(config.PDFTextCommand),
		infoCommand: strings.TrimSpace(config.PDFInfoCommand),
		timeout:     config.PDFTimeout,
		maxChars:    config.MaxPageChars,
	}
}

func (p *popplerPDFParser) Available(_ context.Context) bool {
	if p == nil || p.textCommand == "" {
		return false
	}
	_, err := exec.LookPath(p.textCommand)
	return err == nil
}

func (p *popplerPDFParser) Parse(ctx context.Context, raw []byte) (PDFDocument, *Error) {
	if p == nil || !p.Available(ctx) {
		return PDFDocument{}, newError(ErrUnsupportedContent, "application/pdf parser unavailable", false, nil)
	}
	if len(raw) == 0 {
		return PDFDocument{}, newError(ErrFetchFailed, "empty PDF response", false, nil)
	}
	parseCtx := ctx
	cancel := func() {}
	if p.timeout > 0 {
		parseCtx, cancel = context.WithTimeout(ctx, p.timeout)
	}
	defer cancel()

	file, err := os.CreateTemp("", "amitia-webresearch-*.pdf")
	if err != nil {
		return PDFDocument{}, newError(ErrFetchFailed, "create PDF staging file failed", true, err)
	}
	path := file.Name()
	defer os.Remove(path)
	if chmodErr := file.Chmod(0o600); chmodErr != nil {
		file.Close()
		return PDFDocument{}, newError(ErrFetchFailed, "secure PDF staging file failed", false, chmodErr)
	}
	if _, err = file.Write(raw); err != nil {
		file.Close()
		return PDFDocument{}, newError(ErrFetchFailed, "write PDF staging file failed", true, err)
	}
	if err = file.Close(); err != nil {
		return PDFDocument{}, newError(ErrFetchFailed, "close PDF staging file failed", true, err)
	}

	maxOutput := p.maxChars
	if maxOutput <= 0 {
		maxOutput = 60000
	}
	// Keep headroom for form-feed page separators before final per-page fitting.
	maxOutput *= 2
	stdout := newBoundedCommandBuffer(maxOutput)
	stderr := newBoundedCommandBuffer(8192)
	cmd := exec.CommandContext(parseCtx, p.textCommand, "-layout", "-enc", "UTF-8", path, "-")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(parseCtx.Err(), context.DeadlineExceeded) {
			return PDFDocument{}, newError(ErrFetchFailed, "PDF text extraction timed out", true, parseCtx.Err())
		}
		if errors.Is(stdout.Err(), errCommandOutputLimit) {
			// pdftotext may exit on a closed/limited stdout after enough useful text
			// has been captured. Preserve the bounded prefix and mark it truncated
			// later instead of treating the whole document as unreadable.
		} else {
			message := strings.TrimSpace(stderr.String())
			if message == "" {
				message = err.Error()
			}
			return PDFDocument{}, newError(ErrFetchFailed, "PDF text extraction failed: "+truncateRunes(message, 400), false, err)
		}
	}

	doc := PDFDocument{Pages: parsePDFTextPages(stdout.String())}
	if p.infoCommand != "" {
		if _, err := exec.LookPath(p.infoCommand); err == nil {
			infoCtx := parseCtx
			infoOut := newBoundedCommandBuffer(32768)
			infoErr := newBoundedCommandBuffer(4096)
			infoCmd := exec.CommandContext(infoCtx, p.infoCommand, path)
			infoCmd.Stdout = infoOut
			infoCmd.Stderr = infoErr
			if runErr := infoCmd.Run(); runErr == nil {
				doc.Title, doc.Author, doc.PageCount = parsePDFInfo(infoOut.String())
			}
		}
	}
	if doc.PageCount <= 0 {
		doc.PageCount = len(doc.Pages)
	}
	if len(doc.Pages) == 0 {
		return PDFDocument{}, newError(ErrFetchFailed, "PDF contains no extractable text", false, nil)
	}
	return doc, nil
}

var errCommandOutputLimit = errors.New("command output exceeds limit")

type boundedCommandBuffer struct {
	buf   bytes.Buffer
	limit int
	err   error
}

func newBoundedCommandBuffer(limit int) *boundedCommandBuffer {
	if limit <= 0 {
		limit = 65536
	}
	return &boundedCommandBuffer{limit: limit}
}

func (b *boundedCommandBuffer) Write(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.err = errCommandOutputLimit
		return 0, b.err
	}
	if len(p) > remaining {
		_, _ = b.buf.Write(p[:remaining])
		b.err = errCommandOutputLimit
		return remaining, b.err
	}
	return b.buf.Write(p)
}

func (b *boundedCommandBuffer) String() string { return b.buf.String() }
func (b *boundedCommandBuffer) Err() error     { return b.err }

func parsePDFTextPages(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	parts := strings.Split(value, "\f")
	pages := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		pages = append(pages, part)
	}
	return pages
}

func parsePDFInfo(value string) (title, author string, pages int) {
	for _, line := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		key, rawValue, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		rawValue = strings.TrimSpace(rawValue)
		switch key {
		case "title":
			title = rawValue
		case "author":
			author = rawValue
		case "pages":
			pages, _ = strconv.Atoi(rawValue)
		}
	}
	return title, author, pages
}

func formatPDFDocument(doc PDFDocument, maxChars int) (content string, truncated bool) {
	if maxChars <= 0 {
		maxChars = 60000
	}
	var builder strings.Builder
	for index, page := range doc.Pages {
		page = strings.TrimSpace(page)
		if page == "" {
			continue
		}
		marker := fmt.Sprintf("--- PDF Page %d ---\n", index+1)
		remaining := maxChars - len([]rune(builder.String()))
		if remaining <= len([]rune(marker)) {
			truncated = true
			break
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(marker)
		remaining = maxChars - len([]rune(builder.String()))
		pageRunes := []rune(page)
		if len(pageRunes) > remaining {
			builder.WriteString(string(pageRunes[:maxInt(0, remaining)]))
			truncated = true
			break
		}
		builder.WriteString(page)
	}
	return strings.TrimSpace(builder.String()), truncated
}

func buildEvidenceForPage(page Page, query string, fallbackBlocks []string, maxItems, maxChars int) []Evidence {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(page.ContentType)), "application/pdf") {
		if items := buildPDFEvidence(page, query, maxItems, maxChars); len(items) > 0 {
			return items
		}
	}
	blocks := fallbackBlocks
	if len(blocks) == 0 {
		blocks = splitTextBlocks(page.Content)
	}
	items := buildEvidence(page.ConversationID, page.RefID, query, blocks, maxItems, maxChars)
	for i := range items {
		items[i].PageContentHash = page.ContentHash
	}
	return items
}

type pdfEvidenceCandidate struct {
	page  int
	index int
	text  string
	score float64
}

func buildPDFEvidence(page Page, query string, maxItems, maxChars int) []Evidence {
	if maxItems <= 0 {
		maxItems = 8
	}
	queryTokens := tokenize(query)
	sections := parseFormattedPDFPages(page.Content)
	pageNumbers := make([]int, 0, len(sections))
	for pageNumber := range sections {
		pageNumbers = append(pageNumbers, pageNumber)
	}
	sort.Ints(pageNumbers)
	candidates := make([]pdfEvidenceCandidate, 0)
	globalIndex := 0
	for _, pageNumber := range pageNumbers {
		text := sections[pageNumber]
		for _, block := range splitTextBlocks(text) {
			block = strings.TrimSpace(block)
			if block == "" {
				continue
			}
			score := overlapScore(queryTokens, tokenize(block))
			if len(queryTokens) == 0 {
				score = 1.0 / float64(globalIndex+1)
			}
			candidates = append(candidates, pdfEvidenceCandidate{page: pageNumber, index: globalIndex, text: truncateRunes(block, maxChars), score: score})
			globalIndex++
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].index < candidates[j].index
	})
	now := nowUTC()
	evidenceQuery := normalizedQuery(query)
	items := make([]Evidence, 0, minInt(maxItems, len(candidates)))
	for _, candidate := range candidates {
		if len(items) >= maxItems {
			break
		}
		items = append(items, Evidence{
			ID:              newID("ev"),
			ConversationID:  page.ConversationID,
			PageRefID:       page.RefID,
			PageContentHash: page.ContentHash,
			Query:           evidenceQuery,
			Text:            candidate.text,
			TextHash:        hashText(candidate.text),
			Locator:         EvidenceLocator{Kind: "pdf_page", Page: candidate.page, BlockIndex: candidate.index},
			Relevance:       candidate.score,
			CreatedAt:       now,
		})
	}
	return items
}

func parseFormattedPDFPages(content string) map[int]string {
	pages := make(map[int]string)
	currentPage := -1
	var builder strings.Builder
	flush := func() {
		if currentPage < 0 {
			return
		}
		text := strings.TrimSpace(builder.String())
		if text != "" {
			pages[currentPage] = text
		}
		builder.Reset()
	}
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--- PDF Page ") && strings.HasSuffix(trimmed, " ---") {
			value := strings.TrimSuffix(strings.TrimPrefix(trimmed, "--- PDF Page "), " ---")
			if number, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && number > 0 {
				flush()
				currentPage = number - 1
				continue
			}
		}
		if currentPage >= 0 {
			if builder.Len() > 0 {
				builder.WriteByte('\n')
			}
			builder.WriteString(line)
		}
	}
	flush()
	return pages
}

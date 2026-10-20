package knowledge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type pdfParser struct {
	settings runtimeSettings
	ocrGate  *resourceGate
}

func (p pdfParser) Inspect(ctx context.Context, src SourceRef) (DocumentInfo, error) {
	pages, err := pdfPageCount(ctx, src.Path, p.settings)
	if err != nil {
		return DocumentInfo{}, err
	}
	return DocumentInfo{Format: "pdf", EstimatedUnits: int64(pages), Metadata: map[string]any{"pages": pages, "ocrFallback": commandExists("pdftoppm") && commandExists("tesseract")}}, nil
}
func commandExists(name string) bool { _, err := exec.LookPath(name); return err == nil }
func pdfPageCount(ctx context.Context, path string, settings runtimeSettings) (int, error) {
	if !commandExists("pdfinfo") {
		return 0, fmt.Errorf("pdfinfo is required for PDF ingestion")
	}
	timeout := settings.PDFInfoTimeout
	if timeout <= 0 {
		timeout = defaultRuntimeSettings().PDFInfoTimeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := exec.CommandContext(cctx, "pdfinfo", path).Output()
	if err != nil {
		return 0, fmt.Errorf("pdfinfo failed: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Pages:") {
			n, er := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "Pages:")))
			if er == nil && n > 0 {
				maxPages := settings.MaxPDFPages
				if maxPages <= 0 {
					maxPages = defaultRuntimeSettings().MaxPDFPages
				}
				if n > maxPages {
					return 0, fmt.Errorf("PDF exceeds page budget: %d > %d", n, maxPages)
				}
				return n, nil
			}
		}
	}
	return 0, fmt.Errorf("PDF page count unavailable")
}
func (p pdfParser) StreamUnits(ctx context.Context, src SourceRef, c Cursor, consume func(RawUnit) error) error {
	pages, err := pdfPageCount(ctx, src.Path, p.settings)
	if err != nil {
		return err
	}
	if !commandExists("pdftotext") {
		return fmt.Errorf("pdftotext is required for PDF ingestion")
	}
	var ordinal int64
	for page := 1; page <= pages; page++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		pageTimeout := p.settings.PDFTextPageTimeout
		if pageTimeout <= 0 {
			pageTimeout = defaultRuntimeSettings().PDFTextPageTimeout
		}
		pctx, cancel := context.WithTimeout(ctx, pageTimeout)
		out, er := exec.CommandContext(pctx, "pdftotext", "-f", strconv.Itoa(page), "-l", strconv.Itoa(page), "-layout", src.Path, "-").Output()
		cancel()
		text := cleanText(string(out))
		ocrUsed := false
		if er != nil || len([]rune(text)) < 24 {
			ocr, ocrErr := ocrPDFPage(ctx, src.Path, page, p.settings, p.ocrGate)
			if ocrErr == nil && len([]rune(cleanText(ocr))) > len([]rune(text)) {
				text = cleanText(ocr)
				ocrUsed = true
			}
		}
		if text == "" {
			continue
		}
		if ordinal < c.Ordinal {
			ordinal++
			continue
		}
		if err := consume(RawUnit{UnitType: "page", Title: fmt.Sprintf("Page %d", page), Content: text, Locator: map[string]any{"page": page}, Metadata: map[string]any{"format": "pdf", "ocr": ocrUsed}}); err != nil {
			return err
		}
		ordinal++
	}
	return nil
}
func ocrPDFPage(ctx context.Context, path string, page int, settings runtimeSettings, gate *resourceGate) (string, error) {
	if !commandExists("pdftoppm") || !commandExists("tesseract") {
		return "", fmt.Errorf("OCR helpers unavailable")
	}
	release, err := gate.Acquire(ctx)
	if err != nil {
		return "", err
	}
	defer release()
	tmp, err := os.MkdirTemp("", "amitia-knowledge-ocr-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	prefix := filepath.Join(tmp, "page")
	rasterTimeout := settings.OCRRasterTimeout
	if rasterTimeout <= 0 {
		rasterTimeout = defaultRuntimeSettings().OCRRasterTimeout
	}
	pctx, cancel := context.WithTimeout(ctx, rasterTimeout)
	cmd := exec.CommandContext(pctx, "pdftoppm", "-f", strconv.Itoa(page), "-l", strconv.Itoa(page), "-r", strconv.Itoa(func() int {
		if settings.OCRRasterDPI > 0 {
			return settings.OCRRasterDPI
		}
		return defaultRuntimeSettings().OCRRasterDPI
	}()), "-png", "-singlefile", path, prefix)
	if out, er := cmd.CombinedOutput(); er != nil {
		cancel()
		return "", fmt.Errorf("pdftoppm: %w: %s", er, string(out))
	}
	cancel()
	img := prefix + ".png"
	if stat, statErr := os.Stat(img); statErr != nil {
		return "", statErr
	} else {
		maxBytes := settings.MaxOCRImageBytes
		if maxBytes <= 0 {
			maxBytes = defaultRuntimeSettings().MaxOCRImageBytes
		}
		if stat.Size() > maxBytes {
			return "", fmt.Errorf("OCR raster exceeds image budget: %d > %d", stat.Size(), maxBytes)
		}
	}
	// Prefer Chinese+English when the language pack exists, but retry with the
	// Tesseract default language when chi_sim is not installed. The ingestion
	// path must not fail solely because an optional OCR language pack is absent.
	if out, er := runTesseract(ctx, img, []string{"-l", "chi_sim+eng"}, settings); er == nil {
		return out, nil
	}
	out, er := runTesseract(ctx, img, nil, settings)
	if er != nil {
		return "", er
	}
	return out, nil
}

func runTesseract(ctx context.Context, imagePath string, extra []string, settings runtimeSettings) (string, error) {
	timeout := settings.OCRTimeout
	if timeout <= 0 {
		timeout = defaultRuntimeSettings().OCRTimeout
	}
	octx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{imagePath, "stdout", "--psm", "6"}
	args = append(args, extra...)
	out, err := exec.CommandContext(octx, "tesseract", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tesseract: %w: %s", err, truncateDiagnostic(string(out), 2048))
	}
	return string(out), nil
}

func truncateDiagnostic(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max] + "…"
}

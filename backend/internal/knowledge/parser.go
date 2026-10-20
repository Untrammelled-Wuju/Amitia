package knowledge

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type SourceRef struct{ Path, Filename, MimeType string }
type DocumentInfo struct {
	Format         string
	EstimatedUnits int64
	Metadata       map[string]any
}
type Cursor struct{ Ordinal int64 }
type RawUnit struct {
	UnitType, Title, Content string
	Locator, Metadata        map[string]any
}
type SourceParser interface {
	Inspect(context.Context, SourceRef) (DocumentInfo, error)
	StreamUnits(context.Context, SourceRef, Cursor, func(RawUnit) error) error
}

type ParserRegistry struct {
	settings runtimeSettings
	ocrGate  *resourceGate
}

func NewParserRegistry(runtime ...runtimeSettings) *ParserRegistry {
	settings := loadRuntimeSettings()
	if len(runtime) > 0 {
		settings = runtime[0]
	}
	return &ParserRegistry{settings: settings, ocrGate: newResourceGate(settings.OCRWorkers)}
}
func (r *ParserRegistry) For(src SourceRef) (SourceParser, error) {
	ext := strings.ToLower(filepath.Ext(src.Filename))
	mime := strings.ToLower(src.MimeType)
	switch ext {
	case ".txt":
		return textParser{markdown: false, settings: r.settings}, nil
	case ".md", ".markdown":
		return textParser{markdown: true, settings: r.settings}, nil
	case ".csv":
		return csvParser{settings: r.settings}, nil
	case ".json", ".jsonl", ".ndjson":
		return jsonParser{settings: r.settings}, nil
	case ".docx":
		return docxParser{settings: r.settings}, nil
	case ".doc":
		return legacyDocParser{settings: r.settings}, nil
	case ".xlsx":
		return xlsxParser{settings: r.settings}, nil
	case ".pdf":
		return pdfParser{settings: r.settings, ocrGate: r.ocrGate}, nil
	}
	switch {
	case strings.Contains(mime, "pdf"):
		return pdfParser{settings: r.settings, ocrGate: r.ocrGate}, nil
	case strings.Contains(mime, "wordprocessingml"):
		return docxParser{settings: r.settings}, nil
	case strings.Contains(mime, "spreadsheetml"):
		return xlsxParser{settings: r.settings}, nil
	case strings.Contains(mime, "csv"):
		return csvParser{settings: r.settings}, nil
	case strings.Contains(mime, "json"):
		return jsonParser{settings: r.settings}, nil
	case strings.HasPrefix(mime, "text/"):
		return textParser{markdown: strings.Contains(mime, "markdown"), settings: r.settings}, nil
	}
	return nil, fmt.Errorf("unsupported knowledge format: %s (%s)", ext, src.MimeType)
}

func cleanText(s string) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "�")
	}
	s = strings.ReplaceAll(s, "\x00", "")
	return strings.TrimSpace(s)
}
func checksumText(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
func jsonString(v any) string { b, _ := json.Marshal(v); return string(b) }

type textParser struct {
	markdown bool
	settings runtimeSettings
}

func (p textParser) Inspect(_ context.Context, src SourceRef) (DocumentInfo, error) {
	return DocumentInfo{Format: map[bool]string{true: "markdown", false: "text"}[p.markdown], Metadata: map[string]any{"filename": src.Filename}}, nil
}
func (p textParser) StreamUnits(ctx context.Context, src SourceRef, c Cursor, consume func(RawUnit) error) error {
	f, err := os.Open(src.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	streamBuffer := p.settings.StreamBufferBytes
	if streamBuffer <= 0 {
		streamBuffer = defaultRuntimeSettings().StreamBufferBytes
	}
	maxScanner := p.settings.TextScannerMaxBytes
	if maxScanner <= 0 {
		maxScanner = defaultRuntimeSettings().TextScannerMaxBytes
	}
	initial := streamBuffer
	if initial > maxScanner {
		initial = maxScanner
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, initial), maxScanner)
	unitMax := p.settings.TextUnitMaxBytes
	if unitMax <= 0 {
		unitMax = defaultRuntimeSettings().TextUnitMaxBytes
	}
	var ordinal int64
	title := ""
	headingStack := make([]string, 0, 6)
	inFence := false
	fenceMarker := ""
	buf := strings.Builder{}
	flush := func() error {
		content := cleanText(buf.String())
		buf.Reset()
		if content == "" {
			return nil
		}
		if ordinal < c.Ordinal {
			ordinal++
			return nil
		}
		locator := map[string]any{"section": ordinal}
		metadata := map[string]any{"format": map[bool]string{true: "markdown", false: "text"}[p.markdown]}
		if p.markdown && len(headingStack) > 0 {
			metadata["sectionPath"] = append([]string(nil), headingStack...)
		}
		u := RawUnit{UnitType: "section", Title: title, Content: content, Locator: locator, Metadata: metadata}
		ordinal++
		return consume(u)
	}
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := sc.Text()
		trim := strings.TrimSpace(line)
		if p.markdown && (strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~")) {
			marker := trim[:3]
			if !inFence {
				inFence = true
				fenceMarker = marker
			} else if marker == fenceMarker {
				inFence = false
				fenceMarker = ""
			}
			buf.WriteString(line)
			buf.WriteByte('\n')
			continue
		}
		if p.markdown && !inFence && strings.HasPrefix(trim, "#") {
			level := 0
			for level < len(trim) && trim[level] == '#' {
				level++
			}
			if level == 0 || level > 6 || level >= len(trim) || trim[level] != ' ' {
				buf.WriteString(line)
				buf.WriteByte('\n')
				continue
			}
			if err := flush(); err != nil {
				return err
			}
			heading := strings.TrimSpace(trim[level:])
			if len(headingStack) >= level {
				headingStack = headingStack[:level-1]
			}
			for len(headingStack) < level-1 {
				headingStack = append(headingStack, "")
			}
			headingStack = append(headingStack, heading)
			parts := make([]string, 0, len(headingStack))
			for _, h := range headingStack {
				if strings.TrimSpace(h) != "" {
					parts = append(parts, h)
				}
			}
			title = strings.Join(parts, " > ")
			continue
		}
		if !inFence && trim == "" && buf.Len() > 0 {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if buf.Len() > 0 && buf.Len()+len(line)+1 > unitMax {
			if err := flush(); err != nil {
				return err
			}
		}
		if len(line)+1 > unitMax {
			return fmt.Errorf("text line exceeds unit budget: %d > %d bytes", len(line)+1, unitMax)
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return flush()
}

type csvParser struct{ settings runtimeSettings }

func (p csvParser) Inspect(_ context.Context, src SourceRef) (DocumentInfo, error) {
	return DocumentInfo{Format: "csv", Metadata: map[string]any{"filename": src.Filename}}, nil
}
func (p csvParser) StreamUnits(ctx context.Context, src SourceRef, c Cursor, consume func(RawUnit) error) error {
	f, err := os.Open(src.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	streamBuffer := p.settings.StreamBufferBytes
	if streamBuffer <= 0 {
		streamBuffer = defaultRuntimeSettings().StreamBufferBytes
	}
	rd := csv.NewReader(bufio.NewReaderSize(f, streamBuffer))
	rd.FieldsPerRecord = -1
	rd.ReuseRecord = false
	var headers []string
	var ordinal int64
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		row, err := rd.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		maxCells := p.settings.MaxWorksheetRowCells
		if maxCells <= 0 {
			maxCells = defaultRuntimeSettings().MaxWorksheetRowCells
		}
		if len(row) > maxCells {
			return fmt.Errorf("csv row contains more than %d cells", maxCells)
		}
		maxCellBytes := p.settings.MaxCellTextBytes
		if maxCellBytes <= 0 {
			maxCellBytes = defaultRuntimeSettings().MaxCellTextBytes
		}
		for _, value := range row {
			if len(value) > maxCellBytes {
				return fmt.Errorf("csv cell exceeds %d bytes", maxCellBytes)
			}
		}
		if headers == nil {
			headers = append([]string(nil), row...)
			continue
		}
		if ordinal < c.Ordinal {
			ordinal++
			continue
		}
		record := map[string]string{}
		for i, v := range row {
			key := fmt.Sprintf("column_%d", i+1)
			if i < len(headers) && strings.TrimSpace(headers[i]) != "" {
				key = headers[i]
			}
			record[key] = v
		}
		content := jsonString(record)
		if err := consume(RawUnit{UnitType: "row", Title: fmt.Sprintf("Row %d", ordinal+2), Content: content, Locator: map[string]any{"row": ordinal + 2}, Metadata: map[string]any{"format": "csv"}}); err != nil {
			return err
		}
		ordinal++
	}
	return nil
}

type jsonParser struct{ settings runtimeSettings }

func (p jsonParser) Inspect(_ context.Context, src SourceRef) (DocumentInfo, error) {
	return DocumentInfo{Format: "json", Metadata: map[string]any{"filename": src.Filename}}, nil
}
func (p jsonParser) StreamUnits(ctx context.Context, src SourceRef, c Cursor, consume func(RawUnit) error) error {
	f, err := os.Open(src.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	streamBuffer := p.settings.StreamBufferBytes
	if streamBuffer <= 0 {
		streamBuffer = defaultRuntimeSettings().StreamBufferBytes
	}
	maxJSONUnit := p.settings.JSONUnitMaxBytes
	if maxJSONUnit <= 0 {
		maxJSONUnit = defaultRuntimeSettings().JSONUnitMaxBytes
	}
	br := bufio.NewReaderSize(f, streamBuffer)
	// NDJSON is streamed line-by-line. Standard JSON is streamed at top-level
	// array element granularity so large corpora never require whole-file loads.
	ext := strings.ToLower(filepath.Ext(src.Filename))
	if ext == ".jsonl" || ext == ".ndjson" {
		sc := bufio.NewScanner(br)
		initial := streamBuffer
		if initial > maxJSONUnit {
			initial = maxJSONUnit
		}
		sc.Buffer(make([]byte, initial), maxJSONUnit)
		var n int64
		for sc.Scan() {
			if n < c.Ordinal {
				n++
				continue
			}
			raw := strings.TrimSpace(sc.Text())
			if raw == "" {
				continue
			}
			if len(raw) > maxJSONUnit {
				return fmt.Errorf("jsonl line %d exceeds unit budget: %d > %d bytes", n+1, len(raw), maxJSONUnit)
			}
			if !json.Valid([]byte(raw)) {
				return fmt.Errorf("jsonl line %d: invalid JSON", n+1)
			}
			if err := consume(RawUnit{UnitType: "object", Title: fmt.Sprintf("Item %d", n+1), Content: raw, Locator: map[string]any{"index": n}, Metadata: map[string]any{"format": "jsonl"}}); err != nil {
				return err
			}
			n++
		}
		return sc.Err()
	}
	dec := json.NewDecoder(br)
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return fmt.Errorf("invalid JSON root")
	}
	var n int64
	if delim == '[' {
		for dec.More() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			var raw json.RawMessage
			if err := dec.Decode(&raw); err != nil {
				return err
			}
			if len(raw) > maxJSONUnit {
				return fmt.Errorf("json item %d exceeds unit budget: %d > %d bytes", n+1, len(raw), maxJSONUnit)
			}
			if n >= c.Ordinal {
				if err := consume(RawUnit{UnitType: "object", Title: fmt.Sprintf("Item %d", n+1), Content: string(raw), Locator: map[string]any{"index": n}, Metadata: map[string]any{"format": "json"}}); err != nil {
					return err
				}
			}
			n++
		}
		_, err = dec.Token()
		return err
	}
	if delim == '{' {
		for dec.More() {
			kTok, err := dec.Token()
			if err != nil {
				return err
			}
			key, _ := kTok.(string)
			var raw json.RawMessage
			if err := dec.Decode(&raw); err != nil {
				return err
			}
			if len(raw) > maxJSONUnit {
				return fmt.Errorf("json field %q exceeds unit budget: %d > %d bytes", key, len(raw), maxJSONUnit)
			}
			if n >= c.Ordinal {
				content := string(raw)
				if err := consume(RawUnit{UnitType: "field", Title: key, Content: content, Locator: map[string]any{"field": key}, Metadata: map[string]any{"format": "json"}}); err != nil {
					return err
				}
			}
			n++
		}
		_, err = dec.Token()
		return err
	}
	return fmt.Errorf("unsupported JSON root")
}

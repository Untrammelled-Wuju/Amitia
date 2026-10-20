package knowledge

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func openSafeZip(path string, runtime ...runtimeSettings) (*zip.ReadCloser, error) {
	settings := loadRuntimeSettings()
	if len(runtime) > 0 {
		settings = runtime[0]
	}
	maxFiles := settings.MaxOOXMLFiles
	if maxFiles <= 0 {
		maxFiles = defaultRuntimeSettings().MaxOOXMLFiles
	}
	maxExpanded := settings.MaxOOXMLUncompressedBytes
	if maxExpanded <= 0 {
		maxExpanded = defaultRuntimeSettings().MaxOOXMLUncompressedBytes
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	if len(zr.File) > maxFiles {
		zr.Close()
		return nil, fmt.Errorf("OOXML contains too many entries")
	}
	var total int64
	for _, f := range zr.File {
		if f.UncompressedSize64 > uint64(maxExpanded) {
			zr.Close()
			return nil, fmt.Errorf("OOXML entry too large: %s", f.Name)
		}
		total += int64(f.UncompressedSize64)
		if total > maxExpanded {
			zr.Close()
			return nil, fmt.Errorf("OOXML expanded size exceeds limit")
		}
	}
	return zr, nil
}
func zipFile(zr *zip.ReadCloser, name string) (*zip.File, bool) {
	name = filepath.ToSlash(name)
	for _, f := range zr.File {
		if filepath.ToSlash(f.Name) == name {
			return f, true
		}
	}
	return nil, false
}

type docxParser struct{ settings runtimeSettings }

func (p docxParser) Inspect(_ context.Context, src SourceRef) (DocumentInfo, error) {
	zr, err := openSafeZip(src.Path, p.settings)
	if err != nil {
		return DocumentInfo{}, err
	}
	defer zr.Close()
	if _, ok := zipFile(zr, "word/document.xml"); !ok {
		return DocumentInfo{}, fmt.Errorf("invalid docx: word/document.xml missing")
	}
	return DocumentInfo{Format: "docx", Metadata: map[string]any{"filename": src.Filename}}, nil
}
func (p docxParser) StreamUnits(ctx context.Context, src SourceRef, c Cursor, consume func(RawUnit) error) error {
	zr, err := openSafeZip(src.Path, p.settings)
	if err != nil {
		return err
	}
	defer zr.Close()
	zf, ok := zipFile(zr, "word/document.xml")
	if !ok {
		return fmt.Errorf("invalid docx")
	}
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	dec := xml.NewDecoder(rc)
	var ordinal int64
	var para strings.Builder
	inPara := false
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "p" {
				inPara = true
				para.Reset()
			}
			if inPara && t.Name.Local == "t" {
				var text string
				if err := dec.DecodeElement(&text, &t); err != nil {
					return err
				}
				para.WriteString(text)
			}
		case xml.EndElement:
			if t.Name.Local == "p" && inPara {
				content := cleanText(para.String())
				if content != "" {
					if ordinal >= c.Ordinal {
						if err := consume(RawUnit{UnitType: "paragraph", Title: "", Content: content, Locator: map[string]any{"paragraph": ordinal + 1}, Metadata: map[string]any{"format": "docx"}}); err != nil {
							return err
						}
					}
					ordinal++
				}
				inPara = false
			}
		}
	}
	return nil
}

type xlsxParser struct{ settings runtimeSettings }

func (p xlsxParser) Inspect(_ context.Context, src SourceRef) (DocumentInfo, error) {
	zr, err := openSafeZip(src.Path, p.settings)
	if err != nil {
		return DocumentInfo{}, err
	}
	defer zr.Close()
	sheets := 0
	for _, f := range zr.File {
		if strings.HasPrefix(filepath.ToSlash(f.Name), "xl/worksheets/sheet") && strings.HasSuffix(strings.ToLower(f.Name), ".xml") {
			sheets++
		}
	}
	if sheets == 0 {
		return DocumentInfo{}, fmt.Errorf("invalid xlsx: no worksheets")
	}
	return DocumentInfo{Format: "xlsx", EstimatedUnits: int64(sheets), Metadata: map[string]any{"sheets": sheets}}, nil
}

func readSharedStrings(zr *zip.ReadCloser, runtime ...runtimeSettings) ([]string, error) {
	settings := loadRuntimeSettings()
	if len(runtime) > 0 {
		settings = runtime[0]
	}
	maxStrings := settings.MaxSharedStrings
	if maxStrings <= 0 {
		maxStrings = defaultRuntimeSettings().MaxSharedStrings
	}
	maxStringBytes := settings.MaxSharedStringBytes
	if maxStringBytes <= 0 {
		maxStringBytes = defaultRuntimeSettings().MaxSharedStringBytes
	}
	maxCellBytes := settings.MaxCellTextBytes
	if maxCellBytes <= 0 {
		maxCellBytes = defaultRuntimeSettings().MaxCellTextBytes
	}
	zf, ok := zipFile(zr, "xl/sharedStrings.xml")
	if !ok {
		return nil, nil
	}
	rc, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	dec := xml.NewDecoder(rc)
	var out []string
	var cur strings.Builder
	totalBytes := 0
	inSI := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "si" {
				inSI = true
				cur.Reset()
			}
			if inSI && t.Name.Local == "t" {
				var s string
				if err := dec.DecodeElement(&s, &t); err != nil {
					return nil, err
				}
				if len(s) > maxCellBytes {
					return nil, fmt.Errorf("xlsx shared string exceeds %d bytes", maxCellBytes)
				}
				totalBytes += len(s)
				if int64(totalBytes) > maxStringBytes {
					return nil, fmt.Errorf("xlsx shared strings exceed memory budget")
				}
				cur.WriteString(s)
			}
		case xml.EndElement:
			if t.Name.Local == "si" && inSI {
				if len(out) >= maxStrings {
					return nil, fmt.Errorf("xlsx shared string count exceeds %d", maxStrings)
				}
				out = append(out, cur.String())
				inSI = false
			}
		}
	}
	return out, nil
}

type xlsxCell struct{ Ref, Type, Value, Inline string }

func cellColumn(ref string) string {
	for i, r := range ref {
		if r >= '0' && r <= '9' {
			return ref[:i]
		}
	}
	return ref
}
func sheetNumber(name string) int {
	base := filepath.Base(name)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	n, _ := strconv.Atoi(strings.TrimPrefix(base, "sheet"))
	return n
}
func (p xlsxParser) StreamUnits(ctx context.Context, src SourceRef, c Cursor, consume func(RawUnit) error) error {
	zr, err := openSafeZip(src.Path, p.settings)
	if err != nil {
		return err
	}
	defer zr.Close()
	shared, err := readSharedStrings(zr, p.settings)
	if err != nil {
		return err
	}
	maxCellBytes := p.settings.MaxCellTextBytes
	if maxCellBytes <= 0 {
		maxCellBytes = defaultRuntimeSettings().MaxCellTextBytes
	}
	maxRowCells := p.settings.MaxWorksheetRowCells
	if maxRowCells <= 0 {
		maxRowCells = defaultRuntimeSettings().MaxWorksheetRowCells
	}
	var sheets []*zip.File
	for _, f := range zr.File {
		n := filepath.ToSlash(f.Name)
		if strings.HasPrefix(n, "xl/worksheets/sheet") && strings.HasSuffix(strings.ToLower(n), ".xml") {
			sheets = append(sheets, f)
		}
	}
	sort.Slice(sheets, func(i, j int) bool { return sheetNumber(sheets[i].Name) < sheetNumber(sheets[j].Name) })
	var ordinal int64
	for _, sheet := range sheets {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		rc, err := sheet.Open()
		if err != nil {
			return err
		}
		dec := xml.NewDecoder(rc)
		rowIndex := 0
		var cells []xlsxCell
		var cur *xlsxCell
		var inline strings.Builder
		for {
			tok, er := dec.Token()
			if er == io.EOF {
				break
			}
			if er != nil {
				rc.Close()
				return er
			}
			switch t := tok.(type) {
			case xml.StartElement:
				switch t.Name.Local {
				case "row":
					rowIndex++
					cells = nil
				case "c":
					cc := xlsxCell{}
					for _, a := range t.Attr {
						if a.Name.Local == "r" {
							cc.Ref = a.Value
						}
						if a.Name.Local == "t" {
							cc.Type = a.Value
						}
					}
					cur = &cc
					inline.Reset()
				case "v":
					if cur != nil {
						var v string
						if er := dec.DecodeElement(&v, &t); er != nil {
							rc.Close()
							return er
						}
						if len(v) > maxCellBytes {
							rc.Close()
							return fmt.Errorf("xlsx cell value exceeds %d bytes", maxCellBytes)
						}
						cur.Value = v
					}
				case "t":
					if cur != nil && cur.Type == "inlineStr" {
						var v string
						if er := dec.DecodeElement(&v, &t); er != nil {
							rc.Close()
							return er
						}
						if len(v) > maxCellBytes {
							rc.Close()
							return fmt.Errorf("xlsx inline cell exceeds %d bytes", maxCellBytes)
						}
						inline.WriteString(v)
					}
				}
			case xml.EndElement:
				if t.Name.Local == "c" && cur != nil {
					if len(cells) >= maxRowCells {
						rc.Close()
						return fmt.Errorf("xlsx row contains more than %d cells", maxRowCells)
					}
					cur.Inline = inline.String()
					cells = append(cells, *cur)
					cur = nil
				}
				if t.Name.Local == "row" {
					record := map[string]string{}
					for _, cell := range cells {
						v := cell.Value
						if cell.Type == "s" {
							if idx, er := strconv.Atoi(v); er == nil && idx >= 0 && idx < len(shared) {
								v = shared[idx]
							}
						} else if cell.Type == "inlineStr" {
							v = cell.Inline
						}
						record[cellColumn(cell.Ref)] = v
					}
					if len(record) > 0 {
						if ordinal >= c.Ordinal {
							if er := consume(RawUnit{UnitType: "row", Title: fmt.Sprintf("Sheet %d Row %d", sheetNumber(sheet.Name), rowIndex), Content: jsonString(record), Locator: map[string]any{"sheet": sheetNumber(sheet.Name), "row": rowIndex}, Metadata: map[string]any{"format": "xlsx"}}); er != nil {
								rc.Close()
								return er
							}
						}
						ordinal++
					}
				}
			}
		}
		rc.Close()
	}
	return nil
}

package knowledge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// legacyDocParser converts binary .doc files to a temporary DOCX and then
// feeds that file through the canonical streaming DOCX parser. The original
// source remains immutable; conversion artifacts are ephemeral and never
// become canonical knowledge data.
type legacyDocParser struct{ settings runtimeSettings }

func officeCommand() string {
	for _, name := range []string{"soffice", "libreoffice"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func (legacyDocParser) Inspect(_ context.Context, src SourceRef) (DocumentInfo, error) {
	if officeCommand() == "" {
		return DocumentInfo{}, fmt.Errorf("LibreOffice/soffice is required for legacy DOC ingestion")
	}
	return DocumentInfo{Format: "doc", Metadata: map[string]any{
		"filename":   src.Filename,
		"conversion": "doc->docx",
	}}, nil
}

func (p legacyDocParser) StreamUnits(ctx context.Context, src SourceRef, c Cursor, consume func(RawUnit) error) error {
	cmdPath := officeCommand()
	if cmdPath == "" {
		return fmt.Errorf("LibreOffice/soffice is required for legacy DOC ingestion")
	}
	tmp, err := os.MkdirTemp("", "amitia-knowledge-doc-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	profileDir := filepath.Join(tmp, "profile")
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return err
	}
	outDir := filepath.Join(tmp, "out")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return err
	}

	timeout := p.settings.LegacyDocTimeout
	if timeout <= 0 {
		timeout = defaultRuntimeSettings().LegacyDocTimeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	profileURL := "file://" + filepath.ToSlash(profileDir)
	cmd := exec.CommandContext(cctx, cmdPath,
		"--headless", "--invisible", "--norestore", "--nodefault", "--nolockcheck",
		"-env:UserInstallation="+profileURL,
		"--convert-to", "docx", "--outdir", outDir, src.Path,
	)
	cmd.Env = append(os.Environ(), "HOME="+tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("DOC conversion failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	base := strings.TrimSuffix(filepath.Base(src.Filename), filepath.Ext(src.Filename))
	converted := filepath.Join(outDir, base+".docx")
	if _, err := os.Stat(converted); err != nil {
		matches, _ := filepath.Glob(filepath.Join(outDir, "*.docx"))
		if len(matches) != 1 {
			return fmt.Errorf("DOC conversion did not produce a unique DOCX output")
		}
		converted = matches[0]
	}

	ref := SourceRef{Path: converted, Filename: filepath.Base(converted), MimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"}
	return (docxParser{settings: p.settings}).StreamUnits(ctx, ref, c, func(unit RawUnit) error {
		if unit.Metadata == nil {
			unit.Metadata = map[string]any{}
		}
		unit.Metadata["sourceFormat"] = "doc"
		unit.Metadata["convertedFormat"] = "docx"
		return consume(unit)
	})
}

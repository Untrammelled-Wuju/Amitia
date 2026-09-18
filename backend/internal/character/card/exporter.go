package card

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Exporter struct {
	ResourceBaseDir string
}

func NewExporter(resourceBaseDir string) *Exporter {
	return &Exporter{ResourceBaseDir: resourceBaseDir}
}

type ExportInput struct {
	Name               string
	Description        string
	Personality        string
	Scenario           string
	AlternateGreetings []string
	ExampleMessages    string
	SystemPrompt       string
	PostHistory        string
	Creator            string
	CreatorNotes       string
	CharacterVersion   string
	Tags               []string
	Nickname           string
	GroupOnlyGreetings []string
	Source             string
	Extensions         map[string]any
	CharacterBook      *CharacterBook
	AvatarURL          string
	SourceFormat       string
	Preserved          map[string]json.RawMessage
}

func (e *Exporter) Export(input ExportInput, format string) (*CharacterCardExportResult, []byte, error) {
	card := e.buildCard(input)

	switch format {
	case "v3_charx":
		return e.exportV3CHARX(card, input)
	}

	return nil, nil, ErrUnsupportedFormat
}

func (e *Exporter) buildCard(input ExportInput) *CharacterCard {
	card := &CharacterCard{
		Name:                    input.Name,
		Description:             input.Description,
		Personality:             input.Personality,
		Scenario:                input.Scenario,
		AlternateGreetings:      input.AlternateGreetings,
		ExampleMessages:         input.ExampleMessages,
		SystemPrompt:            input.SystemPrompt,
		PostHistoryInstructions: input.PostHistory,
		Creator:                 input.Creator,
		CreatorNotes:            input.CreatorNotes,
		CharacterVersion:        input.CharacterVersion,
		Tags:                    input.Tags,
		Nickname:                input.Nickname,
		GroupOnlyGreetings:      input.GroupOnlyGreetings,
		Source:                  input.Source,
		Extensions:              input.Extensions,
		CharacterBook:           input.CharacterBook,
		Preserved:               input.Preserved,
	}

	if input.SourceFormat != "" {
		card.SourceFormat = CharacterCardFormat(input.SourceFormat)
	} else {
		card.SourceFormat = FormatV3CHARX
	}

	return card
}

func (e *Exporter) exportV3CHARX(card *CharacterCard, input ExportInput) (*CharacterCardExportResult, []byte, error) {
	cardJSON, err := buildV3JSON(card, card.Preserved)
	if err != nil {
		return nil, nil, ErrExportFailed
	}

	assets := make(map[string][]byte)
	if input.AvatarURL != "" {
		if data, mimeType, err := e.loadResource(input.AvatarURL); err == nil {
			ext := extensionFromMIME(mimeType)
			assets["assets/icon/images/main"+ext] = data
		}
	}

	charxData, err := buildCHARX(cardJSON, assets)
	if err != nil {
		return nil, nil, ErrExportFailed
	}

	filename := sanitizeFilename(card.Name) + ".charx"
	uri, err := e.saveToResource(charxData, filename, "character-cards")
	if err != nil {
		return nil, nil, err
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(charxData))

	return &CharacterCardExportResult{
		ResourceURI: uri,
		Format:      "v3_charx",
		Filename:    filename,
		SizeBytes:   int64(len(charxData)),
		ContentHash: hash,
	}, charxData, nil
}

func (e *Exporter) saveToResource(data []byte, filename string, subdir string) (string, error) {
	dir := filepath.Join(e.ResourceBaseDir, subdir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return "/resources/" + subdir + "/" + filename, nil
}

func (e *Exporter) loadResource(resourceURI string) ([]byte, string, error) {
	if strings.HasPrefix(resourceURI, "/") {
		resourceURI = resourceURI[1:]
	}
	path := filepath.Join(e.ResourceBaseDir, resourceURI)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	mimeType := detectMIMEType(resourceURI)
	return data, mimeType, nil
}

func detectMIMEType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	}
	return "application/octet-stream"
}

func extensionFromMIME(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	}
	return ".png"
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "character"
	}
	safe := strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r >= 0x4e00 {
			return r
		}
		return '_'
	}, name)
	if len([]rune(safe)) > 40 {
		safe = string([]rune(safe)[:40])
	}
	return safe
}

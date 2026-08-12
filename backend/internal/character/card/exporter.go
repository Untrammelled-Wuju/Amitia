package card

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
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
	Name                string
	Description         string
	Personality         string
	Scenario            string
	FirstMessage        string
	AlternateGreetings  []string
	ExampleMessages     string
	SystemPrompt        string
	PostHistory         string
	Creator             string
	CreatorNotes        string
	CharacterVersion    string
	Tags                []string
	Nickname            string
	GroupOnlyGreetings  []string
	Source              string
	Extensions          map[string]any
	CharacterBook       *CharacterBook
	AvatarURL           string
	SourceFormat        string
	Preserved           map[string]json.RawMessage
}

func (e *Exporter) Export(input ExportInput, format string) (*CharacterCardExportResult, []byte, error) {
	card := e.buildCard(input)

	switch format {
	case "v3_charx":
		return e.exportV3CHARX(card, input)
	case "v3_json":
		return e.exportV3JSON(card, input)
	case "v3_png":
		return e.exportV3PNG(card, input)
	case "v2_json":
		return e.exportV2JSON(card, input)
	case "v2_png":
		return e.exportV2PNG(card, input)
	}

	return nil, nil, ErrUnsupportedFormat
}

func (e *Exporter) buildCard(input ExportInput) *CharacterCard {
	card := &CharacterCard{
		Name:                    input.Name,
		Description:             input.Description,
		Personality:             input.Personality,
		Scenario:                input.Scenario,
		FirstMessage:            input.FirstMessage,
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

func (e *Exporter) exportV3JSON(card *CharacterCard, input ExportInput) (*CharacterCardExportResult, []byte, error) {
	cardJSON, err := buildV3JSON(card, card.Preserved)
	if err != nil {
		return nil, nil, ErrExportFailed
	}

	filename := sanitizeFilename(card.Name) + ".json"
	uri, err := e.saveToResource(cardJSON, filename, "character-cards")
	if err != nil {
		return nil, nil, err
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(cardJSON))

	return &CharacterCardExportResult{
		ResourceURI: uri,
		Format:      "v3_json",
		Filename:    filename,
		SizeBytes:   int64(len(cardJSON)),
		ContentHash: hash,
	}, cardJSON, nil
}

func (e *Exporter) exportV2JSON(card *CharacterCard, input ExportInput) (*CharacterCardExportResult, []byte, error) {
	cardJSON, err := buildV2JSON(card, card.Preserved)
	if err != nil {
		return nil, nil, ErrExportFailed
	}

	filename := sanitizeFilename(card.Name) + "_v2.json"
	uri, err := e.saveToResource(cardJSON, filename, "character-cards")
	if err != nil {
		return nil, nil, err
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(cardJSON))

	return &CharacterCardExportResult{
		ResourceURI: uri,
		Format:      "v2_json",
		Filename:    filename,
		SizeBytes:   int64(len(cardJSON)),
		ContentHash: hash,
	}, cardJSON, nil
}

func (e *Exporter) exportV3PNG(card *CharacterCard, input ExportInput) (*CharacterCardExportResult, []byte, error) {
	cardJSON, err := buildV3JSON(card, card.Preserved)
	if err != nil {
		return nil, nil, ErrExportFailed
	}

	var pngData []byte
	if input.AvatarURL != "" {
		pngData, _, _ = e.loadResource(input.AvatarURL)
	}

	if pngData == nil {
		pngData = generatePlaceholderPNG()
	}

	fullData := embedV3InPNG(pngData, cardJSON)

	filename := sanitizeFilename(card.Name) + ".png"
	uri, err := e.saveToResource(fullData, filename, "character-cards")
	if err != nil {
		return nil, nil, err
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(fullData))

	return &CharacterCardExportResult{
		ResourceURI: uri,
		Format:      "v3_png",
		Filename:    filename,
		SizeBytes:   int64(len(fullData)),
		ContentHash: hash,
	}, fullData, nil
}

func (e *Exporter) exportV2PNG(card *CharacterCard, input ExportInput) (*CharacterCardExportResult, []byte, error) {
	cardJSON, err := buildV2JSON(card, card.Preserved)
	if err != nil {
		return nil, nil, ErrExportFailed
	}

	var pngData []byte
	if input.AvatarURL != "" {
		pngData, _, _ = e.loadResource(input.AvatarURL)
	}

	if pngData == nil {
		pngData = generatePlaceholderPNG()
	}

	fullData := embedV2InPNG(pngData, cardJSON)

	filename := sanitizeFilename(card.Name) + "_v2.png"
	uri, err := e.saveToResource(fullData, filename, "character-cards")
	if err != nil {
		return nil, nil, err
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(fullData))

	return &CharacterCardExportResult{
		ResourceURI: uri,
		Format:      "v2_png",
		Filename:    filename,
		SizeBytes:   int64(len(fullData)),
		ContentHash: hash,
	}, fullData, nil
}

func (e *Exporter) saveToResource(data []byte, filename string, subdir string) (string, error) {
	dir := filepath.Join(e.ResourceBaseDir, subdir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
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

func generatePlaceholderPNG() []byte {
	png := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x01, 0xE2, 0x21, 0xBC,
		0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44,
		0xAE, 0x42, 0x60, 0x82,
	}
	return png
}

func embedV3InPNG(pngData []byte, cardJSON []byte) []byte {
	encoded := base64.StdEncoding.EncodeToString(cardJSON)
	return appendPNGTextChunk(pngData, string(encoded), "ccv3")
}

func embedV2InPNG(pngData []byte, cardJSON []byte) []byte {
	encoded := base64.StdEncoding.EncodeToString(cardJSON)
	return appendPNGTextChunk(pngData, string(encoded), "chara")
}

func appendPNGTextChunk(pngData []byte, value string, key string) []byte {
	if len(pngData) < 8 {
		return pngData
	}

	endIdx := bytes.Index(pngData, []byte("IEND"))
	if endIdx < 0 {
		return pngData
	}

	chunkType := []byte("tEXt")
	sep := []byte{0x00}
	keyBytes := []byte(key)
	valBytes := []byte(value)

	chunkData := make([]byte, 0, len(keyBytes)+1+len(valBytes))
	chunkData = append(chunkData, keyBytes...)
	chunkData = append(chunkData, sep...)
	chunkData = append(chunkData, valBytes...)

	length := make([]byte, 4)
	length[0] = byte(len(chunkData) >> 24)
	length[1] = byte(len(chunkData) >> 16)
	length[2] = byte(len(chunkData) >> 8)
	length[3] = byte(len(chunkData))

	crcData := make([]byte, 0, len(chunkType)+len(chunkData))
	crcData = append(crcData, chunkType...)
	crcData = append(crcData, chunkData...)
	crc := crc32Checksum(crcData)
	crcBytes := make([]byte, 4)
	crcBytes[0] = byte(crc >> 24)
	crcBytes[1] = byte(crc >> 16)
	crcBytes[2] = byte(crc >> 8)
	crcBytes[3] = byte(crc)

	fullChunk := make([]byte, 0, 4+4+len(chunkData)+4)
	fullChunk = append(fullChunk, length...)
	fullChunk = append(fullChunk, chunkType...)
	fullChunk = append(fullChunk, chunkData...)
	fullChunk = append(fullChunk, crcBytes...)

	result := make([]byte, 0, len(pngData)+len(fullChunk))
	result = append(result, pngData...)
	result = append(result, fullChunk...)

	return result
}

func crc32Checksum(data []byte) uint32 {
	var crc uint32 = 0xFFFFFFFF
	table := getCRC32Table()
	for _, b := range data {
		crc = table[(crc^uint32(b))&0xFF] ^ (crc >> 8)
	}
	return crc ^ 0xFFFFFFFF
}

func getCRC32Table() [256]uint32 {
	var table [256]uint32
	for i := 0; i < 256; i++ {
		crc := uint32(i)
		for j := 0; j < 8; j++ {
			if crc&1 == 1 {
				crc = 0xEDB88320 ^ (crc >> 1)
			} else {
				crc >>= 1
			}
		}
		table[i] = crc
	}
	return table
}


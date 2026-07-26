package agent_skill

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	frontmatterPattern = regexp.MustCompile(`^---\s*\n([\s\S]*?)\n---`)
	maxFrontmatterSize  = 64 * 1024
	maxInstructionSize  = 128 * 1024
	maxYAMLDocumentSize = 64 * 1024
)

type ParsedSkill struct {
	FrontmatterRaw  string
	InstructionsRaw string
	Fields          map[string]any
}

type SkillParser struct{}

func NewSkillParser() *SkillParser {
	return &SkillParser{}
}

func (p *SkillParser) Parse(raw []byte) (*ParsedSkill, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("SKILL.md is empty")
	}
	if len(raw) > maxFrontmatterSize+maxInstructionSize {
		return nil, fmt.Errorf("SKILL.md exceeds maximum size")
	}

	content := string(raw)
	loc := frontmatterPattern.FindStringSubmatchIndex(content)
	if loc == nil {
		return &ParsedSkill{
			InstructionsRaw: strings.TrimSpace(content),
			Fields:          map[string]any{},
		}, nil
	}

	frontmatterRaw := content[loc[2]:loc[3]]
	if len(frontmatterRaw) > maxYAMLDocumentSize {
		return nil, fmt.Errorf("frontmatter exceeds maximum size")
	}

	fields, err := parseYAMLFields(frontmatterRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid frontmatter: %w", err)
	}

	instructionsRaw := strings.TrimSpace(content[loc[1]:])
	if len(instructionsRaw) > maxInstructionSize {
		instructionsRaw = instructionsRaw[:maxInstructionSize]
	}

	return &ParsedSkill{
		FrontmatterRaw:  frontmatterRaw,
		InstructionsRaw: instructionsRaw,
		Fields:          fields,
	}, nil
}

func (p *SkillParser) ParseStaging(root string) (*ParsedSkill, string, error) {
	return nil, "", fmt.Errorf("staging access requires filesystem abstraction")
}

func parseYAMLFields(raw string) (map[string]any, error) {
	var result map[string]any
	decoder := yaml.NewDecoder(strings.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	if result == nil {
		result = map[string]any{}
	}
	return result, nil
}

func ComputeFrontmatterHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func ComputeInstructionHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func EstimateTokens(text string) int {
	words := strings.Fields(text)
	return len(words) * 4 / 3
}

func SanitizeInstructionText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	if len(text) > maxInstructionSize {
		text = text[:maxInstructionSize]
	}
	return text
}

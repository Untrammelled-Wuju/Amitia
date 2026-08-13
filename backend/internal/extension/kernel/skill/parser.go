package skill

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

type Parser struct {
	now func() time.Time
}

func NewParser() *Parser {
	return &Parser{now: func() time.Time { return time.Now().UTC() }}
}

func (p *Parser) Parse(ctx context.Context, root SkillRoot, policy ParsePolicy, content io.Reader) (ParsedSkill, error) {
	result, err := p.parseInternal(ctx, root, policy, content)
	return result, err
}

func (p *Parser) ParseBytes(ctx context.Context, root SkillRoot, policy ParsePolicy, data []byte) (ParsedSkill, error) {
	return p.Parse(ctx, root, policy, bytes.NewReader(data))
}

func (p *Parser) parseInternal(ctx context.Context, root SkillRoot, policy ParsePolicy, content io.Reader) (ParsedSkill, error) {
	directory := filepath.Base(root.RootURI)

	lex := &frontmatterLexer{r: content}
	lexResult, err := lex.lex(policy)
	if err != nil {
		return ParsedSkill{}, err
	}

	if lexResult == nil {
		return ParsedSkill{
			Diagnostics: []SkillDiagnostic{
				{Severity: DiagnosticSeverityError, Code: "SKILL_FRONTMATTER_MISSING", Message: "missing frontmatter opening delimiter '---'"},
			},
		}, fmt.Errorf("SKILL_FRONTMATTER_MISSING")
	}

	if lexResult.closed == false {
		return ParsedSkill{
			Diagnostics: []SkillDiagnostic{
				{Severity: DiagnosticSeverityError, Code: "SKILL_FRONTMATTER_UNCLOSED", Message: "frontmatter closing delimiter '---' not found"},
			},
		}, fmt.Errorf("SKILL_FRONTMATTER_UNCLOSED")
	}

	fmBytes := lexResult.frontmatter
	body := lexResult.body

	if int64(len(fmBytes)) > policy.MaxFrontmatterBytes {
		return ParsedSkill{
			Diagnostics: []SkillDiagnostic{
				{Severity: DiagnosticSeverityError, Code: "SKILL_FRONTMATTER_TOO_LARGE", Message: fmt.Sprintf("frontmatter exceeded %d bytes", policy.MaxFrontmatterBytes)},
			},
		}, fmt.Errorf("SKILL_FRONTMATTER_TOO_LARGE")
	}

	if len(bytes.TrimSpace(fmBytes)) == 0 {
		return ParsedSkill{
			Diagnostics: []SkillDiagnostic{
				{Severity: DiagnosticSeverityError, Code: "SKILL_FRONTMATTER_EMPTY", Message: "frontmatter is empty"},
			},
		}, fmt.Errorf("SKILL_FRONTMATTER_EMPTY")
	}

	h := sha256.New()
	h.Write(fmBytes)
	h.Write(body)
	contentHash := "sha256:" + hex.EncodeToString(h.Sum(nil))

	pc := &parseContext{
		policy:     policy,
		diags:      []SkillDiagnostic{},
		directory:  directory,
		startTime:  p.now(),
	}

	var yamlRoot yaml.Node
	if err := yaml.Unmarshal(fmBytes, &yamlRoot); err != nil {
		return ParsedSkill{Diagnostics: pc.diags}, fmt.Errorf("SKILL_YAML_INVALID")
	}

	rawMap, err := safeParseYAML(pc, fmBytes, policy)
	if err != nil {
		return ParsedSkill{Diagnostics: pc.diags}, err
	}

	var rootNode *yaml.Node
	if len(yamlRoot.Content) > 0 {
		rootNode = yamlRoot.Content[0]
	}

	parsed := ParsedSkill{
		RawFrontmatter: rawMap,
		ExtraFrontmatter: make(map[string]interface{}),
		Diagnostics:     pc.diags,
		ContentHash:     contentHash,
		Source: SkillSourceDescriptor{
			RootURI: root.RootURI,
			Source:  root.Source,
		},
		Body: string(body),
	}

	iface, ok := rawMap["name"]
	if !ok {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_NAME_REQUIRED",
			Message:  "name is required",
			Field:    "name",
		})
		parsed.Diagnostics = pc.diags
		return parsed, fmt.Errorf("SKILL_NAME_REQUIRED")
	}

	name, ok := iface.(string)
	if !ok || strings.TrimSpace(name) == "" {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_NAME_REQUIRED",
			Message:  "name must be a non-empty string",
			Field:    "name",
		})
		parsed.Diagnostics = pc.diags
		return parsed, fmt.Errorf("SKILL_NAME_REQUIRED")
	}

	if err := validateName(pc, name, directory); err != nil {
		parsed.Diagnostics = pc.diags
		return parsed, err
	}
	parsed.Name = name

	descIface, ok := rawMap["description"]
	if !ok {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_DESCRIPTION_REQUIRED",
			Message:  "description is required",
			Field:    "description",
		})
		parsed.Diagnostics = pc.diags
		return parsed, fmt.Errorf("SKILL_DESCRIPTION_REQUIRED")
	}

	desc, ok := descIface.(string)
	if !ok || strings.TrimSpace(desc) == "" {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_DESCRIPTION_REQUIRED",
			Message:  "description must be a non-empty string",
			Field:    "description",
		})
		parsed.Diagnostics = pc.diags
		return parsed, fmt.Errorf("SKILL_DESCRIPTION_REQUIRED")
	}

	parsed.Description = desc

	if err := validateDescription(pc, desc); err != nil {
		parsed.Diagnostics = pc.diags
		return parsed, err
	}

	if v, ok := rawMap["license"]; ok {
		lic, ok := v.(string)
		if !ok {
			pc.diags = append(pc.diags, SkillDiagnostic{
				Severity: DiagnosticSeverityError,
				Code:     "SKILL_LICENSE_INVALID",
				Message:  "license must be a string",
				Field:    "license",
			})
			parsed.Diagnostics = pc.diags
			return parsed, fmt.Errorf("SKILL_LICENSE_INVALID")
		}
		if len(lic) > 512 {
			pc.diags = append(pc.diags, SkillDiagnostic{
				Severity: DiagnosticSeverityError,
				Code:     "SKILL_LICENSE_TOO_LONG",
				Message:  "license must be at most 512 characters",
				Field:    "license",
			})
			parsed.Diagnostics = pc.diags
			return parsed, fmt.Errorf("SKILL_LICENSE_TOO_LONG")
		}
		parsed.License = lic
	}

	if v, ok := rawMap["compatibility"]; ok {
		compat, ok := v.(string)
		if !ok {
			pc.diags = append(pc.diags, SkillDiagnostic{
				Severity: DiagnosticSeverityError,
				Code:     "SKILL_COMPATIBILITY_INVALID",
				Message:  "compatibility must be a string",
				Field:    "compatibility",
			})
			parsed.Diagnostics = pc.diags
			return parsed, fmt.Errorf("SKILL_COMPATIBILITY_INVALID")
		}
		if len(compat) > 500 {
			pc.diags = append(pc.diags, SkillDiagnostic{
				Severity: DiagnosticSeverityError,
				Code:     "SKILL_COMPATIBILITY_TOO_LONG",
				Message:  "compatibility must be at most 500 characters",
				Field:    "compatibility",
			})
			parsed.Diagnostics = pc.diags
			return parsed, fmt.Errorf("SKILL_COMPATIBILITY_TOO_LONG")
		}
		parsed.Compatibility = compat
	}

	if v, ok := rawMap["metadata"]; ok {
		var metaNode *yaml.Node
		if rootNode != nil {
			for i := 0; i < len(rootNode.Content); i += 2 {
				if rootNode.Content[i].Value == "metadata" {
					metaNode = rootNode.Content[i+1]
					break
				}
			}
		}
		if metaNode == nil {
			pc.diags = append(pc.diags, SkillDiagnostic{
				Severity: DiagnosticSeverityError,
				Code:     "SKILL_METADATA_INVALID",
				Message:  "metadata must be a map<string, string>",
				Field:    "metadata",
			})
			parsed.Diagnostics = pc.diags
			return parsed, fmt.Errorf("SKILL_METADATA_INVALID")
		}
		md, err := parseMetadataFromNode(pc, metaNode, policy)
		if err != nil {
			_ = v
			parsed.Diagnostics = pc.diags
			return parsed, err
		}
		parsed.Metadata = md
	}

	if v, ok := rawMap["allowed-tools"]; ok {
		tools, err := parseAllowedTools(pc, v, policy)
		if err != nil {
			parsed.Diagnostics = pc.diags
			return parsed, err
		}
		parsed.AllowedTools = tools
	}

	knownKeys := map[string]bool{
		"name": true, "description": true, "license": true,
		"compatibility": true, "metadata": true, "allowed-tools": true,
	}

	extraKeys := make([]string, 0)
	for k := range rawMap {
		if !knownKeys[k] {
			extraKeys = append(extraKeys, k)
		}
	}
	sort.Strings(extraKeys)

	if len(extraKeys) > policy.MaxExtraFields {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_EXTRA_FRONTMATTER_TOO_LARGE",
			Message:  fmt.Sprintf("too many extra frontmatter fields: %d", len(extraKeys)),
		})
		parsed.Diagnostics = pc.diags
		return parsed, fmt.Errorf("SKILL_EXTRA_FRONTMATTER_TOO_LARGE")
	}

	for _, k := range extraKeys {
		val := rawMap[k]
		if !isJSONSafeScalarOrSimple(val, 0, 8) {
			pc.diags = append(pc.diags, SkillDiagnostic{
				Severity: DiagnosticSeverityError,
				Code:     "SKILL_EXTRA_FRONTMATTER_INVALID",
				Message:  fmt.Sprintf("extra field '%s' contains unsupported YAML construct", k),
				Field:    k,
			})
			parsed.Diagnostics = pc.diags
			return parsed, fmt.Errorf("SKILL_EXTRA_FRONTMATTER_INVALID")
		}
		parsed.ExtraFrontmatter[k] = val
	}

	if len(strings.TrimSpace(parsed.Body)) == 0 {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityWarning,
			Code:     "SKILL_BODY_EMPTY",
			Message:  "body is empty",
		})
	}

	lines := strings.Split(parsed.Body, "\n")
	if len(lines) > 500 {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityWarning,
			Code:     "SKILL_BODY_LARGE",
			Message:  fmt.Sprintf("body has %d lines, recommended <= 500", len(lines)),
		})
	}

	if utf8.RuneCountInString(parsed.Body) > 5000 {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityWarning,
			Code:     "SKILL_BODY_TOO_LARGE",
			Message:  fmt.Sprintf("body is approximately %d characters, consider progressive loading", utf8.RuneCountInString(parsed.Body)),
		})
	}

	parsed.Diagnostics = pc.diags
	return parsed, nil
}

func (p *Parser) Preview(ctx context.Context, root SkillRoot, policy ParsePolicy, content io.Reader) (SkillParsePreview, error) {
	parsed, err := p.parseInternal(ctx, root, policy, content)
	if err != nil && len(parsed.Diagnostics) == 0 {
		return SkillParsePreview{}, err
	}

	preview := SkillParsePreview{
		Name:                parsed.Name,
		Description:         parsed.Description,
		License:             parsed.License,
		Compatibility:       parsed.Compatibility,
		Metadata:            parsed.Metadata,
		AllowedTools:        parsed.AllowedTools,
		BodyBytes:           len(parsed.Body),
		BodyLines:           len(strings.Split(parsed.Body, "\n")),
		Diagnostics:         parsed.Diagnostics,
		ContentHash:         parsed.ContentHash,
		CompatibilityStatus: CompatibilityStatus(parsed.Diagnostics),
	}

	return preview, nil
}

func CompatibilityStatus(diags []SkillDiagnostic) string {
	hasError := false
	hasWarning := false
	for _, d := range diags {
		switch d.Severity {
		case DiagnosticSeverityError:
			hasError = true
		case DiagnosticSeverityWarning:
			hasWarning = true
		}
	}
	if hasError {
		return SkillCompatStatusBlocked
	}
	if hasWarning {
		return SkillCompatStatusDegraded
	}
	return SkillCompatStatusCompatible
}

var nameRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func validateName(pc *parseContext, name string, directory string) error {
	if len(name) < 1 || len(name) > 64 {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_NAME_TOO_LONG",
			Message:  "name must be 1-64 characters",
			Field:    "name",
		})
		return fmt.Errorf("SKILL_NAME_TOO_LONG")
	}
	if !nameRegex.MatchString(name) {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_NAME_INVALID",
			Message:  "name must match ^[a-z0-9]+(?:-[a-z0-9]+)*$",
			Field:    "name",
		})
		return fmt.Errorf("SKILL_NAME_INVALID")
	}
	if name != directory {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_NAME_DIRECTORY_MISMATCH",
			Message:  fmt.Sprintf("name '%s' must match parent directory '%s'", name, directory),
			Field:    "name",
		})
		return fmt.Errorf("SKILL_NAME_DIRECTORY_MISMATCH")
	}
	return nil
}

func validateDescription(pc *parseContext, desc string) error {
	if len(strings.TrimSpace(desc)) == 0 {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_DESCRIPTION_REQUIRED",
			Message:  "description must be non-empty",
			Field:    "description",
		})
		return fmt.Errorf("SKILL_DESCRIPTION_REQUIRED")
	}
	if utf8.RuneCountInString(desc) > 1024 {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_DESCRIPTION_TOO_LONG",
			Message:  "description must be at most 1024 characters",
			Field:    "description",
		})
		return fmt.Errorf("SKILL_DESCRIPTION_TOO_LONG")
	}
	return nil
}

func parseMetadataFromNode(pc *parseContext, node *yaml.Node, policy ParsePolicy) (map[string]string, error) {
	if node.Kind != yaml.MappingNode {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_METADATA_INVALID",
			Message:  "metadata must be a map<string, string>",
			Field:    "metadata",
		})
		return nil, fmt.Errorf("SKILL_METADATA_INVALID")
	}

	entries := len(node.Content) / 2
	if entries > policy.MaxMetadataEntries {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_METADATA_TOO_LARGE",
			Message:  fmt.Sprintf("metadata has %d entries, max %d", entries, policy.MaxMetadataEntries),
			Field:    "metadata",
		})
		return nil, fmt.Errorf("SKILL_METADATA_TOO_LARGE")
	}

	result := make(map[string]string)
	for i := 0; i < len(node.Content); i += 2 {
		k := node.Content[i]
		v := node.Content[i+1]

		if k.Kind != yaml.ScalarNode || k.Value == "" {
			pc.diags = append(pc.diags, SkillDiagnostic{
				Severity: DiagnosticSeverityError,
				Code:     "SKILL_METADATA_KEY_INVALID",
				Message:  "metadata key must be a non-empty string",
				Field:    "metadata",
				Line:     k.Line,
			})
			return nil, fmt.Errorf("SKILL_METADATA_KEY_INVALID")
		}
		if len(k.Value) > 128 {
			pc.diags = append(pc.diags, SkillDiagnostic{
				Severity: DiagnosticSeverityError,
				Code:     "SKILL_METADATA_KEY_INVALID",
				Message:  fmt.Sprintf("metadata key '%s' exceeds 128 chars", k.Value),
				Field:    "metadata",
			})
			return nil, fmt.Errorf("SKILL_METADATA_KEY_INVALID")
		}
		if v.Kind != yaml.ScalarNode {
			pc.diags = append(pc.diags, SkillDiagnostic{
				Severity: DiagnosticSeverityError,
				Code:     "SKILL_METADATA_VALUE_INVALID",
				Message:  fmt.Sprintf("metadata value for '%s' must be a string scalar", k.Value),
				Field:    "metadata",
				Line:     v.Line,
			})
			return nil, fmt.Errorf("SKILL_METADATA_VALUE_INVALID")
		}
		if v.Tag != "" && v.Tag != "!!str" {
			pc.diags = append(pc.diags, SkillDiagnostic{
				Severity: DiagnosticSeverityError,
				Code:     "SKILL_METADATA_VALUE_INVALID",
				Message:  fmt.Sprintf("metadata value for '%s' must be a string, got %s", k.Value, v.Tag),
				Field:    "metadata",
				Line:     v.Line,
			})
			return nil, fmt.Errorf("SKILL_METADATA_VALUE_INVALID")
		}
		if len(v.Value) > 4096 {
			pc.diags = append(pc.diags, SkillDiagnostic{
				Severity: DiagnosticSeverityError,
				Code:     "SKILL_METADATA_VALUE_INVALID",
				Message:  fmt.Sprintf("metadata value for '%s' exceeds 4096 chars", k.Value),
				Field:    "metadata",
			})
			return nil, fmt.Errorf("SKILL_METADATA_VALUE_INVALID")
		}
		result[k.Value] = v.Value
	}
	return result, nil
}

func parseAllowedTools(pc *parseContext, v interface{}, policy ParsePolicy) ([]string, error) {
	s, ok := v.(string)
	if !ok {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_ALLOWED_TOOLS_INVALID",
			Message:  "allowed-tools must be a space separated string",
			Field:    "allowed-tools",
		})
		return nil, fmt.Errorf("SKILL_ALLOWED_TOOLS_INVALID")
	}

	tokens := strings.Fields(s)
	if len(tokens) > 128 {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_ALLOWED_TOOLS_TOO_LARGE",
			Message:  fmt.Sprintf("allowed-tools has %d tokens, max 128", len(tokens)),
			Field:    "allowed-tools",
		})
		return nil, fmt.Errorf("SKILL_ALLOWED_TOOLS_TOO_LARGE")
	}

	for i, t := range tokens {
		if len(t) > 256 {
			pc.diags = append(pc.diags, SkillDiagnostic{
				Severity: DiagnosticSeverityError,
				Code:     "SKILL_ALLOWED_TOOLS_INVALID",
				Message:  fmt.Sprintf("allowed-tools token %d exceeds 256 chars", i),
				Field:    "allowed-tools",
			})
			return nil, fmt.Errorf("SKILL_ALLOWED_TOOLS_INVALID")
		}
	}

	result := make([]string, 0, len(tokens))
	seen := make(map[string]bool)
	for _, t := range tokens {
		if seen[t] {
			continue
		}
		seen[t] = true
		result = append(result, t)
	}

	if len(result) == 0 {
		return nil, nil
	}
	sort.Strings(result)
	return result, nil
}

func isNilYAML(v interface{}) bool {
	if v == nil {
		return true
	}
	if node, ok := v.(*yaml.Node); ok {
		return node.Kind == 0 || (node.Kind == yaml.ScalarNode && node.Tag == "!!null")
	}
	return false
}

func yamlScalarString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func isJSONSafeScalarOrSimple(v interface{}, depth int, maxDepth int) bool {
	if depth > maxDepth {
		return false
	}
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string, int, int64, uint, uint64, float32, float64, bool:
		return true
	case []interface{}:
		for _, item := range val {
			if !isJSONSafeScalarOrSimple(item, depth+1, maxDepth) {
				return false
			}
		}
		return true
	case map[string]interface{}:
		for _, item := range val {
			if !isJSONSafeScalarOrSimple(item, depth+1, maxDepth) {
				return false
			}
		}
		return true
	case map[interface{}]interface{}:
		for _, item := range val {
			if !isJSONSafeScalarOrSimple(item, depth+1, maxDepth) {
				return false
			}
		}
		return true
	}
	return false
}

type frontmatterLexer struct {
	r       io.Reader
	scanned bytes.Buffer
	bomOK   bool
	state   lexState
	data    []byte
}

type lexState int

const (
	lexStatePre lexState = iota
	lexStateFrontmatter
	lexStateBody
)

type lexResult struct {
	frontmatter []byte
	body        []byte
	closed      bool
}

func (l *frontmatterLexer) lex(policy ParsePolicy) (*lexResult, error) {
	data, err := io.ReadAll(io.LimitReader(l.r, policy.MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > policy.MaxFileBytes {
		return nil, fmt.Errorf("SKILL_FILE_TOO_LARGE")
	}

	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("SKILL_FILE_EMPTY")
	}

	if !utf8.Valid(data) {
		return nil, fmt.Errorf("SKILL_ENCODING_UNSUPPORTED")
	}

	l.data = data
	return l.parse()
}

func (l *frontmatterLexer) parse() (*lexResult, error) {
	d := l.data

	if !bytes.HasPrefix(d, []byte("---")) {
		return nil, nil
	}

	if len(d) >= 6 && d[0] == 0xEF && d[1] == 0xBB && d[2] == 0xBF {
		d = d[3:]
	}

	if !bytes.Equal(d[:3], []byte("---")) {
		return nil, nil
	}

	if len(d) == 3 {
		return &lexResult{frontmatter: []byte{}, body: []byte{}, closed: false}, nil
	}

	rest := d[3:]

	if len(rest) > 0 && (rest[0] == '\n' || rest[0] == '\r') {
	} else if len(rest) > 0 && (rest[0] == ' ' || rest[0] == '\t') {
		return nil, nil
	} else {
		return nil, nil
	}

	idx := bytes.Index(rest, []byte("\n---"))
	if idx < 0 {
		if bytes.HasSuffix(rest, []byte("\n---")) || bytes.Equal(rest, []byte("---")) {
			fmEnd := len(rest)
			if bytes.Equal(rest, []byte("---")) {
				fmEnd = 0
			} else {
				fmEnd = len(rest) - 1
			}
			return &lexResult{
				frontmatter: bytes.TrimSpace(rest[:fmEnd]),
				body:        []byte{},
				closed:      true,
			}, nil
		}
		return &lexResult{frontmatter: bytes.TrimSpace(rest), body: []byte{}, closed: false}, nil
	}

	fmAndRest := rest[:idx+1]
	bodyAndRest := rest[idx+1:]

	fm := bytes.TrimSpace(fmAndRest)

	if len(bodyAndRest) > 0 {
		bodyAndRest = bodyAndRest[3:]
		bodyAndRest = bytes.TrimLeft(bodyAndRest, " \t")
		if len(bodyAndRest) > 0 && bodyAndRest[0] == '\r' {
			bodyAndRest = bodyAndRest[1:]
		}
		if len(bodyAndRest) > 0 && bodyAndRest[0] == '\n' {
			bodyAndRest = bodyAndRest[1:]
		}
	}

	return &lexResult{
		frontmatter: fm,
		body:        bytes.TrimRight(bodyAndRest, "\n\r"),
		closed:      true,
	}, nil
}

func (l *frontmatterLexer) Write(p []byte) (n int, err error) {
	return l.scanned.Write(p)
}

func safeParseYAML(pc *parseContext, data []byte, policy ParsePolicy) (map[string]interface{}, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_YAML_INVALID",
			Message:  fmt.Sprintf("YAML parse error: %v", err),
		})
		return nil, fmt.Errorf("SKILL_YAML_INVALID")
	}

	if len(node.Content) == 0 {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_YAML_INVALID",
			Message:  "YAML root is empty",
		})
		return nil, fmt.Errorf("SKILL_YAML_INVALID")
	}

	root := node.Content[0]

	if err := checkYAMLNode(pc, root, 0, policy); err != nil {
		return nil, err
	}

	var raw map[string]interface{}
	if err := root.Decode(&raw); err != nil {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_YAML_INVALID",
			Message:  fmt.Sprintf("YAML decode error: %v", err),
		})
		return nil, fmt.Errorf("SKILL_YAML_INVALID")
	}

	if root.Kind != yaml.MappingNode {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_YAML_INVALID",
			Message:  "YAML root must be a mapping",
		})
		return nil, fmt.Errorf("SKILL_YAML_INVALID")
	}

	seen := make(map[string]bool)
	for i := 0; i < len(root.Content); i += 2 {
		k := root.Content[i]
		if seen[k.Value] {
			pc.diags = append(pc.diags, SkillDiagnostic{
				Severity: DiagnosticSeverityError,
				Code:     "SKILL_YAML_DUPLICATE_KEY",
				Message:  fmt.Sprintf("duplicate YAML key: %s", k.Value),
				Field:    k.Value,
				Line:     k.Line,
				Column:   k.Column,
			})
			return nil, fmt.Errorf("SKILL_YAML_DUPLICATE_KEY")
		}
		seen[k.Value] = true
	}

	return raw, nil
}

func checkYAMLNode(pc *parseContext, node *yaml.Node, depth int, policy ParsePolicy) error {
	pc.nodeCount++
	if depth > pc.maxDepth {
		pc.maxDepth = depth
	}
	if depth > policy.MaxYAMLDepth {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_YAML_TOO_DEEP",
			Message:  fmt.Sprintf("YAML depth %d exceeds max %d", depth, policy.MaxYAMLDepth),
			Line:     node.Line,
		})
		return fmt.Errorf("SKILL_YAML_TOO_DEEP")
	}
	if pc.nodeCount > policy.MaxYAMLNodes {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_YAML_TOO_COMPLEX",
			Message:  fmt.Sprintf("YAML node count %d exceeds max %d", pc.nodeCount, policy.MaxYAMLNodes),
		})
		return fmt.Errorf("SKILL_YAML_TOO_COMPLEX")
	}

	if node.Anchor != "" {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_YAML_ALIAS_UNSUPPORTED",
			Message:  "YAML anchors are not allowed",
			Line:     node.Line,
		})
		return fmt.Errorf("SKILL_YAML_ALIAS_UNSUPPORTED")
	}

	if node.Kind == yaml.AliasNode {
		pc.diags = append(pc.diags, SkillDiagnostic{
			Severity: DiagnosticSeverityError,
			Code:     "SKILL_YAML_ALIAS_UNSUPPORTED",
			Message:  "YAML aliases are not allowed",
			Line:     node.Line,
		})
		return fmt.Errorf("SKILL_YAML_ALIAS_UNSUPPORTED")
	}

	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Kind != yaml.ScalarNode {
				pc.diags = append(pc.diags, SkillDiagnostic{
					Severity: DiagnosticSeverityError,
					Code:     "SKILL_YAML_INVALID",
					Message:  "metadata keys must be strings",
					Line:     node.Content[i].Line,
				})
				return fmt.Errorf("SKILL_YAML_INVALID")
			}
			if err := checkYAMLNode(pc, node.Content[i], depth+1, policy); err != nil {
				return err
			}
			if err := checkYAMLNode(pc, node.Content[i+1], depth+1, policy); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if err := checkYAMLNode(pc, item, depth+1, policy); err != nil {
				return err
			}
		}
	}
	return nil
}

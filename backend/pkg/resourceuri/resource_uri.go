// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package resourceuri

import (
	"fmt"
	"net/url"
	"path"
	"strings"
	"unicode"
)

type ResourceURI struct {
	root         ResourceRoot
	relativePath string
}

func Parse(raw string) (ResourceURI, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ResourceURI{}, fmt.Errorf("empty uri: %w", ErrInvalidPath)
	}

	schemePrefix := "amitia://"
	lower := strings.ToLower(raw)
	if !strings.HasPrefix(lower, schemePrefix) {
		return ResourceURI{}, fmt.Errorf("%q: %w", raw, ErrInvalidScheme)
	}

	rest := raw[amitiaSchemeLen:]
	if strings.Contains(rest, "#") {
		return ResourceURI{}, fmt.Errorf("%q: %w: fragment not allowed", raw, ErrInvalidPath)
	}
	if idx := strings.Index(rest, "?"); idx >= 0 {
		return ResourceURI{}, fmt.Errorf("%q: %w: query not allowed", raw, ErrInvalidPath)
	}

	u, err := url.Parse("amitia://" + strings.ToLower(rest))
	if err != nil {
		return ResourceURI{}, fmt.Errorf("%q: %w: %v", raw, ErrInvalidPath, err)
	}
	if u.Scheme != "amitia" {
		return ResourceURI{}, fmt.Errorf("%q: %w", raw, ErrInvalidScheme)
	}
	if u.User != nil {
		return ResourceURI{}, fmt.Errorf("%q: %w: userinfo not allowed", raw, ErrInvalidPath)
	}
	if u.Port() != "" {
		return ResourceURI{}, fmt.Errorf("%q: %w: port not allowed", raw, ErrInvalidPath)
	}

	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return ResourceURI{}, fmt.Errorf("%q: %w: empty host", raw, ErrInvalidRoot)
	}
	root, err := parseResourceRoot(host)
	if err != nil {
		return ResourceURI{}, fmt.Errorf("%q: %w", raw, err)
	}

	relPath, err := normalizeAndValidatePath(u.Path)
	if err != nil {
		return ResourceURI{}, fmt.Errorf("%q: %w", raw, err)
	}

	return ResourceURI{
		root:         root,
		relativePath: relPath,
	}, nil
}

const amitiaSchemeLen = 9

func normalizeAndValidatePath(rawPath string) (string, error) {
	if rawPath == "" || rawPath == "/" {
		return "", nil
	}
	if strings.Contains(rawPath, "\\") {
		return "", fmt.Errorf("%q: %w: backslash not allowed", rawPath, ErrPathTraversal)
	}
	if strings.ContainsRune(rawPath, 0x00) {
		return "", fmt.Errorf("%q: %w: nul character", rawPath, ErrInvalidPath)
	}
	for _, r := range rawPath {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("%q: %w: control character", rawPath, ErrInvalidPath)
		}
	}

	decoded, err := url.PathUnescape(rawPath)
	if err != nil {
		return "", fmt.Errorf("%q: %w: %v", rawPath, ErrInvalidPath, err)
	}
	if strings.Contains(decoded, "\\") {
		return "", fmt.Errorf("%q: %w: backslash not allowed", decoded, ErrPathTraversal)
	}
	if strings.ContainsRune(decoded, 0x00) {
		return "", fmt.Errorf("%q: %w: nul character", decoded, ErrInvalidPath)
	}

	segments := strings.Split(decoded, "/")
	for _, seg := range segments {
		if seg == ".." {
			return "", fmt.Errorf("%q: %w", decoded, ErrPathTraversal)
		}
	}

	cleaned := path.Clean(decoded)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string('/')) {
		return "", fmt.Errorf("%q: %w", decoded, ErrPathTraversal)
	}

	filtered := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg == "" || seg == "." {
			continue
		}
		filtered = append(filtered, seg)
	}

	return strings.Join(filtered, "/"), nil
}

func MustParse(raw string) ResourceURI {
	uri, err := Parse(raw)
	if err != nil {
		panic(err)
	}
	return uri
}

func (u ResourceURI) Root() ResourceRoot {
	return u.root
}

func (u ResourceURI) RelativePath() string {
	return u.relativePath
}

func (u ResourceURI) IsRoot() bool {
	return u.relativePath == ""
}

func (u ResourceURI) String() string {
	var b strings.Builder
	b.WriteString("amitia://")
	b.WriteString(string(u.root))
	b.WriteByte('/')
	if u.relativePath != "" {
		parts := strings.Split(u.relativePath, "/")
		for i, p := range parts {
			if i > 0 {
				b.WriteByte('/')
			}
			b.WriteString(url.PathEscape(p))
		}
	}
	return b.String()
}

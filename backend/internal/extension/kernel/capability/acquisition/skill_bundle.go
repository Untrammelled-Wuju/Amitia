package acquisition

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/timeoutpolicy"
)

type SkillBundle struct {
	Files   map[string][]byte
	Root    string
	Archive []byte
}

func LoadSkillBundle(ctx context.Context, sourceURI, skillName, expectedHash string) (SkillBundle, error) {
	parsed, err := url.Parse(sourceURI)
	if err != nil {
		return SkillBundle{}, err
	}
	local := sourceURI
	if parsed.Scheme == "file" {
		local = filepath.FromSlash(parsed.Path)
		if parsed.Host != "" {
			return SkillBundle{}, fmt.Errorf("remote file shares are not skill sources")
		}
		if len(local) > 2 && local[0] == '\\' && local[2] == ':' {
			local = local[1:]
		}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		info, err := os.Lstat(local)
		if err != nil {
			return SkillBundle{}, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return SkillBundle{}, fmt.Errorf("skill source cannot be a symbolic link")
		}
		if info.IsDir() || strings.EqualFold(filepath.Base(local), "SKILL.md") {
			if !info.IsDir() {
				local = filepath.Dir(local)
			}
			files := map[string][]byte{}
			var total int64
			err := filepath.WalkDir(local, func(name string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				if entry.Type()&os.ModeSymlink != 0 {
					return fmt.Errorf("skill directory contains symbolic link")
				}
				if entry.IsDir() {
					return nil
				}
				info, err := entry.Info()
				if err != nil {
					return err
				}
				if !info.Mode().IsRegular() {
					return fmt.Errorf("skill directory contains non-regular file")
				}
				total += info.Size()
				if len(files) >= 512 || total > 50<<20 {
					return fmt.Errorf("skill directory exceeds import limits")
				}
				rel, err := filepath.Rel(local, name)
				if err != nil {
					return err
				}
				raw, err := os.ReadFile(name)
				if err != nil {
					return err
				}
				files[filepath.ToSlash(rel)] = raw
				return nil
			})
			if err != nil {
				return SkillBundle{}, err
			}
			if expectedHash != "" {
				raw, exists := files["SKILL.md"]
				if !exists {
					return SkillBundle{}, fmt.Errorf("SKILL.md is missing")
				}
				if err := verifySkillHash(raw, expectedHash); err != nil {
					return SkillBundle{}, err
				}
			}
			return SkillBundle{Files: files, Root: filepath.Base(local)}, nil
		}
		if info.Size() > 50<<20 {
			return SkillBundle{}, fmt.Errorf("skill source exceeds import limit")
		}
		raw, err := os.ReadFile(local)
		if err != nil {
			return SkillBundle{}, err
		}
		if err := verifySkillHash(raw, expectedHash); err != nil {
			return SkillBundle{}, err
		}
		return SkillBundle{Archive: raw}, nil
	}
	if parsed.User != nil {
		return SkillBundle{}, fmt.Errorf("skill URL cannot contain credentials")
	}
	downloadURI := sourceURI
	subdir := ""
	if strings.EqualFold(parsed.Hostname(), "github.com") {
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) < 5 || (parts[2] != "tree" && parts[2] != "blob") {
			return SkillBundle{}, fmt.Errorf("use a GitHub tree or blob URL pointing to the skill directory")
		}
		subdir = strings.Join(parts[4:], "/")
		if parts[2] == "blob" {
			if path.Base(subdir) != "SKILL.md" {
				return SkillBundle{}, fmt.Errorf("GitHub skill file must be SKILL.md")
			}
			subdir = path.Dir(subdir)
		}
		downloadURI = "https://codeload.github.com/" + parts[0] + "/" + parts[1] + "/zip/" + url.PathEscape(parts[3])
		if skillName == "" {
			skillName = path.Base(subdir)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURI, nil)
	if err != nil {
		return SkillBundle{}, err
	}
	resp, err := timeoutpolicy.Client(&http.Client{Timeout: 45 * time.Second}).Do(req)
	if err != nil {
		return SkillBundle{}, fmt.Errorf("skill source download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return SkillBundle{}, fmt.Errorf("skill source returned HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (50<<20)+1))
	if err != nil {
		return SkillBundle{}, err
	}
	if len(raw) > 50<<20 {
		return SkillBundle{}, fmt.Errorf("skill source exceeds import limit")
	}
	if err := verifySkillHash(raw, expectedHash); err != nil {
		return SkillBundle{}, err
	}
	if subdir != "" {
		reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			return SkillBundle{}, err
		}
		files := map[string][]byte{}
		var total uint64
		for _, file := range reader.File {
			_, relative, found := strings.Cut(file.Name, "/")
			if !found {
				continue
			}
			prefix := strings.Trim(subdir, "/") + "/"
			if subdir == "." {
				prefix = ""
			}
			if !strings.HasPrefix(relative, prefix) || file.FileInfo().IsDir() {
				continue
			}
			name := strings.TrimPrefix(relative, prefix)
			if path.Clean(name) != name || strings.HasPrefix(name, "../") || strings.Contains(name, "\\") || file.Mode()&os.ModeSymlink != 0 {
				return SkillBundle{}, fmt.Errorf("unsafe skill archive path")
			}
			if _, exists := files[name]; exists {
				return SkillBundle{}, fmt.Errorf("duplicate skill archive path")
			}
			total += file.UncompressedSize64
			if len(files) >= 512 || total > 50<<20 {
				return SkillBundle{}, fmt.Errorf("expanded skill exceeds import limits")
			}
			stream, err := file.Open()
			if err != nil {
				return SkillBundle{}, err
			}
			content, readErr := io.ReadAll(io.LimitReader(stream, (50<<20)+1))
			stream.Close()
			if readErr != nil {
				return SkillBundle{}, readErr
			}
			if len(content) > 50<<20 {
				return SkillBundle{}, fmt.Errorf("skill resource exceeds import limit")
			}
			files[name] = content
		}
		if _, exists := files["SKILL.md"]; !exists {
			return SkillBundle{}, fmt.Errorf("selected GitHub directory has no SKILL.md")
		}
		return SkillBundle{Files: files, Root: skillName}, nil
	}
	if bytes.HasPrefix(raw, []byte("PK\x03\x04")) {
		return SkillBundle{Archive: raw}, nil
	}
	return SkillBundle{Files: map[string][]byte{"SKILL.md": raw}, Root: skillName}, nil
}

func verifySkillHash(raw []byte, expected string) error {
	if expected == "" {
		return nil
	}
	digest := sha256.Sum256(raw)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), strings.TrimPrefix(expected, "sha256:")) {
		return fmt.Errorf("skill source SHA-256 mismatch")
	}
	return nil
}

package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	MaxPathDepth     = 64
	MaxPathSegment   = 255
	MaxDirectRead    = 1 << 20
	MaxSingleWrite   = 64 << 20
	MaxListEntries   = 500
	MaxRecursive     = 10000
	MaxRecursiveBytes = 1 << 30
)

var reservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

func validateSegment(seg string) error {
	if seg == "" {
		return fmt.Errorf("%w: empty path segment", ErrInvalidPath)
	}
	if len(seg) > MaxPathSegment {
		return fmt.Errorf("%w: path segment too long", ErrInvalidPath)
	}
	if strings.HasSuffix(seg, " ") || strings.HasSuffix(seg, ".") {
		return fmt.Errorf("%w: trailing space or dot not allowed", ErrInvalidPath)
	}
	for _, r := range seg {
		if r == 0x00 {
			return fmt.Errorf("%w: nul character", ErrInvalidPath)
		}
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: control character", ErrInvalidPath)
		}
	}
	lower := strings.ToLower(seg)
	if reservedNames[lower] {
		return fmt.Errorf("%w: reserved name %q", ErrInvalidPath, seg)
	}
	return nil
}

func ValidateSegment(seg string) error {
	return validateSegment(seg)
}

func AssertWithinRoot(rootPath, childPath string) error {
	return assertWithinRoot(rootPath, childPath)
}

func ValidateRelativePath(rel string) error {
	if rel == "" {
		return nil
	}
	if strings.Contains(rel, "\\") {
		return fmt.Errorf("%w: backslash not allowed", ErrPathTraversal)
	}
	segments := strings.Split(rel, "/")
	depth := 0
	for _, seg := range segments {
		if seg == "" || seg == "." {
			continue
		}
		if seg == ".." {
			return fmt.Errorf("%w: traversal not allowed", ErrPathTraversal)
		}
		if err := validateSegment(seg); err != nil {
			return err
		}
		depth++
	}
	if depth > MaxPathDepth {
		return fmt.Errorf("%w: depth %d exceeds %d", ErrDepthExceeded, depth, MaxPathDepth)
	}
	return nil
}

func ResolveAndValidateRead(rootPath, rel string) (string, error) {
	if err := ValidateRelativePath(rel); err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return "", fmt.Errorf("%w: cannot resolve root: %v", ErrInvalidPath, err)
	}
	target := absRoot
	if rel != "" {
		target = filepath.Join(absRoot, rel)
	}
	cleaned := filepath.Clean(target)
	if err := assertWithinRoot(absRoot, cleaned); err != nil {
		return "", err
	}
	if err := assertNoSymlinks(absRoot, cleaned); err != nil {
		return "", err
	}
	return cleaned, nil
}

func ResolveAndValidateCreate(rootPath, rel string) (string, error) {
	if err := ValidateRelativePath(rel); err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return "", fmt.Errorf("%w: cannot resolve root: %v", ErrInvalidPath, err)
	}
	if rel == "" {
		return "", fmt.Errorf("%w: cannot create root", ErrInvalidPath)
	}
	target := filepath.Join(absRoot, rel)
	cleaned := filepath.Clean(target)
	if err := assertWithinRoot(absRoot, cleaned); err != nil {
		return "", err
	}
	parentDir := filepath.Dir(cleaned)
	if err := assertNoSymlinks(absRoot, parentDir); err != nil {
		return "", err
	}
	if _, err := os.Lstat(cleaned); err == nil {
		info, statErr := os.Lstat(cleaned)
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%w: target is symlink", ErrSymlinkNotAllowed)
		}
	}
	return cleaned, nil
}

func ResolveAndValidateMutation(rootPath, rel string) (string, error) {
	if err := ValidateRelativePath(rel); err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return "", fmt.Errorf("%w: cannot resolve root: %v", ErrInvalidPath, err)
	}
	if rel == "" {
		return "", fmt.Errorf("%w: cannot mutate root", ErrRootMutationDenied)
	}
	target := filepath.Join(absRoot, rel)
	cleaned := filepath.Clean(target)
	if err := assertWithinRoot(absRoot, cleaned); err != nil {
		return "", err
	}
	if err := assertNoSymlinks(absRoot, cleaned); err != nil {
		return "", err
	}
	return cleaned, nil
}

func assertWithinRoot(rootPath, childPath string) error {
	rel, err := filepath.Rel(rootPath, childPath)
	if err != nil {
		return fmt.Errorf("%w: cannot compute relative path", ErrPathTraversal)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%w: path outside root", ErrPathTraversal)
	}
	return nil
}

func assertNoSymlinks(rootPath, targetPath string) error {
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return fmt.Errorf("%w: cannot resolve root: %v", ErrInvalidPath, err)
	}
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("%w: cannot resolve target: %v", ErrInvalidPath, err)
	}
	if absTarget == absRoot {
		return nil
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return fmt.Errorf("%w: cannot compute relative path", ErrPathTraversal)
	}
	segments := strings.Split(rel, string(filepath.Separator))
	current := absRoot
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		current = filepath.Join(current, seg)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("%w: cannot stat %q: %v", ErrInvalidPath, current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: %q is symlink", ErrSymlinkNotAllowed, current)
		}
	}
	return nil
}

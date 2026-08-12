package git

import (
	"path/filepath"
	"regexp"
	"strings"
)

const (
	DefaultMaxStatusEntries    = 500
	DefaultMaxDiffBytes        = 1 << 20
	DefaultMaxTrackedFileBytes = 100 << 20
	DefaultMaxCommitMessageSize = 64 << 10
	DefaultMaxLogLimit         = 200
	DefaultMaxPushDepth        = 1000

	StagingIndex      = "M"
	StagingAdded      = "A"
	StagingDeleted    = "D"
	StagingRenamed    = "R"
	StagingCopied     = "C"
	StagingUnmerged   = "U"
	StagingUntracked  = "?"
	StagingIgnored    = "!"
)

var refNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._\-/]*$`)
var objectHashPattern = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
var shortHashPattern = regexp.MustCompile(`^[0-9a-fA-F]{4,39}$`)

var secretFilePatterns = []string{
	".env", ".env.*",
	"*.pem", "*.key", "*.p12", "*.pfx",
	"id_rsa", "id_ed25519", "id_ecdsa", "id_dsa",
	"credentials.*", "secrets.*", "token.*",
	"*.keystore", "*.jks",
}

var allowedSchemes = map[string]bool{
	"https": true,
	"ssh":   true,
	"file":  true,
}

var blockedSchemes = map[string]bool{
	"git":   true,
	"ftp":   true,
	"ftps":  true,
	"ext":   true,
}

type GitPolicy struct {
	MaxStatusEntries     int
	MaxDiffBytes         int
	MaxTrackedFileBytes  int
	MaxCommitMessageSize int
	MaxLogLimit          int
	SymlinkPolicy        string
	AllowHooks           bool
	AllowExternalFilter  bool
	AllowSubmoduleInit   bool
	AllowLFSPull         bool
	AllowSparseCheckout  bool
	ForcePushAllowed     bool
}

var DefaultGitPolicy = GitPolicy{
	MaxStatusEntries:     DefaultMaxStatusEntries,
	MaxDiffBytes:         DefaultMaxDiffBytes,
	MaxTrackedFileBytes:  DefaultMaxTrackedFileBytes,
	MaxCommitMessageSize: DefaultMaxCommitMessageSize,
	MaxLogLimit:          DefaultMaxLogLimit,
	SymlinkPolicy:        "reject_repository_with_symlink",
	AllowHooks:           false,
	AllowExternalFilter:  false,
	AllowSubmoduleInit:   false,
	AllowLFSPull:         false,
	AllowSparseCheckout:  false,
	ForcePushAllowed:     false,
}

func ValidateRefName(ref string) error {
	if ref == "" {
		return ErrGitRefInvalid
	}
	if ref == "HEAD" {
		return nil
	}
	if strings.HasPrefix(ref, "refs/heads/") {
		name := strings.TrimPrefix(ref, "refs/heads/")
		if !refNamePattern.MatchString(name) {
			return ErrGitRefInvalid
		}
		return nil
	}
	if strings.HasPrefix(ref, "refs/tags/") {
		name := strings.TrimPrefix(ref, "refs/tags/")
		if !refNamePattern.MatchString(name) {
			return ErrGitRefInvalid
		}
		return nil
	}
	if strings.HasPrefix(ref, "refs/remotes/") {
		name := strings.TrimPrefix(ref, "refs/remotes/")
		if !refNamePattern.MatchString(name) {
			return ErrGitRefInvalid
		}
		return nil
	}
	if objectHashPattern.MatchString(ref) {
		return nil
	}
	if shortHashPattern.MatchString(ref) {
		return nil
	}
	if refNamePattern.MatchString(ref) {
		return nil
	}
	return ErrGitRefInvalid
}

func ValidateRemoteURL(url string) error {
	if url == "" {
		return ErrGitRemoteInvalid
	}
	lower := strings.ToLower(url)
	for scheme := range blockedSchemes {
		if lower == scheme+"::" || strings.HasPrefix(lower, scheme+"::") ||
			lower == scheme+"://" || strings.HasPrefix(lower, scheme+"://") {
			return ErrGitRemoteInvalid
		}
	}
	if strings.Contains(url, "://") {
		parts := strings.SplitN(url, "://", 2)
		scheme := strings.ToLower(parts[0])
		if !allowedSchemes[scheme] {
			return ErrGitRemoteInvalid
		}
	}
	return nil
}

func SanitizeRemoteURL(url string) string {
	if url == "" {
		return url
	}
	if idx := strings.Index(url, "://"); idx >= 0 {
		scheme := url[:idx]
		rest := url[idx+3:]
		if atIdx := strings.LastIndex(rest, "@"); atIdx >= 0 {
			userinfo := rest[:atIdx]
			if strings.Contains(userinfo, ":") {
				host := rest[atIdx+1:]
				return scheme + "://" + "***@" + host
			}
		}
	}
	if !strings.Contains(url, "://") && strings.Contains(url, ":") {
		parts := strings.SplitN(url, ":", 2)
		if len(parts) == 2 && !strings.Contains(parts[0], "@") && !strings.Contains(parts[0], "/") {
			host := parts[0]
			if atIdx := strings.LastIndex(host, "@"); atIdx >= 0 {
				return "***:" + parts[1]
			}
		}
	}
	return url
}

func IsSecretFile(name string) bool {
	base := filepath.Base(name)
	for _, pattern := range secretFilePatterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}

func ValidateCommitMessage(msg string) error {
	if msg == "" {
		return ErrGitCommitFailed
	}
	if len(msg) > DefaultMaxCommitMessageSize {
		return ErrGitCommitFailed
	}
	return nil
}

func ValidatePaths(paths []string) error {
	for _, p := range paths {
		if p == "" {
			return ErrGitPathInvalid
		}
		if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "..") {
			return ErrGitPathOutsideRepository
		}
		if strings.Contains(p, "..") {
			return ErrGitPathInvalid
		}
	}
	return nil
}

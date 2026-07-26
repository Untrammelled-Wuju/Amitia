package trusted_service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type PlatformSelector struct {
	current Platform
}

func NewPlatformSelector() *PlatformSelector {
	return &PlatformSelector{current: CurrentPlatform()}
}

func (s *PlatformSelector) Select(def *ServiceRuntimeDefinition) (*PlatformExecutable, error) {
	if def == nil {
		return nil, errors.New("trusted_service: nil definition")
	}
	for i := range def.Executables {
		exe := &def.Executables[i]
		if exe.Platform == s.current {
			return exe, nil
		}
	}
	fallbackPlatform := normalizePlatform(s.current)
	for i := range def.Executables {
		exe := &def.Executables[i]
		if normalizePlatform(exe.Platform) == fallbackPlatform {
			return exe, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrPlatformNotSupported, s.current)
}

func normalizePlatform(p Platform) string {
	parts := strings.SplitN(string(p), "/", 2)
	if len(parts) != 2 {
		return string(p)
	}
	os := parts[0]
	arch := parts[1]
	if arch == "amd64" {
		arch = "amd64"
	}
	return fmt.Sprintf("%s/%s", os, arch)
}

type BinaryVerifier struct {
	allowMissingFile bool
}

func NewBinaryVerifier() *BinaryVerifier {
	return &BinaryVerifier{}
}

func (v *BinaryVerifier) Verify(ctx context.Context, exe *PlatformExecutable, basePath string) error {
	if exe == nil {
		return errors.New("trusted_service: nil executable")
	}
	fullPath := exe.Path
	if !filepath.IsAbs(fullPath) && basePath != "" {
		fullPath = filepath.Join(basePath, fullPath)
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("trusted_service: executable not found: %s", fullPath)
		}
		return fmt.Errorf("trusted_service: stat executable: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("trusted_service: executable is directory: %s", fullPath)
	}
	if !v.checkExecutablePerm(info) {
		return fmt.Errorf("trusted_service: executable not runnable: %s", fullPath)
	}
	if exe.Sha256 == "" {
		return errors.New("trusted_service: missing sha256")
	}
	hash, err := v.hashFile(fullPath)
	if err != nil {
		return fmt.Errorf("trusted_service: hash file: %w", err)
	}
	if hash != exe.Sha256 {
		return fmt.Errorf("%w: expected %s got %s", ErrBinaryHashMismatch, exe.Sha256, hash)
	}
	if !exe.Signature.Trusted {
		return fmt.Errorf("%w: %s", ErrInvalidSignature, "signature not trusted")
	}
	if exe.Signature.Value == "" {
		return fmt.Errorf("%w: missing signature value", ErrInvalidSignature)
	}
	return nil
}

func (v *BinaryVerifier) hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (v *BinaryVerifier) checkExecutablePerm(info os.FileInfo) bool {
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

type ArgsBuilder struct {
	template []string
}

func NewArgsBuilder(template []string) *ArgsBuilder {
	return &ArgsBuilder{template: template}
}

func (b *ArgsBuilder) Build(params map[string]string) ([]string, error) {
	out := make([]string, 0, len(b.template))
	for _, t := range b.template {
		replaced, err := b.substitute(t, params)
		if err != nil {
			return nil, fmt.Errorf("trusted_service: arg template %q: %w", t, err)
		}
		out = append(out, replaced)
	}
	return out, nil
}

func (b *ArgsBuilder) Substitute(tmpl string, params map[string]string) (string, error) {
	return b.substitute(tmpl, params)
}

func (b *ArgsBuilder) substitute(tmpl string, params map[string]string) (string, error) {
	var sb strings.Builder
	i := 0
	for i < len(tmpl) {
		if i+1 < len(tmpl) && tmpl[i] == '$' && tmpl[i+1] == '{' {
			end := strings.IndexByte(tmpl[i+2:], '}')
			if end < 0 {
				return "", errors.New("trusted_service: unterminated ${")
			}
			key := tmpl[i+2 : i+2+end]
			val, ok := params[key]
			if !ok {
				return "", fmt.Errorf("trusted_service: missing param %s", key)
			}
			sb.WriteString(val)
			i = i + 2 + end + 1
			continue
		}
		sb.WriteByte(tmpl[i])
		i++
	}
	return sb.String(), nil
}

type EnvBuilder struct {
	allowed map[string]bool
}

func NewEnvBuilder() *EnvBuilder {
	return &EnvBuilder{
		allowed: map[string]bool{
			"AMITIA_SESSION":     true,
			"AMITIA_INSTANCE":    true,
			"AMITIA_GENERATION":  true,
			"AMITIA_TEMP_DIR":    true,
			"AMITIA_LOG_LEVEL":   true,
			"AMITIA_SECRET_LEASE": true,
			"AMITIA_HOST_API":    true,
			"AMITIA_PROTOCOL":    true,
			"AMITIA_PLATFORM":    true,
		},
	}
}

func (b *EnvBuilder) Build(exe *PlatformExecutable, session, instance string, generation int64, tempDir, logLevel, secretLease string) []string {
	env := []string{
		"AMITIA_SESSION=" + session,
		"AMITIA_INSTANCE=" + instance,
		fmt.Sprintf("AMITIA_GENERATION=%d", generation),
		"AMITIA_TEMP_DIR=" + tempDir,
		"AMITIA_LOG_LEVEL=" + logLevel,
		"AMITIA_SECRET_LEASE=" + secretLease,
		"AMITIA_HOST_API=internal-rpc",
		"AMITIA_PROTOCOL=jsonrpc",
		"AMITIA_PLATFORM=" + string(CurrentPlatform()),
	}
	for k, v := range exe.EnvTemplate {
		if !b.allowed[k] {
			continue
		}
		env = append(env, k+"="+v)
	}
	return env
}

func (b *EnvBuilder) IsAllowed(key string) bool {
	return b.allowed[key]
}

func ValidateTrust(def *ServiceRuntimeDefinition, publisherTrust TrustLevel) error {
	if publisherTrust == TrustLevelUnknown || publisherTrust == TrustLevelCommunity {
		return fmt.Errorf("%w: %s", ErrUnknownPublisher, publisherTrust)
	}
	if !publisherTrust.AllowedForService() {
		return fmt.Errorf("%w: trust level %s", ErrTrustLevelInsufficient, publisherTrust)
	}
	if def.TrustLevel != "" {
		defTrust := TrustLevel(def.TrustLevel)
		if !defTrust.AllowedForService() {
			return fmt.Errorf("%w: definition trust level %s", ErrTrustLevelInsufficient, defTrust)
		}
	}
	if !def.Network.LoopbackOnly && def.Network.AllowInbound {
		return errors.New("trusted_service: inbound network must be loopback only by default")
	}
	if len(def.Executables) == 0 {
		return errors.New("trusted_service: no platform executables declared")
	}
	for i := range def.Executables {
		exe := &def.Executables[i]
		if exe.Path == "" {
			return fmt.Errorf("trusted_service: executable %d missing path", i)
		}
		if exe.Sha256 == "" {
			return fmt.Errorf("trusted_service: executable %d missing sha256", i)
		}
		if !exe.Signature.Trusted {
			return fmt.Errorf("trusted_service: executable %d signature not trusted", i)
		}
	}
	return nil
}

func ValidateNoShell(args []string) error {
	if len(args) == 0 {
		return nil
	}
	first := strings.ToLower(filepath.Base(args[0]))
	if first == "cmd.exe" || first == "powershell.exe" || first == "pwsh" || first == "bash" || first == "sh" || first == "zsh" || first == "fish" {
		return fmt.Errorf("%w: %s", ErrShellDisallowed, first)
	}
	for _, a := range args[1:] {
		if strings.Contains(a, "\n") || strings.Contains(a, "\r") {
			return fmt.Errorf("%w: arg contains newline", ErrShellDisallowed)
		}
	}
	return nil
}

package native_companion

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const ContractVersion = "1"

// Descriptor is the public runtime contract exposed to trusted service
// extensions through AMITIA_NATIVE_COMPANIONS. It is intentionally generic
// and contains no plugin-specific fields.
type Descriptor struct {
	ID           string   `json:"id"`
	Platform     string   `json:"platform"`
	Architecture string   `json:"architecture,omitempty"`
	Path         string   `json:"path"`
	SHA256       string   `json:"sha256"`
	Executable   bool     `json:"executable"`
	Args         []string `json:"args,omitempty"`
}

// Resolve verifies all matching native companions and returns canonical
// descriptors for the requested platform/architecture. Non-matching assets are
// deliberately omitted from the runtime contract.
func Resolve(moduleRoot string, companions []domain.NativeCompanionDefinition, platform, architecture string) ([]Descriptor, error) {
	moduleRoot = filepath.Clean(strings.TrimSpace(moduleRoot))
	if moduleRoot == "" || moduleRoot == "." {
		return nil, fmt.Errorf("module root is required")
	}
	platform = strings.TrimSpace(platform)
	architecture = strings.TrimSpace(architecture)

	out := make([]Descriptor, 0, len(companions))
	for _, companion := range companions {
		if strings.TrimSpace(companion.Platform) != platform {
			continue
		}
		if companion.Architecture != "" && strings.TrimSpace(companion.Architecture) != architecture {
			continue
		}

		rel := filepath.Clean(strings.TrimSpace(companion.Path))
		full := filepath.Join(moduleRoot, rel)
		within, err := filepath.Rel(moduleRoot, full)
		if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("companion %s escapes module root", companion.ID)
		}
		actual, err := fileSHA256(full)
		if err != nil {
			return nil, fmt.Errorf("hash companion %s: %w", companion.ID, err)
		}
		if !strings.EqualFold(actual, companion.SHA256) {
			return nil, fmt.Errorf("companion %s hash mismatch", companion.ID)
		}

		canonical, err := filepath.Abs(full)
		if err != nil {
			return nil, fmt.Errorf("resolve companion %s path: %w", companion.ID, err)
		}
		out = append(out, Descriptor{
			ID:           strings.TrimSpace(companion.ID),
			Platform:     strings.TrimSpace(companion.Platform),
			Architecture: strings.TrimSpace(companion.Architecture),
			Path:         filepath.Clean(canonical),
			SHA256:       strings.ToLower(actual),
			Executable:   companion.Executable,
			Args:         append([]string(nil), companion.Args...),
		})
	}
	return out, nil
}

func Encode(descriptors []Descriptor) (string, error) {
	encoded, err := json.Marshal(descriptors)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func ResolveAndEncode(moduleRoot string, companions []domain.NativeCompanionDefinition, platform, architecture string) (string, error) {
	descriptors, err := Resolve(moduleRoot, companions, platform, architecture)
	if err != nil {
		return "", err
	}
	return Encode(descriptors)
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

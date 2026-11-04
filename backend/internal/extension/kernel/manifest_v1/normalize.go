package manifest_v1

import (
	"regexp"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

var semVerPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-[0-9A-Za-z-.]+)?(?:\+[0-9A-Za-z-.]+)?$`)

var knownPlatforms = map[string]bool{
	"darwin":  true,
	"windows": true,
	"linux":   true,
	"android": true,
	"ios":     true,
}

var knownArchitectures = map[string]bool{
	"amd64": true,
	"arm64": true,
	"arm":   true,
	"386":   true,
}

var capabilityIDPattern = regexp.MustCompile(`^[a-z0-9/._-]{1,256}$`)
var providerIDPattern = regexp.MustCompile(`^[a-z0-9/._-]{1,256}$`)
var labelKeyPattern = regexp.MustCompile(`^[a-z0-9._-]{1,64}$`)
var requiredFeaturePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`)

const maxMetadataBytes = 1 << 20
const maxProvidedCapabilities = 256
const maxProviderLabels = 64

func normalizePlatform(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func normalizeArchitecture(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func effectiveContributionAdapter(kind, adapter string) domain.ContributionAdapter {
	return domain.ResolveContributionAdapter(
		domain.ContributionKind(kind),
		domain.ContributionAdapter(strings.TrimSpace(adapter)),
	)
}

func isExecutableModule(mod ModuleMeta) bool {
	return IsExecutableModuleType(mod.Type)
}

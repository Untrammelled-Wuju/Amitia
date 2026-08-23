package kernel

import "github.com/u-ai/backend/internal/extension/kernel/domain"

// bindAuthoritativePublisherTrust applies a trust decision produced by the
// package verification/trust pipeline to the persisted extension definition
// and every UI provider contribution derived from it. Manifest-authored trust
// values are deliberately ignored by manifest_v2.ToExtensionDefinition.
func bindAuthoritativePublisherTrust(def *domain.ExtensionDefinition, trustLevel string) {
	if def == nil {
		return
	}
	if trustLevel == "" {
		trustLevel = "unknown"
	}
	def.Publisher.TrustLevel = trustLevel
	for moduleIdx := range def.Modules {
		module := &def.Modules[moduleIdx]
		for contributionIdx := range module.Contributions {
			contribution := &module.Contributions[contributionIdx]
			if contribution.Kind != domain.ContributionKindUIProvider {
				continue
			}
			if contribution.Definition == nil {
				contribution.Definition = map[string]any{}
			}
			contribution.Definition["trustLevel"] = trustLevel
		}
	}
}

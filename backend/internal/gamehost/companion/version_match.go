package artifact

import gameprotocol "github.com/u-ai/backend/pkg/gameplugin/protocol"

func versionMatches(constraints []string, version string) bool {
	return gameprotocol.CompatibilityVersionMatches(constraints, version)
}

func validateCompatibilityConstraint(constraint string) error {
	return gameprotocol.ValidateCompatibilityConstraint(constraint)
}

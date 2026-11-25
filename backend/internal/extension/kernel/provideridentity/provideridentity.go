package provideridentity

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const extensionProviderPrefix = "extprov_"

func CatalogProviderKey(pluginID, moduleID, serviceID string) string {
	return strings.TrimSpace(pluginID) + "/" + strings.TrimSpace(moduleID) + "/" + serviceID
}

func ExtensionProviderID(extensionID, moduleID, providerKey, capabilityID string) string {
	parts := []string{extensionID, moduleID}
	if providerKey != "" {
		parts = append(parts, providerKey)
	}
	parts = append(parts, capabilityID)
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return extensionProviderPrefix + hex.EncodeToString(sum[:])
}
